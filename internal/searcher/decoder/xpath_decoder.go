package decoder

import (
	"bytes"
	"strings"
	"github.com/xxxsen/yamdc/internal/model"

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
	DirectorExpr        string
	SeriesExpr          string
	GenreListExpr       string
	CoverExpr           string
	PosterExpr          string
	SampleImageListExpr string
}

func (d *XPathHtmlDecoder) decodeSingle(c *config, node *html.Node, expr string) string {
	if len(expr) == 0 {
		return ""
	}
	res := htmlquery.FindOne(node, expr)
	if res == nil {
		return ""
	}
	return c.DefaultStringProcessor(DecodeSingle(node, expr))
}

func (d *XPathHtmlDecoder) decodeMulti(c *config, node *html.Node, expr string) []string {
	rs := make([]string, 0, 5)
	if len(expr) == 0 {
		return rs
	}
	return c.DefaultStringListProcessor(DecodeList(node, expr))
}

func (d *XPathHtmlDecoder) DecodeHTML(data []byte, opts ...Option) (*model.MovieMeta, error) {
	node, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return d.Decode(node, opts...)
}

func (d *XPathHtmlDecoder) applyOpts(opts ...Option) *config {
	c := &config{
		OnNumberParse:              defaultStringParser,
		OnTitleParse:               defaultStringParser,
		OnPlotParse:                defaultStringParser,
		OnActorListParse:           defaultStringListParser,
		OnReleaseDateParse:         defaultNumberParser,
		OnDurationParse:            defaultNumberParser,
		OnStudioParse:              defaultStringParser,
		OnLabelParse:               defaultStringParser,
		OnSeriesParse:              defaultStringParser,
		OnGenreListParse:           defaultStringListParser,
		OnCoverParse:               defaultStringParser,
		OnPosterParse:              defaultStringParser,
		OnDirectorParse:            defaultStringParser,
		OnSampleImageListParse:     defaultStringListParser,
		DefaultStringProcessor:     defaultStringProcessor,
		DefaultStringListProcessor: defaultStringListProcessor,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (d *XPathHtmlDecoder) Decode(node *html.Node, opts ...Option) (*model.MovieMeta, error) {
	c := d.applyOpts(opts...)
	meta := &model.MovieMeta{
		Number:       c.OnNumberParse(d.decodeSingle(c, node, d.NumberExpr)),
		Title:        c.OnTitleParse(d.decodeSingle(c, node, d.TitleExpr)),
		Plot:         c.OnPlotParse(d.decodeSingle(c, node, d.PlotExpr)),
		Actors:       c.OnActorListParse(d.decodeMulti(c, node, d.ActorListExpr)),
		ReleaseDate:  c.OnReleaseDateParse(d.decodeSingle(c, node, d.ReleaseDateExpr)),
		Duration:     c.OnDurationParse(d.decodeSingle(c, node, d.DurationExpr)),
		Studio:       c.OnStudioParse(d.decodeSingle(c, node, d.StudioExpr)),
		Label:        c.OnLabelParse(d.decodeSingle(c, node, d.LabelExpr)),
		Series:       c.OnSeriesParse(d.decodeSingle(c, node, d.SeriesExpr)),
		Genres:       c.OnGenreListParse(d.decodeMulti(c, node, d.GenreListExpr)),
		Director:     c.OnDirectorParse(d.decodeSingle(c, node, d.DirectorExpr)),
		Cover:        &model.File{Name: c.OnCoverParse(d.decodeSingle(c, node, d.CoverExpr))},
		Poster:       &model.File{Name: c.OnPosterParse(d.decodeSingle(c, node, d.PosterExpr))},
		SampleImages: nil,
	}
	samples := c.OnSampleImageListParse(d.decodeMulti(c, node, d.SampleImageListExpr))
	for _, item := range samples {
		meta.SampleImages = append(meta.SampleImages, &model.File{
			Name: item,
		})
	}
	return meta, nil
}

func DecodeList(node *html.Node, expr string) []string {
	rs := make([]string, 0, 5)
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

func DecodeSingle(node *html.Node, expr string) string {
	res := htmlquery.FindOne(node, expr)
	if res == nil {
		return ""
	}
	return strings.TrimSpace(htmlquery.InnerText(res))
}
