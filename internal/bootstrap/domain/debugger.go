package domain

import (
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
)

func BuildSearcherDebugger(
	cli client.IHTTPClient,
	storage store.IStorage,
	cleaner movieidcleaner.Cleaner,
	plugins []string,
	categoryPlugins map[string][]string,
) *searcher.Debugger {
	return searcher.NewDebugger(cli, storage, cleaner, plugins, categoryPlugins)
}
