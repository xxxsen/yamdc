package yaml

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/model"
	pluginapi "github.com/xxxsen/yamdc/internal/searcher/plugin/api"
)

func TestNormalizeLang(t *testing.T) {
	assert.Equal(t, "", normalizeLang(""))
	assert.NotEmpty(t, normalizeLang("ja"))
	assert.NotEmpty(t, normalizeLang("en"))
	assert.NotEmpty(t, normalizeLang("zh-cn"))
	assert.NotEmpty(t, normalizeLang("zh-tw"))
	assert.Equal(t, "custom", normalizeLang("custom"))
}

// --- checkAcceptedStatus ---

func TestCheckAcceptedStatus(t *testing.T) {
	tests := []struct {
		name    string
		spec    *compiledRequest
		code    int
		wantErr bool
	}{
		{name: "200_default", spec: &compiledRequest{}, code: 200},
		{name: "404_default", spec: &compiledRequest{}, code: 404, wantErr: true},
		{name: "in_accept_list", spec: &compiledRequest{acceptStatusCodes: []int{200, 201}}, code: 201},
		{name: "not_in_accept_list", spec: &compiledRequest{acceptStatusCodes: []int{200}}, code: 404, wantErr: true},
		{name: "in_not_found_list", spec: &compiledRequest{notFoundStatusCodes: []int{404}}, code: 404, wantErr: true},
		{name: "non200_no_accept_list", spec: &compiledRequest{}, code: 500, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAcceptedStatus(tt.spec, tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- decodeBytes ---

func TestDecodeBytes(t *testing.T) {
	data := []byte("hello")
	result, err := decodeBytes(data, "")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	result, err = decodeBytes(data, "utf-8")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	result, err = decodeBytes(data, "utf8")
	require.NoError(t, err)
	assert.Equal(t, data, result)

	eucjpData := []byte{0xa4, 0xa2}
	result, err = decodeBytes(eucjpData, "euc-jp")
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	_, err = decodeBytes(data, "unknown-charset")
	require.Error(t, err)
}

// --- buildURL ---

func TestTimeParse(t *testing.T) {
	_, err := timeParse("2006-01-02", "2024-01-02")
	require.NoError(t, err)

	_, err = timeParse("2006-01-02", "bad")
	require.Error(t, err)
}

func TestSoftTimeParse(t *testing.T) {
	assert.NotZero(t, softTimeParse("2006-01-02", "2024-01-02"))
	assert.Zero(t, softTimeParse("2006-01-02", "bad"))
}

// --- regexpMatch ---

func TestRegexpMatch(t *testing.T) {
	ok, err := regexpMatch(`^ABC`, "ABC-123")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = regexpMatch(`^XYZ`, "ABC-123")
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = regexpMatch(`[invalid`, "ABC")
	require.Error(t, err)
}

// --- spec UnmarshalJSON ---

func TestMovieMetaStringMap(t *testing.T) {
	mv := &model.MovieMeta{
		Number: "N", Title: "T", Cover: &model.File{Name: "C"}, Poster: &model.File{Name: "P"},
	}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "N", m["number"])
	assert.Equal(t, "C", m["cover"])
	assert.Equal(t, "P", m["poster"])

	mv2 := &model.MovieMeta{Number: "N"}
	m2 := movieMetaStringMap(mv2)
	assert.Empty(t, m2["cover"])
}

// --- readVarsFromContext ---

func TestReadVarsFromContext(t *testing.T) {
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, "yaml.var.x", "v1")
	pluginapi.SetContainerValue(ctx, "other.key", "v2")
	vars := readVarsFromContext(ctx)
	assert.Equal(t, "v1", vars["x"])
	assert.NotContains(t, vars, "other.key")
}

// --- ctxVarKey ---

func TestCtxVarKey(t *testing.T) {
	assert.Equal(t, "yaml.var.myvar", ctxVarKey("myvar"))
}

// --- cachedCreator ---

func TestCurrentHost(t *testing.T) {
	ctx := pluginapi.InitContainer(context.Background())
	pluginapi.SetContainerValue(ctx, ctxKeyHost, "https://cached.com")
	assert.Equal(t, "https://cached.com", currentHost(ctx, []string{"https://other.com"}))

	ctx2 := pluginapi.InitContainer(context.Background())
	host := currentHost(ctx2, []string{"https://only.com"})
	assert.Equal(t, "https://only.com", host)
}

// --- assignDateField ---

func TestMovieMetaStringMap_NilFields(t *testing.T) {
	mv := &model.MovieMeta{Number: "N", Title: "T"}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "N", m["number"])
	assert.Equal(t, "T", m["title"])
	assert.Empty(t, m["cover"])
	assert.Empty(t, m["poster"])
}

// --- finalRequest ---

func TestReadResponseBody(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       nopCloser([]byte(`<html><body>hi</body></html>`)),
		Header:     make(http.Header),
	}
	body, node, err := readResponseBody(rsp, "")
	require.NoError(t, err)
	assert.Contains(t, body, "hi")
	assert.NotNil(t, node)
}

// --- collectSelectorResults ---

func TestCheckAcceptedStatus_NotFoundCode(t *testing.T) {
	spec := &compiledRequest{notFoundStatusCodes: []int{302}, acceptStatusCodes: nil}
	err := checkAcceptedStatus(spec, 302)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotFound)
}

func TestCheckAcceptedStatus_NoAcceptCodesNon200(t *testing.T) {
	spec := &compiledRequest{acceptStatusCodes: nil}
	err := checkAcceptedStatus(spec, 500)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotAccepted)
}

func TestCheckAcceptedStatus_AcceptCodesReject(t *testing.T) {
	spec := &compiledRequest{acceptStatusCodes: []int{200, 201}}
	err := checkAcceptedStatus(spec, 500)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStatusCodeNotAccepted)
}

// --- compilePlugin: precheck with variables ---

func TestReadResponseBody_WithCharset(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 200,
		Body:       nopCloser([]byte(`<html><body>hello</body></html>`)),
		Header:     make(http.Header),
	}
	body, node, err := readResponseBody(rsp, "utf-8")
	require.NoError(t, err)
	assert.Contains(t, body, "hello")
	assert.NotNil(t, node)
}

// --- handleResponse readResponseBody error (unsupported charset) ---

func TestDecodeBytes_AllCharsets(t *testing.T) {
	tests := []struct {
		name    string
		charset string
		wantErr bool
	}{
		{"empty", "", false},
		{"utf8", "utf-8", false},
		{"utf8_no_hyphen", "utf8", false},
		{"euc_jp", "euc-jp", false},
		{"unsupported", "windows-1252", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeBytes([]byte("hello"), tt.charset)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNormalizeLang_Extended(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"ja", "ja"},
		{"en", "en"},
		{"zh-cn", "zh-cn"},
		{"zh-tw", "zh-tw"},
		{"fr", "fr"},
		{"  JA  ", "ja"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeLang(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestCheckAcceptedStatus_Extended(t *testing.T) {
	tests := []struct {
		name          string
		acceptCodes   []int
		notFoundCodes []int
		code          int
		wantErr       bool
	}{
		{"ok_default", nil, nil, 200, false},
		{"not_ok_default", nil, nil, 500, true},
		{"not_found_code", nil, []int{302}, 302, true},
		{"accept_ok", []int{200, 301}, nil, 200, false},
		{"accept_not_ok", []int{200}, nil, 500, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &compiledRequest{
				acceptStatusCodes:   tt.acceptCodes,
				notFoundStatusCodes: tt.notFoundCodes,
			}
			err := checkAcceptedStatus(spec, tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMovieMetaStringMap_Full(t *testing.T) {
	mv := &model.MovieMeta{
		Number: "N", Title: "T", Plot: "P", Studio: "S", Label: "L", Series: "SE",
		Cover: &model.File{Name: "c.jpg"}, Poster: &model.File{Name: "p.jpg"},
	}
	m := movieMetaStringMap(mv)
	assert.Equal(t, "T", m["title"])
	assert.Equal(t, "c.jpg", m["cover"])
	assert.Equal(t, "p.jpg", m["poster"])
}

func TestMovieMetaStringMap_NilCoverPoster(t *testing.T) {
	mv := &model.MovieMeta{Number: "N"}
	m := movieMetaStringMap(mv)
	_, hasCover := m["cover"]
	assert.False(t, hasCover)
}
