package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineAssetRoutes(group *gin.RouterGroup) {
	group.GET("/api/assets/*path", a.handleAssetGet)
	group.POST("/api/assets/*path", a.handleAssetPost)
}
