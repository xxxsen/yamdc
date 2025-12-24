package impl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/decoder"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/twostep"

	"github.com/samber/lo"
)

var (
	defaultMadouQuHostList = []string{
		"https://madouqu.com",
	}
)

type madouqu struct {
	api.DefaultPlugin
}

func (m *madouqu) OnGetHosts(ctx context.Context) []string {
	return defaultMadouQuHostList
}

func (m *madouqu) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if !strings.HasPrefix(number, "MADOU") {
		return false, nil
	}
	return true, nil
}

func (m *madouqu) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, api.MustSelectDomain(defaultMadouQuHostList), nil)
}

func (m *madouqu) OnHandleHTTPRequest(ctx context.Context, invoker api.HTTPInvoker, req *http.Request) (*http.Response, error) {
	num := meta.GetNumberId(ctx)
	num = strings.TrimPrefix(num, "MADOU") //移除默认的前缀
	num = strings.Trim(num, "-_")
	ns := []string{
		num,
		strings.ReplaceAll(num, "-", ""),
		strings.ReplaceAll(num, "_", ""),
	}
	ns = lo.Uniq(ns)
	return twostep.HandleMultiLinkSearch(ctx, invoker, &twostep.MultiLinkContext{
		ReqBuilder: func(nid string) (*http.Request, error) {
			uri := fmt.Sprintf("%s://%s/video/%s/", req.URL.Scheme, req.URL.Host, strings.ToLower(nid))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
			if err != nil {
				return nil, err
			}
			return req, nil
		},
		Numbers:         ns,
		ValidStatusCode: []int{http.StatusOK},
		ResultTester: func(raw []byte) (bool, error) {
			return strings.Contains(string(raw), "片名"), nil
		},
	})
}

func (m *madouqu) onDecodeNumber(in string) string {
	// "愛豆番號：IDG-5621"
	slices := strings.Split(in, "：")
	if len(slices) != 2 {
		return ""
	}
	number := slices[1]
	number = strings.TrimSpace(number)
	number = strings.ToUpper(number)
	return number
}

func (m *madouqu) onDecodeTitle(in string) string {
	//"愛豆片名：大同事变男惊条约大同订婚案后续"
	slices := strings.Split(in, "：")
	if len(slices) != 2 {
		return ""
	}
	title := slices[1]
	title = strings.TrimSpace(title)
	return title
}

func (m *madouqu) onDecodeActorList(in []string) []string {
	//"\n                            麻豆女郎：丽丽"
	actors := make([]string, 0)
	for _, actor := range in {
		if strings.Contains(actor, "麻豆女郎：") {
			actor = strings.ReplaceAll(actor, "麻豆女郎：", "")
			actor = strings.TrimSpace(actor)
			if len(actor) == 0 {
				continue
			}
			actors = append(actors, strings.Split(actor, "、")...)
		}
	}
	return actors
}

func (m *madouqu) onDecodeReleaseDate(in string) int64 {
	// 2025-05-03T13:37:39+00:00
	slices := strings.Split(in, "T")
	if len(slices) != 2 {
		return 0
	}
	date := slices[0]
	t, _ := time.Parse(time.DateOnly, date)
	return t.UnixMilli()
}

func (m *madouqu) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//p[contains(text(), "番號：")]/text()`,
		TitleExpr:           `//p[contains(text(), "片名：")]/text()`,
		PlotExpr:            ``,
		ActorListExpr:       `//p[a[@title="model"]]`,
		ReleaseDateExpr:     `//meta[@property="article:published_time"]/@content`,
		DurationExpr:        "",
		StudioExpr:          "",
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          "",
		GenreListExpr:       `//span[@class="meta-category"]/a[@rel="category"]`,
		CoverExpr:           `//meta[@property="og:image"]/@content`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	mv, err := dec.DecodeHTML(data,
		decoder.WithNumberParser(m.onDecodeNumber),
		decoder.WithTitleParser(m.onDecodeTitle),
		decoder.WithActorListParser(m.onDecodeActorList),
		decoder.WithReleaseDateParser(m.onDecodeReleaseDate),
	)
	if err != nil {
		return nil, false, err
	}
	if len(mv.Number) == 0 {
		mv.Number = meta.GetNumberId(ctx)
	}
	return mv, true, nil
}

func init() {
	factory.Register(constant.SSMadouqu, factory.PluginToCreator(&madouqu{}))
}
