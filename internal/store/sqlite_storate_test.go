package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.db")
	storage := MustNewSqliteStorage(file)
	ctx := context.Background()
	//获取数据, 此时返回错误
	_, err := GetDataFrom(ctx, storage, "abc")
	assert.Error(t, err)
	//数据不存在
	exist, err := IsDataExistIn(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.False(t, exist)
	//写入数据
	err = PutDataWithExpireTo(ctx, storage, "abc", []byte("helloworld"), 1*time.Second)
	assert.NoError(t, err)
	//数据存在
	exist, err = IsDataExistIn(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.True(t, exist)
	//正常获取数据
	val, err := GetDataFrom(ctx, storage, "abc")
	assert.NoError(t, err)
	assert.Equal(t, "helloworld", string(val))
	//等待数据过期（避免卡在过期边界导致偶发失败）
	assert.Eventually(t, func() bool {
		exist, err = IsDataExistIn(ctx, storage, "abc")
		assert.NoError(t, err)
		return !exist
	}, 3*time.Second, 50*time.Millisecond)
	_, err = GetDataFrom(ctx, storage, "abc")
	assert.Error(t, err)

	//测试不过期的数据
	err = PutDataTo(ctx, storage, "zzz", []byte("aaa"))
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	exist, err = IsDataExistIn(ctx, storage, "zzz")
	assert.NoError(t, err)
	assert.True(t, exist)
	val, err = GetDataFrom(ctx, storage, "zzz")
	assert.NoError(t, err)
	assert.Equal(t, "aaa", string(val))
}
