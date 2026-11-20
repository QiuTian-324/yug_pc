package router

import (
	"FastGo/internal/middleware/auth"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RouteConfig 定义路由配置结构体
type RouteConfig struct {
	Method      string
	Path        string
	HandlerFunc gin.HandlerFunc
	AuthLevel   int
	Description string
	Group       string
}

// RouteRegistry 路由注册器
type RouteRegistry struct {
	routes []RouteConfig
}

// NewRouteRegistry 创建并返回一个新的路由注册器
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes: make([]RouteConfig, 0),
	}
}

// Register 注册一个新的路由
func (r *RouteRegistry) Register(method, group, path string, handlerFunc gin.HandlerFunc, authLevel int, description string) {
	if group == "" {
		panic("group is required")
	}
	fullPath := "/" + group + path
	r.routes = append(r.routes, RouteConfig{
		Method:      method,
		Path:        fullPath,
		HandlerFunc: handlerFunc,
		AuthLevel:   authLevel,
		Description: description,
		Group:       group,
	})
}

// SetupRoutes 注册所有路由到指定的 Engine
func (r *RouteRegistry) SetupRoutes(engine *gin.Engine) {
	for _, route := range r.routes {
		if err := registerRoute(engine, route); err != nil {
			// 生产环境中建议使用统一的日志组件记录错误
			panic(fmt.Sprintf("路由注册失败 [%s %s]: %v", route.Method, route.Path, err))
		}
	}
}

// registerRoute 内部方法：注册具体路由并处理权限
func registerRoute(engine *gin.Engine, route RouteConfig) error {
	// 获取鉴权中间件
	authMiddleware := auth.GetMiddleware(route.AuthLevel)

	// 构建中间件链，避免 nil 导致的 panic
	middlewares := make([]gin.HandlerFunc, 0, 2)
	if authMiddleware != nil {
		middlewares = append(middlewares, authMiddleware)
	}
	middlewares = append(middlewares, route.HandlerFunc)

	// 使用标准的 HTTP 方法常量，避免魔法字符串
	switch route.Method {
	case http.MethodPost:
		engine.POST(route.Path, middlewares...)
	case http.MethodGet:
		engine.GET(route.Path, middlewares...)
	case http.MethodPut:
		engine.PUT(route.Path, middlewares...)
	case http.MethodDelete:
		engine.DELETE(route.Path, middlewares...)
	default:
		return fmt.Errorf("不支持的 HTTP 方法: %s", route.Method)
	}
	return nil
}
