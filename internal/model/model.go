package model

import "github.com/xxxsen/yamdc/internal/number"

type MovieMeta struct {
	Number           string   `json:"number"`            //番号
	Title            string   `json:"title"`             //标题
	TitleLang        string   `json:"title_lang"`        //标题语言
	TitleTranslated  string   `json:"title_translated"`  //翻译后的title
	Plot             string   `json:"plot"`              //简介
	PlotLang         string   `json:"plot_lang"`         //简介语言
	PlotTranslated   string   `json:"plot_translated"`   //翻译后的plot
	Actors           []string `json:"actors"`            //演员, 如果产生翻译, 翻译结果会直接替换原始列表
	ActorsLang       string   `json:"actors_lang"`       //演员语言
	ReleaseDate      int64    `json:"release_date"`      //发行时间, unix时间戳, 精确到秒
	Duration         int64    `json:"duration"`          //影片时长, 单位为秒
	Studio           string   `json:"studio"`            //制作商
	Label            string   `json:"label"`             //发行商
	Series           string   `json:"series"`            //系列
	Genres           []string `json:"genres"`            //分类, tag
	GenresTranslated []string `json:"genres_translated"` //翻译后的genres
	GenresLang       string   `json:"genres_lang"`       //tag语言
	Cover            *File    `json:"cover"`             //封面
	Poster           *File    `json:"poster"`            //海报
	SampleImages     []*File  `json:"sample_images"`     //样品图
	Director         string   `json:"director"`          //导演
	//非抓取的信息
	SwithConfig SwitchConfig `json:"switch_config"` //开关配置
	ExtInfo     ExtInfo      `json:"ext_info"`      //扩展信息
}

type ScrapeInfo struct {
	Source string `json:"source"`
	DateTs int64  `json:"date_ts"`
}

type SwitchConfig struct {
	DisableNumberReplace    bool `json:"disable_number_replace"`
	DisableReleaseDateCheck bool `json:"disable_release_date_check"`
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
