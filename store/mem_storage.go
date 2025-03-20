package store

import (
	"context"
	"fmt"
	"time"
)

type memStorage struct {
	m map[string][]byte
}

func NewMemStorage() IStorage {
	return &memStorage{
		m: make(map[string][]byte),
	}
}

func (m *memStorage) GetData(ctx context.Context, key string) ([]byte, error) {
	if v, ok := m.m[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *memStorage) PutData(ctx context.Context, key string, value []byte, expire time.Duration) error {
	m.m[key] = value
	return nil
}

func (m *memStorage) IsDataExist(ctx context.Context, key string) (bool, error) {
	_, err := m.GetData(ctx, key)
	if err != nil {
		return false, nil
	}
	return true, nil
}
