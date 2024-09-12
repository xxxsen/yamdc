package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"yamdc/hasher"

	"github.com/google/uuid"
)

type diskStorage struct {
	dir string
}

func NewDiskStorage(dir string) IStorage {
	return &diskStorage{dir: dir}
}

func (s *diskStorage) generateStorePath(key string) string {
	save := hasher.ToSha1(key)
	p1 := save[:2]
	return filepath.Join(s.dir, p1, save)
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
	tempPath := filepath.Join(dir, "tmp."+uuid.NewString())
	defer os.Remove(tempPath) //处理完, 删除临时文件
	if err := os.WriteFile(tempPath, value, 0644); err != nil {
		return fmt.Errorf("write data failed, err:%w", err)
	}
	if err := os.Rename(tempPath, p); err != nil {
		return fmt.Errorf("rename failed, err:%w", err)
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
