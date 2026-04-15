package store

import (
	"context"
	"errors"
	"time"
)

var errNotFound = errors.New("not found")

type memStorage struct {
	m map[string][]byte
}

func NewMemStorage() IStorage {
	return &memStorage{
		m: make(map[string][]byte),
	}
}

func (m *memStorage) GetData(_ context.Context, key string) ([]byte, error) {
	if v, ok := m.m[key]; ok {
		return v, nil
	}
	return nil, errNotFound
}

func (m *memStorage) PutData(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.m[key] = value
	return nil
}

func (m *memStorage) IsDataExist(_ context.Context, key string) (bool, error) {
	_, ok := m.m[key]
	return ok, nil
}
