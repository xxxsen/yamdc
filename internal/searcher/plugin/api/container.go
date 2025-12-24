package api

import (
	"context"
	"fmt"
	"maps"
)

type container struct {
	m map[string]string
}

type containerType struct{}

var (
	defaultContainerTypeKey = containerType{}
)

func InitContainer(ctx context.Context) context.Context {
	c := &container{
		m: make(map[string]string),
	}
	return context.WithValue(ctx, defaultContainerTypeKey, c)
}

func SetKeyValue(ctx context.Context, key string, value string) {
	c := ctx.Value(defaultContainerTypeKey).(*container)
	c.m[key] = value
}

func GetKeyValue(ctx context.Context, key string) (string, bool) {
	c := ctx.Value(defaultContainerTypeKey).(*container)
	v, ok := c.m[key]
	return v, ok
}

func MustGetKeyValue(ctx context.Context, key string) string {
	c := ctx.Value(defaultContainerTypeKey).(*container)
	v, ok := c.m[key]
	if !ok {
		panic(fmt.Errorf("key:%s not found", key))
	}
	return v
}

func ExportContainerData(ctx context.Context) map[string]string {
	c := ctx.Value(defaultContainerTypeKey).(*container)
	m := make(map[string]string)
	maps.Copy(m, c.m)
	return m
}

func ImportContainerData(ctx context.Context, m map[string]string) {
	c := ctx.Value(defaultContainerTypeKey).(*container)
	maps.Copy(c.m, m)
}
