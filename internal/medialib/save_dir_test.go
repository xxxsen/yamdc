package medialib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceListSaveItems(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o644))

	svc := NewService(nil, "", saveDir)
	items, err := svc.ListSaveItems()
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "demo", items[0].RelPath)
	require.Equal(t, "demo", items[0].Name)
	require.Equal(t, 1, items[0].VideoCount)
}

func TestServiceGetAndUpdateSaveDetail(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	require.NoError(t, os.MkdirAll(itemDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o644))

	svc := NewService(nil, "", saveDir)
	detail, err := svc.GetSaveDetail("demo")
	require.NoError(t, err)
	require.Equal(t, "demo", detail.Item.RelPath)
	require.Len(t, detail.Variants, 1)
	require.Equal(t, "ABC-123", detail.Variants[0].Key)

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
}

func TestServiceDeleteSaveExtrafanartFile(t *testing.T) {
	saveDir := t.TempDir()
	itemDir := filepath.Join(saveDir, "demo")
	extraDir := filepath.Join(itemDir, "extrafanart")
	require.NoError(t, os.MkdirAll(extraDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(itemDir, "ABC-123.mp4"), []byte("video"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(extraDir, "cover.jpg"), []byte("image"), 0o644))

	svc := NewService(nil, "", saveDir)
	detail, err := svc.DeleteSaveFile("demo/extrafanart/cover.jpg")
	require.NoError(t, err)
	require.Equal(t, "demo", detail.Item.RelPath)
	_, statErr := os.Stat(filepath.Join(extraDir, "cover.jpg"))
	require.True(t, os.IsNotExist(statErr))
}
