package capture

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/capture/ruleapi"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
)

type testSearcher struct{}

func (s *testSearcher) Name() string {
	return "test"
}

func (s *testSearcher) Search(context.Context, *number.Number) (*model.MovieMeta, bool, error) {
	return nil, false, nil
}

func (s *testSearcher) Check(context.Context) error {
	return nil
}

type staticCleaner struct {
	normalized string
}

func (c *staticCleaner) Clean(input string) (*numbercleaner.Result, error) {
	return &numbercleaner.Result{
		RawInput:   input,
		Normalized: c.normalized,
		Status:     numbercleaner.StatusSuccess,
		Confidence: numbercleaner.ConfidenceHigh,
	}, nil
}

func newTestCapture(t *testing.T, cleaner numbercleaner.Cleaner) *Capture {
	t.Helper()
	cap, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithNumberCleaner(cleaner),
		WithNumberRewriter(ruleapi.WrapFuncAsRewriter(func(in string) (string, error) { return in, nil })),
		WithUncensorTester(ruleapi.WrapFuncAsTester(func(string) (bool, error) { return false, nil })),
		WithNumberCategorier(ruleapi.WrapFuncAsMatcher(func(string) (string, bool, error) { return "", false, nil })),
	)
	require.NoError(t, err)
	return cap
}

func TestResolveFileContextUsesCleanerForFilename(t *testing.T) {
	cap := newTestCapture(t, &staticCleaner{normalized: "ABC-123"})

	fc, err := cap.ResolveFileContext(filepath.Join(t.TempDir(), "ignored.mp4"))
	require.NoError(t, err)
	require.Equal(t, "ABC-123", fc.Number.GenerateFileName())
}

func TestResolveFileContextSkipsCleanerForPreferredNumber(t *testing.T) {
	cap := newTestCapture(t, &staticCleaner{normalized: "ABC-123"})

	fc, err := cap.ResolveFileContext(filepath.Join(t.TempDir(), "ignored.mp4"), "XYZ-999")
	require.NoError(t, err)
	require.Equal(t, "XYZ-999", fc.Number.GenerateFileName())
}
