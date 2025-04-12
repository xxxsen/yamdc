package dynscript

import "strings"

func rewriteTabToSpace(input string) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		i := 0
		for i < len(line) && line[i] == '\t' {
			i++
		}
		newIndent := strings.Repeat("    ", i)
		result = append(result, newIndent+line[i:])
	}

	return strings.Join(result, "\n")
}
