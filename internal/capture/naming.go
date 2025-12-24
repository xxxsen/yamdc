package capture

import "bytes"

const (
	defaultNonActorName     = "佚名"
	defaultMultiActorLimit  = 3
	defaultMultiActorAsName = "多人作品"
	defaultMaxItemCharactor = 200
)

func buildAuthorsName(acts []string) string {
	if len(acts) == 0 {
		return defaultNonActorName
	}
	if len(acts) >= defaultMultiActorLimit {
		return defaultMultiActorAsName
	}
	buf := bytes.NewBuffer(nil)
	for idx, item := range acts {
		if idx != 0 {
			buf.WriteString(",")
		}
		if buf.Len()+1+len(item) > defaultMaxItemCharactor {
			break
		}
		buf.WriteString(item)
	}
	return buf.String()
}

func buildTitle(title string) string {
	if len(title) > defaultMaxItemCharactor {
		return title[:defaultMaxItemCharactor]
	}
	return title
}
