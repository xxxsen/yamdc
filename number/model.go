package number

import (
	"strconv"
)

type externalField struct {
	isUncensor bool
	cat        string
}

type Number struct {
	numberId          string
	isChineseSubtitle bool
	isMultiCD         bool
	multiCDIndex      int
	is4k              bool
	is8k              bool
	isVr              bool
	isLeak            bool
	isHack            bool
	extField          externalField
}

func (n *Number) SetExternalFieldUncensor(v bool) {
	n.extField.isUncensor = v
}

func (n *Number) GetExternalFieldUncensor() bool {
	return n.extField.isUncensor
}

func (n *Number) SetExternalFieldCategory(cat string) {
	n.extField.cat = cat
}

func (n *Number) GetExternalFieldCategory() string {
	return n.extField.cat
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

func (n *Number) GetIs4K() bool {
	return n.is4k
}

func (n *Number) GetIs8K() bool {
	return n.is8k
}

func (n *Number) GetIsVR() bool {
	return n.isVr
}

func (n *Number) GetIsLeak() bool {
	return n.isLeak
}

func (n *Number) GetIsHack() bool {
	return n.isHack
}

func (n *Number) GenerateSuffix(base string) string {
	if n.GetIs4K() {
		base += "-" + defaultSuffix4K
	}
	if n.GetIs8K() {
		base += "-" + defaultSuffix8K
	}
	if n.GetIsVR() {
		base += "-" + defaultSuffixVR
	}
	if n.GetIsChineseSubtitle() {
		base += "-" + defaultSuffixChineseSubtitle
	}
	if n.GetIsLeak() {
		base += "-" + defaultSuffixLeak
	}
	if n.GetIsHack() {
		base += "-" + defaultSuffixHack2
	}
	if n.GetIsMultiCD() {
		base += "-" + defaultSuffixMultiCD + strconv.FormatInt(int64(n.GetMultiCDIndex()), 10)
	}
	return base
}

func (n *Number) GenerateTags() []string {
	rs := make([]string, 0, 5)
	if n.GetExternalFieldUncensor() {
		rs = append(rs, defaultTagUncensored)
	}
	if n.GetIsChineseSubtitle() {
		rs = append(rs, defaultTagChineseSubtitle)
	}
	if n.GetIs4K() {
		rs = append(rs, defaultTag4K)
	}
	if n.GetIs8K() {
		rs = append(rs, defaultTag8K)
	}
	if n.GetIsVR() {
		rs = append(rs, defaultTagVR)
	}
	if n.GetIsLeak() {
		rs = append(rs, defaultTagLeak)
	}
	if n.GetIsHack() {
		rs = append(rs, defaultTagHack)
	}
	return rs
}

func (n *Number) GenerateFileName() string {
	return n.GenerateSuffix(n.GetNumberID())
}
