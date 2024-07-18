package nfo

import "encoding/xml"

type Movie struct {
	XMLName       xml.Name   `xml:"movie,omitempty"`
	Plot          string     `xml:"plot,omitempty"`          //剧情简介?
	Dateadded     string     `xml:"dateadded,omitempty"`     //example: 2022-08-18 06:01:03
	Title         string     `xml:"title,omitempty"`         //标题
	OriginalTitle string     `xml:"originaltitle,omitempty"` //原始标题, 与Title一致
	SortTitle     string     `xml:"sorttitle,omitempty"`     //与Title一致即可
	Set           string     `xml:"set,omitempty"`           //合集名?
	Rating        float64    `xml:"rating,omitempty"`        //评级, 貌似没用
	Release       string     `xml:"release,omitempty"`       //与releaseDate一致即可
	ReleaseDate   string     `xml:"releasedate,omitempty"`   //example: 2022-08-15
	Premiered     string     `xml:"premiered,omitempty"`     //与ReleaseDate保持一致即可
	Runtime       uint64     `xml:"runtime,omitempty"`       //分钟数
	Year          int        `xml:"year,omitempty"`          //发行年份, example: 2022
	Tags          []string   `xml:"tag,omitempty"`           //标签信息
	Studio        string     `xml:"studio,omitempty"`        //发行商
	Maker         string     `xml:"maker,omitempty"`         //与Studio一致即可
	Genres        []string   `xml:"genre,omitempty"`         //与标签保持一致即可
	Art           Art        `xml:"art,omitempty"`           //图片列表
	Mpaa          string     `xml:"mpaa,omitempty"`          //分级信息, 例如JP-18+
	Director      string     `xml:"director,omitempty"`      //导演
	Actors        []Actor    `xml:"actor,omitempty"`         //演员
	Poster        string     `xml:"poster,omitempty"`        //海报
	Thumb         string     `xml:"thumb,omitempty"`         //缩略图
	Label         string     `xml:"label,omitempty"`         //发行商
	ID            string     `xml:"id,omitempty"`            //番号
	Cover         string     `xml:"cover,omitempty"`         //封面
	Fanart        string     `xml:"fanart,omitempty"`        //跟封面一致就好了
	ScrapeInfo    ScrapeInfo `xml:"scrape_info"`             //抓取信息
}

type ScrapeInfo struct {
	Source string `xml:"source"`
	Date   string `xml:"date"`
}

type Actor struct {
	Name  string `xml:"name,omitempty"`
	Role  string `xml:"role,omitempty"`
	Thumb string `xml:"thumb,omitempty"`
}

type Art struct {
	Poster string   `xml:"poster,omitempty"`
	Fanart []string `xml:"fanart,omitempty"`
}
