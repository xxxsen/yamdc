package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineDebugRoutes(group *gin.RouterGroup) {
	group.POST("/api/debug/number-cleaner/explain", a.handleNumberCleanerExplain)
	group.GET("/api/debug/searcher/plugins", a.handleSearcherDebugPlugins)
	group.POST("/api/debug/searcher/search", a.handleSearcherDebugSearch)
	group.POST("/api/debug/plugin-editor/compile", a.handlePluginEditorCompile)
	group.POST("/api/debug/plugin-editor/import", a.handlePluginEditorImport)
	group.POST("/api/debug/plugin-editor/request", a.handlePluginEditorRequest)
	group.POST("/api/debug/plugin-editor/workflow", a.handlePluginEditorWorkflow)
	group.POST("/api/debug/plugin-editor/scrape", a.handlePluginEditorScrape)
	group.POST("/api/debug/plugin-editor/case", a.handlePluginEditorCase)
	group.GET("/api/debug/handlers", a.handleHandlerDebugHandlers)
	group.POST("/api/debug/handler/run", a.handleHandlerDebugRun)
}
