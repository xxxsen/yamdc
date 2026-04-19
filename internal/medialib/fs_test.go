package medialib

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/nfo"
	"github.com/xxxsen/yamdc/internal/repository"
)

func newTestService(t *testing.T) (*Service, string, string) { //nolint:unparam // 签名由接口 / 测试期望固定
	t.Helper()
	libraryDir := t.TempDir()
	saveDir := t.TempDir()
	sqlite, err := repository.NewSQLite(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlite.Close() })
	svc := NewService(sqlite.DB(), libraryDir, saveDir)
	return svc, libraryDir, saveDir
}

func writeNFO(t *testing.T, dir, stem string, mov *nfo.Movie) {
	t.Helper()
	require.NoError(t, nfo.WriteMovieToFile(filepath.Join(dir, stem+".nfo"), mov))
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	require.NoError(t, png.Encode(f, img))
}

// --- firstNonEmpty ---

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"all_empty", []string{"", "  ", ""}, ""},
		{"first_wins", []string{"a", "b"}, "a"},
		{"skip_blank", []string{"", " ", "c"}, "c"},
		{"single", []string{"only"}, "only"},
		{"no_values", nil, ""},
		{"whitespace_trimmed", []string{"  x  "}, "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, firstNonEmpty(tc.values...))
		})
	}
}

// --- trimStrings ---

func TestTrimStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"normal", []string{"a", "b"}, []string{"a", "b"}},
		{"filter_blank", []string{"a", "", "  ", "b"}, []string{"a", "b"}},
		{"nil_input", nil, []string{}},
		{"all_blank", []string{"", "  "}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, trimStrings(tc.input))
		})
	}
}

// --- actorNames ---

func TestActorNames(t *testing.T) {
	tests := []struct {
		name  string
		input []nfo.Actor
		want  []string
	}{
		{"normal", []nfo.Actor{{Name: "Alice"}, {Name: "Bob"}}, []string{"Alice", "Bob"}},
		{"skip_blank", []nfo.Actor{{Name: ""}, {Name: " "}, {Name: "Eve"}}, []string{"Eve"}},
		{"empty", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, actorNames(tc.input))
		})
	}
}

// --- makeActors ---

func TestMakeActors(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []nfo.Actor
	}{
		{"normal", []string{"Alice", "Bob"}, []nfo.Actor{{Name: "Alice"}, {Name: "Bob"}}},
		{"filter_blank", []string{"Alice", "", " "}, []nfo.Actor{{Name: "Alice"}}},
		{"empty", nil, []nfo.Actor{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, makeActors(tc.input))
		})
	}
}

// --- detectFileKind ---

func TestDetectFileKind(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{"nfo", "movie.nfo", "nfo"},
		{"poster_jpg", "abc-poster.jpg", "poster"},
		{"fanart_jpg", "abc-fanart.jpg", "cover"},
		{"cover_png", "abc-cover.png", "cover"},
		{"thumb_jpg", "abc-thumb.jpg", "cover"},
		{"video_mp4", "movie.mp4", "video"},
		{"video_mkv", "movie.mkv", "video"},
		{"image_jpg", "random.jpg", "image"},
		{"image_png", "random.png", "image"},
		{"unknown", "readme.txt", "file"},
		{"strm", "movie.strm", "video"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectFileKind(tc.file))
		})
	}
}

// --- detectArtworkPath ---

func TestDetectArtworkPath(t *testing.T) {
	tests := []struct {
		name   string
		relDir string
		images []string
		kind   string
		want   string
	}{
		{"poster_found", "item", []string{"abc-poster.jpg", "abc-fanart.jpg"}, "poster", "item/abc-poster.jpg"},
		{"fanart_found", "item", []string{"abc-poster.jpg", "abc-fanart.jpg"}, "fanart", "item/abc-fanart.jpg"},
		{"cover_match", "item", []string{"abc-cover.jpg"}, "fanart", "item/abc-cover.jpg"},
		{"fallback_first", "item", []string{"random.jpg"}, "poster", "item/random.jpg"},
		{"no_images", "item", nil, "poster", ""},
		{"empty_images", "item", []string{}, "fanart", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectArtworkPath(tc.relDir, tc.images, tc.kind))
		})
	}
}

// --- splitPlot ---

func TestSplitPlot(t *testing.T) {
	tests := []struct {
		name           string
		plot           string
		plotTranslated string
		wantPlot       string
		wantTranslated string
	}{
		{"both_provided", "original", "translated", "original", "translated"},
		{"marker_split", "original plot [翻译:translated text]", "", "original plot", "translated text"},
		{"no_marker", "just a plot", "", "just a plot", ""},
		{"empty", "", "", "", ""},
		{"marker_no_bracket_suffix", "text [翻译:partial", "", "text [翻译:partial", ""},
		{"translated_whitespace", "  plot  ", "  trans  ", "plot", "trans"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, pt := splitPlot(tc.plot, tc.plotTranslated)
			assert.Equal(t, tc.wantPlot, p)
			assert.Equal(t, tc.wantTranslated, pt)
		})
	}
}

// --- selectPrimaryVariant ---

func TestSelectPrimaryVariant(t *testing.T) {
	tests := []struct {
		name    string
		keys    []string
		dirBase string
		want    string
	}{
		{"empty_keys", nil, "dir", ""},
		{"match_dir_base", []string{"ABC-123", "dir"}, "dir", "dir"},
		{"shortest_key", []string{"ABC-123-CD2", "ABC-123"}, "other", "ABC-123"},
		{"same_length_alpha", []string{"BBB", "AAA"}, "other", "AAA"},
		{"single", []string{"only"}, "other", "only"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, selectPrimaryVariant(tc.keys, tc.dirBase))
		})
	}
}

// --- variantSuffix ---

func TestVariantSuffix(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		primaryKey string
		dirBase    string
		want       string
	}{
		{"primary_prefix", "ABC-123-CD2", "ABC-123", "dir", "CD2"},
		{"dir_prefix", "dir-extra", "", "dir", "extra"},
		{"exact_primary", "ABC-123", "ABC-123", "dir", ""},
		{"exact_dir", "dir", "", "dir", ""},
		{"no_match", "OTHER", "ABC-123", "dir", "OTHER"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, variantSuffix(tc.key, tc.primaryKey, tc.dirBase))
		})
	}
}

// --- variantLabel ---

func TestVariantLabel(t *testing.T) {
	tests := []struct {
		name      string
		suffix    string
		isPrimary bool
		want      string
	}{
		{"primary_no_suffix", "", true, "原始文件"},
		{"non_primary_no_suffix", "", false, "实例"},
		{"with_suffix", "cd2", false, "CD2"},
		{"whitespace_primary", "  ", true, "原始文件"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, variantLabel(tc.suffix, tc.isPrimary))
		})
	}
}

// --- selectVariantNFOPath ---

func TestSelectVariantNFOPath(t *testing.T) {
	abs := "/lib/item"
	tests := []struct {
		name       string
		variant    Variant
		primaryKey string
		want       string
	}{
		{"has_nfo_abs_path", Variant{NFOAbsPath: "/other/x.nfo"}, "key", "/other/x.nfo"},
		{"base_name", Variant{BaseName: "ABC-123"}, "key", filepath.Join(abs, "ABC-123.nfo")},
		{"primary_key", Variant{}, "pk", filepath.Join(abs, "pk.nfo")},
		{"fallback_dir", Variant{}, "", filepath.Join(abs, "item.nfo")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, selectVariantNFOPath(abs, tc.variant, tc.primaryKey))
		})
	}
}

// --- resolveMovieAssetPath ---

func TestResolveMovieAssetPath(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		relDir string
		raw    string
		want   string
	}{
		{"normal", "/root", "item", "poster.jpg", "item/poster.jpg"},
		{"empty_raw", "/root", "item", "", ""},
		{"blank_raw", "/root", "item", "   ", ""},
		{"dot_only", "/root", "item", ".", ""},
		{"traversal", "/root", "item", "../../etc/passwd", ""},
		{"leading_slash", "/root", "item", "/poster.jpg", "item/poster.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveMovieAssetPath(tc.root, tc.relDir, tc.raw))
		})
	}
}

// --- preserveAssetValue ---

func TestPreserveAssetValue(t *testing.T) {
	tests := []struct {
		name    string
		current string
		relPath string
		relDir  string
		want    string
	}{
		{"current_non_empty", "existing", "item/poster.jpg", "item", "existing"},
		{"empty_relpath", "", "", "item", ""},
		{"strip_prefix", "", "item/poster.jpg", "item", "poster.jpg"},
		{"fallback_base", "", "other/poster.jpg", "item", "poster.jpg"},
		{"blank_current", "  ", "item/file.jpg", "item", "file.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, preserveAssetValue(tc.current, tc.relPath, tc.relDir))
		})
	}
}

// --- findVariant ---

func TestFindVariant(t *testing.T) {
	variants := []Variant{{Key: "a"}, {Key: "b"}}
	tests := []struct {
		name    string
		key     string
		wantOK  bool
		wantKey string
	}{
		{"found", "a", true, "a"},
		{"not_found", "c", false, ""},
		{"empty_list_key", "", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := findVariant(variants, tc.key)
			assert.Equal(t, tc.wantOK, ok)
			if ok {
				assert.Equal(t, tc.wantKey, v.Key)
			}
		})
	}
}

// --- pickVariant ---

func TestPickVariant(t *testing.T) {
	tests := []struct {
		name    string
		detail  *Detail
		key     string
		wantOK  bool
		wantKey string
	}{
		{"nil_detail", nil, "a", false, ""},
		{"exact_match", &Detail{
			Variants:          []Variant{{Key: "a"}, {Key: "b"}},
			PrimaryVariantKey: "b",
		}, "a", true, "a"},
		{"fallback_primary", &Detail{
			Variants:          []Variant{{Key: "a"}, {Key: "b"}},
			PrimaryVariantKey: "b",
		}, "missing", true, "b"},
		{"fallback_first", &Detail{
			Variants:          []Variant{{Key: "a"}},
			PrimaryVariantKey: "missing",
		}, "", true, "a"},
		{"empty_variants", &Detail{
			Variants:          nil,
			PrimaryVariantKey: "",
		}, "", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := pickVariant(tc.detail, tc.key)
			assert.Equal(t, tc.wantOK, ok)
			if ok {
				assert.Equal(t, tc.wantKey, v.Key)
			}
		})
	}
}

// --- cloneVariant ---

func TestCloneVariant(t *testing.T) {
	t.Run("nil_src", func(t *testing.T) {
		v := cloneVariant(nil)
		assert.Equal(t, Variant{}, v)
	})
	t.Run("deep_copy", func(t *testing.T) {
		src := &Variant{
			Key:      "k",
			BaseName: "base",
			Meta:     Meta{Actors: []string{"A"}, Genres: []string{"G"}},
			Files:    []FileItem{{Name: "f1"}},
		}
		clone := cloneVariant(src)
		assert.Equal(t, src.Key, clone.Key)
		assert.Equal(t, src.Meta.Actors, clone.Meta.Actors)
		src.Meta.Actors[0] = "CHANGED"
		assert.NotEqual(t, src.Meta.Actors[0], clone.Meta.Actors[0])
		src.Files[0].Name = "CHANGED"
		assert.NotEqual(t, src.Files[0].Name, clone.Files[0].Name)
	})
}

// --- cloneMeta ---

func TestCloneMeta(t *testing.T) {
	src := Meta{
		Title:  "T",
		Actors: []string{"A"},
		Genres: []string{"G"},
	}
	clone := cloneMeta(src)
	assert.Equal(t, src.Title, clone.Title)
	src.Actors[0] = "X"
	assert.NotEqual(t, src.Actors[0], clone.Actors[0])
}

// --- trimMetaFields ---

func TestTrimMetaFields(t *testing.T) {
	meta := Meta{
		Title:           "  title  ",
		TitleTranslated: "  tt  ",
		OriginalTitle:   "  ot  ",
		Plot:            "  plot  ",
		PlotTranslated:  "  pt  ",
		Number:          "  N  ",
		ReleaseDate:     " 2024 ",
		Studio:          " S ",
		Label:           " L ",
		Series:          " Se ",
		Director:        " D ",
		Source:          " src ",
		ScrapedAt:       " sa ",
	}
	trimMetaFields(&meta)
	assert.Equal(t, "title", meta.Title)
	assert.Equal(t, "tt", meta.TitleTranslated)
	assert.Equal(t, "ot", meta.OriginalTitle)
	assert.Equal(t, "plot", meta.Plot)
	assert.Equal(t, "pt", meta.PlotTranslated)
	assert.Equal(t, "N", meta.Number)
	assert.Equal(t, "2024", meta.ReleaseDate)
	assert.Equal(t, "S", meta.Studio)
	assert.Equal(t, "L", meta.Label)
	assert.Equal(t, "Se", meta.Series)
	assert.Equal(t, "D", meta.Director)
	assert.Equal(t, "src", meta.Source)
	assert.Equal(t, "sa", meta.ScrapedAt)
}

// --- applyMetaToMovie ---

func TestApplyMetaToMovie(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		mov := &nfo.Movie{}
		meta := Meta{
			Title:           "Original",
			TitleTranslated: "Translated",
			Number:          "NUM-001",
			ReleaseDate:     "2024-05-10",
			Runtime:         120,
			Studio:          "Studio",
			Label:           "Label",
			Series:          "Series",
			Director:        "Dir",
			Actors:          []string{"A1"},
			Genres:          []string{"G1"},
			Source:          "web",
		}
		applyMetaToMovie(meta, mov)
		assert.Equal(t, "Translated", mov.Title)
		assert.Equal(t, "Original", mov.OriginalTitle)
		assert.Equal(t, "Translated", mov.TitleTranslated)
		assert.Equal(t, "NUM-001", mov.ID)
		assert.Equal(t, "2024-05-10", mov.ReleaseDate)
		assert.Equal(t, 2024, mov.Year)
		assert.Equal(t, uint64(120), mov.Runtime)
		assert.Equal(t, "Studio", mov.Studio)
		assert.Equal(t, "Label", mov.Label)
		assert.Equal(t, "Series", mov.Set)
		assert.Equal(t, "Dir", mov.Director)
		assert.Len(t, mov.Actors, 1)
		assert.Equal(t, "A1", mov.Actors[0].Name)
		assert.Equal(t, []string{"G1"}, mov.Genres)
		assert.Equal(t, "web", mov.ScrapeInfo.Source)
	})
	t.Run("no_translated_title", func(t *testing.T) {
		mov := &nfo.Movie{}
		applyMetaToMovie(Meta{Title: "Base"}, mov)
		assert.Equal(t, "Base", mov.Title)
		assert.Equal(t, "Base", mov.OriginalTitle)
	})
	t.Run("invalid_year", func(t *testing.T) {
		mov := &nfo.Movie{}
		applyMetaToMovie(Meta{ReleaseDate: "abc"}, mov)
		assert.Equal(t, 0, mov.Year)
	})
	t.Run("short_date_parses", func(t *testing.T) {
		mov := &nfo.Movie{}
		applyMetaToMovie(Meta{ReleaseDate: "20"}, mov)
		assert.Equal(t, 20, mov.Year)
	})
}

// --- libraryMetaFromMovie ---

func TestLibraryMetaFromMovie(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		mov := &nfo.Movie{
			Title:         "Translated",
			OriginalTitle: "Original",
			Plot:          "a plot [翻译:translated plot]",
			ID:            "ABC-123",
			ReleaseDate:   "2024-01-02",
			Runtime:       90,
			Studio:        "S",
			Label:         "L",
			Set:           "Set",
			Director:      "D",
			Poster:        "poster.jpg",
			Cover:         "cover.jpg",
			Fanart:        "fanart.jpg",
			Thumb:         "thumb.jpg",
			Actors:        []nfo.Actor{{Name: "A"}},
			Genres:        []string{"G"},
			ScrapeInfo:    nfo.ScrapeInfo{Source: "src", Date: "2024-01-01"},
		}
		meta := libraryMetaFromMovie("/root", "item", mov)
		assert.Equal(t, "Original", meta.Title)
		assert.Equal(t, "Translated", meta.TitleTranslated)
		assert.Equal(t, "ABC-123", meta.Number)
		assert.Equal(t, "a plot", meta.Plot)
		assert.Equal(t, "translated plot", meta.PlotTranslated)
		assert.Equal(t, "item/poster.jpg", meta.PosterPath)
		assert.Equal(t, "item/cover.jpg", meta.CoverPath)
	})
	t.Run("fallback_cover_from_art_fanart", func(t *testing.T) {
		mov := &nfo.Movie{
			Title: "T",
			Art:   nfo.Art{Fanart: []string{"art_fanart.jpg"}},
		}
		meta := libraryMetaFromMovie("/root", "item", mov)
		assert.Equal(t, "item/art_fanart.jpg", meta.CoverPath)
	})
	t.Run("title_translated_deduced", func(t *testing.T) {
		mov := &nfo.Movie{
			Title:         "TranslatedTitle",
			OriginalTitle: "OriginalTitle",
		}
		meta := libraryMetaFromMovie("/root", "item", mov)
		assert.Equal(t, "TranslatedTitle", meta.TitleTranslated)
		assert.Equal(t, "OriginalTitle", meta.OriginalTitle)
	})
	t.Run("title_same_as_original", func(t *testing.T) {
		mov := &nfo.Movie{
			Title:         "Same",
			OriginalTitle: "Same",
		}
		meta := libraryMetaFromMovie("/root", "item", mov)
		assert.Equal(t, "", meta.TitleTranslated)
	})
	t.Run("release_date_fallbacks", func(t *testing.T) {
		mov := &nfo.Movie{Premiered: "2023-06-01"}
		meta := libraryMetaFromMovie("/root", "item", mov)
		assert.Equal(t, "2023-06-01", meta.ReleaseDate)
	})
}

// --- collectVariantEntries ---

func TestCollectVariantEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-123.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-123.nfo"), []byte("<movie/>"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ABC-123-poster.jpg"), []byte("img"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	variantsByKey, keys, topFiles := collectVariantEntries(entries, "item", dir)
	assert.Contains(t, keys, "ABC-123")
	assert.NotNil(t, variantsByKey["ABC-123"])
	assert.Equal(t, "item/ABC-123.mp4", variantsByKey["ABC-123"].VideoPath)
	assert.Equal(t, "item/ABC-123.nfo", variantsByKey["ABC-123"].NFOPath)
	assert.Len(t, topFiles, 3)
}

// --- matchImageFilesToVariants / assignImageToVariant ---

func TestAssignImageToVariant(t *testing.T) {
	tests := []struct {
		name       string
		stem       string
		relPath    string
		wantPoster string
		wantCover  string
	}{
		{"poster", "ABC-123-poster", "item/ABC-123-poster.jpg", "item/ABC-123-poster.jpg", ""},
		{"fanart", "ABC-123-fanart", "item/ABC-123-fanart.jpg", "", "item/ABC-123-fanart.jpg"},
		{"cover", "ABC-123-cover", "item/ABC-123-cover.jpg", "", "item/ABC-123-cover.jpg"},
		{"thumb", "ABC-123-thumb", "item/ABC-123-thumb.jpg", "", "item/ABC-123-thumb.jpg"},
		{"unrelated", "random", "item/random.jpg", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &Variant{Key: "ABC-123"}
			variantsByKey := map[string]*Variant{"ABC-123": v}
			assignImageToVariant(tc.stem, tc.relPath, []string{"ABC-123"}, variantsByKey)
			assert.Equal(t, tc.wantPoster, v.PosterPath)
			assert.Equal(t, tc.wantCover, v.CoverPath)
		})
	}
}

// --- attachFilesToVariants ---

func TestAttachFilesToVariants(t *testing.T) {
	variants := []Variant{
		{Key: "A", Label: "lA", VideoPath: "item/A.mp4", NFOPath: "item/A.nfo"},
		{Key: "B", Label: "lB", VideoPath: "item/B.mp4"},
	}
	files := []FileItem{
		{RelPath: "item/A.mp4", Name: "A.mp4"},
		{RelPath: "item/A.nfo", Name: "A.nfo"},
		{RelPath: "item/B.mp4", Name: "B.mp4"},
		{RelPath: "item/unrelated.txt", Name: "unrelated.txt"},
	}
	resultV, resultF := attachFilesToVariants(variants, files)
	assert.Equal(t, 2, resultV[0].FileCount)
	assert.Equal(t, 1, resultV[1].FileCount)
	assert.Equal(t, "A", resultF[0].VariantKey)
	assert.Equal(t, "", resultF[3].VariantKey)
}

// --- resolveRootPath ---

func TestResolveRootPath(t *testing.T) {
	svc := &Service{}
	tests := []struct {
		name    string
		root    string
		raw     string
		wantRel string
		wantErr bool
	}{
		{"normal", "/root", "item", "item", false},
		{"empty_root", "", "item", "", true},
		{"empty_raw", "/root", "", "", true},
		{"blank_root", "  ", "item", "", true},
		{"blank_raw", "/root", "  ", "", true},
		{"traversal", "/root", "../../etc", "", true},
		{"dot_only", "/root", ".", "", true},
		{"dotdot", "/root", "..", "", true},
		{"leading_slash", "/root", "/sub/item", "sub/item", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rel, _, err := svc.resolveRootPath(tc.root, tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantRel, rel)
			}
		})
	}
}

// --- scanDirEntries ---

func TestScanDirEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "movie.mp4"), []byte("video"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte("<movie/>"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "poster.jpg"), []byte("img"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	result := scanDirEntries(dir, entries, 0)
	assert.True(t, result.hasNFO)
	assert.Equal(t, 1, result.videoCount)
	assert.Equal(t, 3, result.fileCount)
	assert.Contains(t, result.imageNames, "poster.jpg")
	assert.True(t, result.totalSize > 0)
}

// --- listRootItemDirs ---

func TestListRootItemDirs(t *testing.T) {
	svc, libraryDir, _ := newTestService(t)

	t.Run("empty_root", func(t *testing.T) {
		svc2 := &Service{}
		dirs, err := svc2.listRootItemDirs("")
		require.NoError(t, err)
		assert.Empty(t, dirs)
	})

	t.Run("nonexistent_root", func(t *testing.T) {
		dirs, err := svc.listRootItemDirs("/nonexistent/path")
		require.NoError(t, err)
		assert.Empty(t, dirs)
	})

	t.Run("with_item_dirs", func(t *testing.T) {
		item1 := filepath.Join(libraryDir, "movie1")
		require.NoError(t, os.MkdirAll(item1, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item1, "movie1.mp4"), []byte("v"), 0o600))
		item2 := filepath.Join(libraryDir, "movie2")
		require.NoError(t, os.MkdirAll(item2, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item2, "movie2.nfo"), []byte("<movie/>"), 0o600))
		emptyDir := filepath.Join(libraryDir, "emptydir")
		require.NoError(t, os.MkdirAll(emptyDir, 0o755))

		dirs, err := svc.listRootItemDirs(libraryDir)
		require.NoError(t, err)
		assert.Len(t, dirs, 2)
	})

	t.Run("extrafanart_skipped", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "m.mp4"), []byte("v"), 0o600))
		efDir := filepath.Join(item, "extrafanart")
		require.NoError(t, os.MkdirAll(efDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(efDir, "f.mp4"), []byte("v"), 0o600))

		dirs, err := svc.listRootItemDirs(root)
		require.NoError(t, err)
		assert.Len(t, dirs, 1)
	})
}

// --- inspectRootDir ---

func TestInspectRootDir(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("normal_with_nfo", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		writeNFO(t, item, "movie", &nfo.Movie{Title: "Title", ID: "NUM-001", ReleaseDate: "2024-01-02"})
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("video"), 0o600))

		it, ok, err := svc.inspectRootDir(root, item)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "movie", it.RelPath)
		assert.True(t, it.HasNFO)
		assert.Equal(t, 1, it.VideoCount)
	})

	t.Run("no_nfo_no_video", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "empty")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "readme.txt"), []byte("hi"), 0o600))

		_, ok, err := svc.inspectRootDir(root, item)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

// --- readRootDetail ---

func TestReadRootDetail(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("normal", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		writeNFO(t, item, "movie", &nfo.Movie{
			Title:         "Title",
			OriginalTitle: "Original",
			ID:            "NUM-001",
			ReleaseDate:   "2024-01-02",
			Poster:        "poster.jpg",
			Cover:         "cover.jpg",
			Actors:        []nfo.Actor{{Name: "A"}},
		})
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

		detail, err := svc.readRootDetail(root, "movie", item)
		require.NoError(t, err)
		assert.Equal(t, "movie", detail.Item.RelPath)
		assert.Equal(t, "NUM-001", detail.Meta.Number)
		assert.True(t, len(detail.Variants) > 0)
		assert.True(t, len(detail.Files) > 0)
	})

	t.Run("not_found", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "empty")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "readme.txt"), []byte("hi"), 0o600))

		_, err := svc.readRootDetail(root, "empty", item)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

// --- listRootFiles ---

func TestListRootFiles(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "a.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(item, "b.nfo"), []byte("<movie/>"), 0o600))

	files, err := svc.listRootFiles(root, item)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.True(t, files[0].Size > 0)
}

// --- writeVariantNFO ---

func TestWriteVariantNFO(t *testing.T) {
	t.Run("new_nfo", func(t *testing.T) {
		dir := t.TempDir()
		variant := Variant{Key: "ABC", BaseName: "ABC"}
		meta := Meta{Title: "T", Number: "ABC", Actors: []string{"A1"}, Genres: []string{"G1"}}
		err := writeVariantNFO(dir, "item", variant, "ABC", meta)
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "ABC.nfo"))
	})
	t.Run("existing_nfo", func(t *testing.T) {
		dir := t.TempDir()
		nfoPath := filepath.Join(dir, "ABC.nfo")
		require.NoError(t, nfo.WriteMovieToFile(nfoPath, &nfo.Movie{Title: "Old", Poster: "old.jpg"}))
		variant := Variant{Key: "ABC", BaseName: "ABC", NFOAbsPath: nfoPath}
		meta := Meta{Title: "New", Number: "ABC"}
		err := writeVariantNFO(dir, "item", variant, "ABC", meta)
		require.NoError(t, err)
		mov, err := nfo.ParseMovie(nfoPath)
		require.NoError(t, err)
		assert.Equal(t, "New", mov.Title)
		assert.Equal(t, "old.jpg", mov.Poster)
	})
}

// --- updateRootItem ---

func TestUpdateRootItem(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		detail := &Detail{
			Item:              Item{RelPath: "movie", Number: "movie", Name: "movie"},
			PrimaryVariantKey: "movie",
			Variants:          []Variant{{Key: "movie", BaseName: "movie", IsPrimary: true}},
		}
		next, err := svc.updateRootItem(root, detail, item, Meta{Title: "New"})
		require.NoError(t, err)
		assert.NotNil(t, next)
	})
	t.Run("not_dir", func(t *testing.T) {
		root := t.TempDir()
		f := filepath.Join(root, "file.txt")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
		_, err := svc.updateRootItem(root, &Detail{}, f, Meta{})
		assert.ErrorIs(t, err, errLibraryItemNotDir)
	})
	t.Run("nil_detail", func(t *testing.T) {
		dir := t.TempDir()
		_, err := svc.updateRootItem(dir, nil, dir, Meta{})
		assert.ErrorIs(t, err, errLibraryDetailRequired)
	})
	t.Run("empty_variants_creates_default", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		detail := &Detail{
			Item:              Item{RelPath: "movie", Number: "NUM", Name: "movie"},
			PrimaryVariantKey: "pk",
		}
		next, err := svc.updateRootItem(root, detail, item, Meta{Title: "T"})
		require.NoError(t, err)
		assert.NotNil(t, next)
	})
}

// --- validateDirDetail ---

func TestValidateDirDetail(t *testing.T) {
	t.Run("not_exist", func(t *testing.T) {
		err := validateDirDetail("/nonexistent", &Detail{})
		assert.Error(t, err)
	})
	t.Run("not_dir", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
		err := validateDirDetail(f, &Detail{})
		assert.ErrorIs(t, err, errLibraryItemNotDir)
	})
	t.Run("nil_detail", func(t *testing.T) {
		dir := t.TempDir()
		err := validateDirDetail(dir, nil)
		assert.ErrorIs(t, err, errLibraryDetailRequired)
	})
	t.Run("ok", func(t *testing.T) {
		dir := t.TempDir()
		err := validateDirDetail(dir, &Detail{})
		assert.NoError(t, err)
	})
}

// --- pickArtworkTargetName ---

func TestPickArtworkTargetName(t *testing.T) {
	tests := []struct {
		name    string
		detail  *Detail
		variant Variant
		kind    string
		ext     string
		want    string
	}{
		{
			"poster_from_variant",
			&Detail{Item: Item{RelPath: "item"}},
			Variant{PosterPath: "item/poster.jpg"},
			"poster", ".jpg",
			"poster.jpg",
		},
		{
			"cover_from_variant_meta",
			&Detail{Item: Item{RelPath: "item"}},
			Variant{Meta: Meta{CoverPath: "item/cover.jpg"}},
			"cover", ".jpg",
			"cover.jpg",
		},
		{
			"poster_from_detail",
			&Detail{Item: Item{RelPath: "item"}, Meta: Meta{PosterPath: "item/detail-poster.jpg"}},
			Variant{},
			"poster", ".jpg",
			"detail-poster.jpg",
		},
		{
			"cover_from_detail",
			&Detail{Item: Item{RelPath: "item"}, Meta: Meta{CoverPath: "item/detail-cover.jpg"}},
			Variant{},
			"cover", ".jpg",
			"detail-cover.jpg",
		},
		{
			"poster_fallback_generate",
			&Detail{Item: Item{RelPath: "item", Number: "NUM", Name: "movie"}},
			Variant{BaseName: "NUM"},
			"poster", ".png",
			"NUM-poster.png",
		},
		{
			"cover_fallback_generate",
			&Detail{Item: Item{RelPath: "item", Number: "NUM", Name: "movie"}},
			Variant{BaseName: "NUM"},
			"cover", ".png",
			"NUM-fanart.png",
		},
		{
			"nil_detail_fallback_generate",
			nil,
			Variant{BaseName: "NUM"},
			"poster", ".jpg",
			"NUM-poster.jpg",
		},
		{
			"current_path_outside_relpath",
			&Detail{Item: Item{RelPath: "item"}},
			Variant{PosterPath: "other/poster.jpg"},
			"poster", ".jpg",
			"poster.jpg",
		},
		{
			"cover_detail_fallback_fanart",
			&Detail{Item: Item{RelPath: "item"}, Meta: Meta{FanartPath: "item/fanart.jpg"}},
			Variant{},
			"cover", ".jpg",
			"fanart.jpg",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := pickArtworkTargetName(tc.detail, tc.variant, tc.kind, tc.ext)
			assert.Equal(t, tc.want, result)
		})
	}
}

// --- pickFanartTargetName ---

func TestPickFanartTargetName(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		dir := t.TempDir()
		name, err := pickFanartTargetName(dir, "extra.jpg")
		require.NoError(t, err)
		assert.Equal(t, "extrafanart/extra.jpg", name)
	})
	t.Run("collision_increments", func(t *testing.T) {
		dir := t.TempDir()
		efDir := filepath.Join(dir, "extrafanart")
		require.NoError(t, os.MkdirAll(efDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(efDir, "extra.jpg"), []byte("x"), 0o600))
		name, err := pickFanartTargetName(dir, "extra.jpg")
		require.NoError(t, err)
		assert.Equal(t, "extrafanart/extra-2.jpg", name)
	})
	t.Run("non_image_ext", func(t *testing.T) {
		dir := t.TempDir()
		name, err := pickFanartTargetName(dir, "file.txt")
		require.NoError(t, err)
		assert.Equal(t, "extrafanart/file.jpg", name)
	})
	t.Run("empty_base_name", func(t *testing.T) {
		dir := t.TempDir()
		name, err := pickFanartTargetName(dir, ".png")
		require.NoError(t, err)
		assert.Equal(t, "extrafanart/fanart.png", name)
	})
	t.Run("slash_in_name", func(t *testing.T) {
		dir := t.TempDir()
		name, err := pickFanartTargetName(dir, "a/b\\c.jpg")
		require.NoError(t, err)
		assert.Contains(t, name, "extrafanart/")
	})
}

// --- writeVariantArtwork ---

func TestWriteVariantArtwork(t *testing.T) {
	t.Run("poster", func(t *testing.T) {
		dir := t.TempDir()
		detail := &Detail{
			Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
			PrimaryVariantKey: "NUM",
		}
		variant := Variant{Key: "NUM", BaseName: "NUM"}
		err := writeVariantArtwork(dir, detail, variant, "poster", "file.jpg", []byte("imgdata"))
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "NUM-poster.jpg"))
		assert.FileExists(t, filepath.Join(dir, "NUM.nfo"))
	})
	t.Run("cover", func(t *testing.T) {
		dir := t.TempDir()
		detail := &Detail{
			Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
			PrimaryVariantKey: "NUM",
		}
		variant := Variant{Key: "NUM", BaseName: "NUM"}
		err := writeVariantArtwork(dir, detail, variant, "cover", "file.png", []byte("imgdata"))
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "NUM-fanart.png"))
	})
	t.Run("unknown_ext_defaults_jpg", func(t *testing.T) {
		dir := t.TempDir()
		detail := &Detail{
			Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
			PrimaryVariantKey: "NUM",
		}
		variant := Variant{Key: "NUM", BaseName: "NUM"}
		err := writeVariantArtwork(dir, detail, variant, "poster", "file.bmp2", []byte("imgdata"))
		require.NoError(t, err)
	})
}

// --- replaceRootArtwork ---

func TestReplaceRootArtwork(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("poster_replace", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		writeNFO(t, item, "movie", &nfo.Movie{ID: "movie"})

		detail, err := svc.readRootDetail(root, "movie", item)
		require.NoError(t, err)

		next, err := svc.replaceRootArtwork(root, detail, item, "", "poster", "poster.jpg", []byte("img"))
		require.NoError(t, err)
		assert.NotNil(t, next)
	})

	t.Run("fanart_replace", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		writeNFO(t, item, "movie", &nfo.Movie{ID: "movie"})

		detail, err := svc.readRootDetail(root, "movie", item)
		require.NoError(t, err)

		next, err := svc.replaceRootArtwork(root, detail, item, "", "fanart", "extra.jpg", []byte("img"))
		require.NoError(t, err)
		assert.NotNil(t, next)
	})

	t.Run("variant_not_found", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

		detail := &Detail{
			Item:     Item{RelPath: "movie"},
			Variants: nil,
		}
		_, err := svc.replaceRootArtwork(root, detail, item, "nonexistent", "poster", "p.jpg", []byte("img"))
		assert.ErrorIs(t, err, errLibraryVariantNotFound)
	})
}

// --- writeFanart ---

func TestWriteFanart(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	detail, err := svc.writeFanart(root, "movie", item, "extra.jpg", []byte("imgdata"))
	require.NoError(t, err)
	assert.NotNil(t, detail)
	assert.FileExists(t, filepath.Join(item, "extrafanart", "extra.jpg"))
}

// --- deleteRootFile ---

func TestDeleteRootFile(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		efDir := filepath.Join(item, "extrafanart")
		require.NoError(t, os.MkdirAll(efDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(efDir, "extra.jpg"), []byte("img"), 0o600))

		_, err := svc.deleteRootFile(root, "movie", "movie/extrafanart/extra.jpg")
		require.NoError(t, err)
	})

	t.Run("non_extrafanart_rejected", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

		_, err := svc.deleteRootFile(root, "movie", "movie/movie.mp4")
		assert.ErrorIs(t, err, errOnlyExtrafanartDeletable)
	})

	t.Run("file_not_exist", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

		_, err := svc.deleteRootFile(root, "movie", "movie/extrafanart/gone.jpg")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc2 := &Service{}
		_, err := svc2.deleteRootFile("", "movie", "")
		assert.Error(t, err)
	})
}

// --- cropImageRect ---

func TestCropImageRect(t *testing.T) {
	pngData := makePNG(t, 100, 100)

	t.Run("success", func(t *testing.T) {
		result, err := cropImageRect(pngData, 10, 10, 50, 50)
		require.NoError(t, err)
		assert.True(t, len(result) > 0)
	})
	t.Run("out_of_bounds", func(t *testing.T) {
		_, err := cropImageRect(pngData, 0, 0, 200, 200)
		assert.ErrorIs(t, err, errCropRectOutOfBounds)
	})
	t.Run("invalid_image", func(t *testing.T) {
		_, err := cropImageRect([]byte("not an image"), 0, 0, 1, 1)
		assert.Error(t, err)
	})
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	f := filepath.Join(t.TempDir(), "test.png")
	writePNG(t, f, w, h)
	data, err := os.ReadFile(f)
	require.NoError(t, err)
	return data
}

// --- resolveCoverForCrop ---

func TestResolveCoverForCrop(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		writePNG(t, filepath.Join(dir, "cover.jpg"), 100, 100)
		detail := &Detail{Item: Item{RelPath: "item"}}
		variant := Variant{CoverPath: "item/cover.jpg"}
		raw, relPath, err := resolveCoverForCrop(detail, variant, dir)
		require.NoError(t, err)
		assert.True(t, len(raw) > 0)
		assert.Equal(t, "cover.jpg", relPath)
	})
	t.Run("no_cover", func(t *testing.T) {
		detail := &Detail{Item: Item{RelPath: "item"}}
		variant := Variant{}
		_, _, err := resolveCoverForCrop(detail, variant, t.TempDir())
		assert.ErrorIs(t, err, errCoverNotFound)
	})
	t.Run("cover_file_missing", func(t *testing.T) {
		detail := &Detail{Item: Item{RelPath: "item"}}
		variant := Variant{CoverPath: "item/missing.jpg"}
		_, _, err := resolveCoverForCrop(detail, variant, t.TempDir())
		assert.Error(t, err)
	})
}

// --- cropRootPosterFromCover ---

func TestCropRootPosterFromCover(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
		writePNG(t, filepath.Join(item, "movie-fanart.png"), 100, 100)
		writeNFO(t, item, "movie", &nfo.Movie{ID: "movie", Cover: "movie-fanart.png"})

		detail, err := svc.readRootDetail(root, "movie", item)
		require.NoError(t, err)

		next, err := svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 50, 50)
		require.NoError(t, err)
		assert.NotNil(t, next)
	})

	t.Run("validate_fails", func(t *testing.T) {
		_, err := svc.cropRootPosterFromCover("/root", nil, "/nonexistent", "", 0, 0, 1, 1)
		assert.Error(t, err)
	})

	t.Run("variant_not_found", func(t *testing.T) {
		dir := t.TempDir()
		detail := &Detail{Item: Item{RelPath: "item"}}
		_, err := svc.cropRootPosterFromCover("/root", detail, dir, "missing", 0, 0, 1, 1)
		assert.ErrorIs(t, err, errLibraryVariantNotFound)
	})
}

// --- updatePosterInNFO ---

func TestUpdatePosterInNFO(t *testing.T) {
	dir := t.TempDir()
	nfoPath := filepath.Join(dir, "movie.nfo")
	require.NoError(t, nfo.WriteMovieToFile(nfoPath, &nfo.Movie{Title: "T"}))

	variant := Variant{Key: "movie", BaseName: "movie", NFOAbsPath: nfoPath}
	err := updatePosterInNFO(dir, variant, "movie", "new-poster.jpg")
	require.NoError(t, err)

	mov, err := nfo.ParseMovie(nfoPath)
	require.NoError(t, err)
	assert.Equal(t, "new-poster.jpg", mov.Poster)
	assert.Equal(t, "new-poster.jpg", mov.Art.Poster)
}

// --- applyVariantMetaToItem ---

func TestApplyVariantMetaToItem(t *testing.T) {
	t.Run("with_primary_variant", func(t *testing.T) {
		item := &Item{Title: "fallback", Name: "n"}
		variants := []Variant{{
			Key:        "pk",
			PosterPath: "item/poster.jpg",
			CoverPath:  "item/cover.jpg",
			NFOPath:    "item/pk.nfo",
			Meta: Meta{
				Title:           "OrigT",
				TitleTranslated: "TransT",
				Number:          "N-001",
				ReleaseDate:     "2024-01-01",
				Actors:          []string{"A"},
			},
		}}
		scan := dirScanResult{}
		applyVariantMetaToItem(item, variants, "pk", "/root", "item", scan)
		assert.Equal(t, "TransT", item.Title)
		assert.Equal(t, "N-001", item.Number)
		assert.Equal(t, "2024-01-01", item.ReleaseDate)
		assert.Equal(t, []string{"A"}, item.Actors)
		assert.True(t, item.HasNFO)
	})

	t.Run("no_primary_with_nfo", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		nfoPath := filepath.Join(item, "movie.nfo")
		require.NoError(t, nfo.WriteMovieToFile(nfoPath, &nfo.Movie{
			Title: "NFOTitle",
			ID:    "NUM",
		}))

		it := &Item{Title: "fallback", Name: "n"}
		scan := dirScanResult{hasNFO: true, nfoPath: nfoPath}
		applyVariantMetaToItem(it, nil, "missing", root, "movie", scan)
		assert.Equal(t, "NFOTitle", it.Title)
		assert.Equal(t, "NUM", it.Number)
	})

	t.Run("artwork_fallback", func(t *testing.T) {
		it := &Item{Title: "t", Name: "n"}
		scan := dirScanResult{imageNames: []string{"abc-poster.jpg", "abc-fanart.jpg"}}
		applyVariantMetaToItem(it, nil, "missing", "/root", "item", scan)
		assert.Equal(t, "item/abc-poster.jpg", it.PosterPath)
		assert.Equal(t, "item/abc-fanart.jpg", it.CoverPath)
	})
}

// --- finalizeVariants ---

func TestFinalizeVariants(t *testing.T) {
	variantsByKey := map[string]*Variant{
		"ABC-123":     {Key: "ABC-123", BaseName: "ABC-123", Meta: Meta{Number: "ABC-123"}},
		"ABC-123-CD2": {Key: "ABC-123-CD2", BaseName: "ABC-123-CD2", Meta: Meta{Number: "ABC-123-CD2"}},
	}
	keys := []string{"ABC-123", "ABC-123-CD2"}
	result := finalizeVariants(variantsByKey, keys, "ABC-123", "/root", "item", "item")
	assert.Len(t, result, 2)
	assert.True(t, result[0].IsPrimary)
	assert.Equal(t, "ABC-123", result[0].Key)
}

// --- populateVariantFromNFO ---

func TestPopulateVariantFromNFO(t *testing.T) {
	t.Run("no_nfo", func(t *testing.T) {
		v := &Variant{}
		populateVariantFromNFO(v, "/root", "item")
		assert.Equal(t, Meta{}, v.Meta)
	})
	t.Run("with_nfo", func(t *testing.T) {
		dir := t.TempDir()
		nfoPath := filepath.Join(dir, "movie.nfo")
		require.NoError(t, nfo.WriteMovieToFile(nfoPath, &nfo.Movie{
			Title:  "T",
			ID:     "N",
			Poster: "poster.jpg",
			Cover:  "cover.jpg",
		}))
		v := &Variant{NFOAbsPath: nfoPath}
		populateVariantFromNFO(v, dir, ".")
		assert.Equal(t, "N", v.Meta.Number)
		assert.NotEmpty(t, v.PosterPath)
		assert.NotEmpty(t, v.CoverPath)
	})
	t.Run("bad_nfo_file", func(t *testing.T) {
		dir := t.TempDir()
		nfoPath := filepath.Join(dir, "bad.nfo")
		require.NoError(t, os.WriteFile(nfoPath, []byte("not xml"), 0o600))
		v := &Variant{NFOAbsPath: nfoPath}
		populateVariantFromNFO(v, dir, ".")
		assert.Empty(t, v.Meta.Number)
	})
}

// --- scanRootVariants ---

func TestScanRootVariants(t *testing.T) {
	svc, _, _ := newTestService(t)

	t.Run("empty_dir", func(t *testing.T) {
		dir := t.TempDir()
		variants, pk, err := svc.scanRootVariants("/root", "item", dir)
		require.NoError(t, err)
		assert.Nil(t, variants)
		assert.Empty(t, pk)
	})

	t.Run("multi_variant", func(t *testing.T) {
		root := t.TempDir()
		item := filepath.Join(root, "movie")
		require.NoError(t, os.MkdirAll(item, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(item, "ABC-123.mp4"), []byte("v"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(item, "ABC-123-CD2.mp4"), []byte("v"), 0o600))

		variants, pk, err := svc.scanRootVariants(root, "movie", item)
		require.NoError(t, err)
		assert.Len(t, variants, 2)
		assert.Equal(t, "ABC-123", pk)
	})
}

// --- matchImageFilesToVariants ---

func TestMatchImageFilesToVariants(t *testing.T) {
	v := &Variant{Key: "ABC-123"}
	variantsByKey := map[string]*Variant{"ABC-123": v}
	topFiles := []variantTopFile{
		{name: "ABC-123-poster.jpg", stem: "ABC-123-poster", ext: ".jpg", relPath: "item/ABC-123-poster.jpg"},
		{name: "ABC-123-fanart.jpg", stem: "ABC-123-fanart", ext: ".jpg", relPath: "item/ABC-123-fanart.jpg"},
		{name: "ABC-123.mp4", stem: "ABC-123", ext: ".mp4", relPath: "item/ABC-123.mp4"},
	}
	matchImageFilesToVariants(topFiles, []string{"ABC-123"}, variantsByKey)
	assert.Equal(t, "item/ABC-123-poster.jpg", v.PosterPath)
	assert.Equal(t, "item/ABC-123-fanart.jpg", v.CoverPath)
}

// --- Full integration: inspectRootDir with NFO + images + multiple variants ---

func TestInspectRootDirWithNFOAndImages(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	writeNFO(t, item, "ABC-123", &nfo.Movie{
		Title:         "Translated",
		OriginalTitle: "Original",
		ID:            "ABC-123",
		ReleaseDate:   "2024-03-15",
		Poster:        "ABC-123-poster.jpg",
		Cover:         "ABC-123-fanart.jpg",
		Actors:        []nfo.Actor{{Name: "A1"}},
	})
	require.NoError(t, os.WriteFile(filepath.Join(item, "ABC-123.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "ABC-123-poster.jpg"), 10, 10)
	writePNG(t, filepath.Join(item, "ABC-123-fanart.jpg"), 10, 10)

	it, ok, err := svc.inspectRootDir(root, item)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Translated", it.Title)
	assert.Equal(t, "ABC-123", it.Number)
	assert.True(t, it.HasNFO)
	assert.NotEmpty(t, it.PosterPath)
	assert.NotEmpty(t, it.CoverPath)
}

// --- readRootDetail full with variant primary ---

func TestReadRootDetailWithPrimaryVariant(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	writeNFO(t, item, "ABC-123", &nfo.Movie{
		Title:         "Translated",
		OriginalTitle: "Original",
		ID:            "ABC-123",
		ReleaseDate:   "2024-03-15",
		Poster:        "ABC-123-poster.jpg",
		Cover:         "ABC-123-fanart.jpg",
	})
	require.NoError(t, os.WriteFile(filepath.Join(item, "ABC-123.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "ABC-123-poster.jpg"), 10, 10)
	writePNG(t, filepath.Join(item, "ABC-123-fanart.jpg"), 10, 10)

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)
	assert.Equal(t, "ABC-123", detail.PrimaryVariantKey)
	assert.NotEmpty(t, detail.Meta.PosterPath)
	assert.NotEmpty(t, detail.Meta.CoverPath)
	assert.NotEmpty(t, detail.Meta.FanartPath)
	assert.NotEmpty(t, detail.Meta.ThumbPath)
}

// --- listRootFiles with subdirectory ---

func TestListRootFilesWithSubdir(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	subDir := filepath.Join(item, "extrafanart")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "extra.jpg"), []byte("img"), 0o600))

	files, err := svc.listRootFiles(root, item)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

// --- writeVariantNFO with poster/cover in variant ---

func TestWriteVariantNFOWithPosterAndCover(t *testing.T) {
	dir := t.TempDir()
	variant := Variant{
		Key:        "ABC",
		BaseName:   "ABC",
		PosterPath: "item/ABC-poster.jpg",
		CoverPath:  "item/ABC-fanart.jpg",
	}
	meta := Meta{Title: "T", Number: "ABC"}
	err := writeVariantNFO(dir, "item", variant, "ABC", meta)
	require.NoError(t, err)
	mov, parseErr := nfo.ParseMovie(filepath.Join(dir, "ABC.nfo"))
	require.NoError(t, parseErr)
	assert.NotEmpty(t, mov.Poster)
	assert.NotEmpty(t, mov.Cover)
}

// --- cropRootPosterFromCover with same cover path as target ---

func TestCropRootPosterFromCoverSameAsTarget(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "movie-poster.png"), 100, 100)
	writeNFO(t, item, "movie", &nfo.Movie{
		ID:     "movie",
		Cover:  "movie-poster.png",
		Fanart: "movie-poster.png",
		Thumb:  "movie-poster.png",
		Poster: "movie-poster.png",
	})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	next, err := svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 50, 50)
	require.NoError(t, err)
	assert.NotNil(t, next)
}

// --- finalizeVariants sorting ---

func TestFinalizeVariantsSorting(t *testing.T) {
	variantsByKey := map[string]*Variant{
		"ABC-123":     {Key: "ABC-123", BaseName: "ABC-123", Meta: Meta{Number: "ABC-123"}},
		"ABC-123-CD2": {Key: "ABC-123-CD2", BaseName: "ABC-123-CD2", Meta: Meta{Number: "ABC-123-CD2"}},
		"ABC-123-CD3": {Key: "ABC-123-CD3", BaseName: "ABC-123-CD3", Meta: Meta{Number: "ABC-123-CD3"}},
	}
	keys := []string{"ABC-123", "ABC-123-CD2", "ABC-123-CD3"}
	result := finalizeVariants(variantsByKey, keys, "ABC-123", "/root", "item", "item")
	assert.Len(t, result, 3)
	assert.True(t, result[0].IsPrimary)
	assert.Equal(t, "", result[0].Suffix)
	assert.Equal(t, "原始文件", result[0].Label)
}

// --- scanRootVariants with images ---

func TestScanRootVariantsWithImages(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "ABC-123.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "ABC-123-poster.jpg"), 10, 10)
	writePNG(t, filepath.Join(item, "ABC-123-fanart.jpg"), 10, 10)
	writePNG(t, filepath.Join(item, "ABC-123-thumb.jpg"), 10, 10)

	variants, pk, err := svc.scanRootVariants(root, "movie", item)
	require.NoError(t, err)
	assert.Equal(t, "ABC-123", pk)
	assert.Len(t, variants, 1)
	assert.NotEmpty(t, variants[0].PosterPath)
	assert.NotEmpty(t, variants[0].CoverPath)
}

// --- resolveRootPath edge: clean to dot ---

func TestResolveRootPathCleanToDot(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.resolveRootPath("/root", "./")
	assert.Error(t, err)
}

// --- writeVariantArtwork with existing NFO ---

func TestWriteVariantArtworkWithExistingNFO(t *testing.T) {
	dir := t.TempDir()
	nfoPath := filepath.Join(dir, "NUM.nfo")
	require.NoError(t, nfo.WriteMovieToFile(nfoPath, &nfo.Movie{Title: "Old"}))
	detail := &Detail{
		Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
		PrimaryVariantKey: "NUM",
	}
	variant := Variant{Key: "NUM", BaseName: "NUM", NFOAbsPath: nfoPath}
	err := writeVariantArtwork(dir, detail, variant, "poster", "file.jpg", []byte("imgdata"))
	require.NoError(t, err)
	mov, parseErr := nfo.ParseMovie(nfoPath)
	require.NoError(t, parseErr)
	assert.NotEmpty(t, mov.Poster)
	assert.NotEmpty(t, mov.Art.Poster)
}

// --- listRootFiles sorting with same kind files ---

func TestListRootFilesSortSameKind(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "b.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(item, "a.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(item, "c.nfo"), []byte("<movie/>"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(item, "readme.txt"), []byte("x"), 0o600))

	files, err := svc.listRootFiles(root, item)
	require.NoError(t, err)
	assert.True(t, len(files) >= 4)
	for i := 1; i < len(files); i++ {
		if files[i].Kind == files[i-1].Kind {
			assert.True(t, files[i].Name >= files[i-1].Name)
		}
	}
}

// --- replaceRootArtwork cover kind ---

func TestReplaceRootArtworkCover(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	writeNFO(t, item, "movie", &nfo.Movie{ID: "movie"})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	next, err := svc.replaceRootArtwork(root, detail, item, "", "cover", "cover.png", []byte("img"))
	require.NoError(t, err)
	assert.NotNil(t, next)
}

// --- replaceRootArtwork nil detail ---

func TestReplaceRootArtworkNilDetail(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.replaceRootArtwork("/root", nil, "/nonexistent", "", "poster", "p.jpg", []byte("img"))
	assert.Error(t, err)
}

// --- updateRootItem with writeVariantNFO error (read-only dir) ---

func TestUpdateRootItemWriteError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	detail := &Detail{
		Item:              Item{RelPath: "movie", Number: "movie", Name: "movie"},
		PrimaryVariantKey: "movie",
		Variants:          []Variant{{Key: "movie", BaseName: "movie", IsPrimary: true}},
	}

	require.NoError(t, os.Chmod(item, 0o444))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, err := svc.updateRootItem(root, detail, item, Meta{Title: "T"})
	assert.Error(t, err)
}

// --- resolveRootPath edge: rel computes to .. ---

func TestResolveRootPathRelToParent(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.resolveRootPath("/root", "../")
	assert.Error(t, err)
	_, _, err = svc.resolveRootPath("/root", "../../foo")
	assert.Error(t, err)
}

// --- cropRootPosterFromCover with crop error ---

func TestCropRootPosterFromCoverCropError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "movie-fanart.png"), 100, 100)
	writeNFO(t, item, "movie", &nfo.Movie{ID: "movie", Cover: "movie-fanart.png"})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	_, err = svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 200, 200)
	assert.Error(t, err)
}

// --- cropRootPosterFromCover with cover missing ---

func TestCropRootPosterFromCoverMissing(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	_, err = svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 1, 1)
	assert.ErrorIs(t, err, errCoverNotFound)
}

// --- cropRootPosterFromCover with non-image ext on cover ---

func TestCropRootPosterFromCoverNonImageExt(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	pngData := makePNG(t, 100, 100)
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie-fanart.dat"), pngData, 0o600))
	writeNFO(t, item, "movie", &nfo.Movie{ID: "movie", Cover: "movie-fanart.dat"})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	next, err := svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 50, 50)
	require.NoError(t, err)
	assert.NotNil(t, next)
}

// --- writeVariantArtwork with cover kind ---

func TestWriteVariantArtworkCoverKind(t *testing.T) {
	dir := t.TempDir()
	detail := &Detail{
		Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
		PrimaryVariantKey: "NUM",
	}
	variant := Variant{Key: "NUM", BaseName: "NUM"}
	err := writeVariantArtwork(dir, detail, variant, "cover", "file.png", []byte("imgdata"))
	require.NoError(t, err)
	mov, _ := nfo.ParseMovie(filepath.Join(dir, "NUM.nfo"))
	assert.NotEmpty(t, mov.Cover)
	assert.NotEmpty(t, mov.Fanart)
	assert.NotEmpty(t, mov.Thumb)
}

// --- deleteRootFile with is-dir path ---

func TestDeleteRootFileIsDir(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	efDir := filepath.Join(item, "extrafanart", "subdir")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	_, err := svc.deleteRootFile(root, "movie", "movie/extrafanart/subdir")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// --- pickFanartTargetName exhausted ---

func TestPickFanartTargetNameExhausted(t *testing.T) {
	dir := t.TempDir()
	efDir := filepath.Join(dir, "extrafanart")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(efDir, "f.jpg"), []byte("x"), 0o600))
	for i := 2; i <= 1000; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(efDir, fmt.Sprintf("f-%d.jpg", i)), []byte("x"), 0o600))
	}
	_, err := pickFanartTargetName(dir, "f.jpg")
	assert.ErrorIs(t, err, errExtrafanartFilenameExhausted)
}

// --- inspectRootDir with scanRootVariants error ---

func TestInspectRootDirVideoOnly(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	it, ok, err := svc.inspectRootDir(root, item)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, it.VideoCount)
	assert.Equal(t, 1, it.VariantCount)
}

// --- listRootItemDirs with file at top level (not dir) ---

func TestListRootItemDirsWithFiles(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "stray.txt"), []byte("x"), 0o600))
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	dirs, err := svc.listRootItemDirs(root)
	require.NoError(t, err)
	assert.Len(t, dirs, 1)
}

// --- finalizeVariants with same-length and different-length suffix ---

func TestFinalizeVariantsSuffixSorting(t *testing.T) {
	variantsByKey := map[string]*Variant{
		"ABC-123":      {Key: "ABC-123", BaseName: "ABC-123", Meta: Meta{Number: "ABC-123"}},
		"ABC-123-ZZ9":  {Key: "ABC-123-ZZ9", BaseName: "ABC-123-ZZ9", Meta: Meta{Number: "ABC-123-ZZ9"}},
		"ABC-123-AA1":  {Key: "ABC-123-AA1", BaseName: "ABC-123-AA1", Meta: Meta{Number: "ABC-123-AA1"}},
		"ABC-123-LONG": {Key: "ABC-123-LONG", BaseName: "ABC-123-LONG", Meta: Meta{Number: "ABC-123-LONG"}},
	}
	keys := []string{"ABC-123", "ABC-123-ZZ9", "ABC-123-AA1", "ABC-123-LONG"}
	result := finalizeVariants(variantsByKey, keys, "ABC-123", "/root", "item", "item")
	assert.Len(t, result, 4)
	assert.True(t, result[0].IsPrimary)
	assert.Equal(t, "ABC-123-AA1", result[1].BaseName)
	assert.Equal(t, "ABC-123-ZZ9", result[2].BaseName)
	assert.Equal(t, "ABC-123-LONG", result[3].BaseName)
}

// --- scanRootVariants with same-length keys triggering matchKeys sort tiebreaker ---

func TestScanRootVariantsMatchKeySortTiebreaker(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "BBB.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(item, "AAA.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "BBB-poster.jpg"), 10, 10)
	writePNG(t, filepath.Join(item, "AAA-poster.jpg"), 10, 10)

	variants, _, err := svc.scanRootVariants(root, "movie", item)
	require.NoError(t, err)
	assert.Len(t, variants, 2)
}

// --- scanRootVariants with unreadable dir ---

func TestScanRootVariantsReadDirError(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, _, err := svc.scanRootVariants("/root", "item", "/nonexistent/path")
	assert.Error(t, err)
}

// --- inspectRootDir with unreadable directory ---

func TestInspectRootDirUnreadable(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "noperm")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.Chmod(item, 0o000))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, _, err := svc.inspectRootDir(root, item)
	assert.Error(t, err)
}

// --- listRootFiles with nonexistent dir ---

func TestListRootFilesError(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.listRootFiles("/root", "/nonexistent/path")
	assert.Error(t, err)
}

// --- readRootDetail where inspectRootDir errors ---

func TestReadRootDetailInspectError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "noperm")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.Chmod(item, 0o000))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, err := svc.readRootDetail(root, "noperm", item)
	assert.Error(t, err)
}

// --- listRootItemDirs with walk error ---

func TestListRootItemDirsWalkError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "subdir")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	inner := filepath.Join(item, "deep")
	require.NoError(t, os.MkdirAll(inner, 0o755))
	require.NoError(t, os.Chmod(inner, 0o000))
	t.Cleanup(func() { _ = os.Chmod(inner, 0o755) })

	_, err := svc.listRootItemDirs(root)
	assert.Error(t, err)
}

// --- cropImageRect with successful CutImageViaRectangle but edge bounds ---

func TestCropImageRectEdgeBounds(t *testing.T) {
	pngData := makePNG(t, 100, 100)
	result, err := cropImageRect(pngData, 0, 0, 100, 100)
	require.NoError(t, err)
	assert.True(t, len(result) > 0)
}

// --- writeVariantNFO with poster and cover values from variant ---

func TestWriteVariantNFOFull(t *testing.T) {
	dir := t.TempDir()
	variant := Variant{
		Key:        "V1",
		BaseName:   "V1",
		PosterPath: "item/V1-poster.jpg",
		CoverPath:  "item/V1-fanart.jpg",
		Meta: Meta{
			PosterPath: "item/meta-poster.jpg",
			CoverPath:  "item/meta-cover.jpg",
			FanartPath: "item/meta-fanart.jpg",
			ThumbPath:  "item/meta-thumb.jpg",
		},
	}
	meta := Meta{Title: "T", Number: "V1", ReleaseDate: "2024-01-01", Genres: []string{"G"}, Actors: []string{"A"}}
	err := writeVariantNFO(dir, "item", variant, "V1", meta)
	require.NoError(t, err)
	mov, parseErr := nfo.ParseMovie(filepath.Join(dir, "V1.nfo"))
	require.NoError(t, parseErr)
	assert.NotEmpty(t, mov.Poster)
	assert.NotEmpty(t, mov.Cover)
}

// --- updatePosterInNFO with no existing NFO ---

func TestUpdatePosterInNFONoExisting(t *testing.T) {
	dir := t.TempDir()
	variant := Variant{Key: "movie", BaseName: "movie"}
	err := updatePosterInNFO(dir, variant, "movie", "new-poster.jpg")
	require.NoError(t, err)
	mov, parseErr := nfo.ParseMovie(filepath.Join(dir, "movie.nfo"))
	require.NoError(t, parseErr)
	assert.Equal(t, "new-poster.jpg", mov.Poster)
}

// --- resolveMovieAssetPath with filepath.Rel error ---

func TestResolveMovieAssetPathEdge(t *testing.T) {
	result := resolveMovieAssetPath("/root", "item", "sub/file.jpg")
	assert.Equal(t, "item/sub/file.jpg", result)

	result = resolveMovieAssetPath("/root", "item", "../other.jpg")
	assert.Equal(t, "", result)
}

// --- pickFanartTargetName with stat error (not IsNotExist) ---

func TestPickFanartTargetNameStatError(t *testing.T) {
	dir := t.TempDir()
	efDir := filepath.Join(dir, "extrafanart")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(efDir, "f.jpg"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(efDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(efDir, 0o755) })

	_, err := pickFanartTargetName(dir, "new.jpg")
	assert.Error(t, err)
}

// --- replaceRootArtwork writeVariantArtwork error (read-only dir) ---

func TestReplaceRootArtworkWriteError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	writeNFO(t, item, "movie", &nfo.Movie{ID: "movie"})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(item, 0o555))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, err = svc.replaceRootArtwork(root, detail, item, "", "poster", "poster.jpg", []byte("img"))
	assert.Error(t, err)
}

// --- writeFanart with MkdirAll error (read-only dir) ---

func TestWriteFanartMkdirError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))

	require.NoError(t, os.Chmod(item, 0o555))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, err := svc.writeFanart(root, "movie", item, "extra.jpg", []byte("img"))
	assert.Error(t, err)
}

// --- cropRootPosterFromCover with write error ---

func TestCropRootPosterFromCoverWriteError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	require.NoError(t, os.MkdirAll(item, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	writePNG(t, filepath.Join(item, "movie-fanart.png"), 100, 100)
	writeNFO(t, item, "movie", &nfo.Movie{ID: "movie", Cover: "movie-fanart.png"})

	detail, err := svc.readRootDetail(root, "movie", item)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(item, 0o555))
	t.Cleanup(func() { _ = os.Chmod(item, 0o755) })

	_, err = svc.cropRootPosterFromCover(root, detail, item, "", 0, 0, 50, 50)
	assert.Error(t, err)
}

// --- updatePosterInNFO with write error ---

func TestUpdatePosterInNFOWriteError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	variant := Variant{Key: "movie", BaseName: "movie"}
	err := updatePosterInNFO(dir, variant, "movie", "poster.jpg")
	assert.Error(t, err)
}

// --- deleteRootFile with remove error ---

func TestDeleteRootFileRemoveError(t *testing.T) {
	svc, _, _ := newTestService(t)
	root := t.TempDir()
	item := filepath.Join(root, "movie")
	efDir := filepath.Join(item, "extrafanart")
	require.NoError(t, os.MkdirAll(efDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(item, "movie.mp4"), []byte("v"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(efDir, "extra.jpg"), []byte("img"), 0o600))

	require.NoError(t, os.Chmod(efDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(efDir, 0o755) })

	_, err := svc.deleteRootFile(root, "movie", "movie/extrafanart/extra.jpg")
	assert.Error(t, err)
}

// --- writeVariantArtwork with write error ---

func TestWriteVariantArtworkWriteError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	detail := &Detail{
		Item:              Item{RelPath: "item", Number: "NUM", Name: "movie"},
		PrimaryVariantKey: "NUM",
	}
	variant := Variant{Key: "NUM", BaseName: "NUM"}
	err := writeVariantArtwork(dir, detail, variant, "poster", "file.jpg", []byte("imgdata"))
	assert.Error(t, err)
}

// --- writeVariantNFO with write error ---

func TestWriteVariantNFOWriteError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	variant := Variant{Key: "V", BaseName: "V"}
	err := writeVariantNFO(dir, "item", variant, "V", Meta{Title: "T"})
	assert.Error(t, err)
}

// --- listRootItemDirs stat non-IsNotExist error ---

func TestListRootItemDirsStatError(t *testing.T) {
	svc, _, _ := newTestService(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "lib")
	require.NoError(t, os.MkdirAll(root, 0o755))

	require.NoError(t, os.Chmod(root, 0o000))
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	_, err := svc.listRootItemDirs(root)
	assert.Error(t, err)
}
