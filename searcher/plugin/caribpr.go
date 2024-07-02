package plugin

import (
	"av-capture/model"
	"av-capture/searcher"
	"av-capture/searcher/decoder"
	"av-capture/utils"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type cariber struct {
	DefaultPlugin
}

func (p *cariber) OnMakeHTTPRequest(ctx *PluginContext, number string) (*http.Request, error) {
	ctx.SetKey("number", number)
	uri := fmt.Sprintf("https://www.caribbeancompr.com/moviepages/%s/index.html", number)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	return req, err
}

func (p *cariber) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		ts, err := utils.TimeStrToSecond(v)
		if err != nil {
			logutil.GetLogger(ctx).Error("parse duration failed", zap.String("duration", v), zap.Error(err))
			return 0
		}
		return ts
	}
}

func (p *cariber) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			logutil.GetLogger(ctx).Error("parse release date failed", zap.String("release_date", v), zap.Error(err))
			return 0
		}
		return t.UnixMilli()
	}
}

func (p *cariber) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, error) {
	reader := transform.NewReader(strings.NewReader(string(data)), japanese.EUCJP.NewDecoder())
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to decode with eucjp charset, err:%w", err)
	}
	dec := decoder.XPathHtmlDecoder{
		TitleExpr:           `//div[@class='movie-info']/div[@class='section is-wide']/div[@class='heading']/h1/text()`,
		PlotExpr:            `//meta[@name="description"]/@content`,
		ActorListExpr:       `//li[span[contains(text(), "出演")]]/span[@class="spec-content"]/a[@class="spec-item"]/text()`,
		ReleaseDateExpr:     `//li[span[contains(text(), "販売日")]]/span[@class="spec-content"]/text()`,
		DurationExpr:        `//li[span[contains(text(), "再生時間")]]/span[@class="spec-content"]/text()`,
		StudioExpr:          `//li[span[contains(text(), "スタジオ")]]/span[@class="spec-content"]/a/text()`,
		LabelExpr:           ``,
		SeriesExpr:          `//li[span[contains(text(), "シリーズ")]]/span[@class="spec-content"]/a/text()`,
		GenreListExpr:       `//li[span[contains(text(), "タグ")]]/span[@class="spec-content"]/a/text()`,
		CoverExpr:           ``,
		PosterExpr:          ``,
		SampleImageListExpr: `//div[@class='movie-gallery']/div[@class='section is-wide']/div[2]/div[@class='grid-item']/div/a/@href`,
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(p.decodeDuration(ctx.GetContext())),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx.GetContext())),
	)
	if err != nil {
		return nil, err
	}
	meta.Number = ctx.GetKeyOrDefault("number", "").(string)
	meta.Cover.Name = fmt.Sprintf("https://www.caribbeancompr.com/moviepages/%s/images/l_l.jpg", meta.Number)
	return meta, nil
}

func init() {
	searcher.Register(MustNewDefaultSearcher(SSCaribpr, &cariber{}))
}
