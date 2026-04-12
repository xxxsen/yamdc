package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineLibraryRoutes(group *gin.RouterGroup) {
	group.GET("/api/library", gin.WrapF(a.handleListLibrary))
	group.GET("/api/library/item", gin.WrapF(a.handleLibraryItem))
	group.PATCH("/api/library/item", gin.WrapF(a.handleLibraryItem))
	group.DELETE("/api/library/item", gin.WrapF(a.handleLibraryItem))
	group.GET("/api/library/file", gin.WrapF(a.handleLibraryFile))
	group.DELETE("/api/library/file", gin.WrapF(a.handleLibraryFile))
	group.POST("/api/library/asset", gin.WrapF(a.handleLibraryAsset))
	group.POST("/api/library/poster-crop", gin.WrapF(a.handleLibraryPosterCrop))
}
