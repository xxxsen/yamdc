package client

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"github.com/klauspost/compress/zstd"
	"github.com/xxxsen/common/iotool"
)

func getResponseBody(rsp *http.Response) (io.ReadCloser, error) {
	switch rsp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(rsp.Body)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader failed: %w", err)
		}
		return reader, nil
	case "deflate":
		return flate.NewReader(rsp.Body), nil
	case "zstd":
		r, err := zstd.NewReader(rsp.Body)
		if err != nil {
			return nil, fmt.Errorf("create zstd reader failed: %w", err)
		}
		return iotool.WrapReadWriteCloser(r, nil, rsp.Body), nil
	default:
		return rsp.Body, nil
	}
}

func BuildReaderFromHTTPResponse(rsp *http.Response) (io.ReadCloser, error) {
	return getResponseBody(rsp)
}

func ReadHTTPData(rsp *http.Response) ([]byte, error) {
	defer func() {
		_ = rsp.Body.Close()
	}()
	reader, err := BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read http response body failed: %w", err)
	}
	return data, nil
}
