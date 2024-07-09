package plugin

import (
	"net/http"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"
)

type freejavbt struct {
	DefaultPlugin
}

func (p *freejavbt) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	uri := "https://freejavbt.com/zh/" + number.Number()
	ctx.SetKey("number", number.Number())
	return http.NewRequest(http.MethodGet, uri, nil)
}

func (p *freejavbt) parseDuration(v string) int64 {
	rs, _ := utils.ToDuration(v)
	return rs
}

func (p *freejavbt) parseDate(v string) int64 {
	return utils.ToTimestampOrDefault(v, 0)
}

func (p *freejavbt) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          "",
		TitleExpr:           `//h1[@class="text-white"]/strong/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//div[span[contains(text(), "女优")]]/div/a/text()`,
		ReleaseDateExpr:     `//div[span[contains(text(), "日期")]]/span[2]`,
		DurationExpr:        `//div[span[contains(text(), "时长")]]/span[2]`,
		StudioExpr:          `//div[span[contains(text(), "制作")]]/a`,
		LabelExpr:           "",
		DirectorExpr:        `//div[span[contains(text(), "导演")]]/a`,
		SeriesExpr:          "",
		GenreListExpr:       `//div[span[contains(text(), "类别")]]/div/a/text()`,
		CoverExpr:           `//img[@class="video-cover rounded lazyload"]/@data-src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="preview"]/a/img/@data-src`,
	}
	res, err := dec.DecodeHTML(data,
		decoder.WithDurationParser(p.parseDuration),
		decoder.WithReleaseDateParser(p.parseDate),
	)
	if err != nil {
		return nil, false, err
	}
	res.Number = ctx.GetKeyOrDefault("number", "").(string)
	return res, true, nil
}

func init() {
	Register(SSFreeJavBt, PluginToCreator(&freejavbt{}))
}