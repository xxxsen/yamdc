package utils

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
