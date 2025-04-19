package resource

import (
	_ "embed"
)

//go:embed image/subtitle.png
var ResIMGSubtitle []byte

//go:embed image/uncensored.png
var ResIMGUncensored []byte

//go:embed image/4k.png
var ResIMG4K []byte

//go:embed image/leak.png
var ResIMGLeak []byte

//go:embed image/8k.png
var ResIMG8K []byte

//go:embed image/vr.png
var ResIMGVR []byte

//go:embed image/hack.png
var ResIMGHack []byte

//go:embed json/c_number.json.gz
var ResCNumber []byte //数据来源为mdcx
