package nfo

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadWrite(t *testing.T) {
	m := &Movie{
		XMLName:       xml.Name{},
		Plot:          "hello world, this is a test",
		Dateadded:     "2022-01-02",
		Title:         "hello world",
		OriginalTitle: "hello world",
		SortTitle:     "hello world",
		Set:           "aaaa",
		Rating:        111,
		Release:       "2021-01-05",
		ReleaseDate:   "2021-01-05",
		Premiered:     "2021-01-05",
		Runtime:       60,
		Year:          2021,
		Tags:          []string{"t_a", "t_b", "t_c", "t_d"},
		Studio:        "hello_studio",
		Maker:         "hello_maker",
		Genres:        []string{"t_x", "t_y", "t_z"},
		Art:           Art{Poster: "art_poster.jpg", Fanart: []string{"art_fanart_1", "art_fanart_2", "art_fanart_3"}},
		Mpaa:          "JP-18+",
		Director:      "hello_director",
		Actors:        []Actor{{Name: "act_a", Role: "main", Thumb: "act_a.jpg"}},
		Poster:        "poster.jpg",
		Thumb:         "thumb.jpg",
		Label:         "hello_label",
		ID:            "2022-01111",
		Cover:         "cover.jpg",
		Fanart:        "fanart.jpg",
		ScrapeInfo: ScrapeInfo{
			Source: "abc",
			Date:   "2021-03-05",
		},
	}
	buf := bytes.NewBuffer(nil)
	err := WriteMovie(buf, m)
	assert.NoError(t, err)
	newM, err := ParseMovieWithData(buf.Bytes())
	assert.NoError(t, err)
	newM.XMLName = m.XMLName
	assert.Equal(t, m, newM)
}
