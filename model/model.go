package model

import "yamdc/number"

type MovieMeta struct {
	Number       string   `json:"number"`        //番号
	Title        string   `json:"title"`         //标题
	Plot         string   `json:"plot"`          //简介
	Actors       []string `json:"actors"`        //演员
	ReleaseDate  int64    `json:"release_date"`  //发行时间, unix时间戳, 精确到秒
	Duration     int64    `json:"duration"`      //影片时长, 单位为秒
	Studio       string   `json:"studio"`        //制作商
	Label        string   `json:"label"`         //发行商
	Series       string   `json:"series"`        //系列
	Genres       []string `json:"genres"`        //分类, tag
	Cover        *File    `json:"cover"`         //封面
	Poster       *File    `json:"poster"`        //海报
	SampleImages []*File  `json:"sample_images"` //样品图
	Director     string   `json:"director"`      //导演
	ExtInfo      ExtInfo  `json:"ext_info"`
}

type SingleTranslateItem struct {
	Enable         bool   `json:"enable"`
	TranslatedText string `json:"translated_text"`
}

type TranslateInfo struct {
	Title SingleTranslateItem `json:"title"`
	Plot  SingleTranslateItem `json:"plot"`
}

type ScrapeInfo struct {
	Source string `json:"source"`
	DateTs int64  `json:"date_ts"`
}

type ExtInfo struct {
	ScrapeInfo    ScrapeInfo    `json:"scrape_info"`
	TranslateInfo TranslateInfo `json:"translated_info"`
}

type File struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type FileContext struct {
	FullFilePath string
	FileName     string
	FileExt      string
	SaveFileBase string
	SaveDir      string
	Meta         *MovieMeta
	Number       *number.Number
}
