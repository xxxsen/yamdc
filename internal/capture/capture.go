package capture

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/nfo"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/processor"

	"github.com/samber/lo"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/common/replacer"
	"github.com/xxxsen/common/trace"
	"go.uber.org/zap"
)

var (
	errInvalidDir         = errors.New("invalid dir")
	errNoSearcherFound    = errors.New("no searcher found")
	errNoStorageFound     = errors.New("no storage found")
	errInvalidNaming      = errors.New("invalid naming")
	errSearchItemNotFound = errors.New("search item not found")
	errNoTitle            = errors.New("no title")
	errNoNumberFound      = errors.New("no number found")
	errInvalidCover       = errors.New("invalid cover")
	errInvalidPoster      = errors.New("invalid poster")
)

const (
	defaultImageExtName   = ".jpg"
	defaultExtraFanartDir = "extrafanart"
)

var defaultMediaSuffix = []string{
	".mp4", ".wmv", ".flv", ".mpeg", ".m2ts", ".mts", ".mpe", ".mpg", ".m4v", ".avi",
	".mkv", ".rmvb", ".ts", ".mov", ".rm", ".strm",
}

type fcProcessFunc func(ctx context.Context, fc *model.FileContext) error

type Capture struct {
	c      *config
	extMap map[string]struct{}
}

func New(opts ...Option) (*Capture, error) {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if len(c.SaveDir) == 0 || len(c.ScanDir) == 0 {
		return nil, errInvalidDir
	}
	if c.Searcher == nil {
		return nil, errNoSearcherFound
	}
	if c.Processor == nil {
		c.Processor = processor.DefaultProcessor
	}
	if c.Storage == nil {
		return nil, errNoStorageFound
	}
	if c.MovieIDCleaner == nil {
		c.MovieIDCleaner = movieidcleaner.NewPassthroughCleaner()
	}
	if len(c.Naming) == 0 {
		c.Naming = defaultNamingRule
	}
	extMap := lo.SliceToMap(append(c.ExtraMediaExtList, defaultMediaSuffix...), func(in string) (string, struct{}) {
		return strings.ToLower(in), struct{}{}
	})
	return &Capture{c: c, extMap: extMap}, nil
}

func (c *Capture) resolveFileInfo(fc *model.FileContext, file, preferredNumber string) error {
	fc.FileName = filepath.Base(file)
	fc.FileExt = filepath.Ext(file)
	fileNoExt := strings.TrimSpace(preferredNumber)
	useCleanerNormalizedValue := len(fileNoExt) == 0
	var cleaned *movieidcleaner.Result
	if len(fileNoExt) == 0 {
		fileNoExt = fc.FileName[:len(fc.FileName)-len(fc.FileExt)]
	}
	if c.c.MovieIDCleaner != nil {
		var err error
		cleaned, err = c.c.MovieIDCleaner.Clean(fileNoExt)
		if err != nil {
			return fmt.Errorf("clean movie id before rewrite failed, err:%w", err)
		}
		if useCleanerNormalizedValue && cleaned != nil && len(cleaned.Normalized) != 0 {
			fileNoExt = cleaned.Normalized
		}
	}
	// 影片 ID 解析
	info, err := number.Parse(fileNoExt)
	if err != nil {
		return fmt.Errorf("parse number failed, err:%w", err)
	}
	if cleaned != nil {
		if cleaned.UncensorMatched {
			info.SetExternalFieldUncensor(cleaned.Uncensor)
		}
		if cleaned.CategoryMatched {
			info.SetExternalFieldCategory(cleaned.Category)
		}
	}

	fc.Number = info
	fc.SaveFileBase = fc.Number.GenerateFileName()
	return nil
}

func (c *Capture) isMediaFile(f string) bool {
	ext := strings.ToLower(filepath.Ext(f))
	if _, ok := c.extMap[ext]; ok {
		return true
	}
	return false
}

func (c *Capture) readFileList() ([]*model.FileContext, error) {
	fcs := make([]*model.FileContext, 0, 20)
	err := filepath.Walk(c.c.ScanDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !c.isMediaFile(path) {
			return nil
		}
		fc := &model.FileContext{FullFilePath: path}
		if err := c.resolveFileInfo(fc, path, ""); err != nil {
			return err
		}
		fcs = append(fcs, fc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk scan dir failed: %w", err)
	}
	return fcs, nil
}

func (c *Capture) Run(ctx context.Context) error {
	fcs, err := c.readFileList()
	if err != nil {
		return fmt.Errorf("read file list failed, err:%w", err)
	}
	c.displayNumberInfo(ctx, fcs)
	if err := c.processFileList(ctx, fcs); err != nil {
		return fmt.Errorf("proc file list failed, err:%w", err)
	}
	return nil
}

func (c *Capture) ResolveFileContext(file string, preferredNumber ...string) (*model.FileContext, error) {
	fc := &model.FileContext{FullFilePath: file}
	numberInput := ""
	if len(preferredNumber) > 0 {
		numberInput = preferredNumber[0]
	}
	if err := c.resolveFileInfo(fc, file, numberInput); err != nil {
		return nil, err
	}
	return fc, nil
}

func (c *Capture) ScrapeMeta(ctx context.Context, fc *model.FileContext) error {
	steps := []struct {
		name string
		fn   fcProcessFunc
	}{
		{"search", c.doSearch},
		{"process", c.doProcess},
		{"metaverify", c.doMetaVerify},
		{"datadiscard", c.doDataDiscard},
	}
	logger := logutil.GetLogger(ctx).With(zap.String("file", fc.FileName))
	for idx, step := range steps {
		log := logger.With(zap.Int("idx", idx), zap.String("name", step.name))
		log.Debug("step start")
		if err := step.fn(ctx, fc); err != nil {
			log.Error("proc step failed", zap.Error(err))
			return err
		}
		log.Debug("step end")
	}
	return nil
}

func (c *Capture) ImportMeta(ctx context.Context, fc *model.FileContext) error {
	steps := []struct {
		name string
		fn   fcProcessFunc
	}{
		{"metaverify", c.doMetaVerify},
		{"naming", c.doNaming},
		{"savedata", c.doSaveData},
		{"nfo", c.doExport},
	}
	logger := logutil.GetLogger(ctx).With(zap.String("file", fc.FileName))
	for idx, step := range steps {
		log := logger.With(zap.Int("idx", idx), zap.String("name", step.name))
		log.Debug("step start")
		if err := step.fn(ctx, fc); err != nil {
			log.Error("proc step failed", zap.Error(err))
			return err
		}
		log.Debug("step end")
	}
	return nil
}

func (c *Capture) displayNumberInfo(ctx context.Context, fcs []*model.FileContext) {
	logutil.GetLogger(ctx).Info("read movie file succ", zap.Int("count", len(fcs)))
	for _, item := range fcs {
		logutil.GetLogger(ctx).Info("file info",
			zap.String("number", item.Number.GetNumberID()),
			zap.Bool("multi_cd", item.Number.GetIsMultiCD()),
			zap.Int("cd", item.Number.GetMultiCDIndex()), zap.String("file", item.FileName))
	}
}

func (c *Capture) processFileList(ctx context.Context, fcs []*model.FileContext) error {
	var outErr error
	for _, item := range fcs {
		start := time.Now()
		if err := c.processOneFile(ctx, item); err != nil {
			outErr = err
			logutil.GetLogger(ctx).Error("process file failed", zap.Error(err), zap.String("file", item.FullFilePath))
			continue
		}
		logutil.GetLogger(ctx).Info("process file succ",
			zap.String("file", item.FullFilePath),
			zap.Duration("cost", time.Since(start)),
		)
	}
	return outErr
}

func (c *Capture) resolveSaveDir(fc *model.FileContext) error {
	date := "0000-00-00"
	year := "0000"
	month := "00"
	if fc.Meta.ReleaseDate > 0 {
		ts := time.UnixMilli(fc.Meta.ReleaseDate)
		date = ts.Format(time.DateOnly)
		year = fmt.Sprintf("%d", ts.Year())
		month = fmt.Sprintf("%d", ts.Month())
	}
	actor := buildAuthorsName(fc.Meta.Actors)
	title := buildTitle(fc.Meta.Title)
	titleTranslated := buildTitle(fc.Meta.TitleTranslated)
	if len(titleTranslated) == 0 {
		titleTranslated = title
	}
	m := map[string]any{
		NamingReleaseDate:     date,
		NamingReleaseYear:     year,
		NamingReleaseMonth:    month,
		NamingActor:           actor,
		NamingNumber:          fc.Number.GetNumberID(),
		NamingMovieID:         fc.Number.GetNumberID(),
		NamingTitle:           title,
		NamingTitleTranslated: titleTranslated,
	}
	naming := replacer.ReplaceByMap(c.c.Naming, m)
	if len(naming) == 0 {
		return errInvalidNaming
	}
	fc.SaveDir = filepath.Join(c.c.SaveDir, naming)
	return nil
}

func (c *Capture) doSearch(ctx context.Context, fc *model.FileContext) error {
	meta, ok, err := c.c.Searcher.Search(ctx, fc.Number)
	if err != nil {
		return fmt.Errorf("search number failed, number:%s, err:%w", fc.Number.GetNumberID(), err)
	}
	if !ok {
		return errSearchItemNotFound
	}
	if meta.Number != fc.Number.GetNumberID() {
		logutil.GetLogger(ctx).Warn("number not match, may be re-generated, ignore",
			zap.String("search", meta.Number),
			zap.String("file", fc.Number.GetNumberID()),
		)
	}
	fc.Meta = meta
	return nil
}

func (c *Capture) doProcess(ctx context.Context, fc *model.FileContext) error {
	// 执行处理流程, 用于补齐数据或者数据转换
	if err := c.c.Processor.Process(ctx, fc); err != nil {
		// process 不作为关键路径, 一个meta能否可用取决于后续的verify逻辑
		logutil.GetLogger(ctx).Error("process meta failed, go next", zap.Error(err))
	}
	return nil
}

func (c *Capture) doNaming(_ context.Context, fc *model.FileContext) error {
	// 构建保存目录地址
	if err := c.resolveSaveDir(fc); err != nil {
		return fmt.Errorf("resolve save dir failed, err:%w", err)
	}
	// 创建必要的目录
	if err := os.MkdirAll(fc.SaveDir, 0o755); err != nil {
		return fmt.Errorf("make save dir failed, err:%w", err)
	}
	if err := os.MkdirAll(filepath.Join(fc.SaveDir, defaultExtraFanartDir), 0o755); err != nil {
		return fmt.Errorf("make fanart dir failed, err:%w", err)
	}
	c.renameMetaField(fc)
	return nil
}

func (c *Capture) doSaveData(ctx context.Context, fc *model.FileContext) error {
	// 保存元数据并将影片移入指定目录
	if err := c.saveMediaData(ctx, fc); err != nil {
		return fmt.Errorf("save meta data failed, err:%w", err)
	}
	return nil
}

func (c *Capture) doExport(_ context.Context, fc *model.FileContext) error {
	// 导出jellyfin需要的nfo信息
	if err := c.exportNFOData(fc); err != nil {
		return fmt.Errorf("export nfo data failed, err:%w", err)
	}
	return nil
}

func (c *Capture) doMetaVerify(_ context.Context, fc *model.FileContext) error {
	// 全部处理完后必须要保证当前的元数据至少有title, number, cover, title
	if len(fc.Meta.Title) == 0 {
		return errNoTitle
	}
	if len(fc.Meta.Number) == 0 {
		return errNoNumberFound
	}
	if fc.Meta.Cover == nil || len(fc.Meta.Cover.Name) == 0 || len(fc.Meta.Cover.Key) == 0 {
		return errInvalidCover
	}
	if fc.Meta.Poster == nil || len(fc.Meta.Poster.Name) == 0 || len(fc.Meta.Poster.Key) == 0 {
		return errInvalidPoster
	}
	return nil
}

func (c *Capture) doDataDiscard(_ context.Context, fc *model.FileContext) error {
	if c.c.DiscardTranslatedTitle {
		fc.Meta.TitleTranslated = ""
	}
	if c.c.DiscardTranslatedPlot {
		fc.Meta.PlotTranslated = ""
	}
	return nil
}

func (c *Capture) processOneFile(ctx context.Context, fc *model.FileContext) error {
	ctx = trace.WithTraceId(ctx, "TID:N:"+fc.Number.GetNumberID())
	steps := []struct {
		name string
		fn   fcProcessFunc
	}{
		{"search", c.doSearch},
		{"process", c.doProcess},
		{"metaverify", c.doMetaVerify},
		{"naming", c.doNaming},
		{"savedata", c.doSaveData},
		{"datadiscard", c.doDataDiscard},
		{"nfo", c.doExport},
	}
	logger := logutil.GetLogger(ctx).With(zap.String("file", fc.FileName))
	for idx, step := range steps {
		log := logger.With(zap.Int("idx", idx), zap.String("name", step.name))
		log.Debug("step start")
		if err := step.fn(ctx, fc); err != nil {
			log.Error("proc step failed", zap.Error(err))
			return err
		}
		log.Debug("step end")
	}
	logger.Info("process succ",
		zap.String("number_id", fc.Number.GetNumberID()),
		zap.String("scrape_source", fc.Meta.ExtInfo.ScrapeInfo.Source),
		zap.String("release_date", formatTimeToDate(fc.Meta.ReleaseDate)),
		zap.Int("duration", int(fc.Meta.Duration)),
		zap.Int("sample_img_cnt", len(fc.Meta.SampleImages)),
		zap.Strings("genres", fc.Meta.Genres),
		zap.Strings("actors", fc.Meta.Actors),
		zap.String("title", fc.Meta.Title),
		zap.String("title_translated", fc.Meta.TitleTranslated),
		zap.String("plot", fc.Meta.Plot),
		zap.String("plot_translated", fc.Meta.PlotTranslated),
	)
	return nil
}

func (c *Capture) renameMetaField(fc *model.FileContext) {
	if fc.Meta.Cover != nil {
		fc.Meta.Cover.Name = fmt.Sprintf("%s-fanart%s", fc.SaveFileBase, defaultImageExtName)
	}
	if fc.Meta.Poster != nil {
		fc.Meta.Poster.Name = fmt.Sprintf("%s-poster%s", fc.SaveFileBase, defaultImageExtName)
	}
	for idx, item := range fc.Meta.SampleImages { // TODO:这里需要构建子目录, 看看有没有更好的做法
		item.Name = fmt.Sprintf("%s/%s-sample-%d%s", defaultExtraFanartDir, fc.SaveFileBase, idx, defaultImageExtName)
	}
}

func (c *Capture) saveMediaData(ctx context.Context, fc *model.FileContext) error {
	images := make([]*model.File, 0, len(fc.Meta.SampleImages)+2)
	if fc.Meta.Cover != nil {
		images = append(images, fc.Meta.Cover)
	}
	if fc.Meta.Poster != nil {
		images = append(images, fc.Meta.Poster)
	}
	images = append(images, fc.Meta.SampleImages...)
	for _, image := range images {
		target := filepath.Join(fc.SaveDir, image.Name)
		logger := logutil.GetLogger(ctx).With(
			zap.String("image", image.Name),
			zap.String("key", image.Key),
			zap.String("target", target),
		)

		data, err := c.c.Storage.GetData(ctx, image.Key)
		if err != nil {
			logger.Error("read image data failed", zap.Error(err))
			return fmt.Errorf("read image data for %s failed: %w", image.Name, err)
		}

		if err := os.WriteFile(target, data, 0o600); err != nil {
			logger.Error("write image failed", zap.Error(err))
			return fmt.Errorf("write image %s failed: %w", target, err)
		}
		logger.Debug("write image succ")
	}
	movie := filepath.Join(fc.SaveDir, fc.SaveFileBase+fc.FileExt)
	if err := c.moveMovie(fc, fc.FullFilePath, movie); err != nil {
		return fmt.Errorf("move movie to dst dir failed, err:%w", err)
	}
	return nil
}

func (c *Capture) moveMovie(fc *model.FileContext, src, dst string) error {
	if c.c.LinkMode {
		return c.moveMovieByLink(fc, src, dst)
	}
	return c.moveMovieDirect(fc, src, dst)
}

func (c *Capture) moveMovieByLink(_ *model.FileContext, src, dst string) error {
	err := os.Symlink(src, dst)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("symlink %s to %s failed: %w", src, dst, err)
	}
	return nil
}

func (c *Capture) moveMovieDirect(_ *model.FileContext, src, dst string) error {
	return moveFile(src, dst)
}

func (c *Capture) exportNFOData(fc *model.FileContext) error {
	mov := convertMetaToMovieNFO(fc.Meta)
	save := filepath.Join(fc.SaveDir, fc.SaveFileBase+".nfo")
	if err := nfo.WriteMovieToFile(save, mov); err != nil {
		return fmt.Errorf("write movie nfo failed, err:%w", err)
	}
	return nil
}
