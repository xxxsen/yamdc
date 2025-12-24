package impl

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/numberkit"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var (
	defaultFc2DomainList = []string{
		"https://adult.contents.fc2.com",
	}
)

type fc2 struct {
	api.DefaultPlugin
}

func (p *fc2) OnGetHosts(ctx context.Context) []string {
	return defaultFc2DomainList
}

func (p *fc2) OnMakeHTTPRequest(ctx context.Context, n string) (*http.Request, error) {
	nid, ok := numberkit.DecodeFc2ValID(n)
	if !ok {
		return nil, fmt.Errorf("unable to decode fc2 number")
	}
	uri := fmt.Sprintf("%s/article/%s/", api.MustSelectDomain(defaultFc2DomainList), nid)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
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

func (p *fc2) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
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
	metadata.TitleLang = enum.MetaLangJa
	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSFc2, factory.PluginToCreator(&fc2{}))
}
