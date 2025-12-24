package airav

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/parser"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

var defaultAirAvHostList = []string{
	"https://www.airav.wiki",
}

type airav struct {
	api.DefaultPlugin
}

func (p *airav) OnGetHosts(ctx context.Context) []string {
	return defaultAirAvHostList
}

func (p *airav) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	domain := api.MustSelectDomain(defaultAirAvHostList)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/video/barcode/%s?lng=zh-TW", domain, number), nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (p *airav) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
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
		logutil.GetLogger(ctx).Warn("more than one result, may cause data mismatch", zap.Int("count", vdata.Count))
	}
	result := vdata.Result
	avdata := &model.MovieMeta{
		Number:      result.Barcode,
		Title:       result.Name,
		Plot:        result.Description,
		Actors:      p.readActors(&result),
		ReleaseDate: parser.DateOnlyReleaseDateParser(ctx)(result.PublishDate),
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
	factory.Register(constant.SSAirav, factory.PluginToCreator(&airav{}))
}
