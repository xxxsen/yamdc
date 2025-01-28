package ruleapi

type RegexpMatchRule struct {
	Regexp []string
	Match  string
}

type regexpMatchItem struct {
	reg   ITester
	match string
}

type RegexpMatcher struct {
	matchList []*regexpMatchItem
}

func NewRegexpMatcher() *RegexpMatcher {
	return &RegexpMatcher{}
}

func (m *RegexpMatcher) AddRules(rules ...RegexpMatchRule) error {
	for _, rule := range rules {
		t := NewRegexpTester()
		if err := t.AddRules(rule.Regexp...); err != nil {
			return err
		}
		m.matchList = append(m.matchList, &regexpMatchItem{
			reg:   t,
			match: rule.Match,
		})
	}
	return nil
}

func (m *RegexpMatcher) Match(res string) (string, bool, error) {
	for _, matcher := range m.matchList {
		ok, err := matcher.reg.Test(res)
		if err != nil {
			return "", false, err
		}
		if ok {
			return matcher.match, true, nil
		}
	}
	return "", false, nil
}
