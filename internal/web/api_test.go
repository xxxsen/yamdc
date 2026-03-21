package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/store"
)

func TestParseStatusesDefault(t *testing.T) {
	items := parseStatuses("")
	require.Equal(t, []jobdef.Status{
		jobdef.StatusInit,
		jobdef.StatusProcessing,
		jobdef.StatusFailed,
		jobdef.StatusReviewing,
	}, items)
}

func TestParseStatusesCustom(t *testing.T) {
	items := parseStatuses("init, failed ,reviewing")
	require.Equal(t, []jobdef.Status{
		jobdef.StatusInit,
		jobdef.StatusFailed,
		jobdef.StatusReviewing,
	}, items)
}

func TestParseJobRoute(t *testing.T) {
	id, action, err := parseJobRoute("/api/jobs/42/run", "/api/jobs/")
	require.NoError(t, err)
	require.EqualValues(t, 42, id)
	require.Equal(t, "run", action)
}

func TestParseJobRouteNoAction(t *testing.T) {
	id, action, err := parseJobRoute("/api/review/jobs/7", "/api/review/jobs/")
	require.NoError(t, err)
	require.EqualValues(t, 7, id)
	require.Equal(t, "", action)
}

func TestParseJobRouteInvalid(t *testing.T) {
	_, _, err := parseJobRoute("/api/jobs/abc/run", "/api/jobs/")
	require.Error(t, err)
}

func TestHandleAssetDetectContentType(t *testing.T) {
	store.SetStorage(store.NewMemStorage())
	require.NoError(t, store.PutData(context.Background(), "img-key", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}))

	req := httptest.NewRequest(http.MethodGet, "/api/assets/img-key", nil)
	rec := httptest.NewRecorder()

	api := &API{}
	api.handleAsset(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))
}
