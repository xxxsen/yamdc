package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"yamdc/enum"
	"yamdc/model"
	"yamdc/searcher/decoder"
	"yamdc/searcher/parser"
	"yamdc/searcher/plugin/api"
	"yamdc/searcher/plugin/constant"
	"yamdc/searcher/plugin/factory"
	"yamdc/searcher/plugin/meta"
)

var (
	defaultJvrpornHostList = []string{
		"https://jvrporn.com",
	}
)

var (
	defaultNonYearReleaseDate, _ = time.Parse(time.DateOnly, "2000-01-01")
)

type jvrporn struct {
	api.DefaultPlugin
}

func (j *jvrporn) OnGetHosts(ctx context.Context) []string {
	return defaultJvrpornHostList
}

func (j *jvrporn) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if !strings.HasPrefix(number, "JVR-") {
		return false, nil
	}
	return true, nil
}

func (j *jvrporn) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	slices := strings.Split(number, "-")
	if len(slices) != 2 {
		return nil, fmt.Errorf("invalid number for jvrporn")
	}
	id := slices[1]
	uri := fmt.Sprintf("%s/video/%s/", api.MustSelectDomain(defaultJvrpornHostList), id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (j *jvrporn) OnDecorateRequest(ctx context.Context, req *http.Request) error {
	req.AddCookie(&http.Cookie{
		Name:  "adult",
		Value: "true",
	})
	return nil
}

func (j *jvrporn) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          "",
		TitleExpr:           `//h1`,
		PlotExpr:            `//pre`,
		ActorListExpr:       `//div[@class="basic-info"]//td/a[@class="actress"]/span/text()`,
		ReleaseDateExpr:     "",
		DurationExpr:        `//tr[td[span[contains(text(), "Duration")]]]/td[span[@class="bold"]]/span/text()`,
		StudioExpr:          "",
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          "",
		GenreListExpr:       `//tr[td[span[contains(text(), "Tags")]]]/td/a/span[@class="bold"]/text()`,
		CoverExpr:           `//div[@class="video-play-container"]/deo-video/@cover-image`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="gallery-wrap"]/div[@id="snapshot-gallery"]/a/@href`,
	}
	rs, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(parser.DateOnlyReleaseDateParser(ctx)),
		decoder.WithDurationParser(parser.DefaultHHMMSSDurationParser(ctx)),
	)
	if err != nil {
		return nil, false, err
	}
	if len(rs.Title) == 0 {
		return nil, false, nil
	}
	rs.Number = meta.GetNumberId(ctx)
	rs.ReleaseDate = defaultNonYearReleaseDate.UnixMilli() //没有发行时间, 标记一个默认的时间
	rs.TitleLang = enum.MetaLangEn
	rs.PlotLang = enum.MetaLangEn
	rs.GenresLang = enum.MetaLangEn
	rs.ActorsLang = enum.MetaLangEn
	return rs, true, nil
}

func init() {
	factory.Register(constant.SSJvrPorn, factory.PluginToCreator(&jvrporn{}))
}
