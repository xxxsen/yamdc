package ruleapi

type RewriterFunc func(res string) (string, error)

type IRewriter interface {
	Rewrite(res string) (string, error)
}

type fnRewriterWrap struct {
	fn RewriterFunc
}

func WrapFuncAsRewriter(in RewriterFunc) IRewriter {
	return &fnRewriterWrap{fn: in}
}

func (f *fnRewriterWrap) Rewrite(res string) (string, error) {
	return f.fn(res)
}
