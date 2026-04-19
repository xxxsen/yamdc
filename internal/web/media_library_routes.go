package web

import "github.com/gin-gonic/gin"

// 与 registerEngineJobRoutes 结构同形但职责不同, 保留"路由表式"可读性.
//
//nolint:dupl // declarative route table; extracting would harm readability
func (a *API) registerEngineMediaLibraryRoutes(group *gin.RouterGroup) {
	group.GET("/api/media-library", a.handleMediaLibraryList)
	group.GET("/api/media-library/item", a.handleMediaLibraryItemGet)
	group.PATCH("/api/media-library/item", a.handleMediaLibraryItemPatch)
	group.GET("/api/media-library/file", a.handleMediaLibraryFileGet)
	group.DELETE("/api/media-library/file", a.handleMediaLibraryFileDelete)
	group.POST("/api/media-library/asset", a.handleMediaLibraryAsset)
	group.GET("/api/media-library/sync", a.handleMediaLibrarySyncGet)
	group.POST("/api/media-library/sync", a.handleMediaLibrarySyncPost)
	group.GET("/api/media-library/sync/logs", a.handleMediaLibrarySyncLogs)
	group.GET("/api/media-library/move", a.handleMediaLibraryMoveGet)
	group.POST("/api/media-library/move", a.handleMediaLibraryMovePost)
	group.GET("/api/media-library/status", a.handleMediaLibraryStatus)
}
