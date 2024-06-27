package utils

import (
	"av-capture/model"
	"av-capture/nfo"
	"time"
)

func ConvertMetaToMovieNFO(m *model.AvMeta) (*nfo.Movie, error) {
	mv := &nfo.Movie{
		ID:            m.Number,
		Plot:          "",
		Dateadded:     FormatTimeToDate(time.Now().UnixMilli()),
		Title:         m.Title,
		OriginalTitle: m.Title,
		SortTitle:     m.Title,
		Set:           m.Series,
		Rating:        0,
		Release:       FormatTimeToDate(m.ReleaseDate),
		ReleaseDate:   FormatTimeToDate(m.ReleaseDate),
		Premiered:     FormatTimeToDate(m.ReleaseDate),
		Runtime:       uint64(m.Duration) / 60, //分钟数
		Year:          time.UnixMilli(m.ReleaseDate).Year(),
		Tags:          m.Genres,
		Genres:        m.Genres,
		Studio:        m.Studio,
		Maker:         m.Studio,
		Art:           nfo.Art{}, //TODO:
		Mpaa:          "JP-18+",
		Director:      "",
		Label:         m.Label,
		Thumb:         "",
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
