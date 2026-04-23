package client

import (
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/klauspost/compress/zstd"
	"github.com/xxxsen/common/iotool"
)

var ErrHTTPResponseTooLarge = errors.New("http response body exceeds size limit")

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

const defaultMaxHTTPResponseSize = 256 << 20 // 256 MiB

func ReadHTTPData(rsp *http.Response) ([]byte, error) {
	return ReadHTTPDataWithLimit(rsp, defaultMaxHTTPResponseSize)
}

func ReadHTTPDataWithLimit(rsp *http.Response, maxBytes int64) ([]byte, error) {
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
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read http response body failed: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrHTTPResponseTooLarge, maxBytes)
	}
	return data, nil
}
