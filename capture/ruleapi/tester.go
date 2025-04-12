package ruleapi

type TesterFunc func(res string) (bool, error)

type ITester interface {
	Test(res string) (bool, error)
}

type fnTesterWrap struct {
	fn TesterFunc
}

func WrapFuncAsTester(in TesterFunc) ITester {
	return &fnTesterWrap{fn: in}
}
func (f *fnTesterWrap) Test(res string) (bool, error) {
	return f.fn(res)
}
