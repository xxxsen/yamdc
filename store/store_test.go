package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	err := Init("../.vscode/tests/store")
	assert.NoError(t, err)
	{
		key, err := GetDefault().Put([]byte("hello world"))
		assert.NoError(t, err)
		data, err := GetDefault().GetData(key)
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello world"), data)
	}
	{
		err = GetDefault().PutWithNamingKey("aaa-bbb", []byte("hihi"))
		assert.NoError(t, err)
		data, err := GetDefault().GetData("aaa-bbb")
		assert.NoError(t, err)
		assert.Equal(t, []byte("hihi"), data)
	}
}
