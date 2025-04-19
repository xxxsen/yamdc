package handler

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCnumber(t *testing.T) {
	h := &chineseTitleTranslateOptimizer{}
	ctx := context.Background()
	{
		title, ok, err := h.readTitleFromCNumber(ctx, "012413-001")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "波多野结衣超豪华风俗服务", title)
	}
	{
		_, ok, err := h.readTitleFromCNumber(ctx, "111111")
		assert.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestYesJav100(t *testing.T) {
	h := &chineseTitleTranslateOptimizer{}
	ctx := context.Background()
	{
		title, ok, err := h.readTitleFromYesJav(ctx, "jur-036")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "JUR-036 和老公造人运动 但一直被公公内射 新妻优香", title)
	}
	{
		title, ok, err := h.readTitleFromYesJav(ctx, "DASS-541")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "DASS-541 人妻外卖妹与中年男性外遇 橘玛丽", title)
	}
	{
		title, ok, err := h.readTitleFromYesJav(ctx, "111111")
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "", title)
	}
}

func TestEscape(t *testing.T) {
	origin := "jur-036"
	e1 := url.QueryEscape(origin)
	e2 := url.PathEscape(origin)
	t.Logf("e1:%s, e2:%s", e1, e2)
}

func TestTitleExtract(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"[AI無碼] JUR-036 和老公造人运动 但一直被公公内射(中文字幕)", "JUR-036 和老公造人运动 但一直被公公内射"},
		{"JUR-224 『從周三開始，和妻子做愛』的自豪朋友，讓我每週五，每次射3-4發，總共射18發中出的那個妻子。 市來真尋 (中文字幕)", "JUR-224 『從周三開始，和妻子做愛』的自豪朋友，讓我每週五，每次射3-4發，總共射18發中出的那個妻子。 市來真尋"},
		{"SONE-669 鄰居団地妻在陽台晾襪的午後的訊息，是丈夫不在的信號 黑島玲衣 (中文字幕)", "SONE-669 鄰居団地妻在陽台晾襪的午後的訊息，是丈夫不在的信號 黑島玲衣"},
		{"[AI無碼] DASS-541 人妻外卖妹与中年男性外遇 橘玛丽 (中文字幕)", "DASS-541 人妻外卖妹与中年男性外遇 橘玛丽"},
	}
	for _, tst := range tests {
		t.Run(tst.input, func(t *testing.T) {
			h := chineseTitleTranslateOptimizer{}
			result := h.cleanSearchTitle(tst.input)
			assert.Equal(t, tst.expected, result)
		})
	}
}
