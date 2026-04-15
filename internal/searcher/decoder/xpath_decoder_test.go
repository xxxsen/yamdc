package decoder

import (
	"strings"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleHTML() string {
	return `<html><body>
<div id="num">ABC-100</div>
<h1 id="title">  My Title  </h1>
<p id="plot">The plot.</p>
<ul id="actors"><li> A </li><li>B</li><li></li></ul>
<span id="date">20240101</span>
<span id="dur">120</span>
<span id="studio">ST</span>
<span id="label">LB</span>
<span id="series">SR</span>
<ul id="genres"><li>G1</li><li>G2</li></ul>
<img id="cover" src="c.jpg"/>
<img id="poster" src="p.jpg"/>
<ul id="samples"><li>s1.jpg</li><li>  </li><li>s2.jpg</li></ul>
<span id="director">Dir</span>
</body></html>`
}

func sampleDecoder() *XPathHTMLDecoder {
	return &XPathHTMLDecoder{
		NumberExpr:          `//*[@id='num']`,
		TitleExpr:           `//*[@id='title']`,
		PlotExpr:            `//*[@id='plot']`,
		ActorListExpr:       `//*[@id='actors']/li`,
		ReleaseDateExpr:     `//*[@id='date']`,
		DurationExpr:        `//*[@id='dur']`,
		StudioExpr:          `//*[@id='studio']`,
		LabelExpr:           `//*[@id='label']`,
		SeriesExpr:          `//*[@id='series']`,
		GenreListExpr:       `//*[@id='genres']/li`,
		CoverExpr:           `//*[@id='cover']/@src`,
		PosterExpr:          `//*[@id='poster']/@src`,
		SampleImageListExpr: `//*[@id='samples']/li`,
		DirectorExpr:        `//*[@id='director']`,
	}
}

func TestDecodeHTML_Success(t *testing.T) {
	d := sampleDecoder()
	meta, err := d.DecodeHTML([]byte(sampleHTML()))
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "ABC-100", meta.Number)
	assert.Equal(t, "My Title", meta.Title)
	assert.Equal(t, "The plot.", meta.Plot)
	assert.Equal(t, []string{"A", "B"}, meta.Actors)
	assert.Equal(t, int64(20240101), meta.ReleaseDate)
	assert.Equal(t, int64(120), meta.Duration)
	assert.Equal(t, "ST", meta.Studio)
	assert.Equal(t, "LB", meta.Label)
	assert.Equal(t, "SR", meta.Series)
	assert.Equal(t, []string{"G1", "G2"}, meta.Genres)
	assert.Equal(t, "Dir", meta.Director)
	require.NotNil(t, meta.Cover)
	assert.Equal(t, "c.jpg", meta.Cover.Name)
	require.NotNil(t, meta.Poster)
	assert.Equal(t, "p.jpg", meta.Poster.Name)
	require.Len(t, meta.SampleImages, 2)
	assert.Equal(t, "s1.jpg", meta.SampleImages[0].Name)
	assert.Equal(t, "s2.jpg", meta.SampleImages[1].Name)
}

func TestDecodeHTML_ParseError(t *testing.T) {
	d := sampleDecoder()
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("<div>")
	}
	for i := 0; i < 600; i++ {
		sb.WriteString("</div>")
	}
	_, err := d.DecodeHTML([]byte(sb.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse html")
}

func TestDecode_NodeAndEmptyExprs(t *testing.T) {
	doc, err := htmlquery.Parse(strings.NewReader(`<html><body><span id="x">only</span></body></html>`))
	require.NoError(t, err)

	d := &XPathHTMLDecoder{
		NumberExpr: `//*[@id='x']`,
		// all other exprs empty -> zero values / empty slices
	}
	meta, err := d.Decode(doc)
	require.NoError(t, err)
	assert.Equal(t, "only", meta.Number)
	assert.Empty(t, meta.Actors)
	assert.Nil(t, meta.SampleImages)
}

func TestDecode_WithAllOptions(t *testing.T) {
	doc, err := htmlquery.Parse(strings.NewReader(sampleHTML()))
	require.NoError(t, err)
	d := sampleDecoder()

	meta, err := d.Decode(doc,
		WithNumberParser(func(s string) string { return "n:" + s }),
		WithTitleParser(func(s string) string { return "t:" + s }),
		WithPlotParser(func(s string) string { return "p:" + s }),
		WithActorListParser(func(v []string) []string { return append([]string{"head"}, v...) }),
		WithReleaseDateParser(func(_ string) int64 { return 7 }),
		WithDurationParser(func(_ string) int64 { return 8 }),
		WithStudioParser(func(s string) string { return "st:" + s }),
		WithLabelParser(func(s string) string { return "lb:" + s }),
		WithSeriesParser(func(s string) string { return "sr:" + s }),
		WithGenreListParser(func(v []string) []string { return v[:1] }),
		WithCoverParser(func(s string) string { return "cv:" + s }),
		WithPosterParser(func(s string) string { return "ps:" + s }),
		WithDirectorParser(func(s string) string { return "dr:" + s }),
		WithSampleImageListParser(func(v []string) []string { return v[len(v)-1:] }),
		WithDefaultStringProcessor(func(s string) string { return "[" + s + "]" }),
		WithDefaultStringListProcessor(func(v []string) []string {
			out := make([]string, len(v))
			for i := range v {
				out[i] = "(" + v[i] + ")"
			}
			return out
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "n:[ABC-100]", meta.Number)
	assert.Equal(t, "t:[My Title]", meta.Title)
	assert.Equal(t, "p:[The plot.]", meta.Plot)
	assert.Equal(t, []string{"head", "(A)", "(B)"}, meta.Actors)
	assert.Equal(t, int64(7), meta.ReleaseDate)
	assert.Equal(t, int64(8), meta.Duration)
	assert.Equal(t, "st:[ST]", meta.Studio)
	assert.Equal(t, "lb:[LB]", meta.Label)
	assert.Equal(t, "sr:[SR]", meta.Series)
	assert.Equal(t, []string{"(G1)"}, meta.Genres)
	assert.Equal(t, "dr:[Dir]", meta.Director)
	assert.Equal(t, "cv:[c.jpg]", meta.Cover.Name)
	assert.Equal(t, "ps:[p.jpg]", meta.Poster.Name)
	require.Len(t, meta.SampleImages, 1)
	assert.Equal(t, "(s2.jpg)", meta.SampleImages[0].Name)
}

func TestDecodeListAndDecodeSingle(t *testing.T) {
	doc, err := htmlquery.Parse(strings.NewReader(`<root><a>  x  </a><a>y</a><a>   </a></root>`))
	require.NoError(t, err)

	list := DecodeList(doc, "//a")
	assert.Equal(t, []string{"x", "y"}, list)
	assert.Empty(t, DecodeList(doc, "//missing"))

	assert.Equal(t, "x", DecodeSingle(doc, "//a"))
	assert.Empty(t, DecodeSingle(doc, "//missing"))
}

func TestDecode_MissingXPathNodeUsesDefaults(t *testing.T) {
	d := &XPathHTMLDecoder{NumberExpr: `//*[@id='missing']`}
	doc, err := htmlquery.Parse(strings.NewReader(`<html></html>`))
	require.NoError(t, err)
	meta, err := d.Decode(doc)
	require.NoError(t, err)
	assert.Empty(t, meta.Number)
}

func TestDecode_CoverPosterEmptyStringStillAllocatesFile(t *testing.T) {
	d := &XPathHTMLDecoder{
		CoverExpr:  `//*[@id='missing']`,
		PosterExpr: `//*[@id='missing']`,
	}
	doc, err := htmlquery.Parse(strings.NewReader(`<html></html>`))
	require.NoError(t, err)
	meta, err := d.Decode(doc)
	require.NoError(t, err)
	require.NotNil(t, meta.Cover)
	require.NotNil(t, meta.Poster)
	assert.Empty(t, meta.Cover.Name)
	assert.Empty(t, meta.Poster.Name)
}

func TestDefaultNumberParserNonNumeric(t *testing.T) {
	d := &XPathHTMLDecoder{
		ReleaseDateExpr: `//*[@id='bad']`,
		DurationExpr:    `//*[@id='bad']`,
	}
	doc, err := htmlquery.Parse(strings.NewReader(`<span id="bad">not-a-number</span>`))
	require.NoError(t, err)
	meta, err := d.Decode(doc)
	require.NoError(t, err)
	assert.Equal(t, int64(0), meta.ReleaseDate)
	assert.Equal(t, int64(0), meta.Duration)
}

func TestDecodeHTML_InvalidBytesStillParsesAsHTML5(t *testing.T) {
	// malformed but recoverable — should not error from parser
	d := &XPathHTMLDecoder{}
	meta, err := d.DecodeHTML([]byte(`<html><body><unclosed>`))
	require.NoError(t, err)
	require.NotNil(t, meta)
	_ = meta
}

func TestDecode_ModelShape(t *testing.T) {
	d := sampleDecoder()
	doc, err := htmlquery.Parse(strings.NewReader(sampleHTML()))
	require.NoError(t, err)
	meta, err := d.Decode(doc)
	require.NoError(t, err)
	_ = meta
}
