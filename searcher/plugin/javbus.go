package plugin

import (
	"net/http"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"
)

type javbus struct {
	DefaultPlugin
}

func (p *javbus) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	url := "https://www.javbus.com/" + number.Number()
	return http.NewRequest(http.MethodGet, url, nil)
}

func (p *javbus) OnDecorateRequest(ctx *PluginContext, req *http.Request) error {
	req.AddCookie(&http.Cookie{
		Name:  "existmag",
		Value: "mag",
	})
	req.AddCookie(&http.Cookie{
		Name:  "age",
		Value: "verified",
	})
	req.AddCookie(&http.Cookie{
		Name:  "dv",
		Value: "1",
	})
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br, zstd")
	return nil
}

func (p *javbus) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'識別碼:')]]/span[2]/text()`,
		TitleExpr:           `/html/head/title`,
		ActorListExpr:       `//div[@class="star-name"]/a/text()`,
		ReleaseDateExpr:     `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'發行日期:')]]/text()[1]`,
		DurationExpr:        `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'長度:')]]/text()[1]`,
		StudioExpr:          `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'製作商:')]]/a/text()`,
		LabelExpr:           `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'發行商:')]]/a/text()`,
		SeriesExpr:          `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'系列:')]]/a/text()`,
		GenreListExpr:       `//div[@class="row movie"]/div[@class="col-md-3 info"]/p/span[@class="genre"]/label[input[@name="gr_sel"]]/a/text()`,
		CoverExpr:           `//div[@class="row movie"]/div[@class="col-md-9 screencap"]/a[@class="bigImage"]/@href`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@id="sample-waterfall"]/a[@class="sample-box"]/@href`,
	}
	rs, err := dec.DecodeHTML(data,
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
		return nil, false, err
	}
	return rs, true, nil
}

func init() {
	Register(SSJavBus, PluginToCreator(&javbus{}))
}
