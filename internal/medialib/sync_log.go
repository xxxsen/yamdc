package medialib

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xxxsen/yamdc/internal/repository"
)

// SyncLogLevel 是 sync 事件日志的级别取值。刻意做成字符串常量而不是 int,
// 方便前端直接 switch 显示; 也和 yamdc_unified_log_tab 的 level 语义保持一致。
const (
	SyncLogLevelInfo  = "info"
	SyncLogLevelWarn  = "warn"
	SyncLogLevelError = "error"
)

// syncLogRetention 是保留窗口。超过这个时长的行会在每次 sync 收尾时被
// 一条 DELETE 裁剪掉, 避免单张表无限增长吃磁盘。
//
// 注意这是全表 retention (跨 log_type): 1.4 改造后 scrape_job 和
// media_library_sync 共用同一张 yamdc_unified_log_tab, cleanup 按
// created_at 裁剪, 所有 log_type 一起 7 天。这是和 UX 沟通后的选择,
// 回溯价值 > 7 天的排障场景几乎不存在, 统一窗口让 cleanup 代码只有一份。
//
// 取 7 天的理由: 绝大多数用户排查一次失败同步最多回看前一两天,
// 更久远的失败日志 actionable 价值很低; 7 天这个窗口也足够覆盖
// "周末出门、周一回来排查问题" 这种典型节奏。
const syncLogRetention = 7 * 24 * time.Hour

// SyncLogEntry 是 sync 日志对前端暴露的结构。字段 / JSON tag 和 1.4 里
// 单独存在 yamdc_media_library_sync_log_tab 时完全一致, 前端 "查看同步
// 日志" 弹窗不需要跟着改。底层虽然合进了 yamdc_unified_log_tab, 但 UI
// 拿到的 shape 不变: id / run_id / level / rel_path / message / created_at。
type SyncLogEntry struct {
	ID        int64  `json:"id"`
	RunID     string `json:"run_id"`
	Level     string `json:"level"`
	RelPath   string `json:"rel_path"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

// syncLogPayload 是 yamdc_unified_log_tab.msg 在 log_type='media_library_sync'
// 下的 JSON schema. appendSyncLog marshal 写入, ListSyncLogs unmarshal
// 还原, 避免字段名在两处手抖漂移。
type syncLogPayload struct {
	RelPath string `json:"rel_path"`
	Message string `json:"message"`
}

// appendSyncLog 写一行 sync 日志。DB / logRepo 不可用时退化成 no-op,
// 只返回错误供上层决定是否记录到 zap 日志; 当前策略是不把这个错误再
// 二次传播, 免得 "写日志失败" 把 sync 流程带挂。
//
// 统一存到 yamdc_unified_log_tab 后, task_id 列放 run_id 原文,
// payload (rel_path / message) 压进 msg 的 JSON, 对前端展示不可见
// (前端收到的还是 SyncLogEntry)。
func (s *Service) appendSyncLog(ctx context.Context, runID, level, relPath, message string) error {
	if s.logRepo == nil {
		return nil
	}
	// 字段全是 string, marshal 理论上不会失败; 真失败也只会影响一条日志,
	// 静默丢是上层 "不让写日志拖住 sync 主流程" 的一致策略。
	msg, err := json.Marshal(syncLogPayload{RelPath: relPath, Message: message})
	if err != nil {
		return fmt.Errorf("marshal media library sync log payload failed: %w", err)
	}
	if err := s.logRepo.Append(ctx, repository.LogTypeMediaLibrarySync, runID, level, string(msg)); err != nil {
		return fmt.Errorf("append media library sync log failed: %w", err)
	}
	return nil
}

// syncLogListLimit 是 ListSyncLogs 默认返回条数。200 条对应 UI 分页的
// 第一页, 再多的话弹窗滚动体验反而差; 上层传 limit <= 0 时兜底到这个值。
const syncLogListLimit = 200

// ListSyncLogs 按时间逆序返回最近的 sync 日志。limit 上限由调用方 (web 层)
// 决定, 这里仅做防御性兜底: limit <= 0 用默认值, > 1000 由仓储层夹回去,
// 避免调用方传异常值把一次性把内存打爆。
func (s *Service) ListSyncLogs(ctx context.Context, limit int) ([]SyncLogEntry, error) {
	if s.logRepo == nil {
		return []SyncLogEntry{}, nil
	}
	if limit <= 0 {
		limit = syncLogListLimit
	}
	entries, err := s.logRepo.List(ctx, repository.LogListFilter{
		LogType: repository.LogTypeMediaLibrarySync,
		Limit:   limit,
		Order:   repository.LogOrderDesc,
	})
	if err != nil {
		return nil, fmt.Errorf("list media library sync logs failed: %w", err)
	}
	result := make([]SyncLogEntry, 0, len(entries))
	for _, entry := range entries {
		var payload syncLogPayload
		if entry.Msg != "" {
			// 兜底: 历史 / 脏数据解析失败时不要让整个 API 挂,
			// 把原文放进 Message 让用户能看到线索。
			if err := json.Unmarshal([]byte(entry.Msg), &payload); err != nil {
				payload = syncLogPayload{Message: entry.Msg}
			}
		}
		result = append(result, SyncLogEntry{
			ID:        entry.ID,
			RunID:     entry.TaskID,
			Level:     entry.Level,
			RelPath:   payload.RelPath,
			Message:   payload.Message,
			CreatedAt: entry.CreatedAt,
		})
	}
	return result, nil
}

// retentionCutoffMillis 暴露成函数方便测试用较短的保留窗口, 真实调用始终
// 走 syncLogRetention 常量。
func retentionCutoffMillis(now time.Time, window time.Duration) int64 {
	return now.Add(-window).UnixMilli()
}

// cleanupSyncLogs 裁剪超出保留窗口的日志行, 在每次 sync 收尾时调用。
// 合表之后这一条 DELETE 会把所有 log_type 里 created_at < cutoff 的行都
// 收掉 (目前只有 scrape_job / media_library_sync 两种), 走
// idx_yamdc_unified_log_type_created_at 索引, 代价可忽略。
//
// 函数名保留 "SyncLogs" 是为了调用方 (runFullSync 的 defer) 读起来自然,
// 实际上不再局限于 sync log。后续如果需要更精细的 per-type retention,
// 可以在 LogRepository 里加重载, 现阶段全局 7 天足够。
func (s *Service) cleanupSyncLogs(ctx context.Context) error {
	if s.logRepo == nil {
		return nil
	}
	cutoff := retentionCutoffMillis(time.Now(), syncLogRetention)
	if err := s.logRepo.DeleteOlderThan(ctx, cutoff); err != nil {
		return fmt.Errorf("cleanup media library sync logs failed: %w", err)
	}
	return nil
}

// newRunID 为一轮 sync 生成唯一标识。纳秒级时间戳足够在 "sync 之间互斥 +
// 同一实例单机" 这个前提下保证唯一, 比引入 uuid 依赖轻。格式做成
// "sync-<hex>" 是方便日志里肉眼区分, 不是功能需要。
func newRunID(now time.Time) string {
	return fmt.Sprintf("sync-%x", now.UnixNano())
}
