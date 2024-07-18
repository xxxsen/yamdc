package utils

import (
	"fmt"
	"time"
	"yamdc/model"
	"yamdc/nfo"
)

func buildDataWithSingleTranslateItem(origin string, item *model.SingleTranslateItem) string {
	if !item.Enable || len(item.TranslatedText) == 0 {
		return origin
	}
	return fmt.Sprintf("%s [翻译:%s]", origin, item.TranslatedText)
}

func ConvertMetaToMovieNFO(m *model.AvMeta) (*nfo.Movie, error) {
	mv := &nfo.Movie{
		ID:            m.Number,
		Plot:          buildDataWithSingleTranslateItem(m.Plot, &m.ExtInfo.TranslateInfo.Plot),
		Dateadded:     FormatTimeToDate(time.Now().UnixMilli()),
		Title:         buildDataWithSingleTranslateItem(m.Title, &m.ExtInfo.TranslateInfo.Title),
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
		Art:           nfo.Art{},
		Mpaa:          "JP-18+",
		Director:      "",
		Label:         m.Label,
		Thumb:         "",
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
