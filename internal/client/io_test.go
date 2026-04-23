package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildReaderFromHTTPResponse_Identity(t *testing.T) {
	body := io.NopCloser(bytes.NewBufferString("plain"))
	rsp := &http.Response{Header: http.Header{}, Body: body}
	rc, err := BuildReaderFromHTTPResponse(rsp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "plain", string(out))
}

func TestBuildReaderFromHTTPResponse_Gzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte("gzipped"))
	require.NoError(t, gw.Close())

	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(&buf),
	}
	rc, err := BuildReaderFromHTTPResponse(rsp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "gzipped", string(out))
}

func TestBuildReaderFromHTTPResponse_GzipInvalid(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewBufferString("not-gzip")),
	}
	_, err := BuildReaderFromHTTPResponse(rsp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gzip")
}

func TestBuildReaderFromHTTPResponse_Deflate(t *testing.T) {
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = fw.Write([]byte("deflate-data"))
	require.NoError(t, err)
	require.NoError(t, fw.Close())

	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"deflate"}},
		Body:   io.NopCloser(&buf),
	}
	rc, err := BuildReaderFromHTTPResponse(rsp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "deflate-data", string(out))
}

func TestBuildReaderFromHTTPResponse_Zstd(t *testing.T) {
	var zbuf bytes.Buffer
	zw, err := zstd.NewWriter(&zbuf)
	require.NoError(t, err)
	_, err = zw.Write([]byte("zstd-payload"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"zstd"}},
		Body:   io.NopCloser(bytes.NewReader(zbuf.Bytes())),
	}
	rc, err := BuildReaderFromHTTPResponse(rsp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "zstd-payload", string(out))
}

func TestBuildReaderFromHTTPResponse_ZstdInvalid(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"zstd"}},
		Body:   io.NopCloser(bytes.NewBufferString("not-zstd")),
	}
	rc, err := BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		assert.Contains(t, err.Error(), "zstd")
		return
	}
	defer func() { _ = rc.Close() }()
	_, err = io.ReadAll(rc)
	require.Error(t, err)
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestReadHTTPData_Success(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewBufferString("hello")),
	}
	data, err := ReadHTTPData(rsp)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestReadHTTPData_BuildReaderError(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewBufferString("bad")),
	}
	_, err := ReadHTTPData(rsp)
	require.Error(t, err)
}

func TestReadHTTPData_ReadAllError(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(errReader{}),
	}
	_, err := ReadHTTPData(rsp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read http response body")
}

func TestReadHTTPDataWithLimit_WithinLimit(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewBufferString("small")),
	}
	data, err := ReadHTTPDataWithLimit(rsp, 1024)
	require.NoError(t, err)
	assert.Equal(t, "small", string(data))
}

func TestReadHTTPDataWithLimit_ExceedsLimit(t *testing.T) {
	rsp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewReader(make([]byte, 200))),
	}
	_, err := ReadHTTPDataWithLimit(rsp, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHTTPResponseTooLarge)
}

func TestReadHTTPDataWithLimit_ExactLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 64)
	rsp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewReader(payload)),
	}
	data, err := ReadHTTPDataWithLimit(rsp, 64)
	require.NoError(t, err)
	assert.Len(t, data, 64)
}
