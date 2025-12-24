package store

import (
	"context"
	"time"
	"github.com/xxxsen/yamdc/internal/hasher"
)

type DataRewriteFunc func(ctx context.Context, data []byte) ([]byte, error)

type IStorage interface {
	GetData(ctx context.Context, key string) ([]byte, error)
	PutData(ctx context.Context, key string, value []byte, expire time.Duration) error
	IsDataExist(ctx context.Context, key string) (bool, error)
}

func init() {
	SetStorage(NewMemStorage())
}

var defaultInst IStorage

func SetStorage(impl IStorage) {
	defaultInst = impl
}

func getDefaultInst() IStorage {
	return defaultInst
}

func PutData(ctx context.Context, key string, value []byte) error {
	return PutDataWithExpire(ctx, key, value, time.Duration(0))
}

func PutDataWithExpire(ctx context.Context, key string, value []byte, expire time.Duration) error {
	return getDefaultInst().PutData(ctx, key, value, expire)
}

func AnonymousPutData(ctx context.Context, value []byte) (string, error) {
	key := hasher.ToSha1Bytes(value)
	if ok, _ := IsDataExist(ctx, key); ok {
		return key, nil
	}
	if err := PutData(ctx, key, value); err != nil {
		return "", err
	}
	return key, nil
}

func GetData(ctx context.Context, key string) ([]byte, error) {
	return getDefaultInst().GetData(ctx, key)
}

func LoadData(ctx context.Context, key string, expire time.Duration, cb func() ([]byte, error)) ([]byte, error) {
	if v, err := GetData(ctx, key); err == nil {
		return v, nil
	}
	data, err := cb()
	if err != nil {
		return nil, err
	}
	if err := PutDataWithExpire(ctx, key, data, expire); err != nil {
		return nil, err
	}
	return data, nil
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
