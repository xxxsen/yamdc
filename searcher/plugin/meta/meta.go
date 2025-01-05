package meta

import "context"

type numberIdKeyType struct{}

var (
	defaultNumberIdKey = numberIdKeyType{}
)

func SetNumberId(ctx context.Context, nid string) context.Context {
	ctx = context.WithValue(ctx, defaultNumberIdKey, nid)
	return ctx
}

func GetNumberId(ctx context.Context) string {
	nid := ctx.Value(defaultNumberIdKey)
	if nid == nil {
		return ""
	}
	return nid.(string)
}
