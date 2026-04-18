package medialib

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// syncDirtyKey 是存放在 yamdc_kv_tab 里的 dirty flag 键名。
// "dirty" 语义: 我们有理由相信媒体库 DB 和磁盘状态可能不一致, 需要在
// 下一次自动调度窗口里跑一次全量 sync 把 DB 对齐。清干净的条件是跑完
// 一次 runFullSync (无论成功/失败完整收尾), 详见 runFullSync 的 defer 收尾。
const syncDirtyKey = "media_library.sync_dirty"

// markSyncDirty 把 dirty flag 置为 1。调用点包括:
//   - runMove 开始 (用户把新 item 移入库, DB 在 runMove 里按 item 增量 upsert,
//     但只要 move 发生过就标 dirty 兜底: 增量 upsert 若被 crash 掐断, 下次
//     auto-sync 能发现并修正)
//   - recoverTaskStates 检测到上一次 Sync 被中断 (cleanupStaleItems 可能没跑)
//   - 任何其它可能让 DB 偏离磁盘真实状态的路径
//
// 实现层面只写 1 行 kv, 幂等, 并发安全 (sqlite 写是序列化的)。
func (s *Service) markSyncDirty(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO yamdc_kv_tab (key, value, updated_at)
		VALUES (?, '1', ?)
		ON CONFLICT(key) DO UPDATE SET value = '1', updated_at = excluded.updated_at
	`, syncDirtyKey, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("mark media library sync dirty failed: %w", err)
	}
	return nil
}

// clearSyncDirty 把 dirty flag 置为 0。语义: "这一轮 runFullSync 已经对
// 磁盘做过一次完整盘点, DB 状态应当和磁盘对齐"。
//
// 注意: 按设计我们在 runFullSync 走到 finishTask / failTask 两个分支时都会
// clearDirty, 即便过程中有 per-item error。这是刻意的, 避免出现 "一条坏数据
// 导致 dirty 永远是 1、每天凌晨都重跑一次同样会失败" 的无限重试。
// per-item 错误由 sync_log 记录, 用户从 UI 日志弹窗人肉处理。
func (s *Service) clearSyncDirty(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO yamdc_kv_tab (key, value, updated_at)
		VALUES (?, '0', ?)
		ON CONFLICT(key) DO UPDATE SET value = '0', updated_at = excluded.updated_at
	`, syncDirtyKey, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("clear media library sync dirty failed: %w", err)
	}
	return nil
}

// isSyncDirty 读取当前 dirty flag。没有这一行时视作 false (clean),
// 避免一个刚初始化、磁盘上也没任何数据的实例在 03:00 浪费一次全量 sync。
func (s *Service) isSyncDirty(ctx context.Context) (bool, error) {
	if s.db == nil {
		return false, nil
	}
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM yamdc_kv_tab WHERE key = ?`, syncDirtyKey).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("read media library sync dirty failed: %w", err)
	}
	return value == "1", nil
}
