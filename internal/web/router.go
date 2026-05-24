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

// loadAllowedOrigins 读取 YAMDC_ALLOWED_ORIGINS 环境变量 (逗号分隔).
// 默认行为: 未设置或 trim 后为空时返回空切片, 表示 wildcard 模式
// (Access-Control-Allow-Origin: *), 用于本地用户通过任意域名访问场景.
// 显式配置后切换到白名单模式, 仅命中的 Origin 被允许跨域访问.
func loadAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("YAMDC_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil
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
	return out
}

// corsMiddleware 实现 CORS + 状态变更 Origin 校验, 支持两种模式.
//
// 模式 1: wildcard (allowedOrigins 为空)
//
//   - 默认 wildcard, 用户本地启动后可通过任意域名 (例如局域网 IP /
//     自定义 hosts 域名) 访问后端.
//   - 跨域响应统一写入 Access-Control-Allow-Origin: *.
//   - OPTIONS 预检直接返回 204 + Allow-* 头.
//   - 不做 Origin 白名单拦截, 状态变更方法直接进入 handler.
//
// 模式 2: 白名单 (allowedOrigins 非空)
//
//   - 设置 YAMDC_ALLOWED_ORIGINS 后启用, 用于明确收紧的部署场景.
//   - 命中白名单时回写完全相同的 Origin 字面值, 浏览器按同源策略放行.
//   - 未命中白名单的 OPTIONS 预检返回 HTTP 403, 让浏览器放弃后续请求.
//   - 未命中白名单且方法属于 POST/PUT/PATCH/DELETE 的请求返回 HTTP 403,
//     拦在 handler 之前不让任何写副作用执行.
//   - 没有 Origin 头的请求 (本机 CLI / curl / same-origin) 直接放行.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := buildAllowedOriginSet(allowedOrigins)
	wildcard := len(allowedOrigins) == 0
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))

		if wildcard {
			handleWildcardCORS(c, origin)
			return
		}

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

// handleWildcardCORS 在 wildcard 模式下统一写 Allow-Origin: *, 不做白名单
// 拦截; 实际请求方法 (Allow-Methods) 与允许头 (Allow-Headers) 仅在 OPTIONS
// 预检上写入 — 这两个头按 fetch spec 只在预检阶段被浏览器读取, 给 GET/POST
// 实际请求重复写一遍是冗余字节, 不影响安全语义. 即便请求没有 Origin
// (例如本机 curl), 仍然写 "*" 是无害的, 浏览器只在跨域场景下读取此头.
func handleWildcardCORS(c *gin.Context, origin string) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	if origin != "" {
		c.Writer.Header().Set("Vary", "Origin")
	}
	if c.Request.Method == http.MethodOptions {
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
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
