package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
)

func TestNumberTitleHandler(t *testing.T) {
	tests := []struct {
		name      string
		numberID  string
		title     string
		wantTitle string
	}{
		{
			name:      "title already contains number",
			numberID:  "ABC-123",
			title:     "ABC-123 Some Movie Title",
			wantTitle: "ABC-123 Some Movie Title",
		},
		{
			name:      "title does not contain number",
			numberID:  "ABC-123",
			title:     "Some Movie Title",
			wantTitle: "ABC-123 Some Movie Title",
		},
		{
			name:      "title contains clean number form",
			numberID:  "ABC-123",
			title:     "ABC123 Movie",
			wantTitle: "ABC123 Movie",
		},
		{
			name:      "empty title",
			numberID:  "DEF-456",
			title:     "",
			wantTitle: "DEF-456 ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &numberTitleHandler{}
			num, err := number.Parse(tt.numberID)
			require.NoError(t, err)
			fc := &model.FileContext{
				Number: num,
				Meta:   &model.MovieMeta{Title: tt.title},
			}
			err = h.Handle(context.Background(), fc)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTitle, fc.Meta.Title)
		})
	}
}
