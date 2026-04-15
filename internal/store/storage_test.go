package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- mock ----------

type errStorage struct {
	getErr    error
	putErr    error
	existErr  error
	existRet  bool
	lastKey   string
	lastValue []byte
}

func (s *errStorage) GetData(_ context.Context, key string) ([]byte, error) {
	s.lastKey = key
	if s.getErr != nil {
		return nil, s.getErr
	}
	return []byte("data"), nil
}

func (s *errStorage) PutData(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.lastKey = key
	s.lastValue = value
	return s.putErr
}

func (s *errStorage) IsDataExist(_ context.Context, key string) (bool, error) {
	s.lastKey = key
	return s.existRet, s.existErr
}

// ---------- PutDataTo ----------

func TestPutDataTo_NilStorage(t *testing.T) {
	err := PutDataTo(context.Background(), nil, "k", []byte("v"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errStorageNil)
}

func TestPutDataTo_Success(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "k", []byte("v")))
	v, err := GetDataFrom(context.Background(), s, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", string(v))
}

func TestPutDataTo_StorageError(t *testing.T) {
	s := &errStorage{putErr: errors.New("disk full")}
	err := PutDataTo(context.Background(), s, "k", []byte("v"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "put data failed")
}

// ---------- PutDataWithExpireTo ----------

func TestPutDataWithExpireTo_NilStorage(t *testing.T) {
	err := PutDataWithExpireTo(context.Background(), nil, "k", []byte("v"), time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStorageNil)
}

func TestPutDataWithExpireTo_Success(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataWithExpireTo(context.Background(), s, "k", []byte("v"), 0))
}

// ---------- GetDataFrom ----------

func TestGetDataFrom_NilStorage(t *testing.T) {
	_, err := GetDataFrom(context.Background(), nil, "k")
	require.Error(t, err)
	assert.ErrorIs(t, err, errStorageNil)
}

func TestGetDataFrom_Error(t *testing.T) {
	s := &errStorage{getErr: errors.New("io error")}
	_, err := GetDataFrom(context.Background(), s, "k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get data failed")
}

func TestGetDataFrom_Success(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "k", []byte("v")))
	v, err := GetDataFrom(context.Background(), s, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", string(v))
}

// ---------- IsDataExistIn ----------

func TestIsDataExistIn_NilStorage(t *testing.T) {
	_, err := IsDataExistIn(context.Background(), nil, "k")
	require.Error(t, err)
	assert.ErrorIs(t, err, errStorageNil)
}

func TestIsDataExistIn_Error(t *testing.T) {
	s := &errStorage{existErr: errors.New("oops")}
	_, err := IsDataExistIn(context.Background(), s, "k")
	require.Error(t, err)
}

func TestIsDataExistIn_NotExist(t *testing.T) {
	s := NewMemStorage()
	ok, err := IsDataExistIn(context.Background(), s, "nonexist")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIsDataExistIn_Exists(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "k", []byte("v")))
	ok, err := IsDataExistIn(context.Background(), s, "k")
	require.NoError(t, err)
	assert.True(t, ok)
}

// ---------- AnonymousPutDataTo ----------

func TestAnonymousPutDataTo_New(t *testing.T) {
	s := NewMemStorage()
	key, err := AnonymousPutDataTo(context.Background(), s, []byte("hello"))
	require.NoError(t, err)
	assert.NotEmpty(t, key)

	v, err := GetDataFrom(context.Background(), s, key)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(v))
}

func TestAnonymousPutDataTo_AlreadyExists(t *testing.T) {
	s := NewMemStorage()
	key1, err := AnonymousPutDataTo(context.Background(), s, []byte("hello"))
	require.NoError(t, err)
	key2, err := AnonymousPutDataTo(context.Background(), s, []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, key1, key2)
}

func TestAnonymousPutDataTo_PutError(t *testing.T) {
	s := &errStorage{putErr: errors.New("write err"), getErr: errNotFound}
	_, err := AnonymousPutDataTo(context.Background(), s, []byte("hello"))
	require.Error(t, err)
}

// ---------- LoadDataFrom ----------

func TestLoadDataFrom_CacheHit(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "k", []byte("cached")))

	v, err := LoadDataFrom(context.Background(), s, "k", 0, func() ([]byte, error) {
		t.Fatal("callback should not be called")
		return nil, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "cached", string(v))
}

func TestLoadDataFrom_CacheMiss(t *testing.T) {
	s := NewMemStorage()
	v, err := LoadDataFrom(context.Background(), s, "k", 0, func() ([]byte, error) {
		return []byte("fresh"), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(v))

	cached, err := GetDataFrom(context.Background(), s, "k")
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(cached))
}

func TestLoadDataFrom_CallbackError(t *testing.T) {
	s := NewMemStorage()
	_, err := LoadDataFrom(context.Background(), s, "k", 0, func() ([]byte, error) {
		return nil, errors.New("fetch failed")
	})
	require.Error(t, err)
}

func TestLoadDataFrom_PutError(t *testing.T) {
	s := &errStorage{
		getErr: errNotFound,
		putErr: errors.New("disk full"),
	}
	_, err := LoadDataFrom(context.Background(), s, "k", time.Second, func() ([]byte, error) {
		return []byte("v"), nil
	})
	require.Error(t, err)
}

// ---------- AnonymousDataRewriteWithStorage ----------

func TestAnonymousDataRewriteWithStorage_Success(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "old_key", []byte("original")))

	newKey, err := AnonymousDataRewriteWithStorage(context.Background(), s, "old_key", func(_ context.Context, data []byte) ([]byte, error) {
		return append(data, []byte("-modified")...), nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, newKey)

	v, err := GetDataFrom(context.Background(), s, newKey)
	require.NoError(t, err)
	assert.Equal(t, "original-modified", string(v))
}

func TestAnonymousDataRewriteWithStorage_GetError(t *testing.T) {
	s := NewMemStorage()
	key, err := AnonymousDataRewriteWithStorage(context.Background(), s, "missing", func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, nil
	})
	require.Error(t, err)
	assert.Equal(t, "missing", key)
}

func TestAnonymousDataRewriteWithStorage_FnError(t *testing.T) {
	s := NewMemStorage()
	require.NoError(t, PutDataTo(context.Background(), s, "k", []byte("v")))

	key, err := AnonymousDataRewriteWithStorage(context.Background(), s, "k", func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, errors.New("transform err")
	})
	require.Error(t, err)
	assert.Equal(t, "k", key)
}

func TestAnonymousDataRewriteWithStorage_PutError(t *testing.T) {
	s := &errStorage{putErr: errors.New("write err")}
	key, err := AnonymousDataRewriteWithStorage(context.Background(), s, "k", func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte("new"), nil
	})
	require.Error(t, err)
	assert.Equal(t, "k", key)
}
