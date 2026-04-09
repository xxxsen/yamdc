package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineDebugRoutes(group *gin.RouterGroup) {
	group.POST("/api/debug/number-cleaner/explain", gin.WrapF(a.handleNumberCleanerExplain))
	group.GET("/api/debug/searcher/plugins", gin.WrapF(a.handleSearcherDebugPlugins))
	group.POST("/api/debug/searcher/search", gin.WrapF(a.handleSearcherDebugSearch))
	group.POST("/api/debug/plugin-editor/compile", gin.WrapF(a.handlePluginEditorCompile))
	group.POST("/api/debug/plugin-editor/import", gin.WrapF(a.handlePluginEditorImport))
	group.POST("/api/debug/plugin-editor/request", gin.WrapF(a.handlePluginEditorRequest))
	group.POST("/api/debug/plugin-editor/workflow", gin.WrapF(a.handlePluginEditorWorkflow))
	group.POST("/api/debug/plugin-editor/scrape", gin.WrapF(a.handlePluginEditorScrape))
	group.POST("/api/debug/plugin-editor/case", gin.WrapF(a.handlePluginEditorCase))
	group.GET("/api/debug/handlers", gin.WrapF(a.handleHandlerDebugHandlers))
	group.POST("/api/debug/handler/run", gin.WrapF(a.handleHandlerDebugRun))
}
