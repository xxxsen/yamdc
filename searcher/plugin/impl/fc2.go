package impl

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/plugin/constant"
	"yamdc/searcher/plugin/factory"
	"yamdc/searcher/plugin/meta"
	putils "yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var defaultFc2NumberParser = regexp.MustCompile(`^fc2.*?(\d+)$`)

type fc2 struct {
	api.DefaultPlugin
}

func (p *fc2) OnMakeHTTPRequest(ctx context.Context, n *number.Number) (*http.Request, error) {
	number := strings.ToLower(n.GetNumberID())
	res := defaultFc2NumberParser.FindStringSubmatch(number)
	if len(res) != 2 {
		return nil, fmt.Errorf("unabe to decode number")
	}
	number = res[1]
	uri := "https://adult.contents.fc2.com/article/" + number + "/"
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (p *fc2) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		logger := logutil.GetLogger(ctx).With(zap.String("duration", v))
		res := strings.Split(v, ":")
		if len(res) != 2 {
			logger.Error("invalid duration str")
			return 0
		}
		min, err := strconv.ParseUint(res[0], 10, 64)
		if err != nil {
			logger.Error("decode miniute failed", zap.Error(err))
			return 0
		}
		sec, err := strconv.ParseUint(res[1], 10, 64)
		if err != nil {
			logger.Error("decode second failed", zap.Error(err))
			return 0
		}
		return int64(min*60 + sec)
	}
}

func (p *fc2) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		logger := logutil.GetLogger(ctx).With(zap.String("releasedate", v))
		res := strings.Split(v, ":")
		if len(res) != 2 {
			logger.Error("invalid release date str")
			return 0
		}
		date := strings.TrimSpace(res[1])
		t, err := time.Parse("2006/01/02", date)
		if err != nil {
			logger.Error("parse date time failed", zap.Error(err))
			return 0
		}
		return t.UnixMilli()
	}
}

func (p *fc2) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          ``,
		TitleExpr:           `/html/head/title/text()`,
		ActorListExpr:       `//*[@id="top"]/div[1]/section[1]/div/section/div[2]/ul/li[3]/a/text()`,
		ReleaseDateExpr:     `//*[@id="top"]/div[1]/section[1]/div/section/div[2]/div[2]/p/text()`,
		DurationExpr:        `//p[@class='items_article_info']/text()`,
		StudioExpr:          `//*[@id="top"]/div[1]/section[1]/div/section/div[2]/ul/li[3]/a/text()`,
		LabelExpr:           ``,
		SeriesExpr:          ``,
		GenreListExpr:       `//a[@class='tag tagTag']/text()`,
		CoverExpr:           `//div[@class='items_article_MainitemThumb']/span/img/@src`,
		PosterExpr:          `//div[@class='items_article_MainitemThumb']/span/img/@src`, //这东西就一张封面图, 直接当海报得了
		SampleImageListExpr: `//ul[@class="items_article_SampleImagesArea"]/li/a/@href`,
	}
	metadata, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(p.decodeDuration(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	metadata.Number = meta.GetNumberId(ctx)
	putils.EnableDataTranslate(metadata)
	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSFc2, factory.PluginToCreator(&fc2{}))
}
