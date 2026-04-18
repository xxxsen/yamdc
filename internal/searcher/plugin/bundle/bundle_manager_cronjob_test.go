package bundle

import (
	"context"
	"errors"
	"net/http"
	"testing"

	basebundle "github.com/xxxsen/yamdc/internal/bundle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingHTTPClient 让 basebundle.syncAndActivate 里第一次 HTTP 调用 (拉
// latest tag) 直接报错, 触发 RemoteSyncJob.Run 失败路径, 用于 CronJobs
// 聚合层的错误传播测试。保持最小实现, 不跟 movieidcleaner 共用 stub: 两
// 边是独立 package, 复制一个 <20 行的 test helper 比把它提成公共 utility
// 更省事。
type failingHTTPClient struct {
	err error
}

func (f failingHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return nil, f.err
}

func noopPluginCallback() OnDataReadyFunc {
	return func(_ context.Context, _ *ResolvedBundle, _ []string) error { return nil }
}

func TestManagerCronJobsNilReceiver(t *testing.T) {
	var m *Manager
	assert.Nil(t, m.CronJobs())
}

func TestManagerCronJobsEmptyManagers(t *testing.T) {
	m := &Manager{name: "searcher_plugin"}
	assert.Empty(t, m.CronJobs())
}

func TestManagerCronJobsSkipsLocalManagers(t *testing.T) {
	m, err := NewManager("searcher_plugin", t.TempDir(), nil,
		[]Source{
			{SourceType: basebundle.SourceTypeLocal, Location: t.TempDir()},
			{SourceType: basebundle.SourceTypeLocal, Location: t.TempDir()},
		},
		noopPluginCallback(),
	)
	require.NoError(t, err)

	assert.Empty(t, m.CronJobs(), "local bundles should not contribute cron jobs")
}

func TestManagerCronJobsPrefixesWithManagerName(t *testing.T) {
	m, err := NewManager("searcher_plugin", t.TempDir(), nil,
		[]Source{
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/first"},
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/second"},
		},
		noopPluginCallback(),
	)
	require.NoError(t, err)

	jobs := m.CronJobs()
	require.Len(t, jobs, 2)
	for _, job := range jobs {
		assert.Contains(t, job.Name(), "searcher_plugin_")
		assert.Contains(t, job.Name(), "_remote_sync")
	}
}

// TestManagerCronJobsMultiSourceNamesAreUnique 是 regression 防线:
// 多 remote source 场景下各 Job.Name 必须两两不同, 否则 cronscheduler.Register
// 会抛 errDuplicateJobName 导致整个 bootstrap 挂掉 — 曾经因为所有 sub-manager
// 共用同一个外层 name 引出过这个 bug, 后续改 NewManager 里 name 生成策略时
// 这条测试要继续守住。
func TestManagerCronJobsMultiSourceNamesAreUnique(t *testing.T) {
	m, err := NewManager("searcher_plugin", t.TempDir(), nil,
		[]Source{
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/first"},
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/second"},
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/third"},
		},
		noopPluginCallback(),
	)
	require.NoError(t, err)

	jobs := m.CronJobs()
	require.Len(t, jobs, 3)

	seen := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		name := job.Name()
		_, dup := seen[name]
		assert.Falsef(t, dup, "duplicate cron job name: %q", name)
		seen[name] = struct{}{}
	}
}

// TestManagerCronJobsRunPropagatesSubManagerError 覆盖异常 case: 当某条
// sub-manager 的 syncAndActivate 失败 (HTTP 拉 tag 报错) 时, 对应聚合层
// 返回的 cron Job.Run 必须把 error 原样向上抛, 而不是吞掉变成 nil —
// 否则 cron adapter 会把失败的 tick 标成 success, 排障时会误判 "同步已
// 经跑过了"。error 文本带 Job.Name 前缀便于日志反查。
func TestManagerCronJobsRunPropagatesSubManagerError(t *testing.T) {
	cli := failingHTTPClient{err: errors.New("network down")}
	m, err := NewManager("searcher_plugin", t.TempDir(), cli,
		[]Source{
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/first"},
		},
		noopPluginCallback(),
	)
	require.NoError(t, err)

	jobs := m.CronJobs()
	require.Len(t, jobs, 1)

	runErr := jobs[0].Run(context.Background())
	require.Error(t, runErr, "sub-manager sync failure must surface as Run error")
	assert.Contains(t, runErr.Error(), jobs[0].Name(),
		"error must be prefixed with Job.Name for log triage")
}

// TestManagerCronJobsMixedSourcesOnlyIncludeRemotes 验证 CronJobs 只收集
// remote source 对应的 sub-manager: 混合配置下 local 不产生 job, remote
// 全部产生且 name 唯一 — 避免 "忘记过滤 local" 或 "多 remote 碰撞" 两种
// 回归一起守住。
func TestManagerCronJobsMixedSourcesOnlyIncludeRemotes(t *testing.T) {
	m, err := NewManager("searcher_plugin", t.TempDir(), nil,
		[]Source{
			{SourceType: basebundle.SourceTypeLocal, Location: t.TempDir()},
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/first"},
			{SourceType: basebundle.SourceTypeLocal, Location: t.TempDir()},
			{SourceType: basebundle.SourceTypeRemote, Location: "https://github.com/owner/second"},
		},
		noopPluginCallback(),
	)
	require.NoError(t, err)

	jobs := m.CronJobs()
	require.Len(t, jobs, 2, "only remote sources should contribute cron jobs")

	names := map[string]struct{}{}
	for _, job := range jobs {
		names[job.Name()] = struct{}{}
	}
	assert.Len(t, names, 2, "remote job names must be unique")
}
