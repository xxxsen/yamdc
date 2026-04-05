package yamlitest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/factory"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/yamlplugin"
	"github.com/xxxsen/yamdc/internal/store"
	_ "github.com/xxxsen/yamdc/internal/searcher/plugin/register"
)

func runPluginComparison(t *testing.T, pluginName string, numbers []string) {
	t.Helper()
	if len(numbers) == 0 {
		t.Skipf("%s integration numbers not configured", pluginName)
	}

	httpCli := client.MustNewClient()
	for _, rawNumber := range numbers {
		rawNumber := rawNumber
		t.Run(rawNumber, func(t *testing.T) {
			num, err := number.Parse(rawNumber)
			require.NoError(t, err)

			legacyPlugin, err := factory.CreatePlugin(yamlplugin.LegacyPluginName(pluginName), struct{}{})
			require.NoError(t, err)
			yamlPlugin, err := factory.CreatePlugin(pluginName, struct{}{})
			require.NoError(t, err)

			legacySearcher, err := searcher.NewDefaultSearcher(pluginName, legacyPlugin, searcher.WithHTTPClient(httpCli), searcher.WithStorage(store.NewMemStorage()), searcher.WithSearchCache(false))
			require.NoError(t, err)
			yamlSearcher, err := searcher.NewDefaultSearcher(pluginName, yamlPlugin, searcher.WithHTTPClient(httpCli), searcher.WithStorage(store.NewMemStorage()), searcher.WithSearchCache(false))
			require.NoError(t, err)

			legacyMeta, legacyFound, err := legacySearcher.Search(context.Background(), num)
			require.NoError(t, err)
			yamlMeta, yamlFound, err := yamlSearcher.Search(context.Background(), num)
			require.NoError(t, err)

			require.Equal(t, legacyFound, yamlFound, "found mismatch")
			if !legacyFound {
				return
			}

			require.Equal(t, canonicalizeMeta(legacyMeta), canonicalizeMeta(yamlMeta))
		})
	}
}

func canonicalizeMeta(in *model.MovieMeta) *model.MovieMeta {
	if in == nil {
		return nil
	}
	out := *in
	out.ExtInfo = model.ExtInfo{}
	if out.Cover != nil {
		cover := *out.Cover
		cover.Key = ""
		out.Cover = &cover
	}
	if out.Poster != nil {
		poster := *out.Poster
		poster.Key = ""
		out.Poster = &poster
	}
	if len(out.SampleImages) != 0 {
		items := make([]*model.File, 0, len(out.SampleImages))
		for _, item := range out.SampleImages {
			if item == nil {
				continue
			}
			cp := *item
			cp.Key = ""
			items = append(items, &cp)
		}
		out.SampleImages = items
	}
	return &out
}

