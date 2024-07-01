package number

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNumber(t *testing.T) {
	checkList := map[string]*Info{
		"HEYZO-3332.mp4": {
			Number:          "HEYZO-3332",
			IsUncensorMovie: true,
		},
		"052624_01.mp4": {
			Number:          "052624_01",
			IsUncensorMovie: true,
		},
		"052624_01-C.mp4": {
			Number:            "052624_01",
			IsChineseSubtitle: true,
			IsUncensorMovie:   true,
		},
		"052624_01-CD2.mp4": {
			Number:          "052624_01",
			IsUncensorMovie: true,
			IsMultiCD:       true,
			MultiCDIndex:    2,
		},
		"052624_01-CD3-C.mp4": {
			Number:            "052624_01",
			IsUncensorMovie:   true,
			IsMultiCD:         true,
			MultiCDIndex:      3,
			IsChineseSubtitle: true,
		},
	}
	for file, info := range checkList {
		rs, err := ParseWithFileName(file)
		assert.NoError(t, err)
		assert.Equal(t, info.Number, rs.Number)
		assert.Equal(t, info.IsChineseSubtitle, rs.IsChineseSubtitle)
		assert.Equal(t, info.IsMultiCD, rs.IsMultiCD)
		assert.Equal(t, info.MultiCDIndex, rs.MultiCDIndex)
		assert.Equal(t, info.IsUncensorMovie, rs.IsUncensorMovie)
	}
}
