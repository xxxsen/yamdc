package yaml

import "embed"

//go:embed *.yaml
var assets embed.FS

func ReadFile(name string) ([]byte, error) {
	return assets.ReadFile(name)
}
