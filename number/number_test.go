package number

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNumber(t *testing.T) {
	checkList := map[string]*Number{
		"HEYZO-3332.mp4": {
			number:          "HEYZO-3332",
			isUncensorMovie: true,
		},
		"052624_01.mp4": {
			number:          "052624_01",
			isUncensorMovie: true,
		},
		"052624_01-C.mp4": {
			number:            "052624_01",
			isChineseSubtitle: true,
			isUncensorMovie:   true,
		},
		"052624_01-CD2.mp4": {
			number:          "052624_01",
			isUncensorMovie: true,
			isMultiCD:       true,
			multiCDIndex:    2,
		},
		"052624_01-CD3-C.mp4": {
			number:            "052624_01",
			isUncensorMovie:   true,
			isMultiCD:         true,
			multiCDIndex:      3,
			isChineseSubtitle: true,
		},
		"052624_01_cd3_c.mp4": {
			number:            "052624_01",
			isUncensorMovie:   true,
			isMultiCD:         true,
			multiCDIndex:      3,
			isChineseSubtitle: true,
		},
	}
	for file, info := range checkList {
		rs, err := ParseWithFileName(file)
		assert.NoError(t, err)
		assert.Equal(t, info.Number(), rs.Number())
		assert.Equal(t, info.IsChineseSubtitle(), rs.IsChineseSubtitle())
		assert.Equal(t, info.IsMultiCD(), rs.IsMultiCD())
		assert.Equal(t, info.MultiCDIndex(), rs.MultiCDIndex())
		assert.Equal(t, info.IsUncensorMovie(), rs.IsUncensorMovie())
	}
}
