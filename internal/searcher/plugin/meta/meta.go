package meta

import "context"

type numberIDKeyType struct{}

var defaultNumberIDKey = numberIDKeyType{}

func SetNumberID(ctx context.Context, nid string) context.Context {
	ctx = context.WithValue(ctx, defaultNumberIDKey, nid)
	return ctx
}

func GetNumberID(ctx context.Context) string {
	nid := ctx.Value(defaultNumberIDKey)
	if nid == nil {
		return ""
	}
	value, ok := nid.(string)
	if !ok {
		return ""
	}
	return value
}
