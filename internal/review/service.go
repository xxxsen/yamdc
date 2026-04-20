// Package review 把"影片刮削完成 → 人工复核 → 最终导入"这段工作流从
// internal/job 里独立出来。job 包只负责 init → processing → reviewing/failed
// 的常规生命周期; 进入 reviewing 之后的所有交互 (保存 review 数据 / 裁剪海报 /
// 调用 importGuard / 冲突校验 / 落盘 + MarkDone) 都在本包内闭环。
//
// 依赖方向: review → job (单向)。review 通过 job.Service 上导出的几个协作
// 原语 (Claim/Finish/AddJobLog/ResolveJobSourcePath/GetBlockingConflict) 复用
// job 的运行时锁和日志/源文件解析能力, 不反向污染 job 包。
//
// 并发模型: 所有会写 scrape_data (SaveReviewData / CropPosterFromCover)
// 或落盘 (Import) 的对外方法都走 coordinator.Claim/Finish 同一把锁, 保证
// 同一 job_id 上的"保存 / 裁剪 / 导入"操作串行执行, 避免 Import 在读 scrape
// 快照与落盘之间被 SaveReviewData 改掉 review_data 造成 NFO 与 meta 错位。
// 被并发调用的那一侧会立刻拿到 job.ErrJobAlreadyRunning, 交由调用方 (通常是
// 前端) 提示用户稍后重试。
package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdimage "image"
	"strings"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/capture"
	imgutil "github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/store"
)

// 错误集合: review 独占的几个语义错误。跨包共享的错误 (如 job 不存在 /
// job 正在运行 / 冲突) 直接引用 job 包的 Err*, 避免出现两个含义相同但
// 值不同的 sentinel, 给调用方带来 errors.Is 失配的隐患。
var (
	ErrJobNotReviewing          = errors.New("job is not in reviewing status")
	ErrScrapeDataNotFound       = errors.New("scrape data not found")
	ErrScrapeDataNumberMismatch = errors.New(
		"scrape snapshot number mismatches job number, please re-scrape")
	ErrCoverNotFound       = errors.New("cover not found")
	ErrCropRectOutOfBounds = errors.New("crop rectangle out of bounds")
)

// JobCoordinator 抽象出 review 对 job.Service 的依赖, 方便在不启动整套 job 的
// 前提下单测 review 的行为。生产路径由 *job.Service 自动满足该接口。
type JobCoordinator interface {
	Claim(jobID int64) bool
	Finish(jobID int64)
	AddJobLog(ctx context.Context, jobID int64, level, stage, message, detail string)
	ResolveJobSourcePath(ctx context.Context, j *jobdef.Job) (string, error)
	GetBlockingConflict(ctx context.Context, j *jobdef.Job) (*job.Conflict, error)
}

// Service 承载 reviewing 工作流的全部对外能力。
type Service struct {
	jobRepo     *repository.JobRepository
	scrapeRepo  *repository.ScrapeDataRepository
	capture     *capture.Capture
	storage     store.IStorage
	coordinator JobCoordinator
	importGuard func(context.Context) error
}

func NewService(
	coordinator JobCoordinator,
	jobRepo *repository.JobRepository,
	scrapeRepo *repository.ScrapeDataRepository,
	capt *capture.Capture,
	storage store.IStorage,
) *Service {
	return &Service{
		jobRepo:     jobRepo,
		scrapeRepo:  scrapeRepo,
		capture:     capt,
		storage:     storage,
		coordinator: coordinator,
	}
}

// SetImportGuard 注册一个在 Import 落盘前必须通过的前置检查 (例如"媒体库
// move 正在进行, 此时 Import 会和 move 抢目录")。传 nil 等价于不做检查。
func (s *Service) SetImportGuard(fn func(context.Context) error) {
	s.importGuard = fn
}

// SaveReviewData 存用户在 review 页编辑过的 meta。必须在 reviewing 状态下调用,
// 否则直接返回 ErrJobNotReviewing, 不会污染 scrape_data。
// 与 Import / CropPosterFromCover 共享同一把 Claim 锁: 若此时正在 Import,
// 立即返回 job.ErrJobAlreadyRunning, 避免 Import 读完快照再被覆盖。
func (s *Service) SaveReviewData(ctx context.Context, jobID int64, reviewData string) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	if !s.coordinator.Claim(jobID) {
		logger.Warn("save review data skipped because job is already running")
		return job.ErrJobAlreadyRunning
	}
	defer s.coordinator.Finish(jobID)
	j, err := s.loadJobOrNotFound(ctx, jobID)
	if err != nil {
		logger.Error("load job before saving review data failed", zap.Error(err))
		return err
	}
	if j.Status != jobdef.StatusReviewing {
		return ErrJobNotReviewing
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(reviewData), &meta); err != nil {
		return fmt.Errorf("invalid review json: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, reviewData); err != nil {
		logger.Error("save review data failed", zap.Error(err))
		return fmt.Errorf("save review data: %w", err)
	}
	s.coordinator.AddJobLog(ctx, jobID, "info", "review", "review data saved", "")
	logger.Info("review data saved", zap.String("number", meta.Number), zap.String("title", meta.Title))
	return nil
}

// CropPosterFromCover 从当前快照的 cover 中裁出指定矩形作为 poster, 并把新的
// review_data (含更新后的 poster key) 持久化。同样仅在 reviewing 状态下生效。
// 与 Import / SaveReviewData 共享同一把 Claim 锁, 细节见 SaveReviewData 的注释。
func (s *Service) CropPosterFromCover(
	ctx context.Context, jobID int64, x, y, width, height int,
) (*model.File, error) {
	logger := logutil.GetLogger(ctx).With(
		zap.Int64("job_id", jobID),
		zap.Int("x", x), zap.Int("y", y),
		zap.Int("width", width), zap.Int("height", height),
	)
	if !s.coordinator.Claim(jobID) {
		logger.Warn("crop poster skipped because job is already running")
		return nil, job.ErrJobAlreadyRunning
	}
	defer s.coordinator.Finish(jobID)
	meta, err := s.loadReviewingMeta(ctx, logger, jobID)
	if err != nil {
		return nil, err
	}
	if meta.Cover == nil || meta.Cover.Key == "" {
		return nil, ErrCoverNotFound
	}
	posterKey, err := s.cropAndStorePoster(ctx, meta.Cover.Key, x, y, width, height)
	if err != nil {
		return nil, err
	}
	meta.Poster = &model.File{Name: "./poster.jpg", Key: posterKey}
	reviewData, err := json.Marshal(&meta)
	if err != nil {
		return nil, fmt.Errorf("marshal review meta failed: %w", err)
	}
	if err := s.scrapeRepo.SaveReviewData(ctx, jobID, string(reviewData)); err != nil {
		logger.Error("save cropped poster review data failed", zap.Error(err))
		return nil, fmt.Errorf("save cropped poster review data: %w", err)
	}
	s.coordinator.AddJobLog(
		ctx, jobID, "info", "review", "poster cropped from cover",
		fmt.Sprintf("%d,%d,%d,%d", x, y, width, height),
	)
	logger.Info("poster cropped from cover", zap.String("poster_key", meta.Poster.Key))
	return meta.Poster, nil
}

// Reject ("打回") 把一个处于 reviewing 状态的 job 退回到可编辑态:
//  1. 状态 reviewing -> failed, error_msg 记录退回原因 (默认 "rejected by reviewer"),
//     前端拿到 failed 态后用户才能再次编辑 number / 重跑 run。
//  2. 删除对应的 scrape_data, 彻底放弃本次抓取结果; 下一次 run 时 capture
//     会用最新的 number 重新搜一遍, 避免残留的 raw_data 污染新一轮比对
//     (尤其和 verifyScrapeSnapshotMatchesJob 的语义兜底一致)。
//  3. 通过 coordinator.AddJobLog 记录人工操作, 方便日志面板回溯。
//
// 与 SaveReviewData/CropPosterFromCover/Import 共享同一把 Claim 锁, 保证
// 不会和正在进行的 Import 同时操作 scrape_data。并发冲突时直接返回
// job.ErrJobAlreadyRunning。
// 若 job 已经不在 reviewing 态, 返回 ErrJobNotReviewing (不做幂等放过) —
// 这是一个"人工意图"操作, 调用方应先刷新列表确认对象。
func (s *Service) Reject(ctx context.Context, jobID int64, reason string) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	if !s.coordinator.Claim(jobID) {
		logger.Warn("reject skipped because job is already running")
		return job.ErrJobAlreadyRunning
	}
	defer s.coordinator.Finish(jobID)

	j, err := s.loadJobOrNotFound(ctx, jobID)
	if err != nil {
		logger.Error("load job before reject failed", zap.Error(err))
		return err
	}
	if j.Status != jobdef.StatusReviewing {
		return ErrJobNotReviewing
	}

	// error_msg 既是 UI 展示的原因, 也是给 Job.ErrorMsg 做日后审计的记录,
	// 控制长度防止 SQL 层被塞爆; 超长时截到 200 字符 + 省略号。
	reasonText := strings.TrimSpace(reason)
	if reasonText == "" {
		reasonText = "rejected by reviewer"
	}
	const maxReasonLen = 200
	if len(reasonText) > maxReasonLen {
		reasonText = reasonText[:maxReasonLen] + "..."
	}

	// 两阶段持久化顺序: 先删 scrape_data, 再改 status。
	//
	// 这是为了让"中途失败"的状态能被用户通过"再次点 Reject"自愈:
	//   - 先 Delete 后 UpdateStatus: 如果 Delete 失败, 状态仍是 reviewing,
	//     用户重试 Reject 走完整流程即可 (DeleteByJobID 对不存在的行幂等);
	//     如果 UpdateStatus 失败, scrape_data 已清空, 重试时 loadJobOrNotFound
	//     + StatusReviewing 校验仍通过, 再走一次 DeleteByJobID (no-op) +
	//     UpdateStatus 就能闭合, 没有卡死路径。
	//   - 反过来先改状态再删 scrape_data: 若 Delete 失败, status 已 failed,
	//     重试会被 "status != reviewing" 拦住, 残留 scrape_data 就只能等
	//     下一次 Run 走 UpsertRawData 覆盖或整条 Delete 级联清掉 — 无法在
	//     Reject 这个入口自愈。
	//
	// 代价: Delete 成功 + UpdateStatus 失败时, 用户短暂看到 "reviewing 但没
	// scrape 快照" 的中间态。点 Import 会立刻在 scrapeRepo.GetByJobID 里拿到
	// repository.ErrScrapeDataNotFound (被 Import 的 "get scrape data for
	// import: %w" 一层 wrap 抛出), 比"failed + 残留 scrape_data"这种隐蔽
	// 残留更显眼, 可接受。
	if err := s.scrapeRepo.DeleteByJobID(ctx, jobID); err != nil {
		logger.Error("reject: delete scrape data failed", zap.Error(err))
		return fmt.Errorf("reject delete scrape data: %w", err)
	}

	ok, err := s.jobRepo.UpdateStatus(
		ctx, jobID,
		[]jobdef.Status{jobdef.StatusReviewing},
		jobdef.StatusFailed,
		reasonText,
	)
	if err != nil {
		logger.Error("reject: update status failed", zap.Error(err))
		return fmt.Errorf("reject update status: %w", err)
	}
	if !ok {
		// 极罕见: UpdateStatus 返回 ok=false 意味着 status 不在预期的 [reviewing]
		// 白名单里 (例如另一并发路径已经把它改成 done/failed)。虽然我们前面已经
		// 校验过 StatusReviewing, 但这里复用 SQL 层的白名单自保, 对竞态直接把
		// "非 reviewing 不能打回" 的语义一路透传上去。
		return ErrJobNotReviewing
	}

	s.coordinator.AddJobLog(ctx, jobID, "info", "review", "review rejected", reasonText)
	logger.Info("review rejected",
		zap.String("number", strings.TrimSpace(j.Number)),
		zap.String("reason", reasonText),
	)
	return nil
}

// loadJobOrNotFound 把 jobRepo.GetByID 的三种返回统一为 (job, err) 两种:
//   - 命中: (job, nil)
//   - 未找到 (repository.ErrJobNotFound): (nil, job.ErrJobNotFound), 使上层
//     用 errors.Is(err, job.ErrJobNotFound) 可命中 (两个 sentinel 共用同一值,
//     见 job/service.go 的注释)
//   - 其它 DB 错误: (nil, wrapped err)
func (s *Service) loadJobOrNotFound(ctx context.Context, jobID int64) (*jobdef.Job, error) {
	j, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, repository.ErrJobNotFound) {
			return nil, job.ErrJobNotFound
		}
		return nil, fmt.Errorf("load job: %w", err)
	}
	if j == nil {
		return nil, job.ErrJobNotFound
	}
	return j, nil
}

func (s *Service) loadReviewingMeta(
	ctx context.Context, logger *zap.Logger, jobID int64,
) (model.MovieMeta, error) {
	j, err := s.loadJobOrNotFound(ctx, jobID)
	if err != nil {
		logger.Error("load job before review action failed", zap.Error(err))
		return model.MovieMeta{}, err
	}
	if j.Status != jobdef.StatusReviewing {
		return model.MovieMeta{}, ErrJobNotReviewing
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return model.MovieMeta{}, fmt.Errorf("get scrape data: %w", err)
	}
	if data == nil {
		return model.MovieMeta{}, ErrScrapeDataNotFound
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return model.MovieMeta{}, fmt.Errorf("parse review meta failed: %w", err)
	}
	return meta, nil
}

func (s *Service) cropAndStorePoster(
	ctx context.Context, coverKey string, x, y, width, height int,
) (string, error) {
	raw, err := store.GetDataFrom(ctx, s.storage, coverKey)
	if err != nil {
		return "", fmt.Errorf("load cover failed: %w", err)
	}
	img, err := imgutil.LoadImage(raw)
	if err != nil {
		return "", fmt.Errorf("decode cover failed: %w", err)
	}
	bounds := img.Bounds()
	rect := stdimage.Rect(x, y, x+width, y+height)
	if rect.Min.X < bounds.Min.X || rect.Min.Y < bounds.Min.Y ||
		rect.Max.X > bounds.Max.X || rect.Max.Y > bounds.Max.Y {
		return "", ErrCropRectOutOfBounds
	}
	cropped, err := imgutil.CutImageViaRectangle(img, rect)
	if err != nil {
		return "", fmt.Errorf("crop poster failed: %w", err)
	}
	croppedRaw, err := imgutil.WriteImageToBytes(cropped)
	if err != nil {
		return "", fmt.Errorf("encode poster failed: %w", err)
	}
	key, err := store.AnonymousPutDataTo(ctx, s.storage, croppedRaw)
	if err != nil {
		return "", fmt.Errorf("store cropped poster: %w", err)
	}
	return key, nil
}

// Import 把 reviewing 状态的 job 落盘: 解析 final payload, 经 capture
// 做最后一次 metaverify + 目录落地, 落成功后 MarkDone。
// 流程:
//  1. Claim / Finish 保证同一 job 同时只能被一个 Import 占用。
//  2. validateImportPreconditions: 状态/Guard/活跃冲突三连校验。
//  3. verifyScrapeSnapshotMatchesJob: 第二道兜底, 防 number 与 meta 错位。
//  4. performImport: 真正落盘 + MarkDone。
func (s *Service) Import(ctx context.Context, jobID int64) error {
	logger := logutil.GetLogger(ctx).With(zap.Int64("job_id", jobID))
	if !s.coordinator.Claim(jobID) {
		logger.Warn("import skipped because job is already running")
		return job.ErrJobAlreadyRunning
	}
	defer s.coordinator.Finish(jobID)

	j, err := s.validateImportPreconditions(ctx, logger, jobID)
	if err != nil {
		return err
	}
	data, err := s.scrapeRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get scrape data for import: %w", err)
	}
	if data == nil {
		return ErrScrapeDataNotFound
	}
	if err := s.verifyScrapeSnapshotMatchesJob(logger, j, data); err != nil {
		return err
	}
	payload := data.RawData
	if data.ReviewData != "" {
		payload = data.ReviewData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return fmt.Errorf("parse final meta failed: %w", err)
	}
	return s.performImport(ctx, logger, j, jobID, &meta, payload)
}

func (s *Service) validateImportPreconditions(
	ctx context.Context, logger *zap.Logger, jobID int64,
) (*jobdef.Job, error) {
	j, err := s.loadJobOrNotFound(ctx, jobID)
	if err != nil {
		logger.Error("load job before import failed", zap.Error(err))
		return nil, err
	}
	if j.Status != jobdef.StatusReviewing {
		return nil, ErrJobNotReviewing
	}
	if s.importGuard != nil {
		if err := s.importGuard(ctx); err != nil {
			logger.Warn("import blocked by guard", zap.Error(err))
			return nil, err
		}
	}
	conflict, err := s.coordinator.GetBlockingConflict(ctx, j)
	if err != nil && !errors.Is(err, job.ErrNoConflict) {
		logger.Error("check job conflict before import failed", zap.Error(err))
		return nil, fmt.Errorf("get blocking conflict: %w", err)
	}
	if conflict != nil {
		logger.Warn("import blocked by conflict",
			zap.String("reason", conflict.Reason),
			zap.String("target", conflict.Target),
		)
		return nil, fmt.Errorf("%s: %s: %w", conflict.Reason, conflict.Target, job.ErrJobConflict)
	}
	return j, nil
}

// verifyScrapeSnapshotMatchesJob 是 3.2.a 的第二道兜底: SQL 层已经冻结
// reviewing/processing 期间的 canonical number / number_source / conflict_key
// (cleaned_number / number_clean_* 仍随 scanner 刷新, 但不参与目录/NFO 构造,
// 故不影响此校验), 但历史上可能已经存在错位的记录, 或者上游未来新增一条
// 改 number 的路径忘了考虑 scrape_data 一致性。这里在 Import 真正落盘前再
// 对比一次: 若 raw_data 中的 meta.Number 与 job.Number 规范化后仍不一致,
// 拒绝导入并提示用户重新 scrape。
// 只校验 raw_data 是因为它代表 scrape 时的原始快照; review_data 是用户手动
// 编辑结果, number 可能被故意改成别的值。
//
// 对"空 / 坏 JSON / 空 number" 的容忍策略:
//   - 空 RawData: 视为"还没抓", 交给下游 ErrScrapeDataNotFound 或 meta unmarshal
//     报错, 不在本函数拦截;
//   - 坏 JSON: 打 warn 日志, 但不阻塞 Import (legacy 快照可能带历史 bug 的字段);
//   - 两边 number 有一侧为空: 不阻塞 (不具备比较基准);
//   - 比对基准是 **base number** (经 number.Parse 剥掉 variant 后的 id), 再
//     走 number.GetCleanID + EqualFold, 避免因:
//     1) 用户手动把 job number 填成 "PXVR-406-CD2" 之类带 variant 的写法,
//     而 scrape 端拿到的仅是基础影片 ID "PXVR-406";
//     2) 书写差异 "SSIS-001" vs "SSIS001" vs "ssis_001";
//     被误判 mismatch。Parse 失败时退回到旧行为 (直接对 raw string 走 CleanID),
//     保证对不符合 Parse 约束 (例如含 `.`) 的 legacy 数据不会突然变得更严格。
func (s *Service) verifyScrapeSnapshotMatchesJob(
	logger *zap.Logger, j *jobdef.Job, data *repository.ScrapeData,
) error {
	if j == nil || data == nil || strings.TrimSpace(data.RawData) == "" {
		return nil
	}
	var snapshot model.MovieMeta
	if err := json.Unmarshal([]byte(data.RawData), &snapshot); err != nil {
		logger.Warn(
			"scrape snapshot raw_data is not valid JSON, skip number mismatch check",
			zap.String("job_number", strings.TrimSpace(j.Number)),
			zap.Error(err),
		)
		return nil
	}
	snapshotNumber := strings.TrimSpace(snapshot.Number)
	jobNumber := strings.TrimSpace(j.Number)
	if snapshotNumber == "" || jobNumber == "" {
		return nil
	}
	snapshotBase := canonicalBaseForCompare(snapshotNumber)
	jobBase := canonicalBaseForCompare(jobNumber)
	if !strings.EqualFold(snapshotBase, jobBase) {
		logger.Warn(
			"scrape snapshot number mismatches job number, refusing import",
			zap.String("snapshot_number", snapshotNumber),
			zap.String("job_number", jobNumber),
			zap.String("snapshot_base", snapshotBase),
			zap.String("job_base", jobBase),
		)
		return fmt.Errorf(
			"snapshot=%s job=%s: %w",
			snapshotNumber, jobNumber, ErrScrapeDataNumberMismatch,
		)
	}
	return nil
}

// canonicalBaseForCompare 把一个 number 字面量转成"仅基础影片 ID + 大小写/分隔符
// 不敏感"的比对 key: 先走 number.Parse 拿 base ID (剥掉 CD/4K/VR 等 variant),
// 再经 number.GetCleanID 去掉 `-` / `_`。Parse 失败 (例如字符串带 `.` 这种
// 它主动拒绝的字符) 时回退到原值走 CleanID, 保持和老逻辑一致的兜底行为。
func canonicalBaseForCompare(raw string) string {
	if n, err := number.Parse(raw); err == nil {
		return number.GetCleanID(n.GetNumberID())
	}
	return number.GetCleanID(raw)
}

func (s *Service) performImport(
	ctx context.Context, logger *zap.Logger,
	j *jobdef.Job, jobID int64,
	meta *model.MovieMeta, payload string,
) error {
	sourcePath, err := s.coordinator.ResolveJobSourcePath(ctx, j)
	if err != nil {
		return fmt.Errorf("resolve job source path: %w", err)
	}
	fc, err := s.capture.ResolveFileContext(sourcePath, j.Number)
	if err != nil {
		return fmt.Errorf("resolve file context failed: %w", err)
	}
	fc.Meta = meta
	s.coordinator.AddJobLog(ctx, jobID, "info", "import", "import started", "")
	logger.Info("import started", zap.String("number", j.Number))
	if err := s.capture.ImportMeta(ctx, fc); err != nil {
		s.coordinator.AddJobLog(ctx, jobID, "error", "import", "import failed", err.Error())
		logger.Error("import failed", zap.Error(err))
		return fmt.Errorf("import meta: %w", err)
	}
	if err := s.scrapeRepo.SaveFinalData(ctx, jobID, payload); err != nil {
		return fmt.Errorf("save final data: %w", err)
	}
	if err := s.jobRepo.MarkDone(ctx, jobID); err != nil {
		return fmt.Errorf("mark job done: %w", err)
	}
	s.coordinator.AddJobLog(ctx, jobID, "info", "import", "import completed", fc.SaveDir)
	logger.Info("import completed", zap.String("save_dir", fc.SaveDir))
	return nil
}
