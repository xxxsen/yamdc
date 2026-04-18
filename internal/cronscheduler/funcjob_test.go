package cronscheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuncJobReturnsConfiguredNameAndSpec(t *testing.T) {
	j := NewFuncJob("sample_job", "@every 10s", func(context.Context) error { return nil })

	assert.Equal(t, "sample_job", j.Name())
	assert.Equal(t, "@every 10s", j.Spec())
}

func TestFuncJobRunDelegatesToConfiguredFunc(t *testing.T) {
	called := false
	var gotCtx context.Context
	j := NewFuncJob("run_delegate", "@every 5s", func(ctx context.Context) error {
		called = true
		gotCtx = ctx
		return nil
	})

	ctx := context.Background()
	err := j.Run(ctx)

	require.NoError(t, err)
	assert.True(t, called, "run func should be invoked")
	assert.Equal(t, ctx, gotCtx)
}

func TestFuncJobRunPropagatesRunError(t *testing.T) {
	errBoom := errors.New("boom")
	j := NewFuncJob("failing_job", "@every 1s", func(context.Context) error {
		return errBoom
	})

	err := j.Run(context.Background())

	assert.ErrorIs(t, err, errBoom)
}

// TestNewFuncJobPanicsOnNilRun 守住 fail-fast 契约: 运行时才发现 run==nil
// 会让 job 每次 tick 空跑且日志显示成功, 排障极痛苦, 所以 NewFuncJob 在构
// 造期就必须 panic。若将来想放宽 (e.g. 改成返回 error), 这条测试会提醒先
// 同步更新 funcjob.go 的注释和所有调用点的错误处理。
func TestNewFuncJobPanicsOnNilRun(t *testing.T) {
	assert.PanicsWithValue(t,
		"cronscheduler: NewFuncJob run must not be nil",
		func() { _ = NewFuncJob("nil_run_job", "@every 1s", nil) },
		"NewFuncJob must reject nil run at construction time",
	)
}
