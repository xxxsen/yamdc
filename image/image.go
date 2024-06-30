package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"sync"

	"github.com/Kagami/go-face"
)

var once sync.Once
var recInst *face.Recognizer

const (
	defaultAspectRatio = float64(70.3) / 100
)

func Init(modelDir string) error {
	var err error
	once.Do(func() {
		recInst, err = face.NewRecognizer(modelDir)
	})
	if err != nil {
		return err
	}
	return nil
}

func SearchFaces(data []byte) ([]face.Face, error) {
	fces, err := recInst.RecognizeCNN(data)
	if err != nil {
		return nil, err
	}
	if len(fces) == 0 {
		fces, err = recInst.Recognize(data)
	}
	if err != nil {
		return nil, err
	}
	return fces, nil
}

func findMaxFace(fs []face.Face) face.Face {
	var maxArea int
	var m face.Face
	for _, f := range fs {
		p := f.Rectangle.Size()
		if area := p.X * p.Y; area > maxArea {
			m = f
			maxArea = area
		}
	}
	return m
}

func cutHorizontalImage(img image.Image, center int, aspectRatio float64) ([]byte, error) {
	//使用脸中心的横坐标, 以这个横坐标往2边扩展, 直到裁剪出来的海报满足宽高比
	cropWidth := int(float64(img.Bounds().Dy()) * aspectRatio)
	rightWidthEnd := center + cropWidth/2
	leftWidthEnd := center - cropWidth/2
	//如果右边界超过图片边界, 则进行左移, 直至裁剪框落在图片范围内。
	if rightWidthEnd > img.Bounds().Dx() {
		offset := rightWidthEnd - img.Bounds().Dx()
		leftWidthEnd = leftWidthEnd - offset
		rightWidthEnd = img.Bounds().Dx()
	}
	if leftWidthEnd < 0 {
		offset := int(math.Abs(float64(leftWidthEnd)))
		leftWidthEnd = 0
		rightWidthEnd = rightWidthEnd + offset
	}
	if rightWidthEnd > img.Bounds().Dx() || leftWidthEnd < 0 {
		return nil, fmt.Errorf("invalid image")
	}
	rect := image.Rect(leftWidthEnd, 0, rightWidthEnd, img.Bounds().Max.Y)
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)
	return imageToBytes(croppedImg)
}

func cutVerticalImage(img image.Image, center int, aspectRatio float64) ([]byte, error) {
	//使用脸中心的纵坐标, 以这个纵坐标往2边扩展, 直到裁剪出来的海报满足宽高比
	cropHeight := int(float64(img.Bounds().Dx()) / aspectRatio)
	bottomHeightEnd := center + cropHeight/2
	topHeightEnd := center - cropHeight/2
	//如果下边界超过图片边界, 则进行上移, 直至裁剪框落在图片范围内。
	if bottomHeightEnd > img.Bounds().Dy() {
		offset := bottomHeightEnd - img.Bounds().Dy()
		topHeightEnd = topHeightEnd - offset
		bottomHeightEnd = img.Bounds().Max.Y
	}
	if topHeightEnd < 0 {
		offset := int(math.Abs(float64(topHeightEnd)))
		topHeightEnd = 0
		bottomHeightEnd = bottomHeightEnd + offset
	}
	if bottomHeightEnd > img.Bounds().Dy() || topHeightEnd < 0 {
		return nil, fmt.Errorf("invalid image")
	}
	rect := image.Rect(0, topHeightEnd, img.Bounds().Max.X, bottomHeightEnd)
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)
	return imageToBytes(croppedImg)
}

func CutImageWithFaceRec(data []byte) ([]byte, error) {
	img, err := normalizeImage(data)
	if err != nil {
		return nil, err
	}
	fs, err := SearchFaces(data)
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, fmt.Errorf("no face found")
	}
	m := findMaxFace(fs)
	if img.Bounds().Dx() < img.Bounds().Dy() {
		//如果图片宽高比小于预期, 那么这里需要按竖屏图进行裁剪
		return cutVerticalImage(img, m.Rectangle.Dy()/2, defaultAspectRatio)
	}
	return cutHorizontalImage(img, m.Rectangle.Dx()/2, defaultAspectRatio)
}

func imageToBytes(img image.Image) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return nil, fmt.Errorf("unable to convert img to jpg, err:%w", err)
	}
	return buf.Bytes(), nil
}

func normalizeImage(data []byte) (image.Image, error) {
	//将data的image转换成jpg
	img, typ, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("deocode image failed, err:%w", err)
	}
	if typ == "jpeg" {
		return img, nil
	}
	buf := bytes.Buffer{}
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return nil, fmt.Errorf("unable to convert img to jpg, err:%w", err)
	}
	img, _, err = image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid image data, err:%w", err)
	}
	return img, nil
}

func CutCensoredImage(data []byte) ([]byte, error) {
	img, err := normalizeImage(data)
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() > img.Bounds().Dy() { //横屏
		middle := img.Bounds().Dx() //直接取最大值, 由底层函数自行扩展即可
		return cutHorizontalImage(img, middle, defaultAspectRatio)
	}
	//正常不应该出现骑兵封面为竖屏的
	//另一方面, 正常人像应该是上面, 所以从上开始截取
	return cutVerticalImage(img, 0, defaultAspectRatio)
}