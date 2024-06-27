package model

type AvMeta struct {
	Number       string   //番号
	Title        string   //标题
	Plot         string   //简介
	Actors       []string //演员
	ReleaseDate  int64    //发行时间, unix时间戳, 精确到秒
	Duration     int64    //影片时长, 单位为秒
	Studio       string   //制作商
	Label        string   //发行商
	Series       string   //系列
	Genres       []string //分类, tag
	Cover        *Image   //封面
	Poster       *Image   //海报
	SampleImages []*Image //样品图
}

type Image struct {
	Name string
	Data []byte
}

type ExtInfo struct {
	Number    string
	IsMultiCD bool
	CDNumber  int
}

type FileContext struct {
	FullFilePath string
	FileName     string
	FileExt      string
	SaveFileBase string
	SaveDir      string
	Meta         *AvMeta
	Ext          ExtInfo
}
