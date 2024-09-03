package client

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
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

func BuildReaderFromHTTPResponse(rsp *http.Response) (io.ReadCloser, error) {
	return getResponseBody(rsp)
}

func ReadHTTPData(rsp *http.Response) ([]byte, error) {
	defer rsp.Body.Close()
	reader, err := BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
