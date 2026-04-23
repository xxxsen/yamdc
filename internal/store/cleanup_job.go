package store

import (
	"context"
	"fmt"

	"github.com/xxxsen/yamdc/internal/cronscheduler"
)

// cacheCleanupJobName 是本 job 在 cronscheduler 里的稳定唯一标识, 走排障日志。
// 命名用 snake_case 保持和其它 job (media_library_auto_sync / unified_log_cleanup)
// 一致的风格, 前端不可见, 改名要同步更新 bootstrap 里的注册点。
const cacheCleanupJobName = "cache_store_cleanup"

// CacheCleanupExpirer 是 cache_store_cleanup job 需要的最小能力抽象。
//
// 单独拎出接口而不是直接依赖具体 cache store 的理由:
//  1. 让 bootstrap 侧可以对 IStorage 做类型断言, 断言失败就跳过注册, 不会
//     因为缓存存储实现换了 (e.g. 测试里换成 noop) 就崩在启动路径上;
//  2. 单测里拿 mock 也更方便, 不用打开真实 cache DB。
type CacheCleanupExpirer interface {
	CleanupExpired(ctx context.Context) error
}

// NewCacheCleanupJob 构造一个清理过期缓存行的 cron job。Spec 固定为
// CacheCleanupInterval (@every 24h), 不做成参数暴露, 原因见 CacheCleanupInterval
// 注释; 真到需要按配置走的一天再加一层 option 不迟。
//
// 返回的 Job 直接扔给 cronscheduler.Scheduler.Register 即可。Run 里只做
// 委派不做额外包装: adapter 层已经负责 recover / 耗时日志, 这里再包一层
// fmt.Errorf 主要是让 "是哪个 job 失败" 在 error 里也可读 (adapter 日志
// 有 name 字段, 但 error 本身被 errJobRunFailed 二次 wrap 后只有原始 error
// 文本, 加前缀便于 grep)。
func NewCacheCleanupJob(e CacheCleanupExpirer) cronscheduler.Job {
	return cronscheduler.NewFuncJob(
		cacheCleanupJobName,
		"@every "+CacheCleanupInterval.String(),
		func(ctx context.Context) error {
			if e == nil {
				return nil
			}
			if err := e.CleanupExpired(ctx); err != nil {
				return fmt.Errorf("%s: %w", cacheCleanupJobName, err)
			}
			return nil
		},
	)
}
