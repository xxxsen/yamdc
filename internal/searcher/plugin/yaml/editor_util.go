package yaml

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/xxxsen/yamdc/internal/client"
)

func captureHTTPResponse(rsp *http.Response, charset string) (*HTTPResponseDebug, error) {
	defer func() { _ = rsp.Body.Close() }()
	raw, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read http data: %w", err)
	}
	decoded, err := decodeBytes(raw, charset)
	if err != nil {
		return nil, err
	}
	return &HTTPResponseDebug{
		StatusCode:  rsp.StatusCode,
		Headers:     cloneHeader(rsp.Header),
		Body:        string(decoded),
		BodyPreview: previewBody(string(decoded)),
	}, nil
}

func requestDebug(req *http.Request) HTTPRequestDebug {
	headers := make(map[string]string, len(req.Header))
	for key, values := range req.Header {
		headers[key] = strings.Join(values, ", ")
	}
	body := ""
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
		req.Body = io.NopCloser(bytes.NewReader(raw))
	}
	return HTTPRequestDebug{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    body,
	}
}

func cloneHeader(in http.Header) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func previewBody(body string) string {
	const maxLen = 4000
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen]
}

func renderCondition(cond *compiledCondition) string {
	if cond == nil {
		return ""
	}
	return cond.name
}

func ptrRequestDebug(v HTTPRequestDebug) *HTTPRequestDebug {
	return &v
}

func normalizeStringSet(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func equalNormalizedSet(a, b []string) bool {
	na := normalizeStringSet(a)
	nb := normalizeStringSet(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}
