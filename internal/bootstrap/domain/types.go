package domain

type CategoryPlugin struct {
	Name    string
	Plugins []string
}

type PluginSource struct {
	SourceType string
	Location   string
}

type PluginOption struct {
	Disable bool
}

type HandlerOption struct {
	Disable bool
	Args    interface{}
}
