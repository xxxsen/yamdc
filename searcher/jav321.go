package searcher

import (
	"av-capture/searcher/decoder"
	"av-capture/searcher/meta"
	"av-capture/searcher/utils"
	"strings"
)

type jav321 struct {
}

func (s *jav321) onMakeRequest(number string) string {
	number = strings.ReplaceAll(number, "-", "00")
	return "https://www.jav321.com/video/" + number
}

func (s *jav321) defaultStringProcessor(v string) string {
	v = strings.Trim(v, ": \t")
	return strings.TrimSpace(v)
}

func (s *jav321) onDecodeHTTPDate(data []byte) (*meta.AvMeta, error) {
	dec := &decoder.XPathHtmlDecoder{
		NumberExpr:          `//b[contains(text(),"品番")]/following-sibling::node()`,
		TitleExpr:           `/html/body/div[2]/div[1]/div[1]/div[1]/h3/text()`,
		PlotExpr:            `/html/body/div[2]/div[1]/div[1]/div[2]/div[3]/div/text()`,
		ActorListExpr:       `//b[contains(text(),"出演者")]/following-sibling::a[starts-with(@href,"/star")]/text()`,
		ReleaseDateExpr:     `//b[contains(text(),"配信開始日")]/following-sibling::node()`,
		DurationExpr:        `//b[contains(text(),"収録時間")]/following-sibling::node()`,
		StudioExpr:          `//b[contains(text(),"メーカー")]/following-sibling::a[starts-with(@href,"/company")]/text()`,
		LabelExpr:           `//b[contains(text(),"メーカー")]/following-sibling::a[starts-with(@href,"/company")]/text()`,
		SeriesExpr:          `//b[contains(text(),"シリーズ")]/following-sibling::node()`,
		GenreListExpr:       `//b[contains(text(),"ジャンル")]/following-sibling::a[starts-with(@href,"/genre")]/text()`,
		CoverExpr:           `/html/body/div[2]/div[2]/div[1]/p/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="col-md-3"]/div[@class="col-xs-12 col-md-12"]/p/a/img/@src`,
	}
	rs, err := dec.DecodeHTML(data,
		decoder.WithDefaultStringProcessor(s.defaultStringProcessor),
		decoder.WithReleaseDateParser(func(v string) int64 {
			rs, _ := utils.ToTimestamp(v)
			return rs
		}),
		decoder.WithDurationParser(func(v string) int64 {
			rs, _ := utils.ToDuration(v)
			return rs
		}),
	)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func createJav321Plugin(args interface{}) (ISearcher, error) {
	jav := &jav321{}
	return NewDefaultSearcher(SSJav321, &DefaultSearchOption{
		OnMakeRequest:    jav.onMakeRequest,
		OnDecodeHTTPData: jav.onDecodeHTTPDate,
	})
}

func init() {
	Register(SSJav321, createJav321Plugin)
}
