package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) handleHealthz(c *gin.Context) {
	writeSuccess(c.Writer, http.StatusOK, "ok", map[string]string{"status": "ok"})
}
