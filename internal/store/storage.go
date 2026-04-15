package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xxxsen/yamdc/internal/hasher"
)

var errStorageNil = errors.New("storage is nil")

type DataRewriteFunc func(ctx context.Context, data []byte) ([]byte, error)

type IStorage interface {
	GetData(ctx context.Context, key string) ([]byte, error)
	PutData(ctx context.Context, key string, value []byte, expire time.Duration) error
	IsDataExist(ctx context.Context, key string) (bool, error)
}

func PutDataTo(ctx context.Context, storage IStorage, key string, value []byte) error {
	return PutDataWithExpireTo(ctx, storage, key, value, time.Duration(0))
}

func PutDataWithExpireTo(ctx context.Context, storage IStorage, key string, value []byte, expire time.Duration) error {
	if storage == nil {
		return errStorageNil
	}
	if err := storage.PutData(ctx, key, value, expire); err != nil {
		return fmt.Errorf("put data failed: %w", err)
	}
	return nil
}

func AnonymousPutDataTo(ctx context.Context, storage IStorage, value []byte) (string, error) {
	key := hasher.ToSha1Bytes(value)
	if ok, _ := IsDataExistIn(ctx, storage, key); ok {
		return key, nil
	}
	if err := PutDataTo(ctx, storage, key, value); err != nil {
		return "", err
	}
	return key, nil
}

func GetDataFrom(ctx context.Context, storage IStorage, key string) ([]byte, error) {
	if storage == nil {
		return nil, errStorageNil
	}
	data, err := storage.GetData(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get data failed: %w", err)
	}
	return data, nil
}

func LoadDataFrom(
	ctx context.Context,
	storage IStorage,
	key string,
	expire time.Duration,
	cb func() ([]byte, error),
) ([]byte, error) {
	if v, err := GetDataFrom(ctx, storage, key); err == nil {
		return v, nil
	}
	data, err := cb()
	if err != nil {
		return nil, err
	}
	if err := PutDataWithExpireTo(ctx, storage, key, data, expire); err != nil {
		return nil, err
	}
	return data, nil
}

func IsDataExistIn(ctx context.Context, storage IStorage, key string) (bool, error) {
	if storage == nil {
		return false, errStorageNil
	}
	ok, err := storage.IsDataExist(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check data existence failed: %w", err)
	}
	return ok, nil
}

func AnonymousDataRewriteWithStorage(
	ctx context.Context,
	storage IStorage,
	key string,
	fn DataRewriteFunc,
) (string, error) {
	raw, err := GetDataFrom(ctx, storage, key)
	if err != nil {
		return key, err
	}
	newData, err := fn(ctx, raw)
	if err != nil {
		return key, err
	}
	newKey, err := AnonymousPutDataTo(ctx, storage, newData)
	if err != nil {
		return key, err
	}
	return newKey, nil
}
