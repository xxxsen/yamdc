package decoder

import "strconv"

type StringParseFunc func(v string) string
type StringListParseFunc func(v []string) []string
type NumberParseFunc func(v string) int64

type config struct {
	OnNumberParse              StringParseFunc
	OnTitleParse               StringParseFunc
	OnPlotParse                StringParseFunc
	OnActorListParse           StringListParseFunc
	OnReleaseDateParse         NumberParseFunc
	OnDurationParse            NumberParseFunc
	OnStudioParse              StringParseFunc
	OnLabelParse               StringParseFunc
	OnSeriesParse              StringParseFunc
	OnGenreListParse           StringListParseFunc
	OnCoverParse               StringParseFunc
	OnDirectorParse            StringParseFunc
	OnPosterParse              StringParseFunc
	OnSampleImageListParse     StringListParseFunc
	DefaultStringProcessor     StringParseFunc
	DefaultStringListProcessor StringListParseFunc
}

func defaultStringParser(v string) string {
	return v
}

func defaultStringListParser(vs []string) []string {
	return vs
}

func defaultNumberParser(v string) int64 {
	res, _ := strconv.ParseInt(v, 10, 64)
	return res
}

func defaultStringProcessor(v string) string {
	return v
}

func defaultStringListProcessor(vs []string) []string {
	return vs
}

type Option func(c *config)

func WithDefaultStringProcessor(p StringParseFunc) Option {
	return func(c *config) {
		c.DefaultStringProcessor = p
	}
}

func WithDefaultStringListProcessor(p StringListParseFunc) Option {
	return func(c *config) {
		c.DefaultStringListProcessor = p
	}
}

func WithSampleImageListParser(p StringListParseFunc) Option {
	return func(c *config) {
		c.OnSampleImageListParse = p
	}
}

func WithPosterParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnPosterParse = p
	}
}

func WithCoverParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnCoverParse = p
	}
}

func WithGenreListParser(p StringListParseFunc) Option {
	return func(c *config) {
		c.OnGenreListParse = p
	}
}

func WithSeriesParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnSeriesParse = p
	}
}

func WithLabelParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnLabelParse = p
	}
}

func WithStudioParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnStudioParse = p
	}
}

func WithDurationParser(p NumberParseFunc) Option {
	return func(c *config) {
		c.OnDurationParse = p
	}
}

func WithReleaseDateParser(p NumberParseFunc) Option {
	return func(c *config) {
		c.OnReleaseDateParse = p
	}
}

func WithActorListParser(p StringListParseFunc) Option {
	return func(c *config) {
		c.OnActorListParse = p
	}
}

func WithPlotParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnPlotParse = p
	}
}

func WithTitleParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnTitleParse = p
	}
}

func WithNumberParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnNumberParse = p
	}
}

func WithDirectorParser(p StringParseFunc) Option {
	return func(c *config) {
		c.OnDirectorParse = p
	}
}
