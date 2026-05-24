package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"

	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"
	"github.com/xxxsen/yamdc/internal/store"
)

// 声明式路由注册: 与 registerEngineMediaLibraryRoutes 形状类似但语义不同
// (一个负责 job / review 生命周期, 一个负责媒体库), 抽成 "data + loop"
// 反而削弱可读性, 因此保留直写风格.
func (a *API) registerEngineJobRoutes(group *gin.RouterGroup) {
	group.POST("/api/scan", a.handleScan)
	group.GET("/api/jobs", a.handleListJobs)

	group.POST("/api/jobs/:id/run", a.handleJobRun)
	group.POST("/api/jobs/:id/rerun", a.handleJobRerun)
	group.GET("/api/jobs/:id/logs", a.handleJobLogs)
	group.PATCH("/api/jobs/:id/number", a.handleJobUpdateNumber)
	group.DELETE("/api/jobs/:id", a.handleJobDelete)

	group.GET("/api/number/variants", a.handleListNumberVariants)

	group.GET("/api/review/jobs/:id", a.handleReviewGet)
	group.PUT("/api/review/jobs/:id", a.handleReviewSave)
	group.POST("/api/review/jobs/:id/import", a.handleReviewImport)
	group.POST("/api/review/jobs/:id/reject", a.handleReviewReject)
	group.POST("/api/review/jobs/:id/poster-crop", a.handleReviewPosterCrop)
	group.POST("/api/review/jobs/:id/asset", a.handleReviewAsset)
}

func (a *API) handleScan(c *gin.Context) {
	if !requireDependency(c, a.scanner, "scanner") {
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("manual scan requested")
	if err := a.scanner.Scan(c.Request.Context()); err != nil {
		logutil.GetLogger(c.Request.Context()).Error("manual scan failed", zap.Error(err))
		writeFail(c.Writer, errCodeScanFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("manual scan completed")
	writeSuccess(c.Writer, "scan triggered", nil)
}

func (a *API) handleListJobs(c *gin.Context) {
	if !requireDependency(c, a.jobRepo, "jobRepo") {
		return
	}
	if !requireDependency(c, a.jobSvc, "jobSvc") {
		return
	}
	statuses := parseStatuses(c.Query("status"))
	page := 1
	pageSize := 50
	keyword := strings.TrimSpace(c.Query("keyword"))
	all := c.Query("all") == "true"
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" && !all {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if all {
		pageSize = 0
	}
	items, err := a.jobRepo.ListJobs(c.Request.Context(), statuses, keyword, page, pageSize)
	if err != nil {
		writeFail(c.Writer, errCodeListJobsFailed, err.Error())
		return
	}
	if err := a.jobSvc.ApplyConflicts(c.Request.Context(), items.Items); err != nil {
		writeFail(c.Writer, errCodeApplyJobConflictsFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", items)
}

func parseStatuses(raw string) []jobdef.Status {
	if raw == "" {
		return []jobdef.Status{jobdef.StatusInit, jobdef.StatusProcessing, jobdef.StatusFailed, jobdef.StatusReviewing}
	}
	parts := strings.Split(raw, ",")
	statuses := make([]jobdef.Status, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		statuses = append(statuses, jobdef.Status(item))
	}
	return statuses
}

func parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil {
		writeFail(c.Writer, errCodeInvalidJobID, "invalid job id")
		return 0, false
	}
	return id, true
}

// jobOpPrelude 把 "依赖 503 守门 + id 解析" 这段固定形状抽出, 让具体
// handler 只关心 "拿到 id 之后怎么调 service + 写响应". 错误本身仍然
// 由 handler 自己消费 (writeJobOpResult), 避免把外部包的 err 通过 helper
// 形参回传 — 那样会被 wrapcheck 视作"未经包内封装的转发".
func (a *API) jobOpPrelude(c *gin.Context, dep any, depName string) (int64, bool) {
	if !requireDependency(c, dep, depName) {
		return 0, false
	}
	id, ok := parseIDParam(c)
	if !ok {
		return 0, false
	}
	return id, true
}

// writeJobOpResult 统一记录 + 写响应. err 已经在调用点完成消费 (调用点
// 自己拿到的, 不跨函数边界返回), 这里只读它的 message; 因此不存在
// wrapcheck 关心的"把 external err 透传上层"问题.
func writeJobOpResult(c *gin.Context, id int64, opName string, failCode int, successMsg string, err error) {
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn(opName+" failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, failCode, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info(opName+" requested", zap.Int64("job_id", id))
	writeSuccess(c.Writer, successMsg, nil)
}

func (a *API) handleJobRun(c *gin.Context) {
	id, ok := a.jobOpPrelude(c, a.jobSvc, "jobSvc")
	if !ok {
		return
	}
	err := a.jobSvc.Run(c.Request.Context(), id)
	writeJobOpResult(c, id, "job run", errCodeJobRunFailed, "job started", err)
}

func (a *API) handleJobRerun(c *gin.Context) {
	id, ok := a.jobOpPrelude(c, a.jobSvc, "jobSvc")
	if !ok {
		return
	}
	err := a.jobSvc.Rerun(c.Request.Context(), id)
	writeJobOpResult(c, id, "job rerun", errCodeJobRerunFailed, "job restarted", err)
}

func (a *API) handleJobLogs(c *gin.Context) {
	if !requireDependency(c, a.jobSvc, "jobSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	items, err := a.jobSvc.ListLogs(c.Request.Context(), id)
	if err != nil {
		writeFail(c.Writer, errCodeJobLogsFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", items)
}

// handleJobUpdateNumber 支持两种入参形态:
//
//  1. 结构化 (推荐): {"base":"PXVR-406","variants":[{"id":"multi_cd","index":2}]}
//     base 由前端 "文件列表" 页的影片 ID 输入框提供, variants 对应变体下拉 /
//     按钮选择, 后端负责拼装。
//
//  2. 兼容老路径: {"number":"PXVR-406-CD2"} — 直接传完整的 number 字符串。
//     老前端、命令行脚本、集成测试仍会走这条路径, 不能破坏。
//
// 识别规则: 只要请求里显式出现 base 或 variants 字段, 就按结构化处理;
// 否则走老的 number 字段。为保持向后兼容, 既没有 base/variants 又没有 number
// 时, 仍走老逻辑 (传一个空 number 进 service, 由 service 返回业务错误)。
func (a *API) handleJobUpdateNumber(c *gin.Context) {
	if !requireDependency(c, a.jobSvc, "jobSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeFail(c.Writer, errCodeReadBodyFailed, "read body failed")
		return
	}
	var req struct {
		Number   string                    `json:"number"`
		Base     *string                   `json:"base"`
		Variants []number.VariantSelection `json:"variants"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}

	useStructured := req.Base != nil || len(req.Variants) > 0
	ctx := c.Request.Context()
	logger := logutil.GetLogger(ctx)

	var item *jobdef.Job
	if useStructured {
		base := ""
		if req.Base != nil {
			base = *req.Base
		}
		item, err = a.jobSvc.UpdateNumberStructured(ctx, id, base, req.Variants)
		if err != nil {
			logger.Warn(
				"job number update (structured) failed",
				zap.Int64("job_id", id),
				zap.String("base", strings.TrimSpace(base)),
				zap.Int("variants", len(req.Variants)),
				zap.Error(err),
			)
			writeFail(c.Writer, errCodeJobUpdateNumberFailed, err.Error())
			return
		}
		logger.Info(
			"job number updated (structured)",
			zap.Int64("job_id", id),
			zap.String("base", strings.TrimSpace(base)),
			zap.Int("variants", len(req.Variants)),
		)
		writeSuccess(c.Writer, "job number updated", item)
		return
	}

	item, err = a.jobSvc.UpdateNumber(ctx, id, req.Number)
	if err != nil {
		logger.Warn(
			"job number update failed",
			zap.Int64("job_id", id),
			zap.String("number", strings.TrimSpace(req.Number)),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeJobUpdateNumberFailed, err.Error())
		return
	}
	logger.Info(
		"job number updated",
		zap.Int64("job_id", id),
		zap.String("number", strings.TrimSpace(req.Number)),
	)
	writeSuccess(c.Writer, "job number updated", item)
}

// handleListNumberVariants 返回当前支持的 variant 描述符列表, 用于前端
// "文件列表" 页的结构化影片 ID 输入 (base + variant selectors)。返回里包含
// 足够的展示信息 (label/description) 和 schema 信息 (kind + min/max),
// 前端不需要硬编码 suffix 语义, 也不需要硬编码哪些 variant 存在。
func (a *API) handleListNumberVariants(c *gin.Context) {
	descriptors := number.DefaultVariantDescriptors()
	writeSuccess(c.Writer, "ok", gin.H{"variants": descriptors})
}

func (a *API) handleJobDelete(c *gin.Context) {
	id, ok := a.jobOpPrelude(c, a.jobSvc, "jobSvc")
	if !ok {
		return
	}
	err := a.jobSvc.Delete(c.Request.Context(), id)
	writeJobOpResult(c, id, "job delete", errCodeJobDeleteFailed, "job deleted", err)
}

func (a *API) handleReviewGet(c *gin.Context) {
	if !requireDependency(c, a.jobSvc, "jobSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	item, err := a.jobSvc.GetScrapeData(c.Request.Context(), id)
	if err != nil {
		writeFail(c.Writer, errCodeReviewGetFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, "ok", item)
}

func (a *API) handleReviewSave(c *gin.Context) {
	if !requireDependency(c, a.reviewSvc, "reviewSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeFail(c.Writer, errCodeReadBodyFailed, "read body failed")
		return
	}
	var req struct {
		ReviewData string `json:"review_data"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if err := a.reviewSvc.SaveReviewData(c.Request.Context(), id, req.ReviewData); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("review data save failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, errCodeReviewSaveFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("review data saved", zap.Int64("job_id", id))
	writeSuccess(c.Writer, "review data saved", nil)
}

func (a *API) handleReviewImport(c *gin.Context) {
	id, ok := a.jobOpPrelude(c, a.reviewSvc, "reviewSvc")
	if !ok {
		return
	}
	err := a.reviewSvc.Import(c.Request.Context(), id)
	writeJobOpResult(c, id, "review import", errCodeReviewImportFailed, "import completed", err)
}

// handleReviewReject 处理 /api/review/jobs/:id/reject: 把 reviewing 的 job
// 退回到 failed 状态, 删除 scrape_data, 使用户可以重新编辑 number 后 run。
// reason 字段可选 (空则用默认文案); 行为/边界见 review.Service.Reject 注释。
func (a *API) handleReviewReject(c *gin.Context) {
	if !requireDependency(c, a.reviewSvc, "reviewSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	// reason 可选: 兼容 "空 body" 和 "{}": 读 body 失败 / JSON 不合法时仍
	// 走默认 reason, 避免前端因为没填理由而拿到 400。
	var req struct {
		Reason string `json:"reason"`
	}
	body, err := io.ReadAll(c.Request.Body)
	if err == nil && len(body) > 0 {
		_ = json.Unmarshal(body, &req)
	}
	if err := a.reviewSvc.Reject(c.Request.Context(), id, req.Reason); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn(
			"review reject failed",
			zap.Int64("job_id", id),
			zap.String("reason", strings.TrimSpace(req.Reason)),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeReviewRejectFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info(
		"review rejected",
		zap.Int64("job_id", id),
		zap.String("reason", strings.TrimSpace(req.Reason)),
	)
	writeSuccess(c.Writer, "review rejected", nil)
}

func (a *API) handleReviewPosterCrop(c *gin.Context) {
	if !requireDependency(c, a.reviewSvc, "reviewSvc") {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeFail(c.Writer, errCodeReadBodyFailed, "read body failed")
		return
	}
	var req struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Width <= 0 || req.Height <= 0 {
		writeFail(c.Writer, errCodeInvalidCropRectangle, "invalid crop rectangle")
		return
	}
	poster, err := a.reviewSvc.CropPosterFromCover(c.Request.Context(), id, req.X, req.Y, req.Width, req.Height)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn(
			"review poster crop failed",
			zap.Int64("job_id", id),
			zap.Int("x", req.X),
			zap.Int("y", req.Y),
			zap.Int("width", req.Width),
			zap.Int("height", req.Height),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeReviewPosterCropFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info(
		"review poster cropped",
		zap.Int64("job_id", id),
		zap.Int("x", req.X),
		zap.Int("y", req.Y),
		zap.Int("width", req.Width),
		zap.Int("height", req.Height),
		zap.String("poster_key", poster.Key),
	)
	writeSuccess(c.Writer, "poster cropped", poster)
}

// maxUploadImageBytes 是单张上传图片本身 (file) 的硬上限. 32 MiB 足够覆盖
// 现实中合理的封面 / 海报 / fanart 大小; 任何超出值都视为恶意/异常.
const maxUploadImageBytes = 32 << 20

// maxUploadMultipartOverheadBytes 是 multipart 形式带来的额外开销
// (boundary / part header / form 字段), 1 MiB 足够覆盖标准浏览器 +
// 标准 multipart 库的实现. 业务层仍然只允许文件 <= 32 MiB, 这里只是
// 把"整个 multipart request body"的硬上限放宽到 33 MiB, 避免一个刚好
// 32 MiB 的合法图片因为 multipart header 多出几百字节就被
// http.MaxBytesReader 误拒.
const maxUploadMultipartOverheadBytes = 1 << 20

// readUploadImageData 从 multipart 表单读取一张图片, 强制 32 MiB 上限,
// 同时被 review 资产上传 (handleReviewAsset) 与媒体库资产上传
// (handleMediaLibraryAsset) 复用, 确保两条路径有完全一致的尺寸 / 类型
// 校验. 任何 helper 失败都会向客户端写好错误 (HTTP 200 + body code, 或
// 413 给超大文件), 并返回 ok=false 让 caller 直接 return.
//
// 体积保护机制:
//
//  1. http.MaxBytesReader 把请求 body 包成只能读 maxUploadImageBytes +
//     maxUploadMultipartOverheadBytes 字节的 reader; 超过即在 Read 时抛
//     *http.MaxBytesError, 让 handler 立刻拒绝. 因为 multipart body 包含
//     boundary / part header, 它一定大于文件本体, 所以 request body 上限
//     必须比文件上限再宽一点, 否则一张刚好 32 MiB 的合法图片会被误拒.
//  2. 文件本身的 32 MiB 上限通过 header.Size 与 len(data) 做严格校验,
//     不依赖 request body 上限; 双层保护下任意一方失守都还能拦截.
func readUploadImageData(c *gin.Context) ([]byte, string, bool) {
	c.Request.Body = http.MaxBytesReader(
		c.Writer, c.Request.Body,
		maxUploadImageBytes+maxUploadMultipartOverheadBytes,
	)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if isUploadTooLargeErr(err) {
			writeUploadTooLarge(c.Writer)
			return nil, "", false
		}
		writeFail(c.Writer, errCodeInvalidUploadFile, "invalid upload file")
		return nil, "", false
	}
	defer func() { _ = file.Close() }()
	if header != nil && header.Size > maxUploadImageBytes {
		writeUploadTooLarge(c.Writer)
		return nil, "", false
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadImageBytes+1))
	if err != nil {
		if isUploadTooLargeErr(err) {
			writeUploadTooLarge(c.Writer)
			return nil, "", false
		}
		writeFail(c.Writer, errCodeReadUploadFileFailed, "read upload file failed")
		return nil, "", false
	}
	if int64(len(data)) > maxUploadImageBytes {
		writeUploadTooLarge(c.Writer)
		return nil, "", false
	}
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		writeFail(c.Writer, errCodeUploadFileNotImage, "upload file is not an image")
		return nil, "", false
	}
	return data, header.Filename, true
}

// isUploadTooLargeErr 判定某个错误是否来自 http.MaxBytesReader 的上限触发.
// 标准库会在不同代码路径里返回不同包装类型, 这里用 errors.As 处理同时兜底
// 字符串匹配, 防止 stdlib 在不同 minor 版本里换 wrap 方式.
func isUploadTooLargeErr(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "http: request body too large") ||
		strings.Contains(msg, "request body too large")
}

// writeUploadTooLarge 用 HTTP 413 + 项目统一的 { code, message, data } 协议
// 拒绝超大上传. 这条路径与 P0 CORS 的 403 同属"协议外保护层", 所以也用
// 4xx 而不是业务层的 200 + 非 0 code, 与 AGENTS.md 的协议显式区分.
func writeUploadTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	body := responseBody{
		Code:    errCodeUploadFileTooLarge,
		Message: "upload file exceeds 32 MiB limit",
		Data:    nil,
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (a *API) loadReviewMeta(c *gin.Context, id int64) (*model.MovieMeta, bool) {
	scrapeData, err := a.jobSvc.GetScrapeData(c.Request.Context(), id)
	if err != nil {
		writeFail(c.Writer, errCodeReviewGetFailed, err.Error())
		return nil, false
	}
	if scrapeData == nil {
		writeFail(c.Writer, errCodeReviewScrapeDataNotFound, "scrape data not found")
		return nil, false
	}
	payload := scrapeData.ReviewData
	if strings.TrimSpace(payload) == "" {
		payload = scrapeData.RawData
	}
	var meta model.MovieMeta
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		writeFail(c.Writer, errCodeInvalidReviewJSON, "invalid review json")
		return nil, false
	}
	return &meta, true
}

// handleReviewAsset 拆成几段独立 helper 以避免 gocyclo 触线 (>15):
//
//   - guardReviewAssetDeps: 三个依赖的 503 守门 + id 解析 + target 校验.
//   - storeReviewAssetData: 把 multipart 字节存进 a.store, 返回 asset 元.
//   - applyReviewAssetMeta: 根据 target 把 asset 挂到 meta.cover/poster/sample.
//   - persistReviewAsset:   把更新后的 meta 序列化并 SaveReviewData.
//
// 主函数只剩组装与日志.
func (a *API) handleReviewAsset(c *gin.Context) {
	id, target, ok := a.guardReviewAssetDeps(c)
	if !ok {
		return
	}
	data, fileName, ok := readUploadImageData(c)
	if !ok {
		return
	}
	meta, ok := a.loadReviewMeta(c, id)
	if !ok {
		return
	}
	asset, ok := a.storeReviewAssetData(c, fileName, data)
	if !ok {
		return
	}
	applyReviewAssetMeta(meta, target, asset)
	if !a.persistReviewAsset(c, id, target, fileName, asset.Key, meta) {
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("review asset uploaded",
		zap.Int64("job_id", id), zap.String("target", target),
		zap.String("file_name", fileName), zap.String("asset_key", asset.Key))
	writeSuccess(c.Writer, "review asset uploaded", asset)
}

func (a *API) guardReviewAssetDeps(c *gin.Context) (int64, string, bool) {
	if !requireDependency(c, a.reviewSvc, "reviewSvc") {
		return 0, "", false
	}
	if !requireDependency(c, a.store, "store") {
		return 0, "", false
	}
	if !requireDependency(c, a.jobSvc, "jobSvc") {
		return 0, "", false
	}
	id, ok := parseIDParam(c)
	if !ok {
		return 0, "", false
	}
	target := strings.TrimSpace(c.Query("target"))
	if target != "cover" && target != "poster" && target != "fanart" {
		writeFail(c.Writer, errCodeInvalidAssetTarget, "invalid asset target")
		return 0, "", false
	}
	return id, target, true
}

func (a *API) storeReviewAssetData(c *gin.Context, fileName string, data []byte) (*model.File, bool) {
	key, err := store.AnonymousPutDataTo(c.Request.Context(), a.store, data)
	if err != nil {
		writeFail(c.Writer, errCodeReviewAssetStoreFailed, err.Error())
		return nil, false
	}
	return &model.File{Name: filepath.Base(fileName), Key: key}, true
}

func applyReviewAssetMeta(meta *model.MovieMeta, target string, asset *model.File) {
	switch target {
	case "cover":
		meta.Cover = asset
	case "poster":
		meta.Poster = asset
	case "fanart":
		meta.SampleImages = append(meta.SampleImages, asset)
	}
}

func (a *API) persistReviewAsset(
	c *gin.Context, id int64, target, fileName, key string, meta *model.MovieMeta,
) bool {
	reviewData, err := json.Marshal(meta)
	if err != nil {
		writeFail(c.Writer, errCodeReviewMarshalJSONFailed, "marshal review json failed")
		return false
	}
	if err := a.reviewSvc.SaveReviewData(c.Request.Context(), id, string(reviewData)); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("review asset upload save failed",
			zap.Int64("job_id", id), zap.String("target", target),
			zap.String("file_name", fileName), zap.String("asset_key", key), zap.Error(err))
		writeFail(c.Writer, errCodeReviewSaveFailed, err.Error())
		return false
	}
	return true
}
