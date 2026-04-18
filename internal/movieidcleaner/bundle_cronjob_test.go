package movieidcleaner

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerCronJobNilReceiver(t *testing.T) {
	var m *Manager
	assert.Nil(t, m.CronJob())
}

func TestManagerCronJobNilInnerManager(t *testing.T) {
	m := &Manager{}
	assert.Nil(t, m.CronJob())
}

func TestManagerCronJobLocalReturnsNil(t *testing.T) {
	m, err := NewManager(t.TempDir(), stubHTTPClient{}, SourceTypeLocal, t.TempDir(),
		func(_ context.Context, _ *RuleSet, _ []string) error { return nil })
	require.NoError(t, err)

	assert.Nil(t, m.CronJob(), "local bundle must not produce a cron job")
}

func TestManagerCronJobRemoteMetadata(t *testing.T) {
	m, err := NewManager(t.TempDir(), stubHTTPClient{}, SourceTypeRemote,
		"https://github.com/owner/repo",
		func(_ context.Context, _ *RuleSet, _ []string) error { return nil })
	require.NoError(t, err)

	job := m.CronJob()
	require.NotNil(t, job)
	assert.Equal(t, cronJobPrefix+"_number_cleaner_remote_sync", job.Name())
}

// TestManagerCronJobRunPropagatesRemoteSyncError 覆盖异常 case: 当底层
// basebundle.Manager 的 syncAndActivate 失败 (HTTP 拉 tag 报错) 时, 聚合
// 层返回的 cron Job.Run 必须把 error 原样向上抛, 且 error 文本包含 Job.Name
// 前缀, 方便 adapter 日志反查是哪个 job 挂了。底层 wrapsSyncError 测试
// (internal/bundle/manager_cronjob_test.go) 已经守住 basebundle 自己的行为,
// 这条是聚合边界 ("不吞错") 的显式回归守护。
func TestManagerCronJobRunPropagatesRemoteSyncError(t *testing.T) {
	failingClient := stubHTTPClient{
		do: func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		},
	}
	m, err := NewManager(t.TempDir(), failingClient, SourceTypeRemote,
		"https://github.com/owner/repo",
		func(_ context.Context, _ *RuleSet, _ []string) error { return nil })
	require.NoError(t, err)

	job := m.CronJob()
	require.NotNil(t, job)

	runErr := job.Run(context.Background())
	require.Error(t, runErr, "remote sync failure must surface as Run error")
	assert.Contains(t, runErr.Error(), job.Name(),
		"error must be prefixed with Job.Name for log triage")
}
