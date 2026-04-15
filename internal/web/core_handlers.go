package web

import (
	"github.com/gin-gonic/gin"
)

func (a *API) handleHealthz(c *gin.Context) {
	writeSuccess(c.Writer, "ok", map[string]string{"status": "ok"})
}
