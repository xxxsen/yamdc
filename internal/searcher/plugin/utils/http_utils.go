package utils

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/xxxsen/yamdc/internal/client"

	"golang.org/x/net/html"
)

func ReadDataAsHTMLTree(rsp *http.Response) (*html.Node, error) {
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, fmt.Errorf("read http data: %w", err)
	}
	node, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return node, nil
}
