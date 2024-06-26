package utils

import "bytes"

func BuildAuthorsName(acts []string, maxLength int) string {
	buf := bytes.NewBuffer(nil)
	for idx, item := range acts {
		if idx != 0 {
			buf.WriteString(",")
		}
		if buf.Len()+1+len(item) > maxLength {
			break
		}
		buf.WriteString(item)
	}
	return buf.String()
}
