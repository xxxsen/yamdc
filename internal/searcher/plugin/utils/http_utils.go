package utils

import (
	"bytes"
	"net/http"
	"github.com/xxxsen/yamdc/internal/client"

	"golang.org/x/net/html"
)

func ReadDataAsHTMLTree(rsp *http.Response) (*html.Node, error) {
	data, err := client.ReadHTTPData(rsp)
	if err != nil {
		return nil, err
	}
	return html.Parse(bytes.NewReader(data))
}
