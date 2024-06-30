package processor

import (
	"av-capture/model"
	"av-capture/store"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getTestImageDatas() [][]byte {
	rs := make([][]byte, 0, 20)
	err := filepath.Walk("../.vscode/tests/images/", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		if !(strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".bmp")) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		rs = append(rs, raw)
		return nil
	})
	if err != nil {
		panic(err)
	}

	return rs
}

func TestTranscodeImage(t *testing.T) {
	err := store.Init("../.vscode/tests/store")
	assert.NoError(t, err)
	imgs := getTestImageDatas()
	keys := make([]string, 0, len(imgs))
	for _, item := range imgs {
		key, err := store.GetDefault().Put(item)
		assert.NoError(t, err)
		keys = append(keys, key)
	}
	meta := &model.AvMeta{
		Number:      "123",
		Title:       "2343",
		Plot:        "12323",
		Actors:      []string{},
		ReleaseDate: 0,
		Duration:    0,
		Studio:      "",
		Label:       "",
		Series:      "",
		Genres:      []string{},
		Cover: &model.File{
			Key: keys[0],
		},
		Poster: &model.File{
			Key: keys[1],
		},
		SampleImages: []*model.File{
			{
				Key: keys[2],
			},
			{
				Key: keys[3],
			},
		},
	}
	p, err := createTranscodeImageProcessor(struct{}{})
	assert.NoError(t, err)
	fc := &model.FileContext{
		Meta: meta,
	}
	err = p.Process(context.Background(), fc)
	assert.NoError(t, err)
	img2save := make([]*model.File, 0, 5)
	img2save = append(img2save, meta.Cover, meta.Poster)
	img2save = append(img2save, meta.SampleImages...)
	for idx, item := range img2save {
		assert.NotNil(t, item)
		data, err := store.GetDefault().GetData(item.Key)
		assert.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("../.vscode/tests_out/transcode_%d.jpg", idx), data, 0644)
		assert.NoError(t, err)
	}

}
