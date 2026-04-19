package searcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xxxsen/yamdc/internal/store"
)

func TestApplyOpts(t *testing.T) {
	tests := []struct {
		name        string
		opts        []Option
		expectCache bool
	}{
		{name: "empty_options", opts: nil, expectCache: false},
		{name: "with_search_cache", opts: []Option{WithSearchCache(true)}, expectCache: true},
		{name: "with_all_options", opts: []Option{
			WithHTTPClient(&mockHTTPClient{}),
			WithStorage(store.NewMemStorage()),
			WithSearchCache(true),
		}, expectCache: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := applyOpts(tt.opts...)
			assert.Equal(t, tt.expectCache, c.searchCache)
		})
	}
}
