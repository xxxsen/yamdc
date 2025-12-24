package api

import "math/rand"

func SelectDomain(in []string) (string, bool) {
	if len(in) == 0 {
		return "", false
	}
	if len(in) == 1 {
		return in[0], true
	}
	return in[rand.Int()%len(in)], true
}

func MustSelectDomain(in []string) string {
	res, ok := SelectDomain(in)
	if !ok {
		panic("unable to select domain")
	}
	return res
}
