package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
	"go.uber.org/zap"
)

type API struct {
	jobRepo  *repository.JobRepository
	scanner  *scanner.Service
	jobSvc   *job.Service
	saveDir  string
	media    *medialib.Service
	store    store.IStorage
	cleaner  numbercleaner.Cleaner
	debugger *searcher.Debugger
	handlers *phandler.Debugger
}

func NewAPI(jobRepo *repository.JobRepository, scanner *scanner.Service, jobSvc *job.Service, saveDir string, media *medialib.Service, storage store.IStorage, cleaner numbercleaner.Cleaner, debugger *searcher.Debugger, handlers *phandler.Debugger) *API {
	return &API{jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, saveDir: saveDir, media: media, store: storage, cleaner: cleaner, debugger: debugger, handlers: handlers}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", a.handleHealthz)
	mux.HandleFunc("/api/scan", a.handleScan)
	mux.HandleFunc("/api/jobs", a.handleListJobs)
	mux.HandleFunc("/api/jobs/", a.handleJobRoutes)
	mux.HandleFunc("/api/review/jobs/", a.handleReviewRoutes)
	mux.HandleFunc("/api/library", a.handleListLibrary)
	mux.HandleFunc("/api/library/item", a.handleLibraryItem)
	mux.HandleFunc("/api/library/file", a.handleLibraryFile)
	mux.HandleFunc("/api/library/asset", a.handleLibraryAsset)
	mux.HandleFunc("/api/library/poster-crop", a.handleLibraryPosterCrop)
	mux.HandleFunc("/api/media-library", a.handleMediaLibraryList)
	mux.HandleFunc("/api/media-library/item", a.handleMediaLibraryItem)
	mux.HandleFunc("/api/media-library/file", a.handleMediaLibraryFile)
	mux.HandleFunc("/api/media-library/asset", a.handleMediaLibraryAsset)
	mux.HandleFunc("/api/media-library/sync", a.handleMediaLibrarySync)
	mux.HandleFunc("/api/media-library/move", a.handleMediaLibraryMove)
	mux.HandleFunc("/api/media-library/status", a.handleMediaLibraryStatus)
	mux.HandleFunc("/api/debug/number-cleaner/explain", a.handleNumberCleanerExplain)
	mux.HandleFunc("/api/debug/searcher/plugins", a.handleSearcherDebugPlugins)
	mux.HandleFunc("/api/debug/searcher/search", a.handleSearcherDebugSearch)
	mux.HandleFunc("/api/debug/handlers", a.handleHandlerDebugHandlers)
	mux.HandleFunc("/api/debug/handler/run", a.handleHandlerDebugRun)
	mux.HandleFunc("/api/assets/", a.handleAsset)
	return withCORS(mux)
}

func (a *API) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "ok",
		"data": map[string]string{
			"status": "ok",
		},
	})
}

func (a *API) handleNumberCleanerExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.cleaner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"code":    1,
			"message": "number cleaner is not available",
		})
		return
	}
	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"code":    1,
			"message": "invalid json body",
		})
		return
	}
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"code":    1,
			"message": "input is required",
		})
		return
	}
	result, err := a.cleaner.Explain(req.Input)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("number cleaner explain failed", zap.String("input", req.Input), zap.Error(err))
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	logutil.GetLogger(r.Context()).Info("number cleaner explain completed",
		zap.String("input", req.Input),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.Final.NumberID),
		zap.String("status", string(result.Final.Status)),
	)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}

func (a *API) handleSearcherDebugPlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.debugger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"code":    1,
			"message": "searcher debugger is not available",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "ok",
		"data":    a.debugger.Plugins(),
	})
}

func (a *API) handleSearcherDebugSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.debugger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"code":    1,
			"message": "searcher debugger is not available",
		})
		return
	}
	var req searcher.DebugSearchOptions
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"code":    1,
			"message": "invalid json body",
		})
		return
	}
	result, err := a.debugger.DebugSearch(r.Context(), req)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("searcher debug search failed",
			zap.String("input", strings.TrimSpace(req.Input)),
			zap.Strings("plugins", req.Plugins),
			zap.Bool("use_cleaner", req.UseCleaner),
			zap.Error(err),
		)
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	logutil.GetLogger(r.Context()).Info("searcher debug search completed",
		zap.String("input", result.Input),
		zap.String("number_id", result.NumberID),
		zap.Bool("found", result.Found),
		zap.String("matched_plugin", result.MatchedPlugin),
		zap.Strings("used_plugins", result.UsedPlugins),
	)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}

func (a *API) handleHandlerDebugHandlers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.handlers == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"code": 1, "message": "handler debugger is not available"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": a.handlers.Handlers()})
}

func (a *API) handleHandlerDebugRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.handlers == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"code": 1, "message": "handler debugger is not available"})
		return
	}
	var req phandler.DebugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid json body"})
		return
	}
	result, err := a.handlers.Debug(r.Context(), req)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("handler debug run failed",
			zap.String("mode", strings.TrimSpace(req.Mode)),
			zap.String("handler_id", strings.TrimSpace(req.HandlerID)),
			zap.Strings("handler_ids", req.HandlerIDs),
			zap.Error(err),
		)
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	logutil.GetLogger(r.Context()).Info("handler debug run completed",
		zap.String("mode", result.Mode),
		zap.String("handler_id", result.HandlerID),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.NumberID),
		zap.String("result_error", result.Error),
	)
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": result})
}

func (a *API) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	logutil.GetLogger(r.Context()).Info("manual scan requested")
	if err := a.scanner.Scan(r.Context()); err != nil {
		logutil.GetLogger(r.Context()).Error("manual scan failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	logutil.GetLogger(r.Context()).Info("manual scan completed")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "scan triggered",
	})
}

func (a *API) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	statuses := parseStatuses(r.URL.Query().Get("status"))
	page := 1
	pageSize := 50
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	all := r.URL.Query().Get("all") == "true"
	if raw := r.URL.Query().Get("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := r.URL.Query().Get("page_size"); raw != "" && !all {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if all {
		pageSize = 0
	}
	items, err := a.jobRepo.ListJobs(r.Context(), statuses, keyword, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	if err := a.jobSvc.ApplyJobConflicts(r.Context(), items.Items); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "ok",
		"data":    items,
	})
}

func (a *API) handleJobRoutes(w http.ResponseWriter, r *http.Request) {
	id, action, err := parseJobRoute(r.URL.Path, "/api/jobs/")
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	switch action {
	case "run":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if err := a.jobSvc.Run(r.Context(), id); err != nil {
			logutil.GetLogger(r.Context()).Warn("job run failed", zap.Int64("job_id", id), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("job run requested", zap.Int64("job_id", id))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job started"})
	case "rerun":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if err := a.jobSvc.Rerun(r.Context(), id); err != nil {
			logutil.GetLogger(r.Context()).Warn("job rerun failed", zap.Int64("job_id", id), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("job rerun requested", zap.Int64("job_id", id))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job restarted"})
	case "logs":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		items, err := a.jobSvc.ListLogs(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": items})
	case "number":
		if r.Method != http.MethodPatch {
			writeMethodNotAllowed(w)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read body failed"})
			return
		}
		var req struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid json body"})
			return
		}
		item, err := a.jobSvc.UpdateNumber(r.Context(), id, req.Number)
		if err != nil {
			logutil.GetLogger(r.Context()).Warn("job number update failed", zap.Int64("job_id", id), zap.String("number", strings.TrimSpace(req.Number)), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("job number updated", zap.Int64("job_id", id), zap.String("number", strings.TrimSpace(req.Number)))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job number updated", "data": item})
	case "":
		if r.Method != http.MethodDelete {
			writeMethodNotAllowed(w)
			return
		}
		if err := a.jobSvc.Delete(r.Context(), id); err != nil {
			logutil.GetLogger(r.Context()).Warn("job delete failed", zap.Int64("job_id", id), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("job deleted", zap.Int64("job_id", id))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job deleted"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": "route not found"})
	}
}

func (a *API) handleReviewRoutes(w http.ResponseWriter, r *http.Request) {
	id, action, err := parseJobRoute(r.URL.Path, "/api/review/jobs/")
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": err.Error()})
		return
	}
	if action != "" {
		switch action {
		case "import":
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			if err := a.jobSvc.Import(r.Context(), id); err != nil {
				logutil.GetLogger(r.Context()).Warn("review import failed", zap.Int64("job_id", id), zap.Error(err))
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
			logutil.GetLogger(r.Context()).Info("review import completed", zap.Int64("job_id", id))
			writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "import completed"})
			return
		case "poster-crop":
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read body failed"})
				return
			}
			var req struct {
				X      int `json:"x"`
				Y      int `json:"y"`
				Width  int `json:"width"`
				Height int `json:"height"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid json body"})
				return
			}
			if req.Width <= 0 || req.Height <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid crop rectangle"})
				return
			}
			poster, err := a.jobSvc.CropPosterFromCover(r.Context(), id, req.X, req.Y, req.Width, req.Height)
			if err != nil {
				logutil.GetLogger(r.Context()).Warn("review poster crop failed",
					zap.Int64("job_id", id),
					zap.Int("x", req.X),
					zap.Int("y", req.Y),
					zap.Int("width", req.Width),
					zap.Int("height", req.Height),
					zap.Error(err),
				)
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
			logutil.GetLogger(r.Context()).Info("review poster cropped",
				zap.Int64("job_id", id),
				zap.Int("x", req.X),
				zap.Int("y", req.Y),
				zap.Int("width", req.Width),
				zap.Int("height", req.Height),
				zap.String("poster_key", poster.Key),
			)
			writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "poster cropped", "data": poster})
			return
		case "asset":
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			target := strings.TrimSpace(r.URL.Query().Get("target"))
			if target != "cover" && target != "poster" && target != "fanart" {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid asset target"})
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid upload file"})
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read upload file failed"})
				return
			}
			if !strings.HasPrefix(http.DetectContentType(data), "image/") {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "upload file is not an image"})
				return
			}
			scrapeData, err := a.jobSvc.GetScrapeData(r.Context(), id)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
			if scrapeData == nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "scrape data not found"})
				return
			}
			payload := scrapeData.ReviewData
			if strings.TrimSpace(payload) == "" {
				payload = scrapeData.RawData
			}
			var meta model.MovieMeta
			if err := json.Unmarshal([]byte(payload), &meta); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid review json"})
				return
			}
			key, err := store.AnonymousPutDataTo(r.Context(), a.store, data)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
			asset := &model.File{
				Name: filepath.Base(header.Filename),
				Key:  key,
			}
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
				writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": "marshal review json failed"})
				return
			}
			if err := a.jobSvc.SaveReviewData(r.Context(), id, string(reviewData)); err != nil {
				logutil.GetLogger(r.Context()).Warn("review asset upload save failed",
					zap.Int64("job_id", id),
					zap.String("target", target),
					zap.String("file_name", header.Filename),
					zap.String("asset_key", key),
					zap.Error(err),
				)
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
			logutil.GetLogger(r.Context()).Info("review asset uploaded",
				zap.Int64("job_id", id),
				zap.String("target", target),
				zap.String("file_name", header.Filename),
				zap.String("asset_key", key),
			)
			writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "review asset uploaded", "data": asset})
			return
		default:
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": "route not found"})
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		item, err := a.jobSvc.GetScrapeData(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "ok", "data": item})
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read body failed"})
			return
		}
		var req struct {
			ReviewData string `json:"review_data"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid json body"})
			return
		}
		if err := a.jobSvc.SaveReviewData(r.Context(), id, req.ReviewData); err != nil {
			logutil.GetLogger(r.Context()).Warn("review data save failed", zap.Int64("job_id", id), zap.Error(err))
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("review data saved", zap.Int64("job_id", id))
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "review data saved"})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *API) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid upload file"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "read upload file failed"})
			return
		}
		if !strings.HasPrefix(http.DetectContentType(data), "image/") {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "upload file is not an image"})
			return
		}
		key, err := store.AnonymousPutDataTo(r.Context(), a.store, data)
		if err != nil {
			logutil.GetLogger(r.Context()).Error("debug asset upload failed", zap.String("file_name", header.Filename), zap.Error(err))
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		logutil.GetLogger(r.Context()).Info("debug asset uploaded", zap.String("file_name", header.Filename), zap.String("asset_key", key))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code":    0,
			"message": "asset uploaded",
			"data": map[string]string{
				"name": filepath.Base(header.Filename),
				"key":  key,
			},
		})
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/api/assets/")
	key = strings.TrimSpace(key)
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": "invalid asset key"})
		return
	}
	data, err := store.GetDataFrom(r.Context(), a.store, key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"code": 1, "message": "asset not found"})
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
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

func parseJobRoute(path string, prefix string) (int64, string, error) {
	raw := strings.TrimPrefix(path, prefix)
	if raw == path || raw == "" {
		return 0, "", fmt.Errorf("invalid route")
	}
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, "", fmt.Errorf("invalid route")
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid job id")
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return id, action, nil
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
		"code":    1,
		"message": "method not allowed",
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
