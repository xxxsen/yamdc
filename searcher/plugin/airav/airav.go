package airav

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"yamdc/model"
	"yamdc/number"
	"yamdc/searcher/plugin"
	"yamdc/searcher/utils"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type airav struct {
	plugin.DefaultPlugin
}

func (p *airav) OnMakeHTTPRequest(ctx *plugin.PluginContext, number *number.Number) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.airav.wiki/api/video/barcode/%s?lng=zh-TW", number.Number()), nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (p *airav) OnDecodeHTTPData(ctx *plugin.PluginContext, data []byte) (*model.AvMeta, bool, error) {
	vdata := &VideoData{}
	if err := json.Unmarshal(data, vdata); err != nil {
		return nil, false, fmt.Errorf("decode json data failed, err:%w", err)
	}
	if !strings.EqualFold(vdata.Status, "ok") {
		return nil, false, fmt.Errorf("search result:`%s`, not ok", vdata.Status)
	}
	if vdata.Count == 0 {
		return nil, false, nil
	}
	if vdata.Count > 1 {
		logutil.GetLogger(ctx.GetContext()).Warn("more than one result, may cause data mismatch", zap.Int("count", vdata.Count))
	}
	result := vdata.Result
	avdata := &model.AvMeta{
		Number:      result.Barcode,
		Title:       result.Name,
		Plot:        result.Description,
		Actors:      p.readActors(&result),
		ReleaseDate: utils.ToTimestampOrDefault(result.PublishDate, 0),
		Studio:      p.readStudio(&result),
		Genres:      p.readGenres(&result),
		Cover: &model.File{
			Name: result.ImgURL,
		},
		SampleImages: p.readSampleImages(&result),
	}
	return avdata, true, nil
}

func (p *airav) readSampleImages(result *Result) []*model.File {
	rs := make([]*model.File, 0, len(result.Images))
	for _, item := range result.Images {
		rs = append(rs, &model.File{
			Name: item,
		})
	}
	return rs
}

func (p *airav) readGenres(result *Result) []string {
	rs := make([]string, 0, len(result.Tags))
	for _, item := range result.Tags {
		rs = append(rs, item.Name)
	}
	return rs
}

func (p *airav) readStudio(result *Result) string {
	if len(result.Factories) > 0 {
		return result.Factories[0].Name
	}
	return ""
}

func (p *airav) readActors(result *Result) []string {
	rs := make([]string, 0, len(result.Actors))
	for _, item := range result.Actors {
		rs = append(rs, item.Name)
	}
	return rs
}

func init() {
	plugin.Register(plugin.SSAirav, plugin.PluginToCreator(&airav{}))
}
