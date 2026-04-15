package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated read failure")
}

func TestReadDataAsHTMLTree_Success(t *testing.T) {
	rsp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><body><p id="x">hi</p></body></html>`)),
	}
	node, err := ReadDataAsHTMLTree(rsp)
	require.NoError(t, err)
	require.NotNil(t, node)

	// walk to find text "hi"
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) == "hi" {
			found = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	assert.True(t, found)
}

func TestReadDataAsHTMLTree_ReadHTTPDataFails(t *testing.T) {
	rsp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(errReader{}),
	}
	_, err := ReadDataAsHTMLTree(rsp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read http data")
}

func TestReadDataAsHTMLTree_HTMLParseError(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("<div>")
	}
	for i := 0; i < 600; i++ {
		sb.WriteString("</div>")
	}
	rsp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(sb.String()))),
	}
	_, err := ReadDataAsHTMLTree(rsp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse html")
}
