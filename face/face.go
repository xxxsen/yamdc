package face

import "image"

func FindMaxFace(fs []image.Rectangle) image.Rectangle {
	var maxArea int
	var m image.Rectangle
	for _, f := range fs {
		p := f.Size()
		if area := p.X * p.Y; area > maxArea {
			m = f
			maxArea = area
		}
	}
	return m
}
