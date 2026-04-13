package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/webapi"
)

func (a *API) Engine(addr string) (webapi.IWebEngine, error) {
	return webapi.NewEngine(
		"",
		addr,
		webapi.WithExtraMiddlewares(corsMiddleware()),
		webapi.WithRegister(func(group *gin.RouterGroup) {
			a.registerEngineCoreRoutes(group)
			a.registerEngineJobRoutes(group)
			a.registerEngineLibraryRoutes(group)
			a.registerEngineMediaLibraryRoutes(group)
			a.registerEngineDebugRoutes(group)
			a.registerEngineAssetRoutes(group)
		}),
	)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (a *API) registerEngineCoreRoutes(group *gin.RouterGroup) {
	group.GET("/api/healthz", a.handleHealthz)
}
