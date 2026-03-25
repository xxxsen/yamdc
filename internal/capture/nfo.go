package capture

import (
	"time"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/nfo"
)

func formatTimeToDate(ts int64) string {
	t := time.UnixMilli(ts)
	return t.Format(time.DateOnly)
}

func convertMetaToMovieNFO(m *model.MovieMeta) (*nfo.Movie, error) {
	title := m.Title
	if len(m.TitleTranslated) > 0 {
		title = m.TitleTranslated
	}
	mv := &nfo.Movie{
		ID:              m.Number,
		Plot:            m.Plot,
		PlotTranslated:  m.PlotTranslated,
		Dateadded:       formatTimeToDate(time.Now().UnixMilli()),
		Title:           title,
		OriginalTitle:   m.Title,
		TitleTranslated: m.TitleTranslated,
		SortTitle:       m.Title,
		Set:             m.Series,
		Rating:          0,
		Release:         formatTimeToDate(m.ReleaseDate),
		ReleaseDate:     formatTimeToDate(m.ReleaseDate),
		Premiered:       formatTimeToDate(m.ReleaseDate),
		Runtime:         uint64(m.Duration) / 60, //分钟数
		Year:            time.UnixMilli(m.ReleaseDate).Year(),
		Tags:            m.Genres,
		Genres:          m.Genres,
		Studio:          m.Studio,
		Maker:           m.Studio,
		Art:             nfo.Art{},
		Mpaa:            "JP-18+",
		Director:        "",
		Label:           m.Label,
		Thumb:           "",
		ScrapeInfo: nfo.ScrapeInfo{
			Source: m.ExtInfo.ScrapeInfo.Source,
			Date:   time.UnixMilli(m.ExtInfo.ScrapeInfo.DateTs).Format(time.DateOnly),
		},
	}
	if m.Poster != nil {
		mv.Art.Poster = m.Poster.Name
		mv.Poster = m.Poster.Name
		//
		mv.Art.Fanart = append(mv.Art.Fanart, m.Poster.Name)
	}
	if m.Cover != nil {
		mv.Cover = m.Cover.Name
		mv.Fanart = m.Cover.Name
		//
		mv.Art.Fanart = append(mv.Art.Fanart, m.Cover.Name)
	}
	for _, act := range m.Actors {
		mv.Actors = append(mv.Actors, nfo.Actor{
			Name: act,
		})
	}
	for _, image := range m.SampleImages {
		mv.Art.Fanart = append(mv.Art.Fanart, image.Name)
	}
	return mv, nil
}
