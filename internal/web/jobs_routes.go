package web

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

func (a *API) registerEngineJobRoutes(group *gin.RouterGroup) {
	group.POST("/api/scan", a.handleScan)
	group.GET("/api/jobs", a.handleListJobs)

	group.POST("/api/jobs/:id/run", a.handleJobRun)
	group.POST("/api/jobs/:id/rerun", a.handleJobRerun)
	group.GET("/api/jobs/:id/logs", a.handleJobLogs)
	group.PATCH("/api/jobs/:id/number", a.handleJobUpdateNumber)
	group.DELETE("/api/jobs/:id", a.handleJobDelete)

	group.GET("/api/review/jobs/:id", a.handleReviewGet)
	group.PUT("/api/review/jobs/:id", a.handleReviewSave)
	group.POST("/api/review/jobs/:id/import", a.handleReviewImport)
	group.POST("/api/review/jobs/:id/poster-crop", a.handleReviewPosterCrop)
	group.POST("/api/review/jobs/:id/asset", a.handleReviewAsset)
}

func (a *API) handleScan(c *gin.Context) {
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

func (a *API) handleJobRun(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := a.jobSvc.Run(c.Request.Context(), id); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("job run failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, errCodeJobRunFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("job run requested", zap.Int64("job_id", id))
	writeSuccess(c.Writer, "job started", nil)
}

func (a *API) handleJobRerun(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := a.jobSvc.Rerun(c.Request.Context(), id); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("job rerun failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, errCodeJobRerunFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("job rerun requested", zap.Int64("job_id", id))
	writeSuccess(c.Writer, "job restarted", nil)
}

func (a *API) handleJobLogs(c *gin.Context) {
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

func (a *API) handleJobUpdateNumber(c *gin.Context) {
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
		Number string `json:"number"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	item, err := a.jobSvc.UpdateNumber(c.Request.Context(), id, req.Number)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("job number update failed",
			zap.Int64("job_id", id),
			zap.String("number", strings.TrimSpace(req.Number)),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeJobUpdateNumberFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("job number updated",
		zap.Int64("job_id", id),
		zap.String("number", strings.TrimSpace(req.Number)),
	)
	writeSuccess(c.Writer, "job number updated", item)
}

func (a *API) handleJobDelete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := a.jobSvc.Delete(c.Request.Context(), id); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("job delete failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, errCodeJobDeleteFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("job deleted", zap.Int64("job_id", id))
	writeSuccess(c.Writer, "job deleted", nil)
}

func (a *API) handleReviewGet(c *gin.Context) {
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
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := a.reviewSvc.Import(c.Request.Context(), id); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("review import failed", zap.Int64("job_id", id), zap.Error(err))
		writeFail(c.Writer, errCodeReviewImportFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("review import completed", zap.Int64("job_id", id))
	writeSuccess(c.Writer, "import completed", nil)
}

func (a *API) handleReviewPosterCrop(c *gin.Context) {
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
		logutil.GetLogger(c.Request.Context()).Warn("review poster crop failed",
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
	logutil.GetLogger(c.Request.Context()).Info("review poster cropped",
		zap.Int64("job_id", id),
		zap.Int("x", req.X),
		zap.Int("y", req.Y),
		zap.Int("width", req.Width),
		zap.Int("height", req.Height),
		zap.String("poster_key", poster.Key),
	)
	writeSuccess(c.Writer, "poster cropped", poster)
}

func readUploadImageData(c *gin.Context) ([]byte, string, bool) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeFail(c.Writer, errCodeInvalidUploadFile, "invalid upload file")
		return nil, "", false
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		writeFail(c.Writer, errCodeReadUploadFileFailed, "read upload file failed")
		return nil, "", false
	}
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		writeFail(c.Writer, errCodeUploadFileNotImage, "upload file is not an image")
		return nil, "", false
	}
	return data, header.Filename, true
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

func (a *API) handleReviewAsset(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	target := strings.TrimSpace(c.Query("target"))
	if target != "cover" && target != "poster" && target != "fanart" {
		writeFail(c.Writer, errCodeInvalidAssetTarget, "invalid asset target")
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
	key, err := store.AnonymousPutDataTo(c.Request.Context(), a.store, data)
	if err != nil {
		writeFail(c.Writer, errCodeReviewAssetStoreFailed, err.Error())
		return
	}
	asset := &model.File{Name: filepath.Base(fileName), Key: key}
	switch target {
	case "cover":
		meta.Cover = asset
	case "poster":
		meta.Poster = asset
	case "fanart":
		meta.SampleImages = append(meta.SampleImages, asset)
	}
	reviewData, err := json.Marshal(meta)
	if err != nil {
		writeFail(c.Writer, errCodeReviewMarshalJSONFailed, "marshal review json failed")
		return
	}
	if err := a.reviewSvc.SaveReviewData(c.Request.Context(), id, string(reviewData)); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("review asset upload save failed",
			zap.Int64("job_id", id), zap.String("target", target),
			zap.String("file_name", fileName), zap.String("asset_key", key), zap.Error(err))
		writeFail(c.Writer, errCodeReviewSaveFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("review asset uploaded",
		zap.Int64("job_id", id), zap.String("target", target),
		zap.String("file_name", fileName), zap.String("asset_key", key))
	writeSuccess(c.Writer, "review asset uploaded", asset)
}
