package medialib

type Item struct {
	ID           int64    `json:"id"`
	RelPath      string   `json:"rel_path"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Number       string   `json:"number"`
	ReleaseDate  string   `json:"release_date"`
	Actors       []string `json:"actors"`
	UpdatedAt    int64    `json:"updated_at"`
	HasNFO       bool     `json:"has_nfo"`
	PosterPath   string   `json:"poster_path"`
	CoverPath    string   `json:"cover_path"`
	FileCount    int      `json:"file_count"`
	VideoCount   int      `json:"video_count"`
	VariantCount int      `json:"variant_count"`
}

type Meta struct {
	Title           string   `json:"title"`
	TitleTranslated string   `json:"title_translated"`
	OriginalTitle   string   `json:"original_title"`
	Plot            string   `json:"plot"`
	PlotTranslated  string   `json:"plot_translated"`
	Number          string   `json:"number"`
	ReleaseDate     string   `json:"release_date"`
	Runtime         uint64   `json:"runtime"`
	Studio          string   `json:"studio"`
	Label           string   `json:"label"`
	Series          string   `json:"series"`
	Director        string   `json:"director"`
	Actors          []string `json:"actors"`
	Genres          []string `json:"genres"`
	PosterPath      string   `json:"poster_path"`
	CoverPath       string   `json:"cover_path"`
	FanartPath      string   `json:"fanart_path"`
	ThumbPath       string   `json:"thumb_path"`
	Source          string   `json:"source"`
	ScrapedAt       string   `json:"scraped_at"`
}

type FileItem struct {
	Name         string `json:"name"`
	RelPath      string `json:"rel_path"`
	Kind         string `json:"kind"`
	Size         int64  `json:"size"`
	UpdatedAt    int64  `json:"updated_at"`
	VariantKey   string `json:"variant_key,omitempty"`
	VariantLabel string `json:"variant_label,omitempty"`
}

type Variant struct {
	Key        string     `json:"key"`
	Label      string     `json:"label"`
	BaseName   string     `json:"base_name"`
	Suffix     string     `json:"suffix"`
	IsPrimary  bool       `json:"is_primary"`
	VideoPath  string     `json:"video_path"`
	NFOPath    string     `json:"nfo_path"`
	PosterPath string     `json:"poster_path"`
	CoverPath  string     `json:"cover_path"`
	Meta       Meta       `json:"meta"`
	Files      []FileItem `json:"files"`
	FileCount  int        `json:"file_count"`
	NFOAbsPath string     `json:"-"`
}

type Detail struct {
	Item              Item       `json:"item"`
	Meta              Meta       `json:"meta"`
	Variants          []Variant  `json:"variants"`
	PrimaryVariantKey string     `json:"primary_variant_key"`
	Files             []FileItem `json:"files"`
}

type TaskState struct {
	TaskKey       string `json:"task_key"`
	Status        string `json:"status"`
	Total         int    `json:"total"`
	Processed     int    `json:"processed"`
	SuccessCount  int    `json:"success_count"`
	ConflictCount int    `json:"conflict_count"`
	ErrorCount    int    `json:"error_count"`
	Current       string `json:"current"`
	Message       string `json:"message"`
	StartedAt     int64  `json:"started_at"`
	FinishedAt    int64  `json:"finished_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type StatusSnapshot struct {
	Configured bool      `json:"configured"`
	Sync       TaskState `json:"sync"`
	Move       TaskState `json:"move"`
}
