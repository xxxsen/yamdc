package web

import (
	"encoding/json"
	"net/http"
	"strings"

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

func (a *API) handleNumberCleanerExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.cleaner == nil {
		writeFail(w, errCodeNumberCleanerUnavailable, "number cleaner is not available")
		return
	}
	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		writeFail(w, errCodeInputRequired, "input is required")
		return
	}
	result, err := a.cleaner.Explain(req.Input)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("number cleaner explain failed", zap.String("input", req.Input), zap.Error(err))
		writeFail(w, errCodeNumberCleanerExplainFailed, err.Error())
		return
	}
	logutil.GetLogger(r.Context()).Info("number cleaner explain completed",
		zap.String("input", req.Input),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.Final.NumberID),
		zap.String("status", string(result.Final.Status)),
	)
	writeSuccess(w, http.StatusOK, "ok", result)
}

func (a *API) handleSearcherDebugPlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.debugger == nil {
		writeFail(w, errCodeSearcherDebuggerUnavailable, "searcher debugger is not available")
		return
	}
	writeSuccess(w, http.StatusOK, "ok", a.debugger.Plugins())
}

func (a *API) handleSearcherDebugSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.debugger == nil {
		writeFail(w, errCodeSearcherDebuggerUnavailable, "searcher debugger is not available")
		return
	}
	var req searcher.DebugSearchOptions
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
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
		writeFail(w, errCodeSearcherDebugSearchFailed, err.Error())
		return
	}
	logutil.GetLogger(r.Context()).Info("searcher debug search completed",
		zap.String("input", result.Input),
		zap.String("number_id", result.NumberID),
		zap.Bool("found", result.Found),
		zap.String("matched_plugin", result.MatchedPlugin),
		zap.Strings("used_plugins", result.UsedPlugins),
	)
	writeSuccess(w, http.StatusOK, "ok", result)
}

func (a *API) handlePluginEditorCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(w, errCodeInputRequired, "draft is required")
		return
	}
	result, err := a.editor.Compile(r.Context(), req.Draft)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor compile failed", zap.Error(err))
		writeFail(w, errCodePluginEditorCompileFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	req.YAML = strings.TrimSpace(req.YAML)
	if req.YAML == "" {
		writeFail(w, errCodeInputRequired, "yaml is required")
		return
	}
	result, err := a.editor.ImportYAML(r.Context(), req.YAML)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor import failed", zap.Error(err))
		writeFail(w, errCodePluginEditorImportFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data: map[string]interface{}{
			"draft": result,
		},
	})
}

func (a *API) handlePluginEditorRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(w, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(w, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.RequestDebug(r.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor request debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(w, errCodePluginEditorRequestFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorScrape(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(w, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(w, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.ScrapeDebug(r.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor scrape debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(w, errCodePluginEditorScrapeFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(w, errCodeInputRequired, "draft is required")
		return
	}
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		writeFail(w, errCodeInputRequired, "number is required")
		return
	}
	result, err := a.editor.WorkflowDebug(r.Context(), req.Draft, req.Number)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor workflow debug failed",
			zap.String("number", req.Number),
			zap.Error(err),
		)
		writeFail(w, errCodePluginEditorWorkflowFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data:     result,
	})
}

func (a *API) handlePluginEditorCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.editor == nil {
		writeFail(w, errCodePluginEditorUnavailable, "plugin editor is not available")
		return
	}
	var req pluginEditorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
		return
	}
	if req.Draft == nil {
		writeFail(w, errCodeInputRequired, "draft is required")
		return
	}
	if req.Case == nil {
		writeFail(w, errCodeInputRequired, "case is required")
		return
	}
	result, err := a.editor.CaseDebug(r.Context(), req.Draft, *req.Case)
	if err != nil {
		logutil.GetLogger(r.Context()).Warn("plugin editor case debug failed",
			zap.String("case_name", strings.TrimSpace(req.Case.Name)),
			zap.Error(err),
		)
		writeFail(w, errCodePluginEditorCaseFailed, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, "ok", pluginEditorResponse{
		OK:       true,
		Warnings: []string{},
		Data: map[string]interface{}{
			"result": result,
		},
	})
}

func (a *API) handleHandlerDebugHandlers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.handlers == nil {
		writeFail(w, errCodeHandlerDebuggerUnavailable, "handler debugger is not available")
		return
	}
	writeSuccess(w, http.StatusOK, "ok", a.handlers.Handlers())
}

func (a *API) handleHandlerDebugRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.handlers == nil {
		writeFail(w, errCodeHandlerDebuggerUnavailable, "handler debugger is not available")
		return
	}
	var req phandler.DebugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFail(w, errCodeInvalidJSONBody, "invalid json body")
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
		writeFail(w, errCodeHandlerDebugRunFailed, err.Error())
		return
	}
	logutil.GetLogger(r.Context()).Info("handler debug run completed",
		zap.String("mode", result.Mode),
		zap.String("handler_id", result.HandlerID),
		zap.Int("steps", len(result.Steps)),
		zap.String("number_id", result.NumberID),
		zap.String("result_error", result.Error),
	)
	writeSuccess(w, http.StatusOK, "ok", result)
}
