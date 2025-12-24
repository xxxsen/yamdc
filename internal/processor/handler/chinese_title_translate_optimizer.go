package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/resource"

	"github.com/antchfx/htmlquery"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultYesJav100QueryLinkTplt = "https://www.yesjav.com/search.asp?q=%s&"
)

var (
	defaultYesJav100TitleExtractRegexp = regexp.MustCompile(`(?:\[.*?\]\s*)?([A-Z]+-\d+\s+[^()]+)`)
)

type chineseTitleTranslateOptimizer struct {
	once sync.Once
	m    map[string]string
}

func (c *chineseTitleTranslateOptimizer) tryInitCNumber(ctx context.Context) {
	c.once.Do(func() {
		start := time.Now()
		r, err := gzip.NewReader(bytes.NewReader(resource.ResCNumber))
		if err != nil {
			logutil.GetLogger(ctx).Error("failed to read cnumber gzip data from res", zap.Error(err))
			return
		}
		m := make(map[string]string)
		err = json.NewDecoder(r).Decode(&m)
		if err != nil {
			logutil.GetLogger(ctx).Error("failed to decode cnumber json", zap.Error(err))
			return
		}
		c.m = m
		logutil.GetLogger(ctx).Info("cnumber init succ", zap.Int("count", len(c.m)), zap.Duration("cost", time.Since(start)))
	})
}

func (c *chineseTitleTranslateOptimizer) readTitleFromCNumber(ctx context.Context, numberid string) (string, bool, error) {
	c.tryInitCNumber(ctx)
	title, ok := c.m[numberid]
	if !ok {
		return "", false, nil
	}
	return title, true, nil
}

func (c *chineseTitleTranslateOptimizer) encodeNumberId(numberid string) string {
	encodedid := strings.ReplaceAll(numberid, "-", "%2D")
	encodedid = strings.ReplaceAll(encodedid, "_", "%5F")
	return encodedid
}

func (c *chineseTitleTranslateOptimizer) cleanSearchTitle(title string) string {
	sts := defaultYesJav100TitleExtractRegexp.FindStringSubmatch(title)
	if len(sts) <= 1 {
		return ""
	}
	return strings.TrimSpace(sts[1])
}

func (c *chineseTitleTranslateOptimizer) readTitleFromYesJav(ctx context.Context, numberid string) (string, bool, error) {
	//本质上yesjav100 也是个刮削源, 在这里做这些逻辑还是有点奇怪
	numberid = strings.ToUpper(numberid)
	encodedid := c.encodeNumberId(numberid)

	link := fmt.Sprintf(defaultYesJav100QueryLinkTplt, encodedid)
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", false, err
	}
	rsp, err := client.DefaultClient().Do(req)
	if err != nil {
		return "", false, err
	}
	defer rsp.Body.Close()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", false, err
	}
	node, err := htmlquery.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", false, fmt.Errorf("parse html failed: %w", err)
	}
	items := htmlquery.Find(node, `//font[@size="+0.5"]//a[@target="_blank"]`)
	var searchedTitle string
	for _, item := range items {
		res := strings.ToUpper(strings.TrimSpace(htmlquery.InnerText(item)))
		if len(res) == 0 {
			continue
		}
		if !strings.Contains(res, numberid) {
			continue
		}
		if !strings.Contains(res, "(中文字幕)") {
			continue
		}
		searchedTitle = res
		break
	}
	searchedTitle = c.cleanSearchTitle(searchedTitle)
	if len(searchedTitle) == 0 {
		return "", false, nil
	}
	return searchedTitle, true, nil
}

func (c *chineseTitleTranslateOptimizer) Handle(ctx context.Context, fc *model.FileContext) error {
	hlist := []struct {
		name    string
		handler func(ctx context.Context, numberid string) (string, bool, error)
	}{
		{"c_number", c.readTitleFromCNumber},
		{"yesjav100", c.readTitleFromYesJav},
	}
	for _, h := range hlist {
		newTitle, ok, err := h.handler(ctx, fc.Number.GetNumberID())
		if err != nil {
			logutil.GetLogger(ctx).Error("call sub handler for optimized title failed, skip", zap.Error(err), zap.String("title_searcher", h.name), zap.String("numberid", fc.Number.GetNumberID()))
			continue
		}
		if ok {
			logutil.GetLogger(ctx).Info("optimized chinese title found", zap.String("numberid", fc.Number.GetNumberID()), zap.String("title_searcher", h.name), zap.String("title", newTitle))
			fc.Meta.TitleTranslated = newTitle
			return nil
		}
	}
	logutil.GetLogger(ctx).Debug("no optimized chinese title found, skip")
	return nil
}

func init() {
	Register(HChineseTitleTranslateOptimizer, HandlerToCreator(&chineseTitleTranslateOptimizer{}))
}
