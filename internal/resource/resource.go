package resource

import (
	_ "embed"
)

//go:embed image/subtitle.png
var ResIMGSubtitle []byte

//go:embed image/unrated.png
var ResIMGUnrated []byte

//go:embed image/4k.png
var ResIMG4K []byte

//go:embed image/special_edition.png
var ResIMGSpecialEdition []byte

//go:embed image/8k.png
var ResIMG8K []byte

//go:embed image/vr.png
var ResIMGVR []byte

//go:embed image/restored.png
var ResIMGRestored []byte

//go:embed json/c_number.json.gz
var ResCNumber []byte // 数据来源为mdcx
