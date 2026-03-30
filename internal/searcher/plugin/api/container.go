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

func mustGetContainer(ctx context.Context) *container {
	c, ok := ctx.Value(defaultContainerTypeKey).(*container)
	if !ok || c == nil {
		panic("container is not initialized")
	}
	return c
}

func InitContainer(ctx context.Context) context.Context {
	c := &container{
		m: make(map[string]string),
	}
	return context.WithValue(ctx, defaultContainerTypeKey, c)
}

func SetKeyValue(ctx context.Context, key string, value string) {
	c := mustGetContainer(ctx)
	c.m[key] = value
}

func GetKeyValue(ctx context.Context, key string) (string, bool) {
	c := mustGetContainer(ctx)
	v, ok := c.m[key]
	return v, ok
}

func MustGetKeyValue(ctx context.Context, key string) string {
	c := mustGetContainer(ctx)
	v, ok := c.m[key]
	if !ok {
		panic(fmt.Errorf("key:%s not found", key))
	}
	return v
}

func ExportContainerData(ctx context.Context) map[string]string {
	c := mustGetContainer(ctx)
	m := make(map[string]string)
	maps.Copy(m, c.m)
	return m
}

func ImportContainerData(ctx context.Context, m map[string]string) {
	c := mustGetContainer(ctx)
	maps.Copy(c.m, m)
}
