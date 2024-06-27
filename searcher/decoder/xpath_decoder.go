package decoder

import (
	"av-capture/searcher/meta"
	"bytes"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type XPathHtmlDecoder struct {
	NumberExpr          string
	TitleExpr           string
	PlotExpr            string
	ActorListExpr       string
	ReleaseDateExpr     string
	DurationExpr        string
	StudioExpr          string
	LabelExpr           string
	SeriesExpr          string
	GenreListExpr       string
	CoverExpr           string
	PosterExpr          string
	SampleImageListExpr string
}

func (d *XPathHtmlDecoder) decodeSingle(node *html.Node, expr string) string {
	if len(expr) == 0 {
		return ""
	}
	res := htmlquery.FindOne(node, expr)
	if res == nil {
		return ""
	}
	return strings.TrimSpace(htmlquery.InnerText(res))
}

func (d *XPathHtmlDecoder) decodeMulti(node *html.Node, expr string) []string {
	rs := make([]string, 0, 5)
	if len(expr) == 0 {
		return rs
	}
	items := htmlquery.Find(node, expr)
	for _, item := range items {
		if item == nil {
			continue
		}
		res := htmlquery.InnerText(item)
		res = strings.TrimSpace(res)
		if len(res) == 0 {
			continue
		}
		rs = append(rs, res)
	}
	return rs
}

func (d *XPathHtmlDecoder) DecodeHTML(data []byte, opts ...Option) (*meta.AvMeta, error) {
	node, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return d.Decode(node, opts...)
}

func (d *XPathHtmlDecoder) applyOpts(opts ...Option) *config {
	c := &config{
		OnNumberParse:          defaultStringParser,
		OnTitleParse:           defaultStringParser,
		OnPlotParse:            defaultStringParser,
		OnActorListParse:       defaultStringListParser,
		OnReleaseDateParse:     defaultNumberParser,
		OnDurationParse:        defaultNumberParser,
		OnStudioParse:          defaultStringParser,
		OnLabelParse:           defaultStringParser,
		OnSeriesParse:          defaultStringParser,
		OnGenreListParse:       defaultStringListParser,
		OnCoverParse:           defaultStringParser,
		OnPosterParse:          defaultStringParser,
		OnSampleImageListParse: defaultStringListParser,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (d *XPathHtmlDecoder) Decode(node *html.Node, opts ...Option) (*meta.AvMeta, error) {
	c := d.applyOpts(opts...)
	meta := &meta.AvMeta{
		Number:       c.OnNumberParse(d.decodeSingle(node, d.NumberExpr)),
		Title:        c.OnTitleParse(d.decodeSingle(node, d.TitleExpr)),
		Plot:         c.OnPlotParse(d.decodeSingle(node, d.PlotExpr)),
		Actors:       c.OnActorListParse(d.decodeMulti(node, d.ActorListExpr)),
		ReleaseDate:  c.OnReleaseDateParse(d.decodeSingle(node, d.ReleaseDateExpr)),
		Duration:     c.OnDurationParse(d.decodeSingle(node, d.DurationExpr)),
		Studio:       c.OnStudioParse(d.decodeSingle(node, d.StudioExpr)),
		Label:        c.OnLabelParse(d.decodeSingle(node, d.LabelExpr)),
		Series:       c.OnSeriesParse(d.decodeSingle(node, d.SeriesExpr)),
		Genres:       c.OnGenreListParse(d.decodeMulti(node, d.GenreListExpr)),
		Cover:        c.OnCoverParse(d.decodeSingle(node, d.CoverExpr)),
		Poster:       c.OnPosterParse(d.decodeSingle(node, d.PosterExpr)),
		SampleImages: c.OnSampleImageListParse(d.decodeMulti(node, d.SampleImageListExpr)),
	}
	return meta, nil
}
