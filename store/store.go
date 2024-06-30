package store

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var defaultStore *Store

func Init(dataDir string) error {
	var err error
	defaultStore, err = New(dataDir)
	if err != nil {
		return err
	}
	return nil
}

func GetDefault() *Store {
	return defaultStore
}

type Store struct {
	idx     uint32
	dataDir string
}

type IFile interface {
	Name() string
	Open() io.ReadCloser
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	return &Store{idx: 10000, dataDir: dataDir}, nil
}

func (s *Store) Put(data []byte) (string, error) {
	key := s.generateDataKey(data)
	return key, s.persist(key, data)
}

func (s *Store) wrapNamingKey(key string) []byte {
	return []byte(fmt.Sprintf("^#NAMING_START:%s:NAMING_END#$", key))
}

func (s *Store) PutWithNamingKey(key string, data []byte) error {
	key = s.generateDataKey(s.wrapNamingKey(key))
	return s.persist(key, data)
}

func (s *Store) persist(key string, data []byte) error {
	dir, f := s.buildFileLocation(key)
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(f, data, 0644)
}

func (s *Store) buildFileLocation(key string) (string, string) {
	p1 := key[:2]
	p2 := key[2:4]
	p3 := key[4:6]
	dir := filepath.Join(s.dataDir, p1, p2, p3)
	file := filepath.Join(dir, key)
	return dir, file
}

func (s *Store) generateDataKey(data []byte) string {
	m := sha1.New()
	_, _ = m.Write([]byte(fmt.Sprintf("%d", len(data))))
	_, _ = m.Write(data)
	res := m.Sum(nil)
	return hex.EncodeToString(res)
}

func (s *Store) GetData(key string) ([]byte, error) {
	_, f := s.buildFileLocation(key)
	raw, err := os.ReadFile(f)
	return raw, err
}

func (s *Store) GetDataWithNamingKey(key string) ([]byte, error) {
	key = s.generateDataKey(s.wrapNamingKey(key))
	return s.GetData(key)
}
