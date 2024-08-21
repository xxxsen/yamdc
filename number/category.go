package number

import "strings"

type Category string

func (c Category) String() string {
	return string(c)
}

const (
	CatFC2 Category = "fc2"
)

func IsFc2(number string) bool {
	number = strings.ToUpper(number)
	return strings.HasPrefix(number, "FC2")
}

func DetermineCategory(numberId string) []Category {
	if strings.HasPrefix(strings.ToUpper(numberId), "FC2") {
		return []Category{CatFC2}
	}
	return []Category{} //默认无分类
}
