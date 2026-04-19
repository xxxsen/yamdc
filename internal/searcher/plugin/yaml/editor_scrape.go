package yaml

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/antchfx/htmlquery"

	"github.com/xxxsen/yamdc/internal/client"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

type scrapeHTTPResult struct {
	finalReq   *http.Request
	decoded    []byte
	statusCode int
	headers    http.Header
}

func debugScrapeFetch(
	ctx context.Context, cli client.IHTTPClient, plg *SearchPlugin, number string,
) (*scrapeHTTPResult, error) {
	ok, err := plg.OnPrecheckRequest(ctx, strings.TrimSpace(number))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errPrecheckNotMatched
	}
	req, err := plg.OnMakeHTTPRequest(ctx, number)
	if err != nil {
		return nil, err
	}
	finalReq := req
	invoker := func(_ context.Context, target *http.Request) (*http.Response, error) {
		finalReq = target
		return cli.Do(target)
	}
	rsp, err := plg.OnHandleHTTPRequest(ctx, invoker, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rsp.Body.Close() }()
	ok, err = plg.OnPrecheckResponse(ctx, finalReq, rsp)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errResponseTreatedAsNotFound
	}
	rawData, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read response data: %w", err)
	}
	decoded, err := decodeBytes(rawData, plg.spec.finalRequest().decodeCharset)
	if err != nil {
		return nil, err
	}
	return &scrapeHTTPResult{
		finalReq:   finalReq,
		decoded:    decoded,
		statusCode: rsp.StatusCode,
		headers:    rsp.Header,
	}, nil
}

func debugScrapeDecodeFields(ctx context.Context, plg *SearchPlugin, result *ScrapeDebugResult, decoded []byte) {
	switch plg.spec.scrape.format {
	case formatHTML:
		node, err := htmlquery.Parse(bytes.NewReader(decoded))
		if err != nil {
			result.Error = fmt.Sprintf("parse html failed: %s", err.Error())
			return
		}
		result.Meta, err = plg.traceDecodeHTML(ctx, node, result.Fields)
		if err != nil {
			result.Error = fmt.Sprintf("trace html fields failed: %s", err.Error())
			return
		}
	case formatJSON:
		var err error
		result.Meta, err = plg.traceDecodeJSON(ctx, decoded, result.Fields)
		if err != nil {
			result.Error = fmt.Sprintf("trace json fields failed: %s", err.Error())
			return
		}
	default:
		result.Error = fmt.Sprintf("unsupported scrape format:%s", plg.spec.scrape.format)
	}
}

func DebugScrape(
	ctx context.Context, cli client.IHTTPClient, raw *PluginSpec, number string,
) (*ScrapeDebugResult, error) {
	plg, err := newCompiledPlugin(raw)
	if err != nil {
		return nil, err
	}
	ctx = pluginapi.InitContainer(ctx)
	ctx = meta.SetNumberID(ctx, number)
	fetch, err := debugScrapeFetch(ctx, cli, plg, number)
	if err != nil {
		return nil, err
	}
	result := &ScrapeDebugResult{
		Request: requestDebug(fetch.finalReq),
		Response: &HTTPResponseDebug{
			StatusCode:  fetch.statusCode,
			Headers:     cloneHeader(fetch.headers),
			Body:        string(fetch.decoded),
			BodyPreview: previewBody(string(fetch.decoded)),
		},
		Fields: make(map[string]FieldDebugResult, len(plg.spec.scrape.fields)),
	}
	debugScrapeDecodeFields(ctx, plg, result, fetch.decoded)
	if result.Meta != nil {
		plg.applyPostprocess(ctx, result.Meta)
	}
	return result, nil
}
