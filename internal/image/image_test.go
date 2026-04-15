package image

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscodeToJpeg(t *testing.T) {
	t.Parallel()
	raw, err := MakeColorImageData(image.Rect(0, 0, 32, 32), color.RGBA{R: 10, G: 20, B: 30, A: 255})
	require.NoError(t, err)

	out, err := TranscodeToJpeg(raw)
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	assert.True(t, bytes.HasPrefix(out, []byte{0xff, 0xd8, 0xff}), "JPEG SOI")

	_, err = TranscodeToJpeg([]byte("not an image"))
	assert.Error(t, err)
}

func TestLoadImage_invalid(t *testing.T) {
	t.Parallel()
	_, err := LoadImage([]byte{0, 1, 2, 3})
	assert.Error(t, err)
}

func TestWriteImageToFile(t *testing.T) {
	t.Parallel()
	img := MakeColorImage(image.Rect(0, 0, 8, 8), color.RGBA{A: 255})
	dst := filepath.Join(t.TempDir(), "out.jpg")
	require.NoError(t, WriteImageToFile(dst, img))
	b, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.NotEmpty(t, b)

	err = WriteImageToFile(filepath.Join(dst, "nested"), img) // dst is a file, not dir
	assert.Error(t, err)
}

func TestMakeColorImageData_roundTrip(t *testing.T) {
	t.Parallel()
	data, err := MakeColorImageData(image.Rect(0, 0, 4, 4), color.RGBA{R: 1, G: 2, B: 3, A: 255})
	require.NoError(t, err)
	decoded, err := LoadImage(data)
	require.NoError(t, err)
	assert.Equal(t, 4, decoded.Bounds().Dx())
}

func TestWriteImageToBytes_jpegTooLarge(t *testing.T) {
	t.Parallel()
	_, err := WriteImageToBytes(image.NewUniform(color.RGBA{A: 255}))
	require.Error(t, err)
}

func TestCutCensoredImageFromBytes_invalidCrop(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 0, 40))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	_, err := CutCensoredImageFromBytes(buf.Bytes())
	require.Error(t, err)
}
