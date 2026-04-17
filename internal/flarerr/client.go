package flarerr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	errFlareOnlyGET        = errors.New("flare request only supports GET method")
	errFlareResponseStatus = errors.New("flare response status error")
)

const (
	defaultByPassClientTimeout = 40 * time.Second
)

type solveResult struct {
	StatusCode int
	HTML       []byte
	Cookies    []*http.Cookie
}

func (r *solveResult) toHTTPResponse(req *http.Request) *http.Response {
	return &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    r.StatusCode,
		Header:        make(http.Header),
		ContentLength: int64(len(r.HTML)),
		Body:          io.NopCloser(bytes.NewReader(r.HTML)),
		Request:       req,
	}
}

// solveRequest posts the given HTTP request to a FlareSolverr endpoint and
// returns a solveResult containing the rendered HTML and any cookies.
func solveRequest(endpoint string, timeout time.Duration, req *http.Request) (*solveResult, error) {
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("%w, got %s", errFlareOnlyGET, req.Method)
	}
	fr := &flareRequest{
		Cmd:        "request.get",
		URL:        req.URL.String(),
		MaxTimeout: int(timeout.Milliseconds()),
	}
	body, _ := json.Marshal(fr)
	//nolint:gosec,noctx // internal solver endpoint with controlled URL
	resp, err := http.Post(endpoint+"/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post to flare solver: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var frResp flareResponse
	if err := json.NewDecoder(resp.Body).Decode(&frResp); err != nil {
		return nil, fmt.Errorf("decode flare response: %w", err)
	}
	if frResp.Status != "ok" {
		return nil, fmt.Errorf("%w: %s, message: %s", errFlareResponseStatus, frResp.Status, frResp.Message)
	}
	cookies := make([]*http.Cookie, 0, len(frResp.Solution.Cookies))
	for _, fc := range frResp.Solution.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:     fc.Name,
			Value:    fc.Value,
			Path:     fc.Path,
			Domain:   fc.Domain,
			Secure:   fc.Secure,
			HttpOnly: fc.HTTPOnly,
		})
	}
	return &solveResult{
		StatusCode: frResp.Solution.Status,
		HTML:       []byte(frResp.Solution.Response),
		Cookies:    cookies,
	}, nil
}

func normalizeEndpoint(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "http://" + endpoint
	}
	return endpoint
}
