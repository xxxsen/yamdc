package ruleapi

type IMatcher interface {
	Match(res string) (string, bool, error)
}
