package web

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// healthDeepTimeout 是 /api/healthz?deep=1 深度探测的最大耗时。
// 超过后视为依赖不可用。
const healthDeepTimeout = 3 * time.Second

// handleHealthz 响应 /api/healthz:
//   - 默认 (无 query 或 deep=0): 永远返回 200 OK, 只证明进程在跑、监听地址活着。
//     适合给反向代理或 TCP 探活用, 开销极小。
//   - deep=1: 执行 healthCheck (通常是 sql.DB.Ping), 成功返回 200,
//     失败返回 503。适合 docker-compose healthcheck / 本地 cron 定期探活用。
//     若 API 构造时没有传 healthCheck, 返回 200 并在 data.deep 字段标记 "skipped"。
func (a *API) handleHealthz(c *gin.Context) {
	if c.Query("deep") != "1" {
		writeSuccess(c.Writer, "ok", map[string]string{"status": "ok"})
		return
	}
	if a.healthCheck == nil {
		writeSuccess(c.Writer, "ok", map[string]string{"status": "ok", "deep": "skipped"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthDeepTimeout)
	defer cancel()
	if err := a.healthCheck(ctx); err != nil {
		writeJSON(c.Writer, http.StatusServiceUnavailable, responseBody{
			Code:    errCodeUnknown,
			Message: "deep health check failed: " + err.Error(),
			Data:    map[string]string{"status": "unhealthy"},
		})
		return
	}
	writeSuccess(c.Writer, "ok", map[string]string{"status": "ok", "deep": "ok"})
}
