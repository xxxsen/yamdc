package pigo

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pigocore "github.com/esimov/pigo/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/face"
)

func pigoCascadeRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.CommandContext(context.Background(), "go", "list", "-m", "-f", "{{.Dir}}", "github.com/esimov/pigo").Output()
	if err != nil {
		t.Skipf("cannot locate pigo module: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestNewPigo_ReadCascadeMissing(t *testing.T) {
	_, err := NewPigo(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read face cascade")
}

func TestNewPigo_UnpackInvalidCascade(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, defaultFaceFinderCascade), []byte("not-a-cascade"), 0o600))
	assert.Panics(t, func() {
		_, _ = NewPigo(dir)
	})
}

func TestPigo_Name(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)
	assert.Equal(t, face.NamePigo, w.Name())
}

func TestPigo_SearchFaces_InvalidImage(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)
	_, err = w.SearchFaces(context.Background(), []byte("not-an-image"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode image")
}

func TestPigo_SearchFaces_SuccessPath(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			img.Set(x, y, color.RGBA{R: 0x20, G: 0x40, B: 0x80, A: 0xff})
		}
	}
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	for _, r := range recs {
		assert.False(t, r.Empty())
	}
}

func TestPigo_SearchFaces_LargerImage(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			if x > 50 && x < 150 && y > 30 && y < 170 {
				img.Set(x, y, color.RGBA{R: 0xeb, G: 0xc2, B: 0xa1, A: 0xff})
			} else {
				img.Set(x, y, color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff})
			}
		}
	}
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	for _, r := range recs {
		assert.False(t, r.Empty())
	}
}

func TestPigo_SearchFaces_GradientImage(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 128, A: 255})
		}
	}
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	_ = recs
}

func TestPigo_SearchFaces_FacelikeOval(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	const width, height = 320, 320
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	cx, cy := width/2, height/2

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx := float64(x-cx) / 60.0
			dy := float64(y-cy) / 80.0
			dist := dx*dx + dy*dy
			if dist < 1.0 {
				img.Set(x, y, color.RGBA{R: 0xd9, G: 0xab, B: 0x89, A: 0xff})
			} else {
				img.Set(x, y, color.RGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff})
			}
		}
	}

	eyeOffsets := [][2]int{{-20, -15}, {20, -15}}
	for _, off := range eyeOffsets {
		for dy := -5; dy <= 5; dy++ {
			for dx := -8; dx <= 8; dx++ {
				ex, ey := cx+off[0]+dx, cy+off[1]+dy
				if ex >= 0 && ex < width && ey >= 0 && ey < height {
					img.Set(ex, ey, color.RGBA{R: 0x30, G: 0x20, B: 0x10, A: 0xff})
				}
			}
		}
	}

	for dx := -15; dx <= 15; dx++ {
		my := cy + 25
		mx := cx + dx
		if mx >= 0 && mx < width && my >= 0 && my < height {
			img.Set(mx, my, color.RGBA{R: 0x80, G: 0x40, B: 0x30, A: 0xff})
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	for _, r := range recs {
		assert.False(t, r.Empty())
	}
}

func TestPigo_SearchFaces_MultipleFormats(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 500, 500))
	for y := 0; y < 500; y++ {
		for x := 0; x < 500; x++ {
			r := uint8((x*7 + y*3) % 256)
			g := uint8((x*11 + y*5) % 256)
			b := uint8((x*13 + y*7) % 256)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	_ = recs
}

func TestPigo_SearchFaces_RealImage(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	imgData, err := os.ReadFile(filepath.Join(root, "testdata", "sample.jpg"))
	if err != nil {
		t.Skipf("sample.jpg not available: %v", err)
	}

	recs, err := w.SearchFaces(context.Background(), imgData)
	require.NoError(t, err)
	assert.NotEmpty(t, recs, "expected at least one face in sample.jpg")
	for _, r := range recs {
		assert.False(t, r.Empty())
	}
}

func TestPigo_SearchFaces_TestPNG(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	imgData, err := os.ReadFile(filepath.Join(root, "testdata", "test.png"))
	if err != nil {
		t.Skipf("test.png not available: %v", err)
	}

	recs, err := w.SearchFaces(context.Background(), imgData)
	require.NoError(t, err)
	_ = recs
}

func TestPigo_SearchFaces_RandomNoiseForLowQ(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	const sz = 640
	img := image.NewRGBA(image.Rect(0, 0, sz, sz))
	seed := uint64(12345)
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			seed = seed*6364136223846793005 + 1
			img.Set(x, y, color.RGBA{
				R: uint8(seed >> 16), //nolint:gosec
				G: uint8(seed >> 8),  //nolint:gosec
				B: uint8(seed),       //nolint:gosec
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	recs, err := w.SearchFaces(context.Background(), buf.Bytes())
	require.NoError(t, err)
	_ = recs
}

func TestFilterDetections_LowQFiltered(t *testing.T) {
	dets := []pigocore.Detection{
		{Row: 100, Col: 100, Scale: 50, Q: 0.3},
		{Row: 200, Col: 200, Scale: 60, Q: 0.8},
		{Row: 300, Col: 300, Scale: 70, Q: 0.1},
	}
	recs := filterDetections(dets)
	assert.Len(t, recs, 1)
	assert.Equal(t, image.Rect(170, 170, 230, 230), recs[0])
}

func TestFilterDetections_Empty(t *testing.T) {
	recs := filterDetections(nil)
	assert.Empty(t, recs)
}

func TestFilterDetections_AllFiltered(t *testing.T) {
	dets := []pigocore.Detection{
		{Row: 100, Col: 100, Scale: 50, Q: 0.1},
		{Row: 200, Col: 200, Scale: 60, Q: 0.4},
	}
	recs := filterDetections(dets)
	assert.Empty(t, recs)
}

func TestPigo_SearchFaces_BrightCircularPattern(t *testing.T) {
	root := pigoCascadeRoot(t)
	w, err := NewPigo(filepath.Join(root, "cascade"))
	require.NoError(t, err)

	sizes := []int{128, 256, 512}
	for _, sz := range sizes {
		var buf bytes.Buffer
		img := image.NewRGBA(image.Rect(0, 0, sz, sz))
		cx, cy := sz/2, sz/2
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				dx := float64(x-cx) / float64(sz/4)
				dy := float64(y-cy) / float64(sz/3)
				dist := dx*dx + dy*dy
				switch {
				case dist < 0.1:
					img.Set(x, y, color.RGBA{R: 0x20, G: 0x10, B: 0x08, A: 0xff})
				case dist < 1.0:
					img.Set(x, y, color.RGBA{R: 0xe8, G: 0xbe, B: 0x96, A: 0xff})
				default:
					img.Set(x, y, color.RGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xff})
				}
			}
		}
		require.NoError(t, png.Encode(&buf, img))

		recs, err := w.SearchFaces(context.Background(), buf.Bytes())
		require.NoError(t, err)
		for _, r := range recs {
			assert.False(t, r.Empty())
		}
	}
}
