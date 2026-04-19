package capture

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/store"
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

type metaSearcher struct {
	meta *model.MovieMeta
	ok   bool
	err  error
}

func (s *metaSearcher) Name() string { return "meta" }
func (s *metaSearcher) Search(context.Context, *number.Number) (*model.MovieMeta, bool, error) {
	return s.meta, s.ok, s.err
}
func (s *metaSearcher) Check(context.Context) error { return nil }

type staticCleaner struct {
	normalized      string
	category        string
	categoryMatched bool
	uncensor        bool
	uncensorMatched bool
}

func (c *staticCleaner) Clean(input string) (*movieidcleaner.Result, error) {
	return &movieidcleaner.Result{
		RawInput:        input,
		Normalized:      c.normalized,
		Category:        c.category,
		CategoryMatched: c.categoryMatched,
		Uncensor:        c.uncensor,
		UncensorMatched: c.uncensorMatched,
		Status:          movieidcleaner.StatusSuccess,
		Confidence:      movieidcleaner.ConfidenceHigh,
	}, nil
}

func (c *staticCleaner) Explain(input string) (*movieidcleaner.ExplainResult, error) {
	final, err := c.Clean(input)
	if err != nil {
		return nil, err
	}
	return &movieidcleaner.ExplainResult{
		Input: input,
		Final: final,
	}, nil
}

type errCleaner struct{}

func (c *errCleaner) Clean(_ string) (*movieidcleaner.Result, error) {
	return nil, errors.New("clean error")
}

func (c *errCleaner) Explain(_ string) (*movieidcleaner.ExplainResult, error) {
	return nil, errors.New("explain error")
}

type errProcessor struct{}

func (p *errProcessor) Name() string { return "err_proc" }
func (p *errProcessor) Process(context.Context, *model.FileContext) error {
	return errors.New("process error")
}

func newTestCapture(t *testing.T, cleaner movieidcleaner.Cleaner) *Capture {
	t.Helper()
	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
		WithMovieIDCleaner(cleaner),
	)
	require.NoError(t, err)
	return capt
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "missing scan and save dir",
			opts:    []Option{},
			wantErr: true,
		},
		{
			name: "missing save dir",
			opts: []Option{
				WithScanDir("/tmp/scan"),
			},
			wantErr: true,
		},
		{
			name: "missing searcher",
			opts: []Option{
				WithScanDir("/tmp/scan"),
				WithSaveDir("/tmp/save"),
			},
			wantErr: true,
		},
		{
			name: "missing storage",
			opts: []Option{
				WithScanDir("/tmp/scan"),
				WithSaveDir("/tmp/save"),
				WithSeacher(&testSearcher{}),
			},
			wantErr: true,
		},
		{
			name: "valid minimal config",
			opts: []Option{
				WithScanDir("/tmp/scan"),
				WithSaveDir("/tmp/save"),
				WithSeacher(&testSearcher{}),
				WithStorage(store.NewMemStorage()),
			},
			wantErr: false,
		},
		{
			name: "with all options",
			opts: []Option{
				WithScanDir("/tmp/scan"),
				WithSaveDir("/tmp/save"),
				WithSeacher(&testSearcher{}),
				WithStorage(store.NewMemStorage()),
				WithProcessor(processor.DefaultProcessor),
				WithNamingRule("{YEAR}/{MOVIEID}"),
				WithExtraMediaExtList([]string{".webm"}),
				WithMovieIDCleaner(movieidcleaner.NewPassthroughCleaner()),
				WithTransalteTitleDiscard(true),
				WithTranslatedPlotDiscard(true),
				WithLinkMode(true),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

func TestScanDir(t *testing.T) {
	tests := []struct {
		name     string
		capture  *Capture
		expected string
	}{
		{
			name:     "nil capture",
			capture:  nil,
			expected: "",
		},
		{
			name:     "nil config",
			capture:  &Capture{},
			expected: "",
		},
		{
			name: "valid",
			capture: func() *Capture {
				c := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
				return c
			}(),
			expected: func() string {
				c := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
				return c.c.ScanDir
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "valid" {
				assert.NotEmpty(t, tt.capture.ScanDir())
			} else {
				assert.Equal(t, tt.expected, tt.capture.ScanDir())
			}
		})
	}
}

func TestResolveFileContext(t *testing.T) {
	tests := []struct {
		name            string
		cleaner         movieidcleaner.Cleaner
		file            string
		preferredNumber string
		wantNumber      string
		wantErr         bool
	}{
		{
			name:       "uses cleaner for filename",
			cleaner:    &staticCleaner{normalized: "ABC-123"},
			file:       "ignored.mp4",
			wantNumber: "ABC-123",
		},
		{
			name:            "skips cleaner normalized for preferred number",
			cleaner:         &staticCleaner{normalized: "ABC-123"},
			file:            "ignored.mp4",
			preferredNumber: "XYZ-999",
			wantNumber:      "XYZ-999",
		},
		{
			name: "uses cleaner derived fields for preferred number",
			cleaner: &staticCleaner{
				normalized:      "ABC-123",
				category:        "HEYZO",
				categoryMatched: true,
				uncensor:        true,
				uncensorMatched: true,
			},
			file:            "ignored.mp4",
			preferredNumber: "HEYZO-0040",
			wantNumber:      "HEYZO-0040",
		},
		{
			name:    "cleaner returns error",
			cleaner: &errCleaner{},
			file:    "test.mp4",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capt := newTestCapture(t, tt.cleaner)
			filePath := filepath.Join(t.TempDir(), tt.file)
			var fc *model.FileContext
			var err error
			if tt.preferredNumber != "" {
				fc, err = capt.ResolveFileContext(filePath, tt.preferredNumber)
			} else {
				fc, err = capt.ResolveFileContext(filePath)
			}
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNumber, fc.Number.GenerateFileName())
		})
	}
}

func TestIsMediaFile(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{"mp4", "test.mp4", true},
		{"MP4 uppercase", "test.MP4", true},
		{"mkv", "test.mkv", true},
		{"txt non-media", "test.txt", false},
		{"go file", "test.go", false},
		{"no extension", "testfile", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, capt.isMediaFile(tt.file))
		})
	}
}

func TestIsMediaFileWithExtraExt(t *testing.T) {
	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
		WithExtraMediaExtList([]string{".webm"}),
	)
	require.NoError(t, err)
	assert.True(t, capt.isMediaFile("video.webm"))
	assert.True(t, capt.isMediaFile("video.mp4"))
	assert.False(t, capt.isMediaFile("doc.pdf"))
}

func TestReadFileList(t *testing.T) {
	scanDir := t.TempDir()
	saveDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "ABC-123.mp4"), []byte("fake"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "readme.txt"), []byte("text"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "DEF-456.mkv"), []byte("fake2"), 0o600))

	capt, err := New(
		WithScanDir(scanDir),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
	)
	require.NoError(t, err)
	fcs, err := capt.readFileList()
	require.NoError(t, err)
	assert.Len(t, fcs, 2)
}

func TestReadFileListEmpty(t *testing.T) {
	scanDir := t.TempDir()
	capt, err := New(
		WithScanDir(scanDir),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
	)
	require.NoError(t, err)
	fcs, err := capt.readFileList()
	require.NoError(t, err)
	assert.Empty(t, fcs)
}

func TestDoMetaVerify(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	ctx := context.Background()

	tests := []struct {
		name    string
		meta    *model.MovieMeta
		wantErr error
	}{
		{
			name:    "missing title",
			meta:    &model.MovieMeta{Number: "ABC-123", Cover: &model.File{Name: "c", Key: "k"}, Poster: &model.File{Name: "p", Key: "pk"}},
			wantErr: errNoTitle,
		},
		{
			name:    "missing number",
			meta:    &model.MovieMeta{Title: "test", Cover: &model.File{Name: "c", Key: "k"}, Poster: &model.File{Name: "p", Key: "pk"}},
			wantErr: errNoNumberFound,
		},
		{
			name:    "nil cover",
			meta:    &model.MovieMeta{Title: "test", Number: "ABC-123", Poster: &model.File{Name: "p", Key: "pk"}},
			wantErr: errInvalidCover,
		},
		{
			name:    "cover missing name",
			meta:    &model.MovieMeta{Title: "test", Number: "ABC-123", Cover: &model.File{Key: "k"}, Poster: &model.File{Name: "p", Key: "pk"}},
			wantErr: errInvalidCover,
		},
		{
			name:    "nil poster",
			meta:    &model.MovieMeta{Title: "test", Number: "ABC-123", Cover: &model.File{Name: "c", Key: "k"}},
			wantErr: errInvalidPoster,
		},
		{
			name:    "poster missing key",
			meta:    &model.MovieMeta{Title: "test", Number: "ABC-123", Cover: &model.File{Name: "c", Key: "k"}, Poster: &model.File{Name: "p"}},
			wantErr: errInvalidPoster,
		},
		{
			name: "valid",
			meta: &model.MovieMeta{
				Title:  "test",
				Number: "ABC-123",
				Cover:  &model.File{Name: "cover.jpg", Key: "coverkey"},
				Poster: &model.File{Name: "poster.jpg", Key: "posterkey"},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &model.FileContext{Meta: tt.meta}
			err := capt.doMetaVerify(ctx, fc)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDoDataDiscard(t *testing.T) {
	tests := []struct {
		name         string
		discardTitle bool
		discardPlot  bool
		wantTitle    string
		wantPlot     string
	}{
		{"no discard", false, false, "translated", "plot translated"},
		{"discard title", true, false, "", "plot translated"},
		{"discard plot", false, true, "translated", ""},
		{"discard both", true, true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capt, err := New(
				WithScanDir(t.TempDir()),
				WithSaveDir(t.TempDir()),
				WithSeacher(&testSearcher{}),
				WithStorage(store.NewMemStorage()),
				WithTransalteTitleDiscard(tt.discardTitle),
				WithTranslatedPlotDiscard(tt.discardPlot),
			)
			require.NoError(t, err)
			fc := &model.FileContext{
				Meta: &model.MovieMeta{
					TitleTranslated: "translated",
					PlotTranslated:  "plot translated",
				},
			}
			err = capt.doDataDiscard(context.Background(), fc)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTitle, fc.Meta.TitleTranslated)
			assert.Equal(t, tt.wantPlot, fc.Meta.PlotTranslated)
		})
	}
}

func TestDoSearch(t *testing.T) {
	ctx := context.Background()
	num, _ := number.Parse("ABC-123")

	tests := []struct {
		name     string
		searcher *metaSearcher
		wantErr  bool
	}{
		{
			name:     "search error",
			searcher: &metaSearcher{err: errors.New("search failed")},
			wantErr:  true,
		},
		{
			name:     "not found",
			searcher: &metaSearcher{ok: false},
			wantErr:  true,
		},
		{
			name: "found",
			searcher: &metaSearcher{
				meta: &model.MovieMeta{Number: "ABC-123", Title: "Test"},
				ok:   true,
			},
			wantErr: false,
		},
		{
			name: "found with number mismatch",
			searcher: &metaSearcher{
				meta: &model.MovieMeta{Number: "XYZ-999", Title: "Test"},
				ok:   true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capt, err := New(
				WithScanDir(t.TempDir()),
				WithSaveDir(t.TempDir()),
				WithSeacher(tt.searcher),
				WithStorage(store.NewMemStorage()),
			)
			require.NoError(t, err)
			fc := &model.FileContext{Number: num, FileName: "test.mp4"}
			err = capt.doSearch(ctx, fc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, fc.Meta)
			}
		})
	}
}

func TestDoProcess(t *testing.T) {
	ctx := context.Background()
	num, _ := number.Parse("ABC-123")

	t.Run("default processor success", func(t *testing.T) {
		capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
		fc := &model.FileContext{
			Number: num,
			Meta:   &model.MovieMeta{Title: "test"},
		}
		err := capt.doProcess(ctx, fc)
		assert.NoError(t, err)
	})

	t.Run("processor error is not fatal", func(t *testing.T) {
		capt, err := New(
			WithScanDir(t.TempDir()),
			WithSaveDir(t.TempDir()),
			WithSeacher(&testSearcher{}),
			WithStorage(store.NewMemStorage()),
			WithProcessor(&errProcessor{}),
		)
		require.NoError(t, err)
		fc := &model.FileContext{
			Number: num,
			Meta:   &model.MovieMeta{Title: "test"},
		}
		err = capt.doProcess(ctx, fc)
		assert.NoError(t, err)
	})
}

func TestResolveSaveDir(t *testing.T) {
	tests := []struct {
		name       string
		naming     string
		meta       *model.MovieMeta
		numberID   string
		wantErr    bool
		wantSuffix string
	}{
		{
			name:       "default naming with date",
			naming:     "{YEAR}/{MOVIEID}",
			meta:       &model.MovieMeta{ReleaseDate: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC).UnixMilli()},
			numberID:   "ABC-123",
			wantSuffix: filepath.Join("2023", "ABC-123"),
		},
		{
			name:       "no release date",
			naming:     "{YEAR}/{MOVIEID}",
			meta:       &model.MovieMeta{},
			numberID:   "ABC-123",
			wantSuffix: filepath.Join("0000", "ABC-123"),
		},
		{
			name:       "naming with actor",
			naming:     "{ACTOR}/{MOVIEID}",
			meta:       &model.MovieMeta{Actors: []string{"Alice"}},
			numberID:   "ABC-123",
			wantSuffix: filepath.Join("Alice", "ABC-123"),
		},
		{
			name:       "naming with title",
			naming:     "{TITLE}",
			meta:       &model.MovieMeta{Title: "TestTitle"},
			numberID:   "ABC-123",
			wantSuffix: "TestTitle",
		},
		{
			name:       "naming with translated title fallback to title",
			naming:     "{TITLE_TRANSLATED}",
			meta:       &model.MovieMeta{Title: "OrigTitle"},
			numberID:   "ABC-123",
			wantSuffix: "OrigTitle",
		},
		{
			name:       "naming with translated title",
			naming:     "{TITLE_TRANSLATED}",
			meta:       &model.MovieMeta{Title: "OrigTitle", TitleTranslated: "TransTitle"},
			numberID:   "ABC-123",
			wantSuffix: "TransTitle",
		},
		{
			name:       "naming with date and month",
			naming:     "{DATE}/{MONTH}/{NUMBER}",
			meta:       &model.MovieMeta{ReleaseDate: time.Date(2023, 3, 15, 0, 0, 0, 0, time.UTC).UnixMilli()},
			numberID:   "DEF-456",
			wantSuffix: filepath.Join("2023-03-15", "3", "DEF-456"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveDir := t.TempDir()
			capt, err := New(
				WithScanDir(t.TempDir()),
				WithSaveDir(saveDir),
				WithSeacher(&testSearcher{}),
				WithStorage(store.NewMemStorage()),
				WithNamingRule(tt.naming),
			)
			require.NoError(t, err)
			num, err := number.Parse(tt.numberID)
			require.NoError(t, err)
			fc := &model.FileContext{Meta: tt.meta, Number: num}
			err = capt.resolveSaveDir(fc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, strings.HasSuffix(fc.SaveDir, tt.wantSuffix),
					"expected suffix %q in %q", tt.wantSuffix, fc.SaveDir)
			}
		})
	}
}

func TestRenameMetaField(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	fc := &model.FileContext{
		SaveFileBase: "ABC-123",
		Meta: &model.MovieMeta{
			Cover:  &model.File{Name: "old_cover.jpg", Key: "ckey"},
			Poster: &model.File{Name: "old_poster.jpg", Key: "pkey"},
			SampleImages: []*model.File{
				{Name: "old_sample.jpg", Key: "skey"},
			},
		},
	}
	capt.renameMetaField(fc)
	assert.Equal(t, "ABC-123-fanart.jpg", fc.Meta.Cover.Name)
	assert.Equal(t, "ABC-123-poster.jpg", fc.Meta.Poster.Name)
	assert.Equal(t, "extrafanart/ABC-123-sample-0.jpg", fc.Meta.SampleImages[0].Name)
}

func TestRenameMetaFieldNilFields(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	fc := &model.FileContext{
		SaveFileBase: "XYZ-999",
		Meta: &model.MovieMeta{
			Cover:        nil,
			Poster:       nil,
			SampleImages: nil,
		},
	}
	capt.renameMetaField(fc)
	assert.Nil(t, fc.Meta.Cover)
	assert.Nil(t, fc.Meta.Poster)
}

func TestDoNaming(t *testing.T) {
	ctx := context.Background()
	saveDir := t.TempDir()
	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
		WithNamingRule("{MOVIEID}"),
	)
	require.NoError(t, err)
	num, err := number.Parse("ABC-123")
	require.NoError(t, err)
	fc := &model.FileContext{
		SaveFileBase: "ABC-123",
		Number:       num,
		Meta: &model.MovieMeta{
			Title:  "Test",
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "k"},
			Poster: &model.File{Name: "poster.jpg", Key: "k"},
		},
	}
	err = capt.doNaming(ctx, fc)
	require.NoError(t, err)
	assert.DirExists(t, fc.SaveDir)
	assert.DirExists(t, filepath.Join(fc.SaveDir, defaultExtraFanartDir))
}

func TestSaveMediaData(t *testing.T) {
	ctx := context.Background()
	saveDir := t.TempDir()
	storage := store.NewMemStorage()
	require.NoError(t, storage.PutData(ctx, "coverkey", []byte("coverdata"), 0))
	require.NoError(t, storage.PutData(ctx, "posterkey", []byte("posterdata"), 0))
	require.NoError(t, storage.PutData(ctx, "samplekey", []byte("sampledata"), 0))

	scanDir := t.TempDir()
	movieFile := filepath.Join(scanDir, "ABC-123.mp4")
	require.NoError(t, os.WriteFile(movieFile, []byte("moviedata"), 0o600))

	capt, err := New(
		WithScanDir(scanDir),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(storage),
	)
	require.NoError(t, err)

	movieSaveDir := filepath.Join(saveDir, "test_save")
	require.NoError(t, os.MkdirAll(filepath.Join(movieSaveDir, defaultExtraFanartDir), 0o755))

	fc := &model.FileContext{
		FullFilePath: movieFile,
		SaveDir:      movieSaveDir,
		SaveFileBase: "ABC-123",
		FileExt:      ".mp4",
		Meta: &model.MovieMeta{
			Cover:  &model.File{Name: "ABC-123-fanart.jpg", Key: "coverkey"},
			Poster: &model.File{Name: "ABC-123-poster.jpg", Key: "posterkey"},
			SampleImages: []*model.File{
				{Name: "extrafanart/ABC-123-sample-0.jpg", Key: "samplekey"},
			},
		},
	}
	err = capt.saveMediaData(ctx, fc)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(movieSaveDir, "ABC-123-fanart.jpg"))
	assert.FileExists(t, filepath.Join(movieSaveDir, "ABC-123-poster.jpg"))
	assert.FileExists(t, filepath.Join(movieSaveDir, "extrafanart", "ABC-123-sample-0.jpg"))
	assert.FileExists(t, filepath.Join(movieSaveDir, "ABC-123.mp4"))
}

func TestSaveMediaDataStorageError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	saveDir := t.TempDir()
	movieSaveDir := filepath.Join(saveDir, "test_save")
	require.NoError(t, os.MkdirAll(movieSaveDir, 0o755))

	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(storage),
	)
	require.NoError(t, err)
	fc := &model.FileContext{
		SaveDir:      movieSaveDir,
		SaveFileBase: "ABC-123",
		FileExt:      ".mp4",
		Meta: &model.MovieMeta{
			Cover: &model.File{Name: "cover.jpg", Key: "nonexistent_key"},
		},
	}
	err = capt.saveMediaData(ctx, fc)
	assert.Error(t, err)
}

func TestMoveMovieDirect(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "test.mp4")
	dst := filepath.Join(dstDir, "test.mp4")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

	err := capt.moveMovieDirect(nil, src, dst)
	require.NoError(t, err)
	assert.FileExists(t, dst)
	_, err = os.Stat(src)
	assert.True(t, os.IsNotExist(err))
}

func TestMoveMovieByLink(t *testing.T) {
	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
		WithLinkMode(true),
	)
	require.NoError(t, err)

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "test.mp4")
	dst := filepath.Join(dstDir, "test_link.mp4")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))

	err = capt.moveMovieByLink(nil, src, dst)
	require.NoError(t, err)

	target, err := os.Readlink(dst)
	require.NoError(t, err)
	assert.Equal(t, src, target)

	err = capt.moveMovieByLink(nil, src, dst)
	assert.NoError(t, err)
}

func TestMoveMovie(t *testing.T) {
	t.Run("direct mode", func(t *testing.T) {
		capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
		src := filepath.Join(t.TempDir(), "test.mp4")
		dst := filepath.Join(t.TempDir(), "test.mp4")
		require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))
		err := capt.moveMovie(nil, src, dst)
		require.NoError(t, err)
		assert.FileExists(t, dst)
	})

	t.Run("link mode", func(t *testing.T) {
		capt, err := New(
			WithScanDir(t.TempDir()),
			WithSaveDir(t.TempDir()),
			WithSeacher(&testSearcher{}),
			WithStorage(store.NewMemStorage()),
			WithLinkMode(true),
		)
		require.NoError(t, err)
		src := filepath.Join(t.TempDir(), "test.mp4")
		dst := filepath.Join(t.TempDir(), "test_link.mp4")
		require.NoError(t, os.WriteFile(src, []byte("content"), 0o600))
		err = capt.moveMovie(nil, src, dst)
		require.NoError(t, err)
		_, err = os.Readlink(dst)
		assert.NoError(t, err)
	})
}

func TestExportNFOData(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	saveDir := t.TempDir()
	fc := &model.FileContext{
		SaveDir:      saveDir,
		SaveFileBase: "ABC-123",
		Meta: &model.MovieMeta{
			Title:  "Test Title",
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg"},
			Poster: &model.File{Name: "poster.jpg"},
		},
	}
	err := capt.exportNFOData(fc)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(saveDir, "ABC-123.nfo"))
}

func TestScrapeMeta(t *testing.T) {
	ctx := context.Background()
	num, _ := number.Parse("ABC-123")
	storage := store.NewMemStorage()

	t.Run("search not found", func(t *testing.T) {
		capt, err := New(
			WithScanDir(t.TempDir()),
			WithSaveDir(t.TempDir()),
			WithSeacher(&metaSearcher{ok: false}),
			WithStorage(storage),
		)
		require.NoError(t, err)
		fc := &model.FileContext{Number: num, FileName: "test.mp4"}
		err = capt.ScrapeMeta(ctx, fc)
		assert.Error(t, err)
	})

	t.Run("success with valid meta", func(t *testing.T) {
		validMeta := &model.MovieMeta{
			Title:  "Test",
			Number: "ABC-123",
			Cover:  &model.File{Name: "c.jpg", Key: "ck"},
			Poster: &model.File{Name: "p.jpg", Key: "pk"},
		}
		capt, err := New(
			WithScanDir(t.TempDir()),
			WithSaveDir(t.TempDir()),
			WithSeacher(&metaSearcher{meta: validMeta, ok: true}),
			WithStorage(storage),
		)
		require.NoError(t, err)
		fc := &model.FileContext{Number: num, FileName: "test.mp4"}
		err = capt.ScrapeMeta(ctx, fc)
		assert.NoError(t, err)
	})
}

func TestImportMeta(t *testing.T) {
	ctx := context.Background()
	num, _ := number.Parse("ABC-123")
	storage := store.NewMemStorage()
	require.NoError(t, storage.PutData(ctx, "ck", []byte("cover"), 0))
	require.NoError(t, storage.PutData(ctx, "pk", []byte("poster"), 0))

	saveDir := t.TempDir()
	movieFile := filepath.Join(t.TempDir(), "ABC-123.mp4")
	require.NoError(t, os.WriteFile(movieFile, []byte("movie"), 0o600))

	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(storage),
		WithNamingRule("{MOVIEID}"),
	)
	require.NoError(t, err)
	fc := &model.FileContext{
		FullFilePath: movieFile,
		FileName:     "ABC-123.mp4",
		FileExt:      ".mp4",
		SaveFileBase: "ABC-123",
		Number:       num,
		Meta: &model.MovieMeta{
			Title:  "Test",
			Number: "ABC-123",
			Cover:  &model.File{Name: "cover.jpg", Key: "ck"},
			Poster: &model.File{Name: "poster.jpg", Key: "pk"},
		},
	}
	err = capt.ImportMeta(ctx, fc)
	assert.NoError(t, err)
}

func TestImportMetaFailsOnVerify(t *testing.T) {
	ctx := context.Background()
	num, _ := number.Parse("ABC-123")

	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	fc := &model.FileContext{
		Number: num,
		Meta:   &model.MovieMeta{},
	}
	err := capt.ImportMeta(ctx, fc)
	assert.Error(t, err)
}

func TestProcessFileList(t *testing.T) {
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
		err := capt.processFileList(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("file processing error logged but continues", func(t *testing.T) {
		capt, err := New(
			WithScanDir(t.TempDir()),
			WithSaveDir(t.TempDir()),
			WithSeacher(&metaSearcher{ok: false}),
			WithStorage(store.NewMemStorage()),
		)
		require.NoError(t, err)
		num, _ := number.Parse("ABC-123")
		fcs := []*model.FileContext{
			{Number: num, FileName: "test1.mp4", FullFilePath: "/tmp/test1.mp4"},
		}
		err = capt.processFileList(ctx, fcs)
		assert.Error(t, err)
	})
}

func TestDisplayNumberInfo(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	num, _ := number.Parse("ABC-123")
	fcs := []*model.FileContext{
		{Number: num, FileName: "test.mp4"},
	}
	capt.displayNumberInfo(context.Background(), fcs)
}

func TestRun(t *testing.T) {
	scanDir := t.TempDir()
	saveDir := t.TempDir()
	storage := store.NewMemStorage()
	ctx := context.Background()
	require.NoError(t, storage.PutData(ctx, "ck", []byte("cover"), 0))
	require.NoError(t, storage.PutData(ctx, "pk", []byte("poster"), 0))
	require.NoError(t, os.WriteFile(filepath.Join(scanDir, "ABC-123.mp4"), []byte("movie"), 0o600))

	meta := &model.MovieMeta{
		Title:       "Test",
		Number:      "ABC-123",
		Cover:       &model.File{Name: "cover.jpg", Key: "ck"},
		Poster:      &model.File{Name: "poster.jpg", Key: "pk"},
		ReleaseDate: time.Now().UnixMilli(),
	}

	capt, err := New(
		WithScanDir(scanDir),
		WithSaveDir(saveDir),
		WithSeacher(&metaSearcher{meta: meta, ok: true}),
		WithStorage(storage),
		WithNamingRule("{MOVIEID}"),
	)
	require.NoError(t, err)
	err = capt.Run(ctx)
	assert.NoError(t, err)
}

func TestDoNamingReturnsErrorOnBadNaming(t *testing.T) {
	ctx := context.Background()
	capt := &Capture{
		c: &config{
			SaveDir: t.TempDir(),
			Naming:  "",
		},
	}
	num, err := number.Parse("ABC-123")
	require.NoError(t, err)
	fc := &model.FileContext{
		SaveFileBase: "ABC-123",
		Number:       num,
		Meta:         &model.MovieMeta{},
	}
	err = capt.doNaming(ctx, fc)
	assert.ErrorIs(t, err, errInvalidNaming)
}

func TestDoSaveDataError(t *testing.T) {
	ctx := context.Background()
	storage := store.NewMemStorage()
	saveDir := t.TempDir()

	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(storage),
	)
	require.NoError(t, err)
	fc := &model.FileContext{
		SaveDir:      saveDir,
		SaveFileBase: "ABC-123",
		FileExt:      ".mp4",
		Meta: &model.MovieMeta{
			Cover: &model.File{Name: "cover.jpg", Key: "nokey"},
		},
	}
	err = capt.doSaveData(ctx, fc)
	assert.Error(t, err)
}

func TestDoExportError(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	fc := &model.FileContext{
		SaveDir:      "/nonexistent/path/for/write",
		SaveFileBase: "test",
		Meta:         &model.MovieMeta{Title: "t", Number: "N-1"},
	}
	err := capt.doExport(context.Background(), fc)
	assert.Error(t, err)
}

func TestSaveMediaDataNilCoverPoster(t *testing.T) {
	ctx := context.Background()
	saveDir := t.TempDir()
	movieDir := filepath.Join(saveDir, "test")
	require.NoError(t, os.MkdirAll(movieDir, 0o755))
	srcFile := filepath.Join(t.TempDir(), "test.mp4")
	require.NoError(t, os.WriteFile(srcFile, []byte("movie"), 0o600))

	storage := store.NewMemStorage()
	capt, err := New(
		WithScanDir(t.TempDir()),
		WithSaveDir(saveDir),
		WithSeacher(&testSearcher{}),
		WithStorage(storage),
	)
	require.NoError(t, err)
	fc := &model.FileContext{
		FullFilePath: srcFile,
		SaveDir:      movieDir,
		SaveFileBase: "test",
		FileExt:      ".mp4",
		Meta:         &model.MovieMeta{},
	}
	err = capt.saveMediaData(ctx, fc)
	assert.NoError(t, err)
}

func TestMoveMovieByLinkError(t *testing.T) {
	capt := newTestCapture(t, movieidcleaner.NewPassthroughCleaner())
	err := capt.moveMovieByLink(nil, "/tmp/src", "/nonexistent/dir/link")
	assert.Error(t, err)
}

func TestRunReadFileListError(t *testing.T) {
	capt, err := New(
		WithScanDir("/nonexistent/path/that/does/not/exist"),
		WithSaveDir(t.TempDir()),
		WithSeacher(&testSearcher{}),
		WithStorage(store.NewMemStorage()),
	)
	require.NoError(t, err)
	err = capt.Run(context.Background())
	assert.Error(t, err)
}
