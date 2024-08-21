package number

import (
	"strconv"
)

type Number struct {
	numberId          string
	isChineseSubtitle bool
	isMultiCD         bool
	multiCDIndex      int
	isUncensorMovie   bool
	is4k              bool
	isLeak            bool
	cat               Category
}

func (n *Number) GetCategory() Category {
	return n.cat
}

func (n *Number) GetNumberID() string {
	return n.numberId
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
		base += "-" + defaultSuffix4K
	}
	if n.GetIsChineseSubtitle() {
		base += "-" + defaultSuffixChineseSubtitle
	}
	if n.GetIsLeak() {
		base += "-" + defaultSuffixLeak
	}
	if n.GetIsMultiCD() {
		base += "-" + defaultSuffixMultiCD + strconv.FormatInt(int64(n.GetMultiCDIndex()), 10)
	}
	return base
}

func (n *Number) GenerateTags() []string {
	rs := make([]string, 0, 5)
	if n.GetIsUncensorMovie() {
		rs = append(rs, defaultTagUncensored)
	}
	if n.GetIsChineseSubtitle() {
		rs = append(rs, defaultTagChineseSubtitle)
	}
	if n.GetIs4K() {
		rs = append(rs, defaultTag4K)
	}
	if n.GetIsLeak() {
		rs = append(rs, defaultTagLeak)
	}
	return rs
}

func (n *Number) GenerateFileName() string {
	return n.GenerateSuffix(n.GetNumberID())
}
