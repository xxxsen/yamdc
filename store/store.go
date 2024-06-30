package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

var defaultStore *Store

func Init() {
	defaultStore = New()
}

func GetDefault() *Store {
	return defaultStore
}

type Store struct {
	idx uint32
	st  sync.Map
}

type IFile interface {
	Name() string
	Open() io.ReadCloser
}

func New() *Store {
	return &Store{idx: 10000}
}

func (s *Store) Put(data []byte) (string, error) {
	key := s.generateKey()
	s.st.Store(key, data)
	return key, nil
}

func (s *Store) generateKey() string {
	randData := make([]byte, 20)
	_, _ = io.ReadAtLeast(rand.Reader, randData, len(randData))
	sess := atomic.AddUint32(&s.idx, 1)
	tsData := fmt.Sprintf("%d-%d", time.Now().UnixMilli(), sess)
	m := sha1.New()
	_, _ = m.Write([]byte(randData))
	_, _ = m.Write([]byte(tsData))
	res := m.Sum(nil)
	return hex.EncodeToString(res)
}

func (s *Store) Get(key string) (io.ReadCloser, bool) {
	if v, ok := s.st.Load(key); ok {
		return io.NopCloser(bytes.NewReader(v.([]byte))), true
	}

	return nil, false
}

func (s *Store) GetData(key string) ([]byte, error) {
	rc, ok := s.Get(key)
	if !ok {
		return nil, fmt.Errorf("key:%s no found", key)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
