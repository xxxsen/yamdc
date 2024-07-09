package utils

import "strings"

func DedupStringList(in []string) []string {
	rs := make([]string, 0, len(in))
	exist := make(map[string]struct{})
	for _, item := range in {
		if _, ok := exist[item]; ok {
			continue
		}
		exist[item] = struct{}{}
		rs = append(rs, item)
	}
	return rs
}

func StringListToLower(in []string) []string {
	rs := make([]string, 0, len(in))
	for _, item := range in {
		rs = append(rs, strings.ToLower(item))
	}
	return rs
}

func StringListToSet(in []string) map[string]struct{} {
	rs := make(map[string]struct{}, len(in))
	for _, item := range in {
		rs[item] = struct{}{}
	}
	return rs
}
