package decoder

import (
	"av-capture/searcher/meta"
	"av-capture/searcher/utils"
	"bytes"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type XPathHtmlDecoder struct {
	NumberExpr          string
	TitleExpr           string
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

func (d *XPathHtmlDecoder) DecodeHTML(data []byte) (*meta.AvMeta, error) {
	node, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return d.Decode(node)
}

func (d *XPathHtmlDecoder) Decode(node *html.Node) (*meta.AvMeta, error) {
	meta := &meta.AvMeta{
		Number:       d.decodeSingle(node, d.NumberExpr),
		Title:        d.decodeSingle(node, d.TitleExpr),
		Actors:       d.decodeMulti(node, d.ActorListExpr),
		ReleaseDate:  0,
		Duration:     0,
		Studio:       d.decodeSingle(node, d.StudioExpr),
		Label:        d.decodeSingle(node, d.LabelExpr),
		Series:       d.decodeSingle(node, d.SeriesExpr),
		Genres:       d.decodeMulti(node, d.GenreListExpr),
		Cover:        d.decodeSingle(node, d.CoverExpr),
		Poster:       d.decodeSingle(node, d.PosterExpr),
		SampleImages: d.decodeMulti(node, d.SampleImageListExpr),
	}

	if v, err := utils.ToTimestamp(d.decodeSingle(node, d.ReleaseDateExpr)); err == nil {
		meta.ReleaseDate = v
	}
	if v, err := utils.ToDuration(d.decodeSingle(node, d.DurationExpr)); err == nil {
		meta.Duration = v
	}

	return meta, nil
}
