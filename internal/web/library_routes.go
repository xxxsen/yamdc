package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineLibraryRoutes(group *gin.RouterGroup) {
	group.GET("/api/library", a.handleListLibrary)
	group.GET("/api/library/item", a.handleLibraryItemGet)
	group.PATCH("/api/library/item", a.handleLibraryItemPatch)
	group.DELETE("/api/library/item", a.handleLibraryItemDelete)
	group.GET("/api/library/file", a.handleLibraryFileGet)
	group.DELETE("/api/library/file", a.handleLibraryFileDelete)
	group.POST("/api/library/asset", a.handleLibraryAsset)
	group.POST("/api/library/poster-crop", a.handleLibraryPosterCrop)
}
