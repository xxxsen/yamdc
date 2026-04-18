package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/review"
)

func TestNewAPI(t *testing.T) {
	stubReview := review.NewService(nil, nil, nil, nil, nil)
	api := NewAPI(nil, nil, nil, stubReview, "/tmp/save", nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, api)
	assert.Equal(t, "/tmp/save", api.saveDir)
	assert.Same(t, stubReview, api.reviewSvc)
}

// TestNewAPIPanicsWithoutReviewService: 生产路径 reviewSvc 必填,
// 构造时 nil 要立刻 panic 而不是等到 /api/review/* 路由被调时才 nil-deref。
func TestNewAPIPanicsWithoutReviewService(t *testing.T) {
	require.PanicsWithValue(t, "web.NewAPI: reviewSvc is required", func() {
		NewAPI(nil, nil, nil, nil, "/tmp/save", nil, nil, nil, nil, nil, nil, nil)
	})
}
