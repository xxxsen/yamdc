package web

import "github.com/gin-gonic/gin"

func (a *API) registerEngineAssetRoutes(group *gin.RouterGroup) {
	group.GET("/api/assets/*path", gin.WrapF(a.handleAsset))
	group.POST("/api/assets/*path", gin.WrapF(a.handleAsset))
}
