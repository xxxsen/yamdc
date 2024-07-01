package searcher

import (
	"av-capture/model"
	"av-capture/searcher/decoder"
	"net/http"
	"strings"
)

type fc2Searcher struct {
}

func (s *fc2Searcher) onMakeRequest(number string) (*http.Request, error) {
	number = strings.ToLower(number)
	number = strings.ReplaceAll(number, "fc2-ppv-", "")
	number = strings.ReplaceAll(number, "fc2-", "")
	uri := "https://adult.contents.fc2.com/article/" + number + "/"
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (s *fc2Searcher) onDecodeHTTPDate(data []byte) (*model.AvMeta, error) {
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
		PosterExpr:          "",
		SampleImageListExpr: `//ul[@class="items_article_SampleImagesArea"]/li/a/@href`,
	}
	meta, err := dec.DecodeHTML(data)
	if err != nil {
		return nil, err
	}
	//TODO: fix number
	meta.Number = "123"
	return meta, nil
}

func createFc2Searcher(args interface{}) (ISearcher, error) {
	fc2ss := &fc2Searcher{}
	return NewDefaultSearcher(SSFc2, &DefaultSearchOption{
		OnMakeRequest:    fc2ss.onMakeRequest,
		OnDecodeHTTPData: fc2ss.onDecodeHTTPDate,
	})
}

func init() {
	Register(SSFc2, createFc2Searcher)
}
