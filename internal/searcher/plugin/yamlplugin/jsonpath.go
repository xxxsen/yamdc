package yamlplugin

import (
	"encoding/json"
	"fmt"
	"strconv"

	paesslerjsonpath "github.com/PaesslerAG/jsonpath"
)

func evalJSONPathStrings(doc any, expr string) ([]string, error) {
	value, err := paesslerjsonpath.Get(expr, doc)
	if err != nil {
		return nil, fmt.Errorf("eval jsonpath failed, err:%w", err)
	}
	out := make([]string, 0)
	flattenJSONPathValue(value, &out)
	return out, nil
}

func flattenJSONPathValue(value any, out *[]string) {
	switch v := value.(type) {
	case nil:
		return
	case []any:
		for _, item := range v {
			flattenJSONPathValue(item, out)
		}
	case string:
		*out = append(*out, v)
	case float64:
		*out = append(*out, strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		*out = append(*out, strconv.FormatBool(v))
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return
		}
		*out = append(*out, string(raw))
	}
}
