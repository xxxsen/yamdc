package ruleapi

import (
	"regexp"
)

type RegexpRewriteRule struct {
	Rule    string
	Rewrite string
}

type regexpRewriteItem struct {
	reg     *regexp.Regexp
	rewrite string
}

type RegexpRewriter struct {
	rewriteList []*regexpRewriteItem
}

func NewRegexpRewriter() *RegexpRewriter {
	return &RegexpRewriter{}
}

func (r *RegexpRewriter) AddRules(rs ...RegexpRewriteRule) error {
	for _, rule := range rs {
		reg, err := regexp.Compile(rule.Rule)
		if err != nil {
			return err
		}
		r.rewriteList = append(r.rewriteList, &regexpRewriteItem{
			reg:     reg,
			rewrite: rule.Rewrite,
		})
	}
	return nil
}
func (r *RegexpRewriter) Rewrite(res string) (string, error) {
	for _, rewriter := range r.rewriteList {
		res = rewriter.reg.ReplaceAllString(res, rewriter.rewrite)
	}
	return res, nil
}
