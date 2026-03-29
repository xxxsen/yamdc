package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineDebugRoutes(group *gin.RouterGroup) {
	group.POST("/api/debug/number-cleaner/explain", gin.WrapF(a.handleNumberCleanerExplain))
	group.GET("/api/debug/searcher/plugins", gin.WrapF(a.handleSearcherDebugPlugins))
	group.POST("/api/debug/searcher/search", gin.WrapF(a.handleSearcherDebugSearch))
	group.GET("/api/debug/handlers", gin.WrapF(a.handleHandlerDebugHandlers))
	group.POST("/api/debug/handler/run", gin.WrapF(a.handleHandlerDebugRun))
}
