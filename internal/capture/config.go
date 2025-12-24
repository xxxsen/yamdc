package capture

import (
	"fmt"
	"github.com/xxxsen/yamdc/internal/capture/ruleapi"
	"github.com/xxxsen/yamdc/internal/processor"
	"github.com/xxxsen/yamdc/internal/searcher"
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
	SaveDir                string
	Naming                 string
	ExtraMediaExtList      []string
	UncensorTester         ruleapi.ITester
	NumberRewriter         ruleapi.IRewriter
	NumberCategorier       ruleapi.IMatcher
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

func WithUncensorTester(t ruleapi.ITester) Option {
	return func(c *config) {
		c.UncensorTester = t
	}
}

func WithNumberRewriter(t ruleapi.IRewriter) Option {
	return func(c *config) {
		c.NumberRewriter = t
	}
}

func WithNumberCategorier(t ruleapi.IMatcher) Option {
	return func(c *config) {
		c.NumberCategorier = t
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
