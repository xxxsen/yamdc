package ruleapi

type IRewriter interface {
	Rewrite(res string) (string, error)
}
