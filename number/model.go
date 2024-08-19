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

func (n *Number) GetNumber() string {
	return n.number
}

func (n *Number) GetIsChineseSubtitle() bool {
	return n.isChineseSubtitle
}

func (n *Number) GetIsMultiCD() bool {
	return n.isMultiCD
}

func (n *Number) GetMultiCDIndex() int {
	return n.multiCDIndex
}

func (n *Number) GetIsUncensorMovie() bool {
	return n.isUncensorMovie
}

func (n *Number) GetIs4K() bool {
	return n.is4k
}

func (n *Number) GetIsLeak() bool {
	return n.isLeak
}

func (n *Number) GenerateSuffix(base string) string {
	if n.GetIs4K() {
		base += "-4K"
	}
	if n.GetIsChineseSubtitle() {
		base += "-C"
	}
	if n.GetIsLeak() {
		base += "-LEAK"
	}
	if n.GetIsMultiCD() {
		base += "-CD" + strconv.FormatInt(int64(n.GetMultiCDIndex()), 10)
	}
	return base
}
