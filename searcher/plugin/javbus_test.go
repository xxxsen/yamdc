package plugin

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/assert"
)

func TestJavbus(t *testing.T) {
	p, err := NewDefaultSearcher(SSJavBus, &javbus{})
	assert.NoError(t, err)
	res, ok, err := p.Search(context.Background(), "BANK-090")
	assert.NoError(t, err)
	assert.True(t, ok)
	_ = res
}

type pair struct {
	name  string
	expr  string
	multi bool
}

func TestHTTPPath(t *testing.T) {
	raw, err := os.ReadFile("./test_data_ECBM-004")
	assert.NoError(t, err)
	node, err := htmlquery.Parse(bytes.NewReader(raw))
	assert.NoError(t, err)
	ps := []pair{
		{
			name: "title",
			expr: `/html/head/title`,
		},
		{
			name: "number",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'識別碼:')]]/span[2]/text()`,
		},
		{
			name: "releaseData",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'發行日期:')]]/text()[1]`,
		},
		{
			name: "duration",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'長度:')]]/text()[1]`,
		},
		{
			name: "studio",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'製作商:')]]/a/text()`,
		},
		{
			name: "label",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'發行商:')]]/a/text()`,
		},
		{
			name: "series",
			expr: `//div[@class="row movie"]/div[@class="col-md-3 info"]/p[span[contains(text(),'系列:')]]/a/text()`,
		},
		{
			name:  "genre",
			expr:  `//div[@class="row movie"]/div[@class="col-md-3 info"]/p/span[@class="genre"]/label[input[@name="gr_sel"]]/a/text()`,
			multi: true,
		},
		{
			name:  "star",
			expr:  `//div[@class="star-name"]/a/text()`,
			multi: true,
		},
		{
			name: "cover",
			expr: `//div[@class="row movie"]/div[@class="col-md-9 screencap"]/a[@class="bigImage"]/@href`,
		},
		{
			name:  "sample",
			expr:  `//div[@id="sample-waterfall"]/a[@class="sample-box"]/@href`,
			multi: true,
		},
	}
	for _, p := range ps {
		if !p.multi {
			val := htmlquery.FindOne(node, p.expr)
			log.Printf("name:%s, value:%s", p.name, htmlquery.InnerText(val))
			continue
		}
		nodes := htmlquery.Find(node, p.expr)
		log.Printf("name:%s", p.name)
		for _, nd := range nodes {
			log.Printf("--value:%s", htmlquery.InnerText(nd))
		}
	}
}
