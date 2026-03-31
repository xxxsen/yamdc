package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineMediaLibraryRoutes(group *gin.RouterGroup) {
	group.GET("/api/media-library", gin.WrapF(a.handleMediaLibraryList))
	group.GET("/api/media-library/item", gin.WrapF(a.handleMediaLibraryItem))
	group.PATCH("/api/media-library/item", gin.WrapF(a.handleMediaLibraryItem))
	group.GET("/api/media-library/file", gin.WrapF(a.handleMediaLibraryFile))
	group.DELETE("/api/media-library/file", gin.WrapF(a.handleMediaLibraryFile))
	group.POST("/api/media-library/asset", gin.WrapF(a.handleMediaLibraryAsset))
	group.GET("/api/media-library/sync", gin.WrapF(a.handleMediaLibrarySync))
	group.POST("/api/media-library/sync", gin.WrapF(a.handleMediaLibrarySync))
	group.GET("/api/media-library/move", gin.WrapF(a.handleMediaLibraryMove))
	group.POST("/api/media-library/move", gin.WrapF(a.handleMediaLibraryMove))
	group.GET("/api/media-library/status", gin.WrapF(a.handleMediaLibraryStatus))
}
