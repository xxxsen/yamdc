package capture

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xxxsen/yamdc/internal/model"
)

func TestFormatTimeToDate(t *testing.T) {
	tests := []struct {
		name     string
		ts       int64
		expected string
	}{
		{
			name:     "valid date",
			ts:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
			expected: "2023-06-15",
		},
		{
			name:     "epoch",
			ts:       0,
			expected: time.UnixMilli(0).Format(time.DateOnly),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeToDate(tt.ts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMetaToMovieNFO(t *testing.T) {
	tests := []struct {
		name string
		meta *model.MovieMeta
		check func(t *testing.T, meta *model.MovieMeta)
	}{
		{
			name: "full meta",
			meta: &model.MovieMeta{
				Title:           "Original Title",
				TitleTranslated: "Translated Title",
				Number:          "ABC-123",
				Plot:            "Some plot",
				PlotTranslated:  "Translated plot",
				Actors:          []string{"Alice", "Bob"},
				ReleaseDate:     time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
				Duration:        3600,
				Studio:          "StudioX",
				Label:           "LabelY",
				Series:          "SeriesZ",
				Genres:          []string{"Genre1", "Genre2"},
				Cover:           &model.File{Name: "cover.jpg"},
				Poster:          &model.File{Name: "poster.jpg"},
				SampleImages: []*model.File{
					{Name: "sample1.jpg"},
					{Name: "sample2.jpg"},
				},
			},
			check: func(t *testing.T, meta *model.MovieMeta) {
				mov := convertMetaToMovieNFO(meta)
				assert.Equal(t, "ABC-123", mov.ID)
				assert.Equal(t, "Translated Title", mov.Title)
				assert.Equal(t, "Original Title", mov.OriginalTitle)
				assert.Equal(t, "Translated Title", mov.TitleTranslated)
				assert.Equal(t, uint64(60), mov.Runtime)
				assert.Equal(t, 2023, mov.Year)
				assert.Equal(t, "StudioX", mov.Studio)
				assert.Equal(t, "LabelY", mov.Label)
				assert.Equal(t, "SeriesZ", mov.Set)
				assert.Len(t, mov.Actors, 2)
				assert.Equal(t, "Alice", mov.Actors[0].Name)
				assert.Len(t, mov.Genres, 2)
				assert.Equal(t, "poster.jpg", mov.Art.Poster)
				assert.Equal(t, "cover.jpg", mov.Cover)
				assert.Contains(t, mov.Art.Fanart, "poster.jpg")
				assert.Contains(t, mov.Art.Fanart, "cover.jpg")
				assert.Contains(t, mov.Art.Fanart, "sample1.jpg")
				assert.Contains(t, mov.Art.Fanart, "sample2.jpg")
			},
		},
		{
			name: "no translated title",
			meta: &model.MovieMeta{
				Title:  "Only Original",
				Number: "DEF-456",
			},
			check: func(t *testing.T, meta *model.MovieMeta) {
				mov := convertMetaToMovieNFO(meta)
				assert.Equal(t, "Only Original", mov.Title)
			},
		},
		{
			name: "nil cover and poster",
			meta: &model.MovieMeta{
				Title:  "Test",
				Number: "GHI-789",
			},
			check: func(t *testing.T, meta *model.MovieMeta) {
				mov := convertMetaToMovieNFO(meta)
				assert.Empty(t, mov.Art.Poster)
				assert.Empty(t, mov.Cover)
			},
		},
		{
			name: "empty actors",
			meta: &model.MovieMeta{
				Title:  "Test",
				Number: "JKL-012",
				Actors: nil,
			},
			check: func(t *testing.T, meta *model.MovieMeta) {
				mov := convertMetaToMovieNFO(meta)
				assert.Nil(t, mov.Actors)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.meta)
		})
	}
}
