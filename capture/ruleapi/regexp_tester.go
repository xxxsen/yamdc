package ruleapi

import (
	"regexp"
)

type RegexpTester struct {
	testList []*regexp.Regexp
}

func NewRegexpTester() *RegexpTester {
	return &RegexpTester{}
}

func (t *RegexpTester) AddRules(rules ...string) error {
	for _, rule := range rules {
		reg, err := regexp.Compile(rule)
		if err != nil {
			return err
		}
		t.testList = append(t.testList, reg)
	}
	return nil
}

func (t *RegexpTester) Test(res string) (bool, error) {
	for _, tester := range t.testList {
		if tester.MatchString(res) {
			return true, nil
		}
	}
	return false, nil
}
