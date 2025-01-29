package impl

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	"yamdc/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type caribpr struct {
	api.DefaultPlugin
}

func (p *caribpr) OnMakeHTTPRequest(ctx context.Context, number *number.Number) (*http.Request, error) {
	uri := fmt.Sprintf("https://www.caribbeancompr.com/moviepages/%s/index.html", number.GetNumberID())
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	return req, err
}

func (p *caribpr) decodeDuration(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		ts, err := utils.TimeStrToSecond(v)
		if err != nil {
			logutil.GetLogger(ctx).Error("parse duration failed", zap.String("duration", v), zap.Error(err))
			return 0
		}
		return ts
	}
}

func (p *caribpr) decodeReleaseDate(ctx context.Context) decoder.NumberParseFunc {
	return func(v string) int64 {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			logutil.GetLogger(ctx).Error("parse release date failed", zap.String("release_date", v), zap.Error(err))
			return 0
		}
		return t.UnixMilli()
	}
}

func (p *caribpr) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	reader := transform.NewReader(strings.NewReader(string(data)), japanese.EUCJP.NewDecoder())
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, fmt.Errorf("unable to decode with eucjp charset, err:%w", err)
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
	metadata, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(p.decodeDuration(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	metadata.Number = meta.GetNumberId(ctx)
	metadata.Cover.Name = fmt.Sprintf("https://www.caribbeancompr.com/moviepages/%s/images/l_l.jpg", metadata.Number)
	putils.EnableDataTranslate(metadata)
	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSCaribpr, factory.PluginToCreator(&caribpr{}))
}
