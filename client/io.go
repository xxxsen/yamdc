package client

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/klauspost/compress/zstd"
	"github.com/xxxsen/common/iotool"
)

func getResponseBody(rsp *http.Response) (io.ReadCloser, error) {
	switch rsp.Header.Get("Content-Encoding") {
	case "gzip":
		return gzip.NewReader(rsp.Body)
	case "deflate":
		return flate.NewReader(rsp.Body), nil
	case "zstd":
		r, err := zstd.NewReader(rsp.Body)
		if err != nil {
			return nil, err
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
	defer rsp.Body.Close()
	reader, err := BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
