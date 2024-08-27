package store

import (
	"context"

	"github.com/google/uuid"
)

type DataRewriteFunc func(ctx context.Context, data []byte) ([]byte, error)

type IStorage interface {
	GetData(ctx context.Context, key string) ([]byte, error)
	PutData(ctx context.Context, key string, value []byte) error
	IsDataExist(ctx context.Context, key string) (bool, error)
}

var defaultInst IStorage

func SetStorage(impl IStorage) {
	defaultInst = impl
}

func getDefaultInst() IStorage {
	return defaultInst
}

func PutData(ctx context.Context, key string, value []byte) error {
	return getDefaultInst().PutData(ctx, key, value)
}

func AnonymousPutData(ctx context.Context, value []byte) (string, error) {
	key := uuid.NewString()
	if err := PutData(ctx, key, value); err != nil {
		return "", err
	}
	return key, nil
}

func GetData(ctx context.Context, key string) ([]byte, error) {
	return getDefaultInst().GetData(ctx, key)
}

func IsDataExist(ctx context.Context, key string) (bool, error) {
	return getDefaultInst().IsDataExist(ctx, key)
}

func AnonymousDataRewrite(ctx context.Context, key string, fn DataRewriteFunc) (string, error) {
	raw, err := GetData(ctx, key)
	if err != nil {
		return key, err
	}
	newData, err := fn(ctx, raw)
	if err != nil {
		return key, err
	}
	newKey, err := AnonymousPutData(ctx, newData)
	if err != nil {
		return key, err
	}
	return newKey, nil
}
