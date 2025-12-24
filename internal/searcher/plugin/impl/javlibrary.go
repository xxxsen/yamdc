package impl

import (
	"context"
	"fmt"
	"net/http"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
)

var defaultJavLibraryHostList = []string{
	"https://www.javlibrary.com",
}

type javlibrary struct {
	api.DefaultPlugin
}

func (j *javlibrary) OnGetHosts(ctx context.Context) []string {
	return defaultJavLibraryHostList
}

func (j *javlibrary) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	uri := fmt.Sprintf("%s/cn/vl_searchbyid.php?keyword=%s", api.MustSelectDomain(defaultJavLibraryHostList), number)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (j *javlibrary) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//tbody/tr[td[contains(text(), "识别码:")]]/td[@class="text"]/text()`,
		TitleExpr:           `//div[@id="video_title"]/h3[@class="post-title text"]/a[@rel="bookmark"]/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//tbody/tr[td[contains(text(), "演员:")]]/td[@class="text"]//span[@class="star"]/a/text()`,
		ReleaseDateExpr:     `//tbody/tr[td[contains(text(), "发行日期:")]]/td[@class="text"]/text()`,
		DurationExpr:        `//tbody/tr[td[contains(text(), "长度:")]]/td/span[@class="text"]/text()`,
		StudioExpr:          `//tbody/tr[td[contains(text(), "制作商:")]]/td[@class="text"]//span[@class="maker"]/a/text()`,
		LabelExpr:           `//tbody/tr[td[contains(text(), "发行商:")]]/td[@class="text"]//span[@class="label"]/a/text()`,
		DirectorExpr:        `//tbody/tr[td[contains(text(), "导演:")]]/td[@class="text"]/text()`,
		SeriesExpr:          "",
		GenreListExpr:       `//tbody/tr[td[contains(text(), "类别:")]]/td[@class="text"]/span[@class="genre"]/a/text()`,
		CoverExpr:           `//img[@id="video_jacket_img"]/@src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="previewthumbs"]/a/@href`,
	}
	mm, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.MinuteOnlyDurationParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	if mm.Number == "" {
		return nil, false, nil
	}
	if mm.Director == "----" {
		mm.Director = ""
	}
	mm.TitleLang = enum.MetaLangJa
	return mm, true, nil
}

func init() {
	factory.Register(constant.SSJavLibrary, factory.PluginToCreator(&javlibrary{}))
}
