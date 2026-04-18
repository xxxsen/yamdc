package medialib

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/repository"
	"go.uber.org/zap"
)

const (
	TaskSync = "media_library_sync"
	TaskMove = "media_library_move"
)

// AutoSyncCronSpec 是自动 sync 的 cron 表达式, 每天本地时间 03:00 触发。
// 对应决策见 1.4 设计记录: 03:00 避开整点备份脚本高峰, 也给 00:00 完成
// 的磁盘合并/索引构建足够冷却时间, 且基本不会和用户活跃窗口撞车。
//
// 触发最终走 TriggerAutoSync -> triggerFullSyncWithReason, 和手动
// "同步媒体库" 完全等价 (同一互斥 + bgWG 路径), 因此不会和 move 任务并发。
//
// 注意这个常量属于 "服务内部配置" 而非 "cron scheduler 接口的一部分";
// 导出它是为了 bootstrap 可以在注册 Job 时读到。如果将来要做成 config
// 可调, 改这里就够, 不动 cronscheduler 包。
const AutoSyncCronSpec = "0 3 * * *"

// LogCleanupCronSpec 是日志 retention 清理的 cron 表达式。和 auto sync 同
// 在 03:00 触发是刻意的: 都挪到深夜低负载时段, 而且 cleanup 先跑 (SQLite
// 是串行写, robfig/cron 同一时刻的 job 按注册顺序依次触发, 我们把
// LogCleanupJob 注册在 AutoSync 之前, 保证裁旧再 sync)。
//
// 该 cleanup 不依赖 sync 是否触发 (避免 "用户只刮不入库 -> sync 从不跑
// -> 日志无限累积" 的 bug), 所以必须走独立 cron 条目。
const LogCleanupCronSpec = "0 3 * * *"

var (
	errLibraryDirNotConfigured = errors.New("library dir is not configured")
	errSaveDirNotConfigured    = errors.New("save dir is not configured")
	errMoveTaskRunning         = errors.New("move to media library is running")
	errSyncAlreadyRunning      = errors.New("media library sync is already running")
	errSyncTaskRunning         = errors.New("media library sync is running")
	errMoveAlreadyRunning      = errors.New("move to media library is already running")
)

type Service struct {
	db         *sql.DB
	logRepo    *repository.LogRepository
	libraryDir string
	saveDir    string

	mu          sync.Mutex
	syncRunning bool
	moveRunning bool
	bgWG        sync.WaitGroup

	// shutdownCtx / shutdownCancel 控制后台 goroutine 的生命周期。
	// Stop 会 cancel 这个 ctx, 正在跑的 Trigger* 派生 runFullSync / runMove
	// 能响应退出; bgWG 负责阻塞外部 cleanup 等这些 goroutine 完整返回后再让
	// 底层 sqlite / tempdir 被清理, 避免 "DB 已关闭、goroutine 还在写 DB"
	// 的竞争。Stop 必须在 WaitBackground 之前调用。
	//
	// 自动 sync 的定时调度职责 1.5 起从这里挪到 internal/cronscheduler;
	// 本 Service 不再自管 scheduler goroutine, 对应字段 (schedulerClock /
	// schedulerStartupDelay) 随之删除。
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

type ListItemsOptions struct {
	Keyword    string
	Year       string
	SizeFilter string
	Sort       string
	Order      string
}

func NewService(db *sql.DB, libraryDir, saveDir string) *Service {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	// logRepo 只是个 *sql.DB 的薄封装, 在 NewService 里一次性构造, 内部复用。
	// 保留 NewService 现有签名是刻意的: 外面大量单测和业务代码都以 (db, libraryDir, saveDir)
	// 方式构造 Service, 不想因为一次 refactor 把所有调用点都改掉。
	var logRepo *repository.LogRepository
	if db != nil {
		logRepo = repository.NewLogRepository(db)
	}
	return &Service{
		db:             db,
		logRepo:        logRepo,
		libraryDir:     libraryDir,
		saveDir:        saveDir,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
}

func (s *Service) IsConfigured() bool {
	return s.libraryDir != ""
}

// Start 做崩溃恢复: 如果进程上次是在 sync/move 跑到一半被 kill 的,
// task_state_tab 里会留下 running 状态, Start 把它改成 interrupted 以便
// UI 正确显示。定时调度不在这里拉起, 由 bootstrap 的 cron scheduler 统一
// 负责 (1.5 重构)。
func (s *Service) Start(ctx context.Context) {
	if s.db == nil {
		return
	}
	if err := s.recoverTaskStates(ctx); err != nil {
		logutil.GetLogger(ctx).Error("recover media library task states failed", zap.Error(err))
	}
}

// Stop 取消 shutdownCtx, 让正在跑的 runFullSync / runMove 响应退出;
// WaitBackground 负责等这些 goroutine 完整返回。
// 调用顺序建议: Stop 先走, 立即发 cancel 信号; 接着 WaitBackground 阻塞
// 等 sync/move 收完尾; 最后关闭底层 sqlite。cron scheduler 的停止职责
// 由 bootstrap 独立管理, 顺序见 actions_app.go 的注释。
func (s *Service) Stop() {
	if s.shutdownCancel == nil {
		return
	}
	s.shutdownCancel()
}

// ListItems 按 options 拉取媒体库列表。keyword / year / size 过滤和排序全部下推到 SQL,
// 只把已经筛选+排序好的行反序列化回来, 避免 1.4 里描述的 "一把 SELECT * + 全表 Unmarshal" 问题。
//
// 精确匹配字段 (title/number/name/release_year/total_size) 来自专用索引列,
// 由 upsertDetail 在写入时同步更新; 002 migration 升级场景下, 历史行的这几列
// 仍是默认零值, 需要用户手动触发一次 "同步媒体库" 让 upsertDetail 重写覆盖,
// 否则 keyword/year/size 过滤会漏命中这些旧行。
func (s *Service) ListItems(ctx context.Context, options ListItemsOptions) ([]Item, error) {
	query, args := buildListItemsQuery(options)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list media library items failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	items := make([]Item, 0, 32)
	for rows.Next() {
		var id int64
		var raw string
		var createdAt int64
		if err := rows.Scan(&id, &raw, &createdAt); err != nil {
			return nil, fmt.Errorf("scan media library item failed: %w", err)
		}
		var item Item
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode media library item failed: %w", err)
		}
		item.ID = id
		item.CreatedAt = createdAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media library items failed: %w", err)
	}
	return items, nil
}

func (s *Service) GetDetail(ctx context.Context, id int64) (*Detail, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT detail_json
		FROM yamdc_media_library_tab
		WHERE id = ?
	`, id).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get media library detail failed: %w", err)
	}
	var detail Detail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("decode media library detail failed: %w", err)
	}
	detail.Item.ID = id
	return &detail, nil
}

func (s *Service) GetDetailByRelPath(ctx context.Context, relPath string) (*Detail, error) {
	var id int64
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, detail_json
		FROM yamdc_media_library_tab
		WHERE rel_path = ?
	`, relPath).Scan(&id, &raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get media library detail by rel path failed: %w", err)
	}
	var detail Detail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("decode media library detail failed: %w", err)
	}
	detail.Item.ID = id
	return &detail, nil
}

func (s *Service) UpdateItem(ctx context.Context, id int64, meta Meta) (*Detail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	absPath := filepath.Join(s.libraryDir, filepath.FromSlash(detail.Item.RelPath))
	next, err := s.updateRootItem(s.libraryDir, detail, absPath, meta)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) ReplaceAsset(ctx context.Context, id int64, variantKey, kind, originalName string, data []byte) (
	*Detail,
	error,
) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	absPath := filepath.Join(s.libraryDir, filepath.FromSlash(detail.Item.RelPath))
	next, err := s.replaceRootArtwork(s.libraryDir, detail, absPath, variantKey, kind, originalName, data)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) DeleteFile(ctx context.Context, id int64, fileRelPath string) (*Detail, error) {
	detail, err := s.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	next, err := s.deleteRootFile(s.libraryDir, detail.Item.RelPath, fileRelPath)
	if err != nil {
		return nil, err
	}
	if err := s.upsertDetail(ctx, next); err != nil {
		return nil, err
	}
	next.Item.ID = id
	next.Item.CreatedAt = detail.Item.CreatedAt
	return next, nil
}

func (s *Service) ResolveLibraryPath(raw string) (string, string, error) {
	return s.resolveRootPath(s.libraryDir, raw)
}

func (s *Service) TriggerFullSync(ctx context.Context) error {
	return s.triggerFullSyncWithReason(ctx, "manual")
}

// triggerFullSyncWithReason 是手动 (TriggerFullSync) 和自动 (scheduler)
// 触发共享的入口, reason 仅用于日志 + task_state_tab.message 区分来源。
// 所有排他性检查都在这里统一执行, 上层 (web handler / scheduler) 只需要
// 根据返回 error 类型决定怎么向用户展示。
func (s *Service) triggerFullSyncWithReason(ctx context.Context, reason string) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskSync), zap.String("reason", reason))
	if !s.IsConfigured() {
		logger.Warn("media library sync skipped because library dir is not configured")
		return errLibraryDirNotConfigured
	}
	if s.isMoveRunning() {
		logger.Warn("media library sync skipped because move task is running")
		return errMoveTaskRunning
	}
	if syncState, err := s.getTaskState(ctx, TaskSync); err == nil && syncState.Status == "running" {
		logger.Warn("media library sync skipped because sync task is already running")
		return errSyncAlreadyRunning
	}
	logger.Info("media library sync triggered")
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		// runFullSync 内部已通过 failTask 把错误写入 task_state_tab 并记日志,
		// 前端可以通过 /api/media-library/status 看到具体失败原因,
		// 因此这里显式忽略返回值是安全的。
		_ = s.runFullSync(context.WithoutCancel(ctx), reason)
	}()
	return nil
}

func (s *Service) recoverTaskStates(ctx context.Context) error {
	for _, taskKey := range []string{TaskSync, TaskMove} {
		if err := s.recoverTaskState(ctx, taskKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recoverTaskState(ctx context.Context, taskKey string) error {
	state, err := s.getTaskState(ctx, taskKey)
	if err != nil {
		return err
	}
	if state.Status != "running" {
		return nil
	}
	now := time.Now().UnixMilli()
	state.Status = "failed"
	state.Message = "server restarted while task running"
	state.FinishedAt = now
	state.UpdatedAt = now
	if err := s.saveTaskState(ctx, state); err != nil {
		return err
	}
	logutil.GetLogger(ctx).Warn("recover media library task state from running to failed", zap.String("task", taskKey))
	// Sync 被进程重启中断: cleanupStaleItems 可能没跑, 之后新增/删除的目录
	// 都会让 DB 和磁盘脱节, 只能通过一次完整 sync 重新对齐。所以这里强制
	// 标 dirty, 下一次 startup 延迟窗口或 03:00 自动触发兜底。
	// Move 被中断理论上不会让 DB 失真 (move 本身只改磁盘, 而且 1.4 之后
	// move 会走 per-item upsert, 下一节会提到), 这里还是顺带标 dirty,
	// 属于 "多做一点总比漏一点好" 的保守选择。
	if err := s.markSyncDirty(ctx); err != nil {
		logutil.GetLogger(ctx).Warn("mark media library sync dirty after recover failed",
			zap.String("task", taskKey), zap.Error(err))
	}
	if taskKey == TaskSync {
		if err := s.appendSyncLog(ctx, "", SyncLogLevelError, "",
			"检测到上一次媒体库同步被中断 (可能是进程被重启或崩溃), 下一次自动同步窗口会重新对齐。"); err != nil {
			logutil.GetLogger(ctx).Warn("append recover sync log failed", zap.Error(err))
		}
	}
	return nil
}

func (s *Service) TriggerMove(ctx context.Context) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskMove))
	if !s.IsConfigured() {
		logger.Warn("move to media library skipped because library dir is not configured")
		return errLibraryDirNotConfigured
	}
	if s.saveDir == "" {
		logger.Warn("move to media library skipped because save dir is not configured")
		return errSaveDirNotConfigured
	}
	if s.isSyncRunning() {
		logger.Warn("move to media library skipped because sync task is running")
		return errSyncTaskRunning
	}
	if !s.claimMove() {
		logger.Warn("move to media library skipped because move task is already running")
		return errMoveAlreadyRunning
	}
	logger.Info("move to media library triggered")
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		// runMove 内部已通过 failTask 把错误写入 task_state_tab 并记日志,
		// 前端可以通过 /api/media-library/status 看到具体失败原因,
		// 因此这里显式忽略返回值是安全的。
		_ = s.runMove(context.WithoutCancel(ctx))
	}()
	return nil
}

func (s *Service) IsMoveRunning() bool {
	return s.isMoveRunning()
}

func (s *Service) GetStatusSnapshot(ctx context.Context) (*StatusSnapshot, error) {
	syncState, err := s.getTaskState(ctx, TaskSync)
	if err != nil {
		return nil, err
	}
	moveState, err := s.getTaskState(ctx, TaskMove)
	if err != nil {
		return nil, err
	}
	return &StatusSnapshot{
		Configured: s.IsConfigured(),
		Sync:       syncState,
		Move:       moveState,
	}, nil
}

func (s *Service) runFullSync(ctx context.Context, reason string) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskSync), zap.String("reason", reason))
	startedAt := time.Now()
	if !s.IsConfigured() || (reason != "move" && s.isMoveRunning()) || !s.claimSync() {
		return nil
	}
	defer s.finishSync()

	runID := newRunID(startedAt)
	logger = logger.With(zap.String("run_id", runID))
	if err := s.appendSyncLog(ctx, runID, SyncLogLevelInfo, "",
		fmt.Sprintf("媒体库同步开始 (reason=%s)", reason)); err != nil {
		logger.Warn("append sync start log failed", zap.Error(err))
	}
	// finalize: 无论 sync 从哪条分支退出都清 dirty。
	// "即使有 error 也清 dirty" 是刻意的: 避免 dirty 永远是 1 导致每天
	// 03:00 重跑同一个必败 sync, 一直写同样的 error 日志, 用户只能从
	// UI 的 '查看同步日志' 里发现并人肉处理。
	//
	// 日志 retention 裁剪 1.5 起挪到 LogCleanupJob (独立 cron 条目),
	// 不再挂在 sync 收尾: sync 可能因为 dirty=false 几天都不触发, 但
	// scrape_job 日志一直在进, cleanup 不能被 sync 绑死。
	defer func() {
		if err := s.clearSyncDirty(ctx); err != nil {
			logger.Warn("clear media library sync dirty failed", zap.Error(err))
		}
	}()

	itemDirs, err := s.listRootItemDirs(s.libraryDir)
	if err != nil {
		s.failTask(ctx, logger, TaskSync, "list media library directories failed", err)
		_ = s.appendSyncLog(ctx, runID, SyncLogLevelError, "",
			fmt.Sprintf("列出媒体库目录失败: %s", err.Error()))
		return err
	}
	logger.Info("media library sync started", zap.Int("total", len(itemDirs)))
	state := newRunningTaskState(TaskSync, len(itemDirs), "同步媒体库中")
	_ = s.saveTaskState(ctx, state)
	keep := s.syncAllItems(ctx, logger, itemDirs, &state, runID)
	deletedCount := s.cleanupStaleItems(ctx, logger, keep, &state)
	s.finishTask(ctx, &state, fmt.Sprintf("媒体库同步完成 (%s)", reason))
	duration := time.Since(startedAt)
	logger.Info("media library sync completed",
		zap.Int("total", state.Total),
		zap.Int("success_count", state.SuccessCount),
		zap.Int("error_count", state.ErrorCount),
		zap.Int("deleted_count", deletedCount),
		zap.Duration("duration", duration),
	)
	summaryLevel := SyncLogLevelInfo
	if state.ErrorCount > 0 {
		// pipeline 整体收尾成功, 但有 per-item 失败, 用 warn 让用户在日志
		// 弹窗里能一眼筛出来。
		summaryLevel = SyncLogLevelWarn
	}
	if err := s.appendSyncLog(ctx, runID, summaryLevel, "",
		fmt.Sprintf("媒体库同步完成: total=%d success=%d error=%d deleted=%d duration=%s",
			state.Total, state.SuccessCount, state.ErrorCount, deletedCount,
			duration.Round(time.Second))); err != nil {
		logger.Warn("append sync completion log failed", zap.Error(err))
	}
	return nil
}

func (s *Service) syncAllItems(
	ctx context.Context,
	logger *zap.Logger,
	itemDirs []string,
	state *TaskState,
	runID string,
) map[string]struct{} {
	keep := make(map[string]struct{}, len(itemDirs))
	for index, absPath := range itemDirs {
		logger.Info("media library sync item started",
			zap.Int("index", index+1), zap.Int("total", len(itemDirs)), zap.String("abs_path", absPath))
		itemStartedAt := time.Now()
		result := s.syncOneItem(ctx, logger, keep, absPath, runID)
		state.Processed = index + 1
		state.Current = result.RelPath
		if result.Success {
			state.SuccessCount++
		}
		if result.Failed {
			state.ErrorCount++
		}
		logger.Info("media library sync item finished",
			zap.Int("index", index+1), zap.Int("total", len(itemDirs)),
			zap.String("rel_path", result.RelPath), zap.Bool("success", result.Success),
			zap.Bool("failed", result.Failed), zap.Duration("duration", time.Since(itemStartedAt)))
		s.persistTaskProgress(ctx, state)
	}
	return keep
}

func (s *Service) cleanupStaleItems(
	ctx context.Context,
	logger *zap.Logger,
	keep map[string]struct{},
	state *TaskState,
) int {
	deletedCount, err := s.deleteMissing(ctx, keep)
	if err != nil {
		logger.Warn("delete stale media library items failed", zap.Error(err))
		state.ErrorCount++
	} else if deletedCount > 0 {
		logger.Info("media library stale rows deleted", zap.Int("count", deletedCount))
	}
	return deletedCount
}

func (s *Service) runMove(ctx context.Context) error {
	logger := logutil.GetLogger(ctx).With(zap.String("task", TaskMove))
	defer s.finishMove()

	// 标 dirty: move 本身只是 rename 目录, 后面 moveOneItem 会对每个成功
	// 移过去的 item 做 per-item upsertDetail 把 DB 写对, 所以正常路径下
	// 不需要额外全量 sync。但 rename 后到 upsertDetail 之间一旦 crash,
	// 新 item 就既在磁盘又不在 DB, 靠 dirty + 下次 auto sync 兜底。
	// 开头就置 dirty 的另一个考量: 即使所有 upsert 都失败, dirty 也已经
	// 在, 不会漏记。
	if err := s.markSyncDirty(ctx); err != nil {
		logger.Warn("mark media library sync dirty at move start failed", zap.Error(err))
	}

	itemDirs, err := s.listRootItemDirs(s.saveDir)
	if err != nil {
		s.failTask(ctx, logger, TaskMove, "list save directories before move failed", err)
		return err
	}
	logger.Info("move to media library started", zap.Int("total", len(itemDirs)))
	state := newRunningTaskState(TaskMove, len(itemDirs), "移动到媒体库中")
	_ = s.saveTaskState(ctx, state)
	for index, absPath := range itemDirs {
		logger.Info("move to media library item started",
			zap.Int("index", index+1),
			zap.Int("total", len(itemDirs)),
			zap.String("abs_path", absPath),
		)
		itemStartedAt := time.Now()
		result := s.moveOneItem(ctx, logger, absPath)
		state.Processed = index + 1
		state.Current = result.RelPath
		if result.Success {
			state.SuccessCount++
		}
		if result.Conflict {
			state.ConflictCount++
		}
		if result.Failed {
			state.ErrorCount++
		}
		logger.Info("move to media library item finished",
			zap.Int("index", index+1),
			zap.Int("total", len(itemDirs)),
			zap.String("rel_path", result.RelPath),
			zap.Bool("success", result.Success),
			zap.Bool("conflict", result.Conflict),
			zap.Bool("failed", result.Failed),
			zap.Duration("duration", time.Since(itemStartedAt)),
		)
		s.persistTaskProgress(ctx, &state)
	}
	s.finishTask(ctx, &state, "移动到媒体库完成")
	logger.Info("move to media library completed",
		zap.Int("total", state.Total),
		zap.Int("success_count", state.SuccessCount),
		zap.Int("conflict_count", state.ConflictCount),
		zap.Int("error_count", state.ErrorCount),
	)
	return nil
}

func moveDirectory(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	linkErr := &os.LinkError{}
	ok := errors.As(err, &linkErr)
	if !ok || !errors.Is(linkErr.Err, syscall.EXDEV) {
		return fmt.Errorf("rename directory: %w", err)
	}
	if err := copyDirectory(src, dst); err != nil {
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("remove source after copy: %w", err)
	}
	return nil
}

type itemTaskResult struct {
	RelPath  string
	Success  bool
	Conflict bool
	Failed   bool
}

func newRunningTaskState(taskKey string, total int, message string) TaskState {
	now := time.Now().UnixMilli()
	return TaskState{
		TaskKey:   taskKey,
		Status:    "running",
		Total:     total,
		Message:   message,
		StartedAt: now,
		UpdatedAt: now,
	}
}

func (s *Service) failTask(ctx context.Context, logger *zap.Logger, taskKey, message string, err error) {
	logger.Error(message, zap.Error(err))
	now := time.Now().UnixMilli()
	_ = s.saveTaskState(ctx, TaskState{
		TaskKey:    taskKey,
		Status:     "failed",
		Message:    err.Error(),
		UpdatedAt:  now,
		FinishedAt: now,
	})
}

func (s *Service) persistTaskProgress(ctx context.Context, state *TaskState) {
	state.UpdatedAt = time.Now().UnixMilli()
	_ = s.saveTaskState(ctx, *state)
}

func (s *Service) finishTask(ctx context.Context, state *TaskState, message string) {
	state.Status = "completed"
	state.Message = message
	state.Current = ""
	state.UpdatedAt = time.Now().UnixMilli()
	state.FinishedAt = state.UpdatedAt
	_ = s.saveTaskState(ctx, *state)
}

func (s *Service) syncOneItem(
	ctx context.Context,
	logger *zap.Logger,
	keep map[string]struct{},
	absPath string,
	runID string,
) itemTaskResult {
	relPath, err := filepath.Rel(s.libraryDir, absPath)
	if err != nil {
		logger.Warn("resolve media library relative path failed", zap.String("abs_path", absPath), zap.Error(err))
		_ = s.appendSyncLog(ctx, runID, SyncLogLevelWarn, absPath,
			fmt.Sprintf("无法解析相对路径: %s", err.Error()))
		return itemTaskResult{Failed: true}
	}
	relPath = filepath.ToSlash(relPath)
	detail, err := s.readRootDetail(s.libraryDir, relPath, absPath)
	if err != nil {
		logger.Warn("read media library detail failed", zap.String("rel_path", relPath), zap.Error(err))
		_ = s.appendSyncLog(ctx, runID, SyncLogLevelWarn, relPath,
			fmt.Sprintf("读取 item 详情失败: %s", err.Error()))
		return itemTaskResult{RelPath: relPath, Failed: true}
	}
	if err := s.upsertDetail(ctx, detail); err != nil {
		logger.Warn("upsert media library detail failed", zap.String("rel_path", relPath), zap.Error(err))
		_ = s.appendSyncLog(ctx, runID, SyncLogLevelWarn, relPath,
			fmt.Sprintf("写入 item 详情失败: %s", err.Error()))
		return itemTaskResult{RelPath: relPath, Failed: true}
	}
	keep[relPath] = struct{}{}
	logger.Info("media library detail synced",
		zap.String("rel_path", relPath),
		zap.String("title", detail.Item.Title),
		zap.String("number", detail.Item.Number),
		zap.String("release_date", detail.Item.ReleaseDate),
		zap.Int("variant_count", len(detail.Variants)),
		zap.Int("file_count", len(detail.Files)),
	)
	return itemTaskResult{RelPath: relPath, Success: true}
}

func (s *Service) moveOneItem(ctx context.Context, logger *zap.Logger, absPath string) itemTaskResult {
	relPath, err := filepath.Rel(s.saveDir, absPath)
	if err != nil {
		logger.Warn("resolve save relative path failed", zap.String("abs_path", absPath), zap.Error(err))
		return itemTaskResult{Failed: true}
	}
	relPath = filepath.ToSlash(relPath)
	targetAbs := filepath.Join(s.libraryDir, filepath.FromSlash(relPath))
	if _, err := os.Stat(targetAbs); err == nil {
		logger.Warn("move to media library skipped because target already exists",
			zap.String("rel_path", relPath),
			zap.String("src_path", absPath),
			zap.String("dst_path", targetAbs),
		)
		return itemTaskResult{RelPath: relPath, Conflict: true}
	} else if !os.IsNotExist(err) {
		logger.Warn("check target media library path failed", zap.String("rel_path", relPath), zap.Error(err))
		return itemTaskResult{RelPath: relPath, Failed: true}
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		logger.Warn("create media library parent directory failed", zap.String("rel_path", relPath), zap.Error(err))
		return itemTaskResult{RelPath: relPath, Failed: true}
	}
	if err := moveDirectory(absPath, targetAbs); err != nil {
		logger.Warn("move directory to media library failed", zap.String("rel_path", relPath), zap.Error(err))
		return itemTaskResult{RelPath: relPath, Failed: true}
	}
	logger.Info("move directory to media library succeeded",
		zap.String("rel_path", relPath),
		zap.String("src_path", absPath),
		zap.String("dst_path", targetAbs),
	)
	// per-item 增量 upsert: 目录刚搬过来, 直接把 item_json / 索引列写 DB,
	// 用户回到前端立即能看到这条新入库记录, 不用等 "同步媒体库" 或下次
	// 03:00 自动任务。比起 3.4 时代那条 "太慢了, 用户手动触发即可" 的注释,
	// 这里的代价是 O(被移动数) 次 readRootDetail + INSERT/UPSERT, 相对
	// 上一段 moveDirectory 的 IO 可忽略。
	//
	// 就算这一段失败, 目录已经在媒体库里, dirty flag 在 runMove 开头已经
	// 置 1, 下一次 auto-sync 会兜底把它补进 DB。所以这里拿 warn 级别记录
	// 即可, 不把整个 item 标 Failed (磁盘搬迁成功才是 move 本职)。
	detail, err := s.readRootDetail(s.libraryDir, relPath, targetAbs)
	if err != nil {
		logger.Warn("read moved item detail failed; will be picked up by next auto sync",
			zap.String("rel_path", relPath), zap.Error(err))
		return itemTaskResult{RelPath: relPath, Success: true}
	}
	if err := s.upsertDetail(ctx, detail); err != nil {
		logger.Warn("upsert moved item detail failed; will be picked up by next auto sync",
			zap.String("rel_path", relPath), zap.Error(err))
		return itemTaskResult{RelPath: relPath, Success: true}
	}
	return itemTaskResult{RelPath: relPath, Success: true}
}

func copyDirectory(src, dst string) error {
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("compute relative path: %w", err)
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			if err := os.MkdirAll(target, info.Mode()); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent directory: %w", err)
		}
		in, err := os.Open(path) //nolint:gosec // filepath.WalkDir callback on trusted directory
		if err != nil {
			return fmt.Errorf("open source file: %w", err)
		}
		defer func() {
			_ = in.Close()
		}()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return fmt.Errorf("create target file: %w", err)
		}
		if _, err := out.ReadFrom(in); err != nil {
			_ = out.Close()
			return fmt.Errorf("copy file data: %w", err)
		}
		return out.Close()
	})
	if err != nil {
		return fmt.Errorf("copy directory: %w", err)
	}
	return nil
}

func (s *Service) upsertDetail(ctx context.Context, detail *Detail) error {
	itemRaw, err := json.Marshal(detail.Item)
	if err != nil {
		return fmt.Errorf("marshal media library item failed: %w", err)
	}
	detailRaw, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal media library detail failed: %w", err)
	}
	now := time.Now().UnixMilli()
	// 1.4: number/name/release_year/total_size 是专供 ListItems 过滤/排序的索引列,
	// 必须在每次 upsert 时和 item_json 保持一致, 否则 ListItems 会漏命中。
	releaseYr := releaseYear(detail.Item.ReleaseDate)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO yamdc_media_library_tab (
			rel_path, title, release_date, updated_at, poster_path, cover_path,
			item_json, detail_json, created_at,
			number, name, release_year, total_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			title = excluded.title,
			release_date = excluded.release_date,
			updated_at = excluded.updated_at,
			poster_path = excluded.poster_path,
			cover_path = excluded.cover_path,
			item_json = excluded.item_json,
			detail_json = excluded.detail_json,
			number = excluded.number,
			name = excluded.name,
			release_year = excluded.release_year,
			total_size = excluded.total_size
	`, detail.Item.RelPath, detail.Item.Title, detail.Item.ReleaseDate, detail.Item.UpdatedAt, detail.Item.PosterPath,
		detail.Item.CoverPath, string(itemRaw), string(detailRaw), now,
		detail.Item.Number, detail.Item.Name, releaseYr, detail.Item.TotalSize)
	if err != nil {
		return fmt.Errorf("upsert media library detail failed: %w", err)
	}
	return nil
}

func (s *Service) deleteMissing(ctx context.Context, keep map[string]struct{}) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rel_path FROM yamdc_media_library_tab`)
	if err != nil {
		return 0, fmt.Errorf("list existing media library rel paths failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	paths := make([]string, 0, 32)
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			return 0, fmt.Errorf("scan media library rel path failed: %w", err)
		}
		if _, ok := keep[relPath]; !ok {
			paths = append(paths, relPath)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate media library rel paths failed: %w", err)
	}
	for _, relPath := range paths {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM yamdc_media_library_tab WHERE rel_path = ?`, relPath); err != nil {
			return 0, fmt.Errorf("delete stale media library row failed: %w", err)
		}
	}
	return len(paths), nil
}

func (s *Service) getTaskState(ctx context.Context, taskKey string) (TaskState, error) {
	state := TaskState{TaskKey: taskKey, Status: "idle"}
	err := s.db.QueryRowContext(ctx, `
		SELECT status, total, processed, success_count, conflict_count, error_count, current, message, started_at,
			finished_at, updated_at
		FROM yamdc_task_state_tab
		WHERE task_key = ?
	`, taskKey).Scan(
		&state.Status,
		&state.Total,
		&state.Processed,
		&state.SuccessCount,
		&state.ConflictCount,
		&state.ErrorCount,
		&state.Current,
		&state.Message,
		&state.StartedAt,
		&state.FinishedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, nil
		}
		return TaskState{}, fmt.Errorf("get task state failed: %w", err)
	}
	return state, nil
}

func (s *Service) saveTaskState(ctx context.Context, state TaskState) error {
	if state.TaskKey == "" {
		return nil
	}
	if state.UpdatedAt == 0 {
		state.UpdatedAt = time.Now().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO yamdc_task_state_tab (
			task_key, status, total, processed, success_count, conflict_count, error_count, current, message, started_at,
				finished_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_key) DO UPDATE SET
			status = excluded.status,
			total = excluded.total,
			processed = excluded.processed,
			success_count = excluded.success_count,
			conflict_count = excluded.conflict_count,
			error_count = excluded.error_count,
			current = excluded.current,
			message = excluded.message,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			updated_at = excluded.updated_at
	`, state.TaskKey, state.Status, state.Total, state.Processed, state.SuccessCount, state.ConflictCount,
		state.ErrorCount, state.Current, state.Message, state.StartedAt, state.FinishedAt, state.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save task state failed: %w", err)
	}
	return nil
}

func (s *Service) claimSync() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.syncRunning {
		return false
	}
	s.syncRunning = true
	return true
}

func (s *Service) finishSync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncRunning = false
}

func (s *Service) claimMove() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.moveRunning {
		return false
	}
	s.moveRunning = true
	return true
}

func (s *Service) finishMove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveRunning = false
}

func (s *Service) isMoveRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveRunning
}

func (s *Service) isSyncRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncRunning
}

// WaitBackground 阻塞直到通过 TriggerFullSync / TriggerMove 启动的后台
// goroutine 全部返回。目前主要用于测试: 在关闭底层 sqlite / 清理 tempdir
// 之前同步等待, 避免异步 DB 写入与清理竞争。生产侧若需要 graceful shutdown,
// 也可直接调用本方法等后台任务收尾。
func (s *Service) WaitBackground() {
	s.bgWG.Wait()
}

// buildListItemsQuery 拼出 ListItems 使用的 SQL。filter/sort 全部下推到 SQL 层,
// 依赖 002 migration 添加的索引列 (number/name/release_year/total_size)。
//
// keyword 精匹配: LOWER(title/number/name) LIKE LOWER('%k%')
//   - 002 之前的实现在 item_json 上做一次粗 LIKE, 命中后仍需要在 Go 里反序列化
//     每一行再逐字段比较, 库大以后代价线性飙升。
//   - 现在直接在 3 个专用列上 LIKE, 命中行即为结果集, 无需额外字段比较;
//     LOWER 做一遍函数调用是为了保持 "对 ASCII 大小写不敏感" 的既有语义,
//     同时对 CJK 幂等, 不改变匹配行为。
//
// year / size: 用 release_year / total_size 索引列做等值或区间过滤,
// 完全等价于原 Go 层的 releaseYear() + matchSizeFilter() 语义。
//
// sort: 走 ORDER BY + 固定 tie-breaker (updated_at, id), 和原 sortItems 的语义一致。
func buildListItemsQuery(options ListItemsOptions) (string, []any) {
	var sb strings.Builder
	sb.WriteString("SELECT id, item_json, created_at FROM yamdc_media_library_tab")

	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)

	keyword := strings.TrimSpace(options.Keyword)
	if keyword != "" {
		// 旧实现用 strings.Contains 做字面子串匹配, keyword 里的 '%' / '_' 是普通字符;
		// 直接用 LIKE 则会被当作通配符, 语义漂移。escapeLikePattern 把它们转义掉,
		// 并配合 `ESCAPE '\'` 子句锁定转义字符, 保持和旧实现一致。
		pattern := "%" + escapeLikePattern(strings.ToLower(keyword)) + "%"
		conditions = append(conditions,
			`(LOWER(title) LIKE ? ESCAPE '\' OR LOWER(number) LIKE ? ESCAPE '\' OR LOWER(name) LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern, pattern)
	}
	year := strings.TrimSpace(options.Year)
	if year != "" && year != "all" {
		conditions = append(conditions, "release_year = ?")
		args = append(args, year)
	}
	if low, high, ok := sizeFilterBounds(options.SizeFilter); ok {
		if low > 0 {
			conditions = append(conditions, "total_size >= ?")
			args = append(args, low)
		}
		if high > 0 {
			conditions = append(conditions, "total_size < ?")
			args = append(args, high)
		}
	}
	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}
	sortExpr := sortModeToColumn(options.Sort)
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(options.Order), "asc") {
		direction = "ASC"
	}
	// tie-breaker 跟 direction 对齐, 和历史 sortItems 的行为保持一致 (同方向二级排序)。
	// 当主排序列本身就是 updated_at 时避免把它再重复一次, 冗余项不影响结果, 只是读起来噪。
	if sortExpr == "updated_at" {
		fmt.Fprintf(&sb, " ORDER BY updated_at %s, id %s", direction, direction)
	} else {
		fmt.Fprintf(&sb, " ORDER BY %s %s, updated_at %s, id %s",
			sortExpr, direction, direction, direction)
	}
	return sb.String(), args
}

// escapeLikePattern 把 LIKE 通配符 ('%' / '_') 和转义符 '\' 本身全部前置 '\',
// 让 `LIKE ... ESCAPE '\'` 真正做字面子串匹配。
// 这个函数假设调用方已经把要转义的字符串 trim / lower 过, 不会改大小写或首尾空白。
func escapeLikePattern(s string) string {
	if !strings.ContainsAny(s, `%_\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// sortModeToColumn 把前端传的 sort 模式映射到 SQL 的 ORDER BY 表达式。
// "title" 模式下 Go 层曾经用 firstNonEmpty(Title, Name) 作比较键, 这里用
// CASE 表达式等价翻译: title 为空时 fallback 到 name。
func sortModeToColumn(sortMode string) string {
	switch strings.TrimSpace(sortMode) {
	case "title":
		return "CASE WHEN title = '' THEN name ELSE title END"
	case "size":
		return "total_size"
	case "year":
		return "release_year"
	case "ingested":
		return "created_at"
	case "updated":
		return "updated_at"
	default:
		return "updated_at"
	}
}

// sizeFilterBounds 把 size 过滤 token 翻译成 [low, high) 字节数区间。
// 返回第 3 位 ok=false 表示 "不过滤" ("", "all", 或未知值);
// low=0 代表 "下限不约束", high=0 代表 "上限不约束"。
//
// 与旧 matchSizeFilter 保持逐个 case 的语义一致 (含 lt-1 / lt-5 这种只有上限的桶),
// 未知 token 和 "all" 一样返回 ok=false, 避免下推到 SQL 后反而把结果全过滤空。
func sizeFilterBounds(sizeFilter string) (int64, int64, bool) {
	const gb = int64(1024 * 1024 * 1024)
	switch sizeFilter {
	case "", "all":
		return 0, 0, false
	case "lt-1":
		return 0, gb, true
	case "1-2":
		return gb, 2 * gb, true
	case "2-5":
		return 2 * gb, 5 * gb, true
	case "lt-5":
		return 0, 5 * gb, true
	case "5-10":
		return 5 * gb, 10 * gb, true
	case "10-20":
		return 10 * gb, 20 * gb, true
	case "5-20":
		return 5 * gb, 20 * gb, true
	case "20-50":
		return 20 * gb, 50 * gb, true
	case "50-plus":
		return 50 * gb, 0, true
	default:
		return 0, 0, false
	}
}

func releaseYear(value string) string {
	for start := 0; start+4 <= len(value); start++ {
		chunk := value[start : start+4]
		valid := true
		for _, char := range chunk {
			if char < '0' || char > '9' {
				valid = false
				break
			}
		}
		if valid {
			return chunk
		}
	}
	return ""
}
