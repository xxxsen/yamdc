package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
)

var (
	errHDCoverResponseNotOK = errors.New("hd cover response not ok")
	errHDCoverTooSmall      = errors.New("skip hd cover, too small")
)

const (
	defaultHDCoverLinkTemplate = "https://awsimgsrc.dmm.co.jp/pics_dig/digital/video/%s/%spl.jpg"
	defaultMinCoverSize        = 20 * 1024 // 20k
)

type highQualityCoverHandler struct {
	httpClient client.IHTTPClient
	storage    store.IStorage
}

func (h *highQualityCoverHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	res := strings.Split(fc.Number.GetNumberID(), "-")
	if len(res) != 2 { // 仅存在2个part的情况下才需要处理, 否则直接跳过
		return nil
	}
	num := strings.ToLower(strings.ReplaceAll(fc.Number.GetNumberID(), "-", "00"))
	link := fmt.Sprintf(defaultHDCoverLinkTemplate, num, num)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return fmt.Errorf("build hd cover link failed, err:%w", err)
	}
	rsp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request hd cover failed, err:%w", err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("hd cover response not ok, code:%d: %w", rsp.StatusCode, errHDCoverResponseNotOK)
	}
	raw, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("read hd cover data failed, err:%w", err)
	}
	if len(raw) < defaultMinCoverSize {
		return fmt.Errorf("skip hd cover, too small, size:%d: %w", len(raw), errHDCoverTooSmall)
	}
	if _, err := image.LoadImage(raw); err != nil {
		return fmt.Errorf("hd cover server return non-image data, err:%w", err)
	}
	key, err := store.AnonymousPutDataTo(ctx, h.storage, raw)
	if err != nil {
		return fmt.Errorf("write hd cover data failed, err:%w", err)
	}
	fc.Meta.Cover.Key = key
	return nil
}

func init() {
	Register(HHDCoverHandler, func(_ interface{}, deps appdeps.Runtime) (IHandler, error) {
		return &highQualityCoverHandler{
			httpClient: deps.HTTPClient,
			storage:    deps.Storage,
		}, nil
	})
}
