package domain

import "errors"

var (
	ErrPluginNotFound       = errors.New("plugin not found")
	ErrRuleSourceNotFound   = errors.New("rule source not found")
	ErrBundleSourceNotFound = errors.New("bundle source not found")
	ErrNoPluginSources      = errors.New("no searcher plugin sources configured")
)
