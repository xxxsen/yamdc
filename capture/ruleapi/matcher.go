package ruleapi

type MatcherFunc func(res string) (string, bool, error)

type IMatcher interface {
	Match(res string) (string, bool, error)
}

type fnMatcherWrap struct {
	fn MatcherFunc
}

func WrapFuncAsMatcher(in MatcherFunc) IMatcher {
	return &fnMatcherWrap{fn: in}
}

func (f *fnMatcherWrap) Match(res string) (string, bool, error) {
	return f.fn(res)
}
