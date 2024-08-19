package number

import "strconv"

type Number struct {
	number            string
	isChineseSubtitle bool
	isMultiCD         bool
	multiCDIndex      int
	isUncensorMovie   bool
	is4k              bool
	isLeak            bool
}

func (n *Number) Number() string {
	return n.number
}

func (n *Number) IsChineseSubtitle() bool {
	return n.isChineseSubtitle
}

func (n *Number) IsMultiCD() bool {
	return n.isMultiCD
}

func (n *Number) MultiCDIndex() int {
	return n.multiCDIndex
}

func (n *Number) IsUncensorMovie() bool {
	return n.isUncensorMovie
}

func (n *Number) Is4K() bool {
	return n.is4k
}

func (n *Number) IsLeak() bool {
	return n.isLeak
}

func (n *Number) GenerateSuffix(base string) string {
	if n.Is4K() {
		base += "-4K"
	}
	if n.IsChineseSubtitle() {
		base += "-C"
	}
	if n.IsLeak() {
		base += "-LEAK"
	}
	if n.IsMultiCD() {
		base += "-CD" + strconv.FormatInt(int64(n.MultiCDIndex()), 10)
	}
	return base
}
