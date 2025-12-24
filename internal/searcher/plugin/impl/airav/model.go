package airav

type VideoData struct {
	Count  int    `json:"count"`
	Result Result `json:"result"`
	Status string `json:"status"`
}

type Result struct {
	ID          int         `json:"id"`
	Vid         string      `json:"vid"`
	Slug        interface{} `json:"slug"`
	Barcode     string      `json:"barcode"`
	ActorsName  string      `json:"actors_name"`
	Name        string      `json:"name"`
	ImgURL      string      `json:"img_url"`
	OtherImages []string    `json:"other_images"`
	Photo       interface{} `json:"photo"`
	PublishDate string      `json:"publish_date"`
	Description string      `json:"description"`
	Actors      []Actor     `json:"actors"`
	Images      []string    `json:"images"`
	Tags        []Tag       `json:"tags"`
	Factories   []Factory   `json:"factories"`
	MaybeLike   []MaybeLike `json:"maybe_like_videos"`
	QCURL       string      `json:"qc_url"`
	View        int         `json:"view"`
	OtherDesc   interface{} `json:"other_desc"`
	VideoURL    VideoURL    `json:"video_url"`
}

type Actor struct {
	Name   string `json:"name"`
	NameCn string `json:"name_cn"`
	NameJp string `json:"name_jp"`
	NameEn string `json:"name_en"`
	ID     string `json:"id"`
}

type Tag struct {
	Name string `json:"name"`
}

type Factory struct {
	Name string `json:"name"`
}

type MaybeLike struct {
	Vid     string `json:"vid"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	ImgURL  string `json:"img_url"`
	Barcode string `json:"barcode"`
}

type VideoURL struct {
	URLCdn    string `json:"url_cdn"`
	URLHlsCdn string `json:"url_hls_cdn"`
}
