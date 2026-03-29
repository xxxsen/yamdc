package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/jobdef"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/store"
)

type API struct {
	jobRepo *repository.JobRepository
	scanner *scanner.Service
	jobSvc  *job.Service
	saveDir string
	media   *medialib.Service
}

func NewAPI(jobRepo *repository.JobRepository, scanner *scanner.Service, jobSvc *job.Service, saveDir string, media *medialib.Service) *API {
	return &API{jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, saveDir: saveDir, media: media}
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

func (a *API) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if err := a.scanner.Scan(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
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
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job started"})
	case "rerun":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if err := a.jobSvc.Rerun(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
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
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "job number updated", "data": item})
	case "":
		if r.Method != http.MethodDelete {
			writeMethodNotAllowed(w)
			return
		}
		if err := a.jobSvc.Delete(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
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
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
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
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
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
			key, err := store.AnonymousPutData(r.Context(), data)
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
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
				return
			}
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
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
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
		key, err := store.AnonymousPutData(r.Context(), data)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 1, "message": err.Error()})
			return
		}
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
	data, err := store.GetData(r.Context(), key)
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
