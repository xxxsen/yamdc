package model

import "yamdc/number"

type MovieMeta struct {
	Number           string   `json:"number"`            //番号
	Title            string   `json:"title"`             //标题
	TitleLang        string   `json:"title_lang"`        //标题语言
	TitleTranslated  string   `json:"title_translated"`  //翻译后的title
	Plot             string   `json:"plot"`              //简介
	PlotLang         string   `json:"plot_lang"`         //简介语言
	PlotTranslated   string   `json:"plot_translated"`   //翻译后的plot
	Actors           []string `json:"actors"`            //演员
	ActorsLang       string   `json:"actors_lang"`       //演员语言
	ActorsTranslated []string `json:"actors_translated"` //翻译后的actors
	ReleaseDate      int64    `json:"release_date"`      //发行时间, unix时间戳, 精确到秒
	Duration         int64    `json:"duration"`          //影片时长, 单位为秒
	Studio           string   `json:"studio"`            //制作商
	Label            string   `json:"label"`             //发行商
	Series           string   `json:"series"`            //系列
	Genres           []string `json:"genres"`            //分类, tag
	GenresLang       string   `json:"genres_lang"`       //tag语言
	GenresTranslated []string `json:"genres_translated"` //翻译后的tag
	Cover            *File    `json:"cover"`             //封面
	Poster           *File    `json:"poster"`            //海报
	SampleImages     []*File  `json:"sample_images"`     //样品图
	Director         string   `json:"director"`          //导演
	ExtInfo          ExtInfo  `json:"ext_info"`
}

type ScrapeInfo struct {
	Source string `json:"source"`
	DateTs int64  `json:"date_ts"`
}

type ExtInfo struct {
	ScrapeInfo ScrapeInfo `json:"scrape_info"`
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
