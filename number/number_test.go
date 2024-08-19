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
		"k0009-c_cd1-4k.mp4": {
			number:            "K0009",
			isUncensorMovie:   true,
			isMultiCD:         true,
			multiCDIndex:      1,
			isChineseSubtitle: true,
			is4k:              true,
		},
		"n001-Cd1-4k.mp4": {
			number:          "N001",
			isUncensorMovie: true,
			isMultiCD:       true,
			multiCDIndex:    1,
			is4k:            true,
		},
		"c-4k.mp4": {
			number:            "C",
			isChineseSubtitle: false,
			is4k:              true,
		},
		"-c-4k.mp4": {
			number:            "",
			isChineseSubtitle: true,
			is4k:              true,
		},
		"abc-leak-c.mp4": {
			number:            "ABC",
			isLeak:            true,
			isChineseSubtitle: true,
		},
	}
	for file, info := range checkList {
		rs, err := ParseWithFileName(file)
		assert.NoError(t, err)
		assert.Equal(t, info.GetNumber(), rs.GetNumber())
		assert.Equal(t, info.GetIsChineseSubtitle(), rs.GetIsChineseSubtitle())
		assert.Equal(t, info.GetIsMultiCD(), rs.GetIsMultiCD())
		assert.Equal(t, info.GetMultiCDIndex(), rs.GetMultiCDIndex())
		assert.Equal(t, info.GetIsUncensorMovie(), rs.GetIsUncensorMovie())
		assert.Equal(t, info.GetIs4K(), rs.GetIs4K())
	}
}
