package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	file := filepath.Join(os.TempDir(), "cache.db")
	_ = os.Remove(file)
	SetStorage(MustNewSqliteStorage(file))
	ctx := context.Background()
	//获取数据, 此时返回错误
	_, err := GetData(ctx, "abc")
	assert.Error(t, err)
	//数据不存在
	exist, err := IsDataExist(ctx, "abc")
	assert.NoError(t, err)
	assert.False(t, exist)
	//写入数据
	err = PutDataWithExpire(ctx, "abc", []byte("helloworld"), 1*time.Second)
	assert.NoError(t, err)
	//数据存在
	exist, err = IsDataExist(ctx, "abc")
	assert.NoError(t, err)
	assert.True(t, exist)
	//正常获取数据
	val, err := GetData(ctx, "abc")
	assert.NoError(t, err)
	assert.Equal(t, "helloworld", string(val))
	time.Sleep(1 * time.Second)
	//数据过期
	exist, err = IsDataExist(ctx, "abc")
	assert.NoError(t, err)
	assert.False(t, exist)
	_, err = GetData(ctx, "abc")
	assert.Error(t, err)

	//测试不过期的数据
	err = PutData(ctx, "zzz", []byte("aaa"))
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	exist, err = IsDataExist(ctx, "zzz")
	assert.NoError(t, err)
	assert.True(t, exist)
	val, err = GetData(ctx, "zzz")
	assert.NoError(t, err)
	assert.Equal(t, "aaa", string(val))
}
