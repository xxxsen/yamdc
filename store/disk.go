package store

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type diskStorage struct {
	dir string
}

func NewDiskStorage(dir string) IStorage {
	return &diskStorage{dir: dir}
}

func (s *diskStorage) keyMd5(key string) string {
	h := md5.New()
	_, _ = h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *diskStorage) generateStorePath(key string) string {
	save := s.keyMd5(key)
	p1 := save[:2]
	p2 := save[2:4]
	p3 := save[4:6]
	return filepath.Join(s.dir, p1, p2, p3, save)
}

func (s *diskStorage) GetData(ctx context.Context, key string) ([]byte, error) {
	return os.ReadFile(s.generateStorePath(key))
}

func (s *diskStorage) PutData(ctx context.Context, key string, value []byte) error {
	p := s.generateStorePath(key)
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir failed, err:%w", err)
	}
	if err := os.WriteFile(p, value, 0644); err != nil {
		return fmt.Errorf("write data failed, err:%w", err)
	}
	return nil
}

func (s *diskStorage) IsDataExist(ctx context.Context, key string) (bool, error) {
	p := s.generateStorePath(key)
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
