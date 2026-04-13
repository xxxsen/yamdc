package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/logutil"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/searcher"
	plugyaml "github.com/xxxsen/yamdc/internal/searcher/plugin/yaml"
	"go.uber.org/zap"
)

type pluginEditorRequest struct {
	Draft  *plugyaml.PluginSpec `json:"draft"`
	Number string               `json:"number"`
	Case   *plugyaml.CaseSpec   `json:"case"`
	YAML   string               `json:"yaml"`
}

type pluginEditorResponse struct {
	OK       bool        `json:"ok"`
	Warnings []string    `json:"warnings"`
	Data     interface{} `json:"data"`
}

func (a *API) handleMovieIDCleanerExplain(c *gin.Context) {
	if a.cleaner == nil {
		writeFail(c.Writer, errCodeMovieIDCleanerUnavailable, "movieid cleaner is not available")
		return
	}
	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		writeFail(c.Writer, errCodeInputRequired, "input is required")
		return
	}
	result, err := a.cleaner.Explain(req.Input)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("movieid cleaner explain failed", zap.String("input", req.Input), zap.Error(err))
		writeFail(c.Writer, errCodeMovieIDCleanerExplainFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("movieid cleaner explain completed",
		zap.String("input", req.Input),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.Final.NumberID),
		zap.String("status", string(result.Final.Status)),
	)
	writeSuccess(c.Writer, http.StatusOK, "ok", result)
}

func (a *API) handleSearcherDebugPlugins(c *gin.Context) {
	if a.debugger == nil {
		writeFail(c.Writer, errCodeSearcherDebuggerUnavailable, "searcher debugger is not available")
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", a.debugger.Plugins())
}

func (a *API) handleSearcherDebugSearch(c *gin.Context) {
	if a.debugger == nil {
		writeFail(c.Writer, errCodeSearcherDebuggerUnavailable, "searcher debugger is not available")
		return
	}
	var req searcher.DebugSearchOptions
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	result, err := a.debugger.DebugSearch(c.Request.Context(), req)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("searcher debug search failed",
			zap.String("input", strings.TrimSpace(req.Input)),
			zap.Strings("plugins", req.Plugins),
			zap.Bool("use_cleaner", req.UseCleaner),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeSearcherDebugSearchFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("searcher debug search completed",
		zap.String("input", result.Input),
		zap.String("number_id", result.NumberID),
		zap.Bool("found", result.Found),
		zap.String("matched_plugin", result.MatchedPlugin),
		zap.Strings("used_plugins", result.UsedPlugins),
	)
	writeSuccess(c.Writer, http.StatusOK, "ok", result)
}

func (a *API) handlePluginEditorCompile(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor compile decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(c.Writer, errCodeInputRequired, "draft is required")
		return
	}
	result, err := a.editor.Compile(c.Request.Context(), req.Draft)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor compile failed", zap.Error(err))
		writeFail(c.Writer, errCodePluginEditorCompileFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorImport(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor import decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	req.YAML = strings.TrimSpace(req.YAML)
	if req.YAML == "" {
		writeFail(c.Writer, errCodeInputRequired, "yaml is required")
		return
	}
	result, err := a.editor.ImportYAML(c.Request.Context(), req.YAML)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor import failed", zap.Error(err))
		writeFail(c.Writer, errCodePluginEditorImportFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data: map[string]interface{}{
			"draft": result,
		},
	})
}

func (a *API) handlePluginEditorRequest(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor request decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(c.Writer, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(c.Writer, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.RequestDebug(c.Request.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor request debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodePluginEditorRequestFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorScrape(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor scrape decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(c.Writer, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(c.Writer, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.ScrapeDebug(c.Request.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor scrape debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodePluginEditorScrapeFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorWorkflow(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor workflow decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(c.Writer, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(c.Writer, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.WorkflowDebug(c.Request.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor workflow debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodePluginEditorWorkflowFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorCase(c *gin.Context) {
	if a.editor == nil {
		writeFail(c.Writer, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor case decode failed", zap.Error(err))
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(c.Writer, errCodeInputRequired, "draft is required")
		return
	}
	if req.Case == nil {
		writeFail(c.Writer, errCodeInputRequired, "case is required")
		return
	}
	result, err := a.editor.CaseDebug(c.Request.Context(), req.Draft, *req.Case)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("plugin editor case debug failed",
			zap.String("case_name", strings.TrimSpace(req.Case.Name)),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodePluginEditorCaseFailed, err.Error())
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data: map[string]interface{}{
			"result": result,
		},
	})
}

func (a *API) handleHandlerDebugHandlers(c *gin.Context) {
	if a.handlers == nil {
		writeFail(c.Writer, errCodeHandlerDebuggerUnavailable, "handler debugger is not available")
		return
	}
	writeSuccess(c.Writer, http.StatusOK, "ok", a.handlers.Handlers())
}

func (a *API) handleHandlerDebugRun(c *gin.Context) {
	if a.handlers == nil {
		writeFail(c.Writer, errCodeHandlerDebuggerUnavailable, "handler debugger is not available")
		return
	}
	var req phandler.DebugRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeFail(c.Writer, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	result, err := a.handlers.Debug(c.Request.Context(), req)
	if err != nil {
		logutil.GetLogger(c.Request.Context()).Warn("handler debug run failed",
			zap.String("mode", strings.TrimSpace(req.Mode)),
			zap.String("handler_id", strings.TrimSpace(req.HandlerID)),
			zap.Strings("handler_ids", req.HandlerIDs),
			zap.Error(err),
		)
		writeFail(c.Writer, errCodeHandlerDebugRunFailed, err.Error())
		return
	}
	logutil.GetLogger(c.Request.Context()).Info("handler debug run completed",
		zap.String("mode", result.Mode),
		zap.String("handler_id", result.HandlerID),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.NumberID),
		zap.String("result_error", result.Error),
	)
	writeSuccess(c.Writer, http.StatusOK, "ok", result)
}
