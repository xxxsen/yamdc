package yaml

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/enum"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/meta"
)

func checkAcceptedStatus(spec *compiledRequest, code int) error {
	for _, item := range spec.notFoundStatusCodes {
		if code == item {
			return fmt.Errorf("status code %d: %w", code, errStatusCodeNotFound)
		}
	}
	if len(spec.acceptStatusCodes) == 0 {
		if code != http.StatusOK {
			return fmt.Errorf("status code %d: %w", code, errStatusCodeNotAccepted)
		}
		return nil
	}
	for _, item := range spec.acceptStatusCodes {
		if code == item {
			return nil
		}
	}
	return fmt.Errorf("status code %d: %w", code, errStatusCodeNotAccepted)
}

func readResponseBody(rsp *http.Response, charset string) (string, *html.Node, error) {
	defer func() { _ = rsp.Body.Close() }()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return "", nil, fmt.Errorf("read response body: %w", err)
	}
	decoded, err := decodeBytes(raw, charset)
	if err != nil {
		return "", nil, err
	}
	node, err := htmlquery.Parse(bytes.NewReader(decoded))
	if err != nil {
		return "", nil, fmt.Errorf("parse html: %w", err)
	}
	return string(decoded), node, nil
}

func decodeBytes(data []byte, charset string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8":
		return data, nil
	case "euc-jp":
		reader := transform.NewReader(bytes.NewReader(data), japanese.EUCJP.NewDecoder())
		out, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("decode euc-jp: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedCharset, charset)
	}
}

func normalizeLang(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "":
		return ""
	case "ja":
		return enum.MetaLangJa
	case "en":
		return enum.MetaLangEn
	case "zh-cn":
		return enum.MetaLangZH
	case "zh-tw":
		return enum.MetaLangZHTW
	default:
		return in
	}
}

func buildURL(host, path string) string {
	u, err := url.Parse(host)
	if err != nil {
		return host + path
	}
	ref, err := url.Parse(path)
	if err != nil {
		return host + path
	}
	return u.ResolveReference(ref).String()
}

func movieMetaStringMap(mv *model.MovieMeta) map[string]string {
	out := map[string]string{
		"number": mv.Number,
		"title":  mv.Title,
		"plot":   mv.Plot,
		"studio": mv.Studio,
		"label":  mv.Label,
		"series": mv.Series,
	}
	if mv.Cover != nil {
		out["cover"] = mv.Cover.Name
	}
	if mv.Poster != nil {
		out["poster"] = mv.Poster.Name
	}
	return out
}

func readVarsFromContext(ctx context.Context) map[string]string {
	out := map[string]string{}
	for key, value := range pluginapi.ExportContainerData(ctx) {
		if strings.HasPrefix(key, "yaml.var.") {
			out[strings.TrimPrefix(key, "yaml.var.")] = value
		}
	}
	return out
}

func ctxVarKey(name string) string { return "yaml.var." + name }

func currentHost(ctx context.Context, hosts []string) string {
	if host, ok := pluginapi.GetContainerValue(ctx, ctxKeyHost); ok && host != "" {
		return host
	}
	host := pluginapi.MustSelectDomain(hosts)
	pluginapi.SetContainerValue(ctx, ctxKeyHost, host)
	return host
}

func ctxNumber(ctx context.Context) string {
	return meta.GetNumberID(ctx)
}

func regexpMatch(pattern, value string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compile regexp: %w", err)
	}
	return re.MatchString(value), nil
}

func timeParse(layout, value string) (int64, error) {
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("parse time %q: %w", value, err)
	}
	return t.UnixMilli(), nil
}

func softTimeParse(layout, value string) int64 {
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}
