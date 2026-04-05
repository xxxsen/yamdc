package register

import (
	_ "github.com/xxxsen/yamdc/internal/searcher/plugin/impl"
	_ "github.com/xxxsen/yamdc/internal/searcher/plugin/impl/airav"
	"github.com/xxxsen/yamdc/internal/searcher/plugin/yamlplugin"
)

func init() {
	yamlplugin.RegisterBuiltins()
}
