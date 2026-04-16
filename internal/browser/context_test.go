package browser

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithParams_GetParams(t *testing.T) {
	tests := []struct {
		name   string
		params *Params
	}{
		{
			name:   "nil params returns nil",
			params: nil,
		},
		{
			name: "round-trip params",
			params: &Params{
				WaitSelector:       "//div",
				WaitTimeout:        5 * time.Second,
				WaitStableDuration: 3 * time.Second,
				Cookies: []*http.Cookie{
					{Name: "a", Value: "1"},
				},
				Headers: http.Header{"X-H": []string{"v"}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.params != nil {
				ctx = WithParams(ctx, tc.params)
			}
			got := GetParams(ctx)
			if tc.params == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tc.params.WaitSelector, got.WaitSelector)
				assert.Equal(t, tc.params.WaitTimeout, got.WaitTimeout)
				assert.Equal(t, tc.params.WaitStableDuration, got.WaitStableDuration)
				assert.Equal(t, tc.params.Cookies, got.Cookies)
				assert.Equal(t, tc.params.Headers, got.Headers)
			}
		})
	}
}

func TestGetParams_EmptyContext(t *testing.T) {
	got := GetParams(context.Background())
	assert.Nil(t, got)
}

func TestWithParams_Overwrite(t *testing.T) {
	p1 := &Params{WaitSelector: "//a"}
	p2 := &Params{WaitSelector: "//b"}
	ctx := WithParams(context.Background(), p1)
	ctx = WithParams(ctx, p2)
	got := GetParams(ctx)
	require.NotNil(t, got)
	assert.Equal(t, "//b", got.WaitSelector)
}
