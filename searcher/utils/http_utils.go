package utils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"golang.org/x/net/html"
)

func getResponseBody(rsp *http.Response) (io.ReadCloser, error) {
	switch rsp.Header.Get("Content-Encoding") {
	case "gzip":
		return gzip.NewReader(rsp.Body)
	case "deflate":
		return flate.NewReader(rsp.Body), nil
	default:
		return rsp.Body, nil
	}
}

func ReadHTTPData(rsp *http.Response) ([]byte, error) {
	defer rsp.Body.Close()
	reader, err := getResponseBody(rsp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func ReadDataAsHTMLTree(rsp *http.Response) (*html.Node, error) {
	data, err := ReadHTTPData(rsp)
	if err != nil {
		return nil, err
	}
	return html.Parse(bytes.NewReader(data))
}
