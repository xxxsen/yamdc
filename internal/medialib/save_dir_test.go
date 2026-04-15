package medialib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/nfo"
)

// --- IsSaveConfigured ---

func TestIsSaveConfigured(t *testing.T) {
	tests := []struct {
		name    string
		saveDir string
		want    bool
	}{
		{"configured", "/save", true},
		{"empty", "", false},
		{"blank", "  ", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(nil, "", tc.saveDir)
			assert.Equal(t, tc.want, svc.IsSaveConfigured())
		})
	}
}

// --- ResolveSavePath ---

func TestResolveSavePath(t *testing.T) {
	svc := NewService(nil, "", "/save")
	rel, abs, err := svc.ResolveSavePath("sub/item")
	require.NoError(t, err)
	assert.Equal(t, "sub/item", rel)
	assert.Contains(t, abs, "sub")

	svc2 := NewService(nil, "", "")
	_, _, err = svc2.ResolveSavePath("item")
	assert.Error(t, err)
}

// --- ListSaveItems ---

func TestServiceListSaveItems(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))

		svc := NewService(nil, "", saveDir)
		items, err := svc.ListSaveItems()
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, "demo", items[0].RelPath)
		require.Equal(t, "demo", items[0].Name)
		require.Equal(t, 1, items[0].VideoCount)
	})

	t.Run("not_configured", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.ListSaveItems()
		assert.ErrorIs(t, err, errSaveDirNotConfigured)
	})

	t.Run("nonexistent_dir", func(t *testing.T) {
		svc := NewService(nil, "", "/nonexistent/save/path")
		items, err := svc.ListSaveItems()
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("empty_dir", func(t *testing.T) {
		saveDir := t.TempDir()
		svc := NewService(nil, "", saveDir)
		items, err := svc.ListSaveItems()
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("sort_by_updated", func(t *testing.T) {
		saveDir := t.TempDir()
		for _, name := range []string{"aaa", "bbb"} {
			d := filepath.Join(saveDir, name)
			require.NoError(t, os.MkdirAll(d, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(d, name+".mp4"), []byte("v"), 0o600))
		}
		svc := NewService(nil, "", saveDir)
		items, err := svc.ListSaveItems()
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
}

// --- GetSaveDetail ---

func TestServiceGetSaveDetail(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))

		svc := NewService(nil, "", saveDir)
		detail, err := svc.GetSaveDetail("demo")
		require.NoError(t, err)
		require.Equal(t, "demo", detail.Item.RelPath)
		require.Len(t, detail.Variants, 1)
		require.Equal(t, "ABC-123", detail.Variants[0].Key)
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.GetSaveDetail("demo")
		assert.Error(t, err)
	})
}

// --- UpdateSaveItem ---

func TestServiceUpdateSaveItem(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))

		svc := NewService(nil, "", saveDir)
		updated, err := svc.UpdateSaveItem("demo", Meta{
			Title:           "Original Title",
			TitleTranslated: "Translated Title",
			Number:          "ABC-123",
			ReleaseDate:     "2024-01-02",
			Actors:          []string{"Actor A"},
		})
		require.NoError(t, err)
		require.Equal(t, "Translated Title", updated.Item.Title)
		require.Equal(t, "ABC-123", updated.Meta.Number)
		require.FileExists(t, filepath.Join(itemDir, "ABC-123.nfo"))
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.UpdateSaveItem("demo", Meta{})
		assert.Error(t, err)
	})
}

// --- ReplaceSaveAsset ---

func TestServiceReplaceSaveAsset(t *testing.T) {
	t.Run("poster", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))
		writeNFO(t, itemDir, "ABC-123", &nfo.Movie{ID: "ABC-123"})

		svc := NewService(nil, "", saveDir)
		detail, err := svc.ReplaceSaveAsset("demo", "", "poster", "poster.jpg", []byte("imgdata"))
		require.NoError(t, err)
		assert.NotNil(t, detail)
	})

	t.Run("fanart", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))

		svc := NewService(nil, "", saveDir)
		detail, err := svc.ReplaceSaveAsset("demo", "", "fanart", "extra.jpg", []byte("imgdata"))
		require.NoError(t, err)
		assert.NotNil(t, detail)
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.ReplaceSaveAsset("demo", "", "poster", "p.jpg", []byte("img"))
		assert.Error(t, err)
	})
}

// --- CropSavePoster ---

func TestServiceCropSavePoster(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))
		writePNG(t, filepath.Join(itemDir, "ABC-123-fanart.png"), 100, 100)
		writeNFO(t, itemDir, "ABC-123", &nfo.Movie{ID: "ABC-123", Cover: "ABC-123-fanart.png"})

		svc := NewService(nil, "", saveDir)
		detail, err := svc.CropSavePoster("demo", "", 0, 0, 50, 50)
		require.NoError(t, err)
		assert.NotNil(t, detail)
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.CropSavePoster("demo", "", 0, 0, 1, 1)
		assert.Error(t, err)
	})
}

// --- DeleteSaveFile ---

func TestServiceDeleteSaveFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		extraDir := filepath.Join(itemDir, "extrafanart")
		require.NoError(t, os.MkdirAll(extraDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(extraDir, "cover.jpg"), []byte("image"), 0o600))

		svc := NewService(nil, "", saveDir)
		detail, err := svc.DeleteSaveFile("demo/extrafanart/cover.jpg")
		require.NoError(t, err)
		require.Equal(t, "demo", detail.Item.RelPath)
		_, statErr := os.Stat(filepath.Join(extraDir, "cover.jpg"))
		require.True(t, os.IsNotExist(statErr))
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		_, err := svc.DeleteSaveFile("demo/extrafanart/f.jpg")
		assert.Error(t, err)
	})
}

// --- DeleteSaveItem ---

func TestServiceDeleteSaveItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		saveDir := t.TempDir()
		itemDir := filepath.Join(saveDir, "demo")
		require.NoError(t, os.MkdirAll(itemDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o600))

		svc := NewService(nil, "", saveDir)
		err := svc.DeleteSaveItem("demo")
		require.NoError(t, err)
		_, statErr := os.Stat(itemDir)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("not_dir", func(t *testing.T) {
		saveDir := t.TempDir()
		f := filepath.Join(saveDir, "file.txt")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))

		svc := NewService(nil, "", saveDir)
		err := svc.DeleteSaveItem("file.txt")
		assert.ErrorIs(t, err, errLibraryItemNotDir)
	})

	t.Run("nonexistent", func(t *testing.T) {
		saveDir := t.TempDir()
		svc := NewService(nil, "", saveDir)
		err := svc.DeleteSaveItem("nonexistent")
		assert.Error(t, err)
	})

	t.Run("invalid_path", func(t *testing.T) {
		svc := NewService(nil, "", "")
		err := svc.DeleteSaveItem("demo")
		assert.Error(t, err)
	})
}

// --- UpdateSaveItem readRootDetail error ---

func TestUpdateSaveItemReadDetailError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "empty")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "readme.txt"), []byte("x"), 0o600))

	svc := NewService(nil, "", saveDir)
	_, err := svc.UpdateSaveItem("empty", Meta{Title: "T"})
	assert.Error(t, err)
}

// --- ReplaceSaveAsset readRootDetail error ---

func TestReplaceSaveAssetReadDetailError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "empty")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "readme.txt"), []byte("x"), 0o600))

	svc := NewService(nil, "", saveDir)
	_, err := svc.ReplaceSaveAsset("empty", "", "poster", "p.jpg", []byte("img"))
	assert.Error(t, err)
}

// --- CropSavePoster readRootDetail error ---

func TestCropSavePosterReadDetailError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "empty")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "readme.txt"), []byte("x"), 0o600))

	svc := NewService(nil, "", saveDir)
	_, err := svc.CropSavePoster("empty", "", 0, 0, 1, 1)
	assert.Error(t, err)
}

// --- DeleteSaveFile with error computing rel path ---

func TestDeleteSaveFileErrorPaths(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "file.mp4"), []byte("v"), 0o600))

	svc := NewService(nil, "", saveDir)
	_, err := svc.DeleteSaveFile("demo/file.mp4")
	assert.Error(t, err)
}

// --- ListSaveItems sort tiebreaker (same UpdatedAt) ---

func TestListSaveItemsSortTiebreaker(t *testing.T) {
	saveDir := t.TempDir()
	for _, name := range []string{"bbb", "aaa", "ccc"} {
		d := filepath.Join(saveDir, name)
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, name+".mp4"), []byte("v"), 0o600))
	}
	svc := NewService(nil, "", saveDir)
	items, err := svc.ListSaveItems()
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

// --- ListSaveItems with inspectRootDir returning error for bad perms ---

func TestListSaveItemsInspectError(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "noperm")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "movie.mp4"), []byte("v"), 0o600))

	require.NoError(t, os.Chmod(itemDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(itemDir, 0o755) })

	svc := NewService(nil, "", saveDir)
	_, err := svc.ListSaveItems()
	assert.Error(t, err)
}

// --- DeleteSaveItem when RemoveAll succeeds for nested ---

func TestDeleteSaveItemNested(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	subDir := filepath.Join(itemDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("x"), 0o600))

	svc := NewService(nil, "", saveDir)
	err := svc.DeleteSaveItem("demo")
	require.NoError(t, err)
	_, statErr := os.Stat(itemDir)
	assert.True(t, os.IsNotExist(statErr))
}
