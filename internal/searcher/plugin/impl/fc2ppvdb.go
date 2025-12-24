package impl

import (
	"context"
	"fmt"
	"net/http"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/numberkit"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

var defaultFc2PPVDBDomains = []string{
	"https://fc2ppvdb.com",
}

type fc2ppvdb struct {
	api.DefaultPlugin
}

func (p *fc2ppvdb) OnGetHosts(ctx context.Context) []string {
	return defaultFc2PPVDBDomains
}

func (p *fc2ppvdb) OnMakeHTTPRequest(ctx context.Context, nid string) (*http.Request, error) {
	vid, ok := numberkit.DecodeFc2ValID(nid)
	if !ok {
		return nil, fmt.Errorf("unable to decode fc2 vid")
	}
	link := fmt.Sprintf("%s/articles/%s", api.MustSelectDomain(defaultFc2PPVDBDomains), vid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (p *fc2ppvdb) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[contains(text(), "ID")]/span/text()`,
		TitleExpr:           `//div[@class="w-full lg:pl-8 px-2 lg:w-3/5"]/h2/a/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//div[contains(text(), "女優")]/span/a/text()`,
		ReleaseDateExpr:     `//div[contains(text(), "販売日")]/span/text()`,
		DurationExpr:        `//div[contains(text(), "収録時間")]/span/text()`,
		StudioExpr:          `//div[contains(text(), "販売者")]/span/a/text()`,
		LabelExpr:           "",
		DirectorExpr:        `//div[contains(text(), "販売者")]/span/a/text()`,
		SeriesExpr:          "",
		GenreListExpr:       `//div[contains(text(), "タグ")]/span/a/text()`,
		CoverExpr:           `//div[@class="lg:w-2/5 w-full mb-12 md:mb-0"]/a/img/@src`,
		PosterExpr:          `//div[@class="lg:w-2/5 w-full mb-12 md:mb-0"]/a/img/@src`,
		SampleImageListExpr: "",
	}
	mdata, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.DefaultHHMMSSDurationParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	if len(mdata.Number) == 0 {
		return nil, false, nil
	}
	mdata.Number = meta.GetNumberId(ctx)
	mdata.TitleLang = enum.MetaLangJa
	return mdata, true, nil
}

func init() {
	factory.Register(constant.SSFc2PPVDB, factory.PluginToCreator(&fc2ppvdb{}))
}
