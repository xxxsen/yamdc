package capture

import (
	"fmt"

	"github.com/xxxsen/yamdc/internal/numbercleaner"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
)

const (
	NamingReleaseDate     = "DATE"
	NamingReleaseYear     = "YEAR"
	NamingReleaseMonth    = "MONTH"
	NamingActor           = "ACTOR"
	NamingNumber          = "NUMBER"
	NamingTitle           = "TITLE"
	NamingTitleTranslated = "TITLE_TRANSLATED"
)

var (
	defaultNamingRule = fmt.Sprintf("{%s}/{%s}", NamingReleaseYear, NamingNumber)
)

type config struct {
	ScanDir                string
	Searcher               searcher.ISearcher
	Processor              processor.IProcessor
	Storage                store.IStorage
	SaveDir                string
	Naming                 string
	ExtraMediaExtList      []string
	NumberCleaner          numbercleaner.Cleaner
	DiscardTranslatedTitle bool
	DiscardTranslatedPlot  bool
	LinkMode               bool
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

func WithStorage(s store.IStorage) Option {
	return func(c *config) {
		c.Storage = s
	}
}

func WithNamingRule(r string) Option {
	return func(c *config) {
		c.Naming = r
	}
}

func WithExtraMediaExtList(lst []string) Option {
	return func(c *config) {
		c.ExtraMediaExtList = lst
	}
}

func WithNumberCleaner(cl numbercleaner.Cleaner) Option {
	return func(c *config) {
		c.NumberCleaner = cl
	}
}

func WithTransalteTitleDiscard(v bool) Option {
	return func(c *config) {
		c.DiscardTranslatedTitle = v
	}
}

func WithTranslatedPlotDiscard(v bool) Option {
	return func(c *config) {
		c.DiscardTranslatedPlot = v
	}
}

func WithLinkMode(v bool) Option {
	return func(c *config) {
		c.LinkMode = v
	}
}

func (c *Capture) ScanDir() string {
	if c == nil || c.c == nil {
		return ""
	}
	return c.c.ScanDir
}
