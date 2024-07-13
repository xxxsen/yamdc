package plugin

import (
	"fmt"
	"net/http"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/decoder"
	"yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type javdb struct {
	DefaultPlugin
}

func (p *javdb) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	ctx.SetKey("number", number.Number())
	link := fmt.Sprintf("https://javdb.com/search?q=%s&f=all", number.Number())
	return http.NewRequest(http.MethodGet, link, nil)
}

func (p *javdb) OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, req *http.Request) (*http.Response, error) {
	rsp, err := invoker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("read response failed, err:%w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code:%d", rsp.StatusCode)
	}
	node, err := utils.ReadDataAsHTMLTree(rsp)
	if err != nil {
		return nil, fmt.Errorf("unable to read rsp as node:%w", err)
	}
	link, ok := p.searchBestLink(ctx, ctx.GetKeyOrDefault("number", "").(string), node)
	if !ok {
		return nil, fmt.Errorf("unable to match best link number")
	}
	req, err = http.NewRequest(http.MethodGet, "https://javdb.com"+link, nil)
	if err != nil {
		return nil, fmt.Errorf("rebuild search result link failed, err:%w", err)
	}
	return invoker(ctx, req)
}

func (p *javdb) searchBestLink(ctx *PluginContext, originNumber string, node *html.Node) (string, bool) {
	num := utils.NormalizeNumber(originNumber)
	linklist := decoder.DecodeList(node, `//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/@href`)
	numberlist := decoder.DecodeList(node, `//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/div[@class="video-title"]/strong`)
	logutil.GetLogger(ctx.GetContext()).Debug("read link/number list succ",
		zap.String("number", originNumber), zap.Int("link_count", len(linklist)), zap.Int("number_count", len(numberlist)))
	if len(linklist) != len(numberlist) {
		return "", false
	}
	for idx, number := range numberlist {
		link := linklist[idx]
		logutil.GetLogger(ctx.GetContext()).Debug("match best link succ",
			zap.String("origin_number", originNumber), zap.String("link_number", number), zap.String("link", link))
		if utils.NormalizeNumber(number) == num {
			return link, true
		}
	}
	return "", false
}

func (p *javdb) parseDuration(in string) int64 {
	rs, _ := utils.ToDuration(in)
	return rs
}

func (p *javdb) parseReleaseDate(in string) int64 {
	return utils.ToTimestampOrDefault(in, 0)
}

func (p *javdb) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//a[@class="button is-white copy-to-clipboard"]/@data-clipboard-text`,
		TitleExpr:           `//h2[@class="title is-4"]/strong[@class="current-title"]`,
		PlotExpr:            "",
		ActorListExpr:       `//div[strong[contains(text(), "演員")]]/span[@class="value"]/a`,
		ReleaseDateExpr:     `//div[strong[contains(text(), "日期")]]/span[@class="value"]`,
		DurationExpr:        `//div[strong[contains(text(), "時長")]]/span[@class="value"]`,
		StudioExpr:          `//div[strong[contains(text(), "片商")]]/span[@class="value"]`,
		LabelExpr:           "",
		DirectorExpr:        "",
		SeriesExpr:          `//div[strong[contains(text(), "系列")]]/span[@class="value"]`,
		GenreListExpr:       `//div[strong[contains(text(), "類別")]]/span[@class="value"]/a`,
		CoverExpr:           `//div[@class="column column-video-cover"]/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: `//div[@class="tile-images preview-images"]/a[@class="tile-item"]/@href`,
	}
	meta, err := dec.DecodeHTML(data, decoder.WithReleaseDateParser(p.parseReleaseDate), decoder.WithDurationParser(p.parseDuration))
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func init() {
	Register(SSJavDB, PluginToCreator(&javdb{}))
}
