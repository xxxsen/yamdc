package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
)

const (
	defaultHDCoverLinkTemplate = "https://awsimgsrc.dmm.co.jp/pics_dig/digital/video/%s/%spl.jpg"
	defaultMinCoverSize        = 20 * 1024 //20k
)

type highQualityCoverHandler struct {
}

func (h *highQualityCoverHandler) Handle(ctx context.Context, fc *model.FileContext) error {
	res := strings.Split(fc.Number.GetNumberID(), "-")
	if len(res) != 2 { //仅存在2个part的情况下才需要处理, 否则直接跳过
		return nil
	}
	num := strings.ToLower(strings.ReplaceAll(fc.Number.GetNumberID(), "-", "00"))
	link := fmt.Sprintf(defaultHDCoverLinkTemplate, num, num)
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return fmt.Errorf("build hd cover link failed, err:%w", err)
	}
	rsp, err := client.DefaultClient().Do(req)
	if err != nil {
		return fmt.Errorf("request hd cover failed, err:%w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("hd cover response not ok, code:%d", rsp.StatusCode)
	}
	raw, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("read hd cover data failed, err:%w", err)
	}
	if len(raw) < defaultMinCoverSize {
		return fmt.Errorf("skip hd cover, too small, size:%d", len(raw))
	}
	if _, err := image.LoadImage(raw); err != nil {
		return fmt.Errorf("hd cover server return non-image data, err:%w", err)
	}
	key, err := store.AnonymousPutData(ctx, raw)
	if err != nil {
		return fmt.Errorf("write hd cover data failed, err:%w", err)
	}
	fc.Meta.Cover.Key = key
	return nil
}

func init() {
	Register(HHDCoverHandler, HandlerToCreator(&highQualityCoverHandler{}))
}
