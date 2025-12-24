package impl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

var defaultCaribprHostList = []string{
	"https://www.caribbeancompr.com",
}

type caribpr struct {
	api.DefaultPlugin
}

func (p *caribpr) OnGetHosts(ctx context.Context) []string {
	return defaultCaribprHostList
}

func (p *caribpr) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	uri := fmt.Sprintf("%s/moviepages/%s/index.html", api.MustSelectDomain(defaultCaribprHostList), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	return req, err
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
		decoder.WithDurationParser(parser.DefaultHHMMSSDurationParser(ctx)),
		decoder.WithReleaseDateParser(p.decodeReleaseDate(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	metadata.Number = meta.GetNumberId(ctx)
	metadata.Cover.Name = fmt.Sprintf("https://www.caribbeancompr.com/moviepages/%s/images/l_l.jpg", metadata.Number) //TODO: 看看能不能直接从元数据提取而不是直接拼链接
	metadata.TitleLang = enum.MetaLangJa
	metadata.PlotLang = enum.MetaLangJa
	return metadata, true, nil
}

func init() {
	factory.Register(constant.SSCaribpr, factory.PluginToCreator(&caribpr{}))
}
