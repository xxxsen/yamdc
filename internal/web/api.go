package web

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/movieidcleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/review"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	plugineditor "github.com/xxxsen/yamdc/internal/searcher/plugin/editor"
	"github.com/xxxsen/yamdc/internal/store"
)

// HealthCheckFunc 用于 /api/healthz?deep=1 的深度探测。
// 通常实现为 sql.DB.PingContext, 传 nil 表示不提供深度探测。
type HealthCheckFunc func(ctx context.Context) error

type API struct {
	jobRepo     *repository.JobRepository
	scanner     *scanner.Service
	jobSvc      *job.Service
	reviewSvc   *review.Service
	saveDir     string
	media       *medialib.Service
	store       store.IStorage
	cleaner     movieidcleaner.Cleaner
	debugger    *searcher.Debugger
	handlers    *phandler.Debugger
	editor      *plugineditor.Service
	healthCheck HealthCheckFunc
}

// NewAPI 组装 HTTP 层 API 对象, 签名保持不变以避免大范围改 caller。
//
// 设计取舍:
//
//   - 不再在构造时对 reviewSvc 做特判 panic。生产组装层 (internal/bootstrap)
//     必须自行通过集成测试保证依赖齐全; 单元测试 / 轻量 server 允许传任意
//     组合 nil 依赖, 各 handler 自己用 requireDependency 做 503 守门。
//   - 任何 handler 在访问需要的字段前必须先调用 requireDependency, nil 时
//     直接返回 HTTP 503 + errCodeServiceUnavailable, 不走 handler 主路径。
//
// 这样, 只挂 /api/healthz 的最小场景仍然可以启动 (健康检查从设计上就允许
// 不依赖任何后端服务); 其它路由不会再因为构造期遗漏依赖在运行期 nil-deref。
func NewAPI(
	jobRepo *repository.JobRepository,
	scanner *scanner.Service,
	jobSvc *job.Service,
	reviewSvc *review.Service,
	saveDir string,
	media *medialib.Service,
	storage store.IStorage,
	cleaner movieidcleaner.Cleaner,
	debugger *searcher.Debugger,
	handlers *phandler.Debugger,
	editor *plugineditor.Service,
	healthCheck HealthCheckFunc,
) *API {
	return &API{
		jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, reviewSvc: reviewSvc,
		saveDir: saveDir, media: media, store: storage,
		cleaner: cleaner, debugger: debugger, handlers: handlers, editor: editor,
		healthCheck: healthCheck,
	}
}

// requireDependency 检查 handler 所依赖的某个对象是否已注入. 因为 Go 接口
// nil 与 nil pointer 在 reflect 层并不严格相同, 外加我们既需要 *T 也需要
// interface 型依赖 (例如 movieidcleaner.Cleaner / store.IStorage), 这里
// 直接接受 any, 调用方负责传入 typed nil 或具体值.
//
// 行为:
//   - dep 不为 nil (含已实现的 interface 值): 返回 true, handler 继续执行.
//   - dep 为 nil: 写入 HTTP 503 + body { code: errCodeServiceUnavailable,
//     message: "<name> dependency is not available" }, abort 当前请求并
//     返回 false. caller 直接 return.
//
// 503 是 "服务暂不可用" 的标准语义. 这里属于"协议外的可用性保护层", 与
// CORS 403 / 上传 413 同类, 与业务层 200 + non-zero code 显式区分.
func requireDependency(c *gin.Context, dep any, name string) bool {
	if !isDependencyAvailable(dep) {
		body := responseBody{
			Code:    errCodeServiceUnavailable,
			Message: fmt.Sprintf("%s dependency is not available", name),
			Data:    nil,
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, body)
		return false
	}
	return true
}

// isDependencyAvailable 判定 dep 是否为有效注入.
//
// Go 接口 nil 比对的两个陷阱:
//  1. 直接 dep == nil 只能识别"完全没赋值"的 nil interface, 无法识别
//     "interface 里包了一个 typed nil 指针" 这种情况 (例如 var s *job.Service
//     var d any = s; d == nil 为 false).
//  2. 不同 handler 的依赖既有 *T 也有接口型 (store.IStorage / Cleaner),
//     无法用单一的具体类型断言一次性兜住.
//
// 方案: 先 fast-path 走具体类型断言覆盖已知所有依赖类型 (零反射开销),
// 兜底用 reflect.Value.IsNil 处理 "typed-nil 包到 any 里" 的边角. 这样
// 即使未来再加新依赖类型, 没及时挂到 switch 上也不会被误判为可用.
func isDependencyAvailable(dep any) bool {
	if dep == nil {
		return false
	}
	if available, matched := matchKnownDependency(dep); matched {
		return available
	}
	return reflectNonNil(dep)
}

// matchKnownDependency 走具体类型断言的 fast-path, 覆盖当前所有已知依赖类型.
// 命中时返回 (是否非 nil, true); 未命中返回 (false, false), 由 reflect 兜底.
func matchKnownDependency(dep any) (bool, bool) {
	switch v := dep.(type) {
	case *job.Service:
		return v != nil, true
	case *review.Service:
		return v != nil, true
	case *scanner.Service:
		return v != nil, true
	case *repository.JobRepository:
		return v != nil, true
	case *medialib.Service:
		return v != nil, true
	case store.IStorage:
		return v != nil, true
	case movieidcleaner.Cleaner:
		return v != nil, true
	case *searcher.Debugger:
		return v != nil, true
	case *phandler.Debugger:
		return v != nil, true
	case *plugineditor.Service:
		return v != nil, true
	}
	return false, false
}

// reflectNonNil 兜底处理 typed-nil 包进 any 的情况: 对引用类型 (Ptr / Interface
// / Chan / Func / Map / Slice) 调用 reflect.Value.IsNil 判定; 其它 Kind 视为
// 值类型, 直接判定为可用.
func reflectNonNil(dep any) bool {
	rv := reflect.ValueOf(dep)
	if !isNilableKind(rv.Kind()) {
		return true
	}
	return !rv.IsNil()
}

// isNilableKind 列出 reflect.Value.IsNil 合法调用的 Kind 集合. 用 if-链而非
// switch+default 是为了避免 exhaustive linter 强制把全部 27 个 Kind 摊开.
func isNilableKind(k reflect.Kind) bool {
	if k == reflect.Ptr || k == reflect.Interface || k == reflect.Chan {
		return true
	}
	if k == reflect.Func || k == reflect.Map || k == reflect.Slice {
		return true
	}
	return false
}
