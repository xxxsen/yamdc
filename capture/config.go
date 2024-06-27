package capture

import (
	"av-capture/processor"
	"av-capture/searcher"
)

const (
	NamingReleaseDate  = "{DATE}"
	NamingReleaseYear  = "{YEAR}"
	NamingReleaseMonth = "{MONTH}"
	NamingActor        = "{ACTOR}"
	NamingNumber       = "{NUMBER}"
)

const (
	defaultNamingRule = NamingReleaseYear + "/" + NamingActor + "/" + NamingNumber
)

type config struct {
	ScanDir   string
	Searcher  searcher.ISearcher
	Processor processor.IProcessor
	SaveDir   string
	Naming    string
}

type Option func(c *config)

func WithScanDir(dir string) Option {
	return func(c *config) {
		c.ScanDir = dir
	}
}

func WithSaveDir(dir string) Option {
	return func(c *config) {
		c.SaveDir = dir
	}
}

func WithSeacher(ss searcher.ISearcher) Option {
	return func(c *config) {
		c.Searcher = ss
	}
}

func WithProcessor(p processor.IProcessor) Option {
	return func(c *config) {
		c.Processor = p
	}
}

func WithNamingRule(r string) Option {
	return func(c *config) {
		c.Naming = r
	}
}
