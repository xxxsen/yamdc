package number

import (
	"strconv"

	"github.com/xxxsen/yamdc/internal/tag"
)

type externalField struct {
	isUnrated bool
	cat       string
}

type Number struct {
	numberID          string
	isChineseSubtitle bool
	isMultiCD         bool
	multiCDIndex      int
	is4k              bool
	is8k              bool
	isVr              bool
	isSpecialEdition  bool
	isRestored        bool
	extField          externalField
}

func (n *Number) SetExternalFieldUnrated(v bool) {
	n.extField.isUnrated = v
}

func (n *Number) GetExternalFieldUnrated() bool {
	return n.extField.isUnrated
}

func (n *Number) SetExternalFieldCategory(cat string) {
	n.extField.cat = cat
}

func (n *Number) GetExternalFieldCategory() string {
	return n.extField.cat
}

func (n *Number) GetNumberID() string {
	return n.numberID
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

func (n *Number) GetIsSpecialEdition() bool {
	return n.isSpecialEdition
}

func (n *Number) GetIsRestored() bool {
	return n.isRestored
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
	if n.GetIsSpecialEdition() {
		base += "-" + defaultSuffixSpecialEdition
	}
	if n.GetIsRestored() {
		base += "-" + defaultSuffixRestored2
	}
	if n.GetIsMultiCD() {
		base += "-" + defaultSuffixMultiCD + strconv.FormatInt(int64(n.GetMultiCDIndex()), 10)
	}
	return base
}

func (n *Number) GenerateTags() []string {
	rs := make([]string, 0, 5)
	if n.GetExternalFieldUnrated() {
		rs = append(rs, tag.Unrated)
	}
	if n.GetIsChineseSubtitle() {
		rs = append(rs, tag.ChineseSubtitle)
	}
	if n.GetIs4K() {
		rs = append(rs, tag.Res4K)
	}
	if n.GetIs8K() {
		rs = append(rs, tag.Res8K)
	}
	if n.GetIsVR() {
		rs = append(rs, tag.VR)
	}
	if n.GetIsSpecialEdition() {
		rs = append(rs, tag.SpecialEdition)
	}
	if n.GetIsRestored() {
		rs = append(rs, tag.Restored)
	}
	return rs
}

func (n *Number) GenerateFileName() string {
	return n.GenerateSuffix(n.GetNumberID())
}
