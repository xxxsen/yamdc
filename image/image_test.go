package image

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func TestCutImageWithFaceRec(t *testing.T) {
	err := Init("../.vscode/tests/models")
	assert.NoError(t, err)
	datas := getTestImageDatas()
	for idx, data := range datas {
		raw, err := CutImageWithFaceRec(data)
		assert.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("../.vscode/tests_out/test_%d.jpg", idx+1), raw, 0644)
		assert.NoError(t, err)
	}
}
