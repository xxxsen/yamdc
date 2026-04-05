package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/constant"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

var defaultManyVidsHosts = []string{
	"https://www.manyvids.com",
}

type manyvids struct {
	api.DefaultPlugin
}

type manyVidsVideoResponse struct {
	StatusCode    int                `json:"statusCode"`
	StatusMessage string             `json:"statusMessage"`
	Data          *manyVidsVideoData `json:"data"`
}

type manyVidsVideoData struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	LaunchDate    string         `json:"launchDate"`
	VideoDuration string         `json:"videoDuration"`
	Screenshot    string         `json:"screenshot"`
	TagList       []manyVidsTag  `json:"tagList"`
	Model         *manyVidsModel `json:"model"`
}

type manyVidsTag struct {
	Label string `json:"label"`
}

type manyVidsModel struct {
	DisplayName string `json:"displayName"`
}

func (p *manyvids) OnGetHosts(ctx context.Context) []string {
	return defaultManyVidsHosts
}

func (p *manyvids) OnPrecheckRequest(ctx context.Context, number string) (bool, error) {
	if !strings.HasPrefix(number, "MANYVIDS") {
		return false, nil
	}
	return true, nil
}

func (p *manyvids) OnMakeHTTPRequest(ctx context.Context, number string) (*http.Request, error) {
	id := strings.TrimPrefix(number, "MANYVIDS-")
	uri := fmt.Sprintf("https://api.manyvids.com/store/video/%s", id)
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

func (p *manyvids) OnDecorateRequest(ctx context.Context, req *http.Request) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://www.manyvids.com")
	req.Header.Set("Referer", "https://www.manyvids.com/")
	req.Header.Set("Sec-GPC", "1")
	return nil
}

func (p *manyvids) OnDecorateMediaRequest(ctx context.Context, req *http.Request) error {
	req.Header.Set("Referer", "https://www.manyvids.com/")
	req.Header.Set("Origin", "https://www.manyvids.com")
	req.Header.Set("Sec-GPC", "1")
	return nil
}

func (p *manyvids) OnDecodeHTTPData(ctx context.Context, data []byte) (*model.MovieMeta, bool, error) {
	raw := &manyVidsVideoResponse{}
	if err := json.Unmarshal(data, raw); err != nil {
		return nil, false, fmt.Errorf("decode json data failed, err:%w", err)
	}
	if raw.StatusCode != http.StatusOK || raw.Data == nil {
		return nil, false, nil
	}

	mv := &model.MovieMeta{
		Number:    meta.GetNumberId(ctx),
		Title:     strings.TrimSpace(raw.Data.Title),
		Plot:      strings.TrimSpace(raw.Data.Description),
		Cover:     &model.File{Name: strings.TrimSpace(raw.Data.Screenshot)},
		Poster:    &model.File{Name: strings.TrimSpace(raw.Data.Screenshot)},
		TitleLang: enum.MetaLangEn,
		PlotLang:  enum.MetaLangEn,
	}

	if raw.Data.Model != nil && strings.TrimSpace(raw.Data.Model.DisplayName) != "" {
		mv.Actors = []string{strings.TrimSpace(raw.Data.Model.DisplayName)}
	}
	if datePart, _, ok := strings.Cut(strings.TrimSpace(raw.Data.LaunchDate), "T"); ok && datePart != "" {
		mv.ReleaseDate = parseManyVidsDate(datePart)
	}
	mv.Duration = parseManyVidsDuration(raw.Data.VideoDuration)
	for _, tag := range raw.Data.TagList {
		label := strings.TrimSpace(tag.Label)
		if label == "" {
			continue
		}
		mv.Genres = append(mv.Genres, label)
	}
	return mv, true, nil
}

func parseManyVidsDate(value string) int64 {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func parseManyVidsDuration(value string) int64 {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}
	min, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0
	}
	return min*60 + sec
}

func init() {
	factory.Register(constant.SSManyVids, factory.PluginToCreator(&manyvids{}))
}
