package web

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/xxxsen/common/webapi"
)

// 默认允许的本机前端开发源. 不允许 "*"; 详见 corsMiddleware 注释.
var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://127.0.0.1:3000",
}

// stateChangingMethods 列出会写状态 / 触发副作用的 HTTP 方法.
// 这些方法即便没有显式 CORS 也可能被某些浏览器作为 "简单请求" 直接发出
// (例如 POST application/x-www-form-urlencoded), 因此我们在 handler 之前
// 做强制 Origin 校验, 避免任意第三方网页通过用户浏览器跨站触发本机副作用.
var stateChangingMethods = map[string]struct{}{
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

func (a *API) Engine(addr string) (webapi.IWebEngine, error) {
	allowedOrigins := loadAllowedOrigins()
	engine, err := webapi.NewEngine(
		"",
		addr,
		webapi.WithExtraMiddlewares(corsMiddleware(allowedOrigins)),
		webapi.WithRegister(func(group *gin.RouterGroup) {
			a.registerEngineCoreRoutes(group)
			a.registerEngineJobRoutes(group)
			a.registerEngineLibraryRoutes(group)
			a.registerEngineMediaLibraryRoutes(group)
			a.registerEngineDebugRoutes(group)
			a.registerEngineAssetRoutes(group)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create web engine failed: %w", err)
	}
	return engine, nil
}

// loadAllowedOrigins 读取 YAMDC_ALLOWED_ORIGINS 环境变量 (逗号分隔), 不
// 设置时返回 defaultAllowedOrigins. 用环境变量是为了让本地用户可以按需
// 加入自己的前端域名 (例如局域网开发机), 而不需要改源码.
func loadAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("YAMDC_ALLOWED_ORIGINS"))
	if raw == "" {
		out := make([]string, len(defaultAllowedOrigins))
		copy(out, defaultAllowedOrigins)
		return out
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		out = append(out, defaultAllowedOrigins...)
	}
	return out
}

// corsMiddleware 基于显式白名单实现 CORS + 状态变更 Origin 校验.
//
// 安全模型:
//
//  1. 不再返回 Access-Control-Allow-Origin: *. 收到带 Origin 的请求时,
//     只有当 Origin 命中白名单才会回写完全相同的字面值, 否则跨站浏览器会
//     按浏览器策略阻断响应可读性.
//  2. 对 POST/PUT/PATCH/DELETE 等状态变更方法做 Origin 强制校验:
//     - 没有 Origin 头: 视为本机 CLI / curl / same-origin 请求, 放行,
//     由后端 handler 决定响应.
//     - 有 Origin 但不在白名单: 返回 HTTP 403 + body { code, message },
//     拦在 handler 之前, 不让任何写副作用执行.
//  3. 预检请求 (OPTIONS):
//     - 命中白名单: 返回 204 No Content + Allow-* 头.
//     - 不命中白名单 (含 "未知 Origin"): 返回 HTTP 403, 让浏览器直接放弃
//     后续真正的请求. 没有 Origin 头的 OPTIONS 走 same-origin 兼容路径,
//     仍然返回 204 但不带 Access-Control-Allow-Origin.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := buildAllowedOriginSet(allowedOrigins)
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))
		isAllowedOrigin := origin != "" && allowed.contains(origin)

		if origin != "" && isAllowedOrigin {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || isAllowedOrigin {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			abortWithOriginForbidden(c, origin)
			return
		}

		if _, mutates := stateChangingMethods[c.Request.Method]; mutates && origin != "" && !isAllowedOrigin {
			abortWithOriginForbidden(c, origin)
			return
		}

		c.Next()
	}
}

// abortWithOriginForbidden 用项目统一的 { code, message, data } 协议向
// 客户端返回 403, 并保持 HTTP 状态码 403 让 CORS / 安全层与业务层 200
// 协议显式区分: 业务错误仍然 HTTP 200, 跨站源拦截在协议外用 403 阻断.
func abortWithOriginForbidden(c *gin.Context, origin string) {
	body := responseBody{
		Code:    errCodeOriginForbidden,
		Message: fmt.Sprintf("origin %q is not allowed", origin),
		Data:    nil,
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.AbortWithStatusJSON(http.StatusForbidden, body)
}

// allowedOriginSet 是一个一次构造、多次读取的容器, 内部用 map 去重.
// 用 sync.Once 确保即使 corsMiddleware 被多次实例化也不会重复构造.
type allowedOriginSet struct {
	once sync.Once
	set  map[string]struct{}
	raw  []string
}

func buildAllowedOriginSet(origins []string) *allowedOriginSet {
	return &allowedOriginSet{raw: origins}
}

func (s *allowedOriginSet) contains(origin string) bool {
	s.once.Do(func() {
		s.set = make(map[string]struct{}, len(s.raw))
		for _, item := range s.raw {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			s.set[item] = struct{}{}
		}
	})
	_, ok := s.set[origin]
	return ok
}

func (a *API) registerEngineCoreRoutes(group *gin.RouterGroup) {
	group.GET("/api/healthz", a.handleHealthz)
}
