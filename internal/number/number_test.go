package number

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/tag"
)

func TestNumber(t *testing.T) {
	checkList := map[string]*Number{
		"DEMO-3332.mp4": {
			numberID: "DEMO-3332",
		},
		"052624_01.mp4": {
			numberID: "052624_01",
		},
		"052624_01-C.mp4": {
			numberID:          "052624_01",
			isChineseSubtitle: true,
		},
		"052624_01-CD2.mp4": {
			numberID:     "052624_01",
			isMultiCD:    true,
			multiCDIndex: 2,
		},
		"052624_01-CD3-C.mp4": {
			numberID:          "052624_01",
			isMultiCD:         true,
			multiCDIndex:      3,
			isChineseSubtitle: true,
		},
		"052624_01_cd3_c.mp4": {
			numberID:          "052624_01",
			isMultiCD:         true,
			multiCDIndex:      3,
			isChineseSubtitle: true,
		},
		"k0009-c_cd1-4k.mp4": {
			numberID:          "K0009",
			isMultiCD:         true,
			multiCDIndex:      1,
			isChineseSubtitle: true,
			is4k:              true,
		},
		"n001-Cd1-4k.mp4": {
			numberID:     "N001",
			isMultiCD:    true,
			multiCDIndex: 1,
			is4k:         true,
		},
		"c-4k.mp4": {
			numberID:          "C",
			isChineseSubtitle: false,
			is4k:              true,
		},
		"-c-4k.mp4": {
			numberID:          "",
			isChineseSubtitle: true,
			is4k:              true,
		},
		"abc-leak-c.mp4": {
			numberID:          "ABC",
			isSpecialEdition:  true,
			isChineseSubtitle: true,
		},
		"xyz-8k-vr.mp4": {
			numberID: "XYZ",
			is8k:     true,
			isVr:     true,
		},
		"hack1-u.mp4": {
			numberID:   "HACK1",
			isRestored: true,
		},
		"hack2-uc.mp4": {
			numberID:   "HACK2",
			isRestored: true,
		},
		"uhd-2160p.mp4": {
			numberID: "UHD",
			is4k:     true,
		},
		"badcd-cdxx.mp4": {
			numberID: "BADCD-CDXX",
		},
	}
	for file, info := range checkList {
		rs, err := ParseWithFileName(file)
		assert.NoError(t, err)
		assert.Equal(t, info.GetNumberID(), rs.GetNumberID())
		assert.Equal(t, info.GetIsChineseSubtitle(), rs.GetIsChineseSubtitle())
		assert.Equal(t, info.GetIsMultiCD(), rs.GetIsMultiCD())
		assert.Equal(t, info.GetMultiCDIndex(), rs.GetMultiCDIndex())
		assert.Equal(t, info.GetIs4K(), rs.GetIs4K())
		assert.Equal(t, info.GetIs8K(), rs.GetIs8K())
		assert.Equal(t, info.GetIsVR(), rs.GetIsVR())
		assert.Equal(t, info.GetIsSpecialEdition(), rs.GetIsSpecialEdition())
		assert.Equal(t, info.GetIsRestored(), rs.GetIsRestored())
	}
}

func TestAlnumber(t *testing.T) {
	assert.Equal(t, "movie12345", GetCleanID("movie-12345"))
	assert.Equal(t, "", GetCleanID(""))
	assert.Equal(t, "AB", GetCleanID("A_B"))
}

func TestSetFiledByExternal(t *testing.T) {
	n, err := Parse("abc-123")
	assert.NoError(t, err)
	n.SetExternalFieldUncensor(true)
	n.SetExternalFieldCategory("abc")
	assert.Equal(t, "abc", n.GetExternalFieldCategory())
	assert.True(t, n.GetExternalFieldUncensor())
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	_, err := Parse("")
	assert.ErrorIs(t, err, errEmptyNumberStr)

	_, err = Parse("has.dot")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errContainsExtName))
}

func TestParseWithFileNameEmptyBase(t *testing.T) {
	t.Parallel()
	// Base name ".hidden" yields empty stem before extension handling in ParseWithFileName
	_, err := ParseWithFileName(".mp4")
	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptyNumberStr)
}

func TestGenerateSuffixTagsFileName(t *testing.T) {
	n, err := ParseWithFileName("id-4k-8k-vr-leak-uc-cd7-c.mp4")
	require.NoError(t, err)
	assert.Equal(t, "ID", n.GetNumberID())
	assert.True(t, n.GetIs4K())
	assert.True(t, n.GetIs8K())
	assert.True(t, n.GetIsVR())
	assert.True(t, n.GetIsSpecialEdition())
	assert.True(t, n.GetIsRestored())
	assert.True(t, n.GetIsChineseSubtitle())
	assert.True(t, n.GetIsMultiCD())
	assert.Equal(t, 7, n.GetMultiCDIndex())

	wantSuffix := "ID-4K-8K-VR-C-LEAK-UC-CD7"
	assert.Equal(t, wantSuffix, n.GenerateSuffix(n.GetNumberID()))
	assert.Equal(t, wantSuffix, n.GenerateFileName())

	n.SetExternalFieldUncensor(true)
	tags := n.GenerateTags()
	assert.Contains(t, tags, tag.Unrated)
	assert.Contains(t, tags, tag.ChineseSubtitle)
	assert.Contains(t, tags, tag.Res4K)
	assert.Contains(t, tags, tag.Res8K)
	assert.Contains(t, tags, tag.VR)
	assert.Contains(t, tags, tag.SpecialEdition)
	assert.Contains(t, tags, tag.Restored)
}

func TestGenerateSuffixMinimal(t *testing.T) {
	n, err := Parse("plain")
	require.NoError(t, err)
	assert.Equal(t, "PLAIN", n.GenerateFileName())
	assert.Empty(t, n.GenerateTags())
}
