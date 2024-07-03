package capture

import (
	"av-capture/constant"
	"av-capture/model"
	"av-capture/nfo"
	"av-capture/number"
	"av-capture/processor"
	"av-capture/store"
	"av-capture/utils"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/common/trace"
	"go.uber.org/zap"
)

const (
	defaultImageExtName   = ".jpg"
	defaultExtraFanartDir = "extrafanart"
)

type fcProcessFunc func(ctx context.Context, fc *model.FileContext) error

type Capture struct {
	c *config
}

func New(opts ...Option) (*Capture, error) {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if len(c.SaveDir) == 0 || len(c.ScanDir) == 0 {
		return nil, fmt.Errorf("invalid dir")
	}
	if c.Searcher == nil {
		return nil, fmt.Errorf("no searcher found")
	}
	if c.Processor == nil {
		c.Processor = processor.DefaultProcessor
	}
	if len(c.Naming) == 0 {
		c.Naming = defaultNamingRule
	}
	return &Capture{c: c}, nil
}

func (c *Capture) resolveFileInfo(fc *model.FileContext, file string) error {
	fc.FileName = filepath.Base(file)
	fc.FileExt = filepath.Ext(file)
	fileNoExt := fc.FileName[:len(fc.FileName)-len(fc.FileExt)]
	info, err := number.Parse(fileNoExt)
	if err != nil {
		return fmt.Errorf("parse number failed, err:%w", err)
	}
	fc.Number = info
	fc.SaveFileBase = fc.Number.Number()
	if fc.Number.IsMultiCD() {
		fc.SaveFileBase += "-CD" + strconv.FormatInt(int64(fc.Number.MultiCDIndex()), 10)
	}
	return nil
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
		if !utils.IsVideoFile(path) {
			return nil
		}
		fc := &model.FileContext{FullFilePath: path}
		if err := c.resolveFileInfo(fc, path); err != nil {
			return err
		}
		fcs = append(fcs, fc)
		return nil
	})
	if err != nil {
		return nil, err
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

func (c *Capture) displayNumberInfo(ctx context.Context, fcs []*model.FileContext) {
	logutil.GetLogger(ctx).Info("read movie file succ", zap.Int("count", len(fcs)))
	for _, item := range fcs {
		logutil.GetLogger(ctx).Info("file info",
			zap.String("number", item.Number.Number()),
			zap.Bool("multi_cd", item.Number.IsMultiCD()),
			zap.Int("cd", item.Number.MultiCDIndex()), zap.String("file", item.FileName))
	}
}

func (c *Capture) processFileList(ctx context.Context, fcs []*model.FileContext) error {
	var outErr error
	for _, item := range fcs {
		if err := c.processOneFile(ctx, item); err != nil {
			outErr = err
			logutil.GetLogger(ctx).Error("process file failed", zap.Error(err), zap.String("file", item.FullFilePath))
			continue
		}
		logutil.GetLogger(ctx).Info("process file succ", zap.String("file", item.FullFilePath))
	}
	return outErr
}

func (c *Capture) resolveSaveDir(fc *model.FileContext) error {
	now := time.UnixMilli(fc.Meta.ReleaseDate)
	date := now.Format(time.DateOnly)
	year := fmt.Sprintf("%d", now.Year())
	month := fmt.Sprintf("%d", now.Month())
	actor := "佚名"
	if len(fc.Meta.Actors) > 0 {
		actor = utils.BuildAuthorsName(fc.Meta.Actors, 256)
	}
	naming := c.c.Naming
	naming = strings.ReplaceAll(naming, NamingReleaseDate, date)
	naming = strings.ReplaceAll(naming, NamingReleaseYear, year)
	naming = strings.ReplaceAll(naming, NamingReleaseMonth, month)
	naming = strings.ReplaceAll(naming, NamingActor, actor)
	naming = strings.ReplaceAll(naming, NamingNumber, fc.Number.Number())
	if len(naming) == 0 {
		return fmt.Errorf("invalid naming")
	}
	fc.SaveDir = filepath.Join(c.c.SaveDir, naming)
	return nil
}

func (c *Capture) doSearch(ctx context.Context, fc *model.FileContext) error {
	meta, ok, err := c.c.Searcher.Search(ctx, fc.Number)
	if err != nil {
		return fmt.Errorf("search number failed, number:%s, err:%w", fc.Number.Number, err)
	}
	if !ok {
		return fmt.Errorf("search item not found")
	}
	c.paddingMetaTag(fc)
	if meta.Number != fc.Number.Number() {
		logutil.GetLogger(ctx).Warn("number not match, may be re-generated, ignore", zap.String("search", meta.Number), zap.String("file", fc.Number.Number()))
	}
	fc.Meta = meta
	return nil
}

func (c *Capture) paddingMetaTag(fc *model.FileContext) {
	if fc == nil || fc.Meta == nil {
		return
	}
	if fc.Number.IsUncensorMovie() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.TagUncensored)
	}
	if fc.Number.IsChineseSubtitle() {
		fc.Meta.Genres = append(fc.Meta.Genres, constant.TagChineseSubtitle)
	}
}

func (c *Capture) doProcess(ctx context.Context, fc *model.FileContext) error {
	//执行处理流程, 用于补齐数据或者数据转换
	if err := c.c.Processor.Process(ctx, fc); err != nil {
		//process 不作为关键路径, 一个meta能否可用取决于后续的verify逻辑
		logutil.GetLogger(ctx).Error("process meta failed, go next", zap.Error(err))
	}
	return nil
}

func (c *Capture) doNaming(ctx context.Context, fc *model.FileContext) error {
	//构建保存目录地址
	if err := c.resolveSaveDir(fc); err != nil {
		return fmt.Errorf("resolve save dir failed, err:%w", err)
	}
	//创建必要的目录
	if err := os.MkdirAll(fc.SaveDir, 0755); err != nil {
		return fmt.Errorf("make save dir failed, err:%w", err)
	}
	if err := os.MkdirAll(filepath.Join(fc.SaveDir, defaultExtraFanartDir), 0755); err != nil {
		return fmt.Errorf("make fanart dir failed, err:%w", err)
	}
	//数据重命名
	if err := c.renameMetaField(fc); err != nil {
		return fmt.Errorf("rename meta field failed, err:%w", err)
	}
	return nil
}

func (c *Capture) doSaveData(ctx context.Context, fc *model.FileContext) error {
	//保存元数据并将影片移入指定目录
	if err := c.saveMediaData(fc); err != nil {
		return fmt.Errorf("save meta data failed, err:%w", err)
	}
	return nil
}

func (c *Capture) doExport(ctx context.Context, fc *model.FileContext) error {
	// 导出jellyfin需要的nfo信息
	if err := c.exportNFOData(fc); err != nil {
		return fmt.Errorf("export nfo data failed, err:%w", err)
	}
	return nil
}

func (c *Capture) doMetaVerify(ctx context.Context, fc *model.FileContext) error {
	//全部处理完后必须要保证当前的元数据至少有title, number, cover, title
	if len(fc.Meta.Title) == 0 {
		return fmt.Errorf("no title")
	}
	if len(fc.Meta.Number) == 0 {
		return fmt.Errorf("no number found")
	}
	if fc.Meta.Cover == nil || len(fc.Meta.Cover.Name) == 0 || len(fc.Meta.Cover.Key) == 0 {
		return fmt.Errorf("invalid cover")
	}
	if fc.Meta.Poster == nil || len(fc.Meta.Poster.Name) == 0 || len(fc.Meta.Poster.Key) == 0 {
		return fmt.Errorf("invalid poster")
	}
	return nil
}

func (c *Capture) processOneFile(ctx context.Context, fc *model.FileContext) error {
	ctx = trace.WithTraceId(ctx, "TID:N:"+fc.Number.Number())
	steps := []struct {
		name string
		fn   fcProcessFunc
	}{
		{"search", c.doSearch},
		{"process", c.doProcess},
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
	logger.Info("process succ")
	return nil
}

func (c *Capture) renameMetaField(fc *model.FileContext) error {
	if fc.Meta.Cover != nil {
		fc.Meta.Cover.Name = fmt.Sprintf("%s-fanart%s", fc.SaveFileBase, utils.GetExtName(fc.Meta.Cover.Name, defaultImageExtName))
	}
	if fc.Meta.Poster != nil {
		fc.Meta.Poster.Name = fmt.Sprintf("%s-poster%s", fc.SaveFileBase, utils.GetExtName(fc.Meta.Poster.Name, defaultImageExtName))
	}
	for idx, item := range fc.Meta.SampleImages { //TODO:这里需要构建子目录, 看看有没有更好的做法
		item.Name = fmt.Sprintf("%s/%s-sample-%d%s", defaultExtraFanartDir, fc.SaveFileBase, idx, utils.GetExtName(item.Name, defaultImageExtName))
	}
	return nil
}

func (c *Capture) saveMediaData(fc *model.FileContext) error {
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
		logger := logutil.GetLogger(context.Background()).With(zap.String("image", image.Name), zap.String("key", image.Key), zap.String("target", target))

		data, err := store.GetDefault().GetData(image.Key)
		if err != nil {
			logger.Error("read image data failed", zap.Error(err))
			return err
		}

		if err := os.WriteFile(target, data, 0644); err != nil {
			logger.Error("write image failed", zap.Error(err))
			return err
		}
		logger.Debug("write image succ")
	}
	movie := filepath.Join(fc.SaveDir, fc.SaveFileBase+fc.FileExt)
	if err := c.moveMovie(fc, fc.FullFilePath, movie); err != nil {
		return fmt.Errorf("move movie to dst dir failed, err:%w", err)
	}
	return nil
}

func (c *Capture) moveMovie(fc *model.FileContext, src string, dst string) error {
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
	}
	return err
}

func (c *Capture) moveMovieDirect(_ *model.FileContext, src, dst string) error {
	return os.Rename(src, dst)
}

func (c *Capture) exportNFOData(fc *model.FileContext) error {
	mov, err := utils.ConvertMetaToMovieNFO(fc.Meta)
	if err != nil {
		return fmt.Errorf("convert meta to movie nfo failed, err:%w", err)
	}
	save := filepath.Join(fc.SaveDir, fc.SaveFileBase+".nfo")
	if err := nfo.WriteMovieToFile(save, mov); err != nil {
		return fmt.Errorf("write movie nfo failed, err:%w", err)
	}
	return nil
}
