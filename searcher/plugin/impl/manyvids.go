package impl

import (
	"context"
	"fmt"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
	"yamdc/enum"
	"yamdc/model"
	"yamdc/searcher/decoder"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/plugin/constant"
	"yamdc/searcher/plugin/factory"
	"yamdc/searcher/plugin/meta"
	"yamdc/utils"
)

var defaultManyVidsHosts = []string{
	"https://www.manyvids.com",
}

type manyvids struct {
	api.DefaultPlugin
}

func (p *manyvids) OnGetHosts(ctx context.Context) []string {
	return defaultManyVidsHosts
}

func (p *manyvids) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	//番号格式
	//example: MANYVIDS-123456
	if !strings.HasPrefix(number, "MANYVIDS") {
		return false, nil
	}
	return true, nil
}
func (p *manyvids) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	num := strings.TrimPrefix(number, "MANYVIDS-") //移除默认的前缀
	uri := fmt.Sprintf("%s/Video/%s", api.MustSelectDomain(defaultManyVidsHosts), num)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *manyvids) decodeTitle(ctx context.Context) decoder.StringParseFunc {
	return func(v string) string {
		// MollyRedWolf - 标题 - ManyVids
		// 去除最后的 ManyVids
		v = strings.TrimSuffix(v, "- ManyVids")

		v = strings.TrimSpace(v)

		return v
	}
}

func (p *manyvids) decodeGenre(ctx context.Context) decoder.StringListParseFunc {
	return func(gs []string) []string {

		for i, g := range gs {
			gs[i] = strings.TrimPrefix(g, "#")
			gs[i] = strings.TrimSpace(gs[i])
		}
		return gs
	}
}

func (p *manyvids) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		// 21m30s
		// 01h16m56s
		return utils.HumanDurationToSecond(v)
	}
}
func (p *manyvids) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		logger := logutil.GetLogger(ctx).With(zap.String("releasedate", v))
		t, err := time.Parse("Jan 2, 2006", strings.TrimSpace(v))
		if err != nil {
			logger.Error("parse date time failed", zap.Error(err))
			return 0
		}
		return t.UnixMilli()
	}
}

func (p *manyvids) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr: ``,
		//TitleExpr:           `/html/head/title/text()`,
		TitleExpr:           `//h1[starts-with(@class, 'VideoMetaInfo_title')]/text()`,
		PlotExpr:            `//div[contains(@class, 'VideoDetail_description')]//p/text()`,
		ActorListExpr:       `//div[starts-with(@class, 'VideoProfileCard_videoProfileCard')]//div[contains(@class, 'VideoProfileCard_creatorDetails')]//a[contains(@class, 'VideoProfileCard_link')]`,
		ReleaseDateExpr:     `//span[starts-with(@class, 'VideoDetail_date')]/text()`,
		DurationExpr:        `//div[contains(@class, 'VideoDetail_details')]//span[5]/text()`,
		StudioExpr:          ``,
		LabelExpr:           ``,
		DirectorExpr:        ``,
		SeriesExpr:          ``,
		GenreListExpr:       `//div[starts-with(@class, 'TagList_tagList')]//a[starts-with(@class, 'Tag_mavTag')]`,
		CoverExpr:           `//meta[@property='og:image']/@content`,
		PosterExpr:          `//meta[@property='og:image']/@content`,
		SampleImageListExpr: ``,
	}
	metadata, err := dec.DecodeHTML(data,
		decoder.WithTitleParser(p.decodeTitle(ctx)),
		decoder.WithGenreListParser(p.decodeGenre(ctx)),
		decoder.WithDurationParser(p.decodeDuration(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)))
	if err != nil {
		return nil, false, err
	}

	metadata.Number = meta.GetNumberId(ctx)
	metadata.TitleLang = enum.MetaLangEn
	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSManyVids, factory.PluginToCreator(&manyvids{}))
}
