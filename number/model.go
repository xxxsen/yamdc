package number

type Number struct {
	number            string
	isChineseSubtitle bool
	isMultiCD         bool
	multiCDIndex      int
	isUncensorMovie   bool
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
