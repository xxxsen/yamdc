package capture

import (
	"av-capture/model"
	"av-capture/nfo"
	"av-capture/processor"
	"av-capture/utils"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var (
	defaultCDParserRegexp = regexp.MustCompile(`^(.*?)-[cC][dD](\d+)`)
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

func (c *Capture) resolveMultiCDInfo(fc *model.FileContext) error {
	matches := defaultCDParserRegexp.FindStringSubmatch(fc.FileName)
	if len(matches) <= 2 {
		return nil
	}
	fc.Ext.Number = matches[1]
	fc.Ext.IsMultiCD = true
	cdn, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return fmt.Errorf("parse cd number failed, err:%w", err)
	}
	fc.Ext.CDNumber = int(cdn)
	return nil
}

func (c *Capture) resolveFileInfo(fc *model.FileContext, file string) error {
	fc.FileName = filepath.Base(file)
	fc.FileExt = filepath.Ext(file)
	fc.Ext.Number = fc.FileName[:len(fc.FileName)-len(fc.FileExt)]
	if err := c.resolveMultiCDInfo(fc); err != nil {
		return fmt.Errorf("resolve multi cd failed, err:%w", err)
	}
	if len(fc.Ext.Number) == 0 {
		return fmt.Errorf("invalid number")
	}

	fc.SaveFileBase = fc.Ext.Number
	if fc.Ext.IsMultiCD {
		fc.SaveFileBase += "-CD" + strconv.FormatInt(int64(fc.Ext.CDNumber), 10)
	}
	return nil
}

func (c *Capture) readFileList() ([]*model.FileContext, error) {
	fcs := make([]*model.FileContext, 0, 20)
	err := filepath.Walk(c.c.ScanDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
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
	for _, item := range fcs {
		logutil.GetLogger(ctx).Info("read file succ",
			zap.String("number", item.Ext.Number),
			zap.Bool("multi_cd", item.Ext.IsMultiCD),
			zap.Int("cd", item.Ext.CDNumber), zap.String("file", item.FileName))
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

func (c *Capture) verifyMetaData(meta *model.AvMeta) error {
	//检查基础字段是否都有数据
	if len(meta.Number) == 0 {
		return fmt.Errorf("no number")
	}
	if len(meta.Title) == 0 {
		return fmt.Errorf("no title")
	}
	if meta.Cover == nil {
		return fmt.Errorf("no cover")
	}
	return nil
}

func (c *Capture) resolveSaveDir(fc *model.FileContext) error {
	now := time.UnixMilli(fc.Meta.ReleaseDate)
	date := now.Format(time.DateOnly)
	year := fmt.Sprintf("%d", now.Year())
	month := fmt.Sprintf("%d", now.Month())
	actor := "unknown"
	if len(fc.Meta.Actors) > 0 {
		actor = utils.BuildAuthorsName(fc.Meta.Actors, 256)
	}
	naming := c.c.Naming
	naming = strings.ReplaceAll(naming, NamingReleaseDate, date)
	naming = strings.ReplaceAll(naming, NamingReleaseYear, year)
	naming = strings.ReplaceAll(naming, NamingReleaseMonth, month)
	naming = strings.ReplaceAll(naming, NamingActor, actor)
	if len(naming) == 0 {
		return fmt.Errorf("invalid naming")
	}
	fc.SaveDir = filepath.Join(c.c.SaveDir, naming)
	return nil
}

func (c *Capture) doSearch(ctx context.Context, fc *model.FileContext) error {
	meta, err := c.c.Searcher.Search(fc.Ext.Number)
	if err != nil {
		return fmt.Errorf("search number failed, number:%s, err:%w", fc.Ext.Number, err)
	}
	//验证元信息
	if err := c.verifyMetaData(meta); err != nil {
		return fmt.Errorf("verify meta failed, err:%w", err)
	}
	fc.Meta = meta
	return nil
}

func (c *Capture) doProcess(ctx context.Context, fc *model.FileContext) error {
	//执行处理流程, 用于补齐数据或者数据转换
	if err := c.c.Processor.Process(ctx, fc.Meta); err != nil {
		return fmt.Errorf("process meta failed, err:%w", err)
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

func (c *Capture) processOneFile(ctx context.Context, fc *model.FileContext) error {
	steps := []struct {
		name string
		fn   fcProcessFunc
	}{
		{"search", c.doSearch},
		{"process", c.doProcess},
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
	images := make([]*model.Image, 0, len(fc.Meta.SampleImages)+2)
	if fc.Meta.Cover != nil {
		images = append(images, fc.Meta.Cover)
	}
	if fc.Meta.Poster != nil {
		images = append(images, fc.Meta.Poster)
	}
	images = append(images, fc.Meta.SampleImages...)
	for _, image := range images {
		logger := logutil.GetLogger(context.Background()).With(zap.String("image", image.Name))
		target := filepath.Join(fc.SaveDir, image.Name)
		if err := os.WriteFile(target, image.Data, 0644); err != nil {
			logger.Error("write image failed", zap.Error(err))
			return err
		}
		logger.Debug("write image succ")
	}
	movie := filepath.Join(fc.SaveDir, fc.SaveFileBase, fc.FileExt)
	if err := os.Rename(fc.FullFilePath, movie); err != nil {
		return fmt.Errorf("move movie to save dir failed, err:%w", err)
	}
	return nil
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
