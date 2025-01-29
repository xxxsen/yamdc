package ruleapi

type ITester interface {
	Test(res string) (bool, error)
}
