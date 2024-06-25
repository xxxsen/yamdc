package plugin

import (
	"net/http"
	"strings"
)

type javbus struct {
}

func (p *javbus) makeRequest(number string) string {
	return "https://www.javbus.com/" + number

}

func (p *javbus) decorateRequest(req *http.Request) error {
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
	req.Header.Add("Referer", "https://www.javbus.com")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br, zstd")
	return nil
}

func (p *javbus) onDataDecode(data []byte) (*AvMeta, error) {
	dec := XPathHtmlDecoder{
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
	rs, err := dec.DecodeHTML(data)
	if err != nil {
		return nil, err
	}
	if len(rs.Cover) > 0 {
		rs.Poster = p.tryExtraPosterFromCover(rs.Cover)
	}
	return rs, nil
}

func (p *javbus) tryExtraPosterFromCover(cover string) string {
	idx := strings.LastIndex(cover, "/")
	if idx < 0 {
		return ""
	}
	urlbase := cover[:idx]
	filename := cover[idx+1:]
	extIdx := strings.LastIndex(filename, ".")
	if extIdx < 0 {
		return ""
	}
	filenameNoExt := filename[:extIdx]
	ext := filename[extIdx+1:]
	if !strings.HasSuffix(filenameNoExt, "_b") {
		return ""
	}
	filenameNoExt = filenameNoExt[:len(filenameNoExt)-2]
	return urlbase + "/" + filenameNoExt + "." + ext
}

func createJavbusPlugin(args interface{}) (IPlugin, error) {
	jav := &javbus{}
	plg, err := NewDefaultPlugin(PlgJavBus, &DefaultPluginOption{
		OnMakeRequest:     jav.makeRequest,
		OnDecorateRequest: jav.decorateRequest,
		OnDecodeHTTPData:  jav.onDataDecode,
	})
	if err != nil {
		return nil, err
	}
	return plg, nil

}

func init() {
	Register(PlgJavBus, createJavbusPlugin)
}
