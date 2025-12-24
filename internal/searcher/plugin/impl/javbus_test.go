package impl

import (
	"context"
	"testing"
	"yamdc/internal/number"
	"yamdc/internal/searcher"

	"github.com/stretchr/testify/assert"
)

func TestJavbus(t *testing.T) {
	ss, err := searcher.NewDefaultSearcher("test", &javbus{})
	assert.NoError(t, err)
	ctx := context.Background()
	num, err := number.Parse("STZY-015")
	assert.NoError(t, err)
	meta, ok, err := ss.Search(ctx, num)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "STZY-015", meta.Number)
	assert.Equal(t, 3900, int(meta.Duration))
	assert.Equal(t, 1742256000000, int(meta.ReleaseDate))
	assert.Equal(t, 1, len(meta.Actors))
	assert.True(t, len(meta.Title) > 0)
	assert.True(t, len(meta.Plot) > 0)
	assert.True(t, len(meta.Series) > 0)
	t.Logf("data:%+v", *meta)
}
