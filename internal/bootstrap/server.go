package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"FastGo/internal/global"
	"FastGo/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Server 包装 Gin 引擎与原生 HTTP 服务器，集中管理路由注册、启动和关闭。
type Server struct {
	app    *App
	engine *gin.Engine
	srv    *http.Server
}

// NewServer 完成 HTTP 服务的基础装配：引擎模式、中间件、监听参数与全局引用。
func NewServer(app *App) *Server {
	// 生产环境关闭 Gin 的调试与路由高亮输出，避免无关日志干扰。
	gin.SetMode(gin.ReleaseMode)

	// gin.New 不会默认挂载 Logger 和 Recovery，便于由项目统一接管全局中间件。
	engine := gin.New()

	// 注册全局中间件。Gin 按洋葱模型执行：
	// 请求进入时依次经过 Logger、Recovery、Cors；处理完成后按相反顺序回传响应。
	engine.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.Cors(),
	)

	// 持有全局引擎，供业务层的 SetupRoutes 等初始化逻辑注册路由。
	global.Engine = engine

	// 配置 net/http 服务器；读取、写入超时从应用配置构造监听地址。
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", app.Config.Srv.Port),
		Handler:        engine,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 限制请求头总大小为 1 MB。
	}

	return &Server{
		app:    app,
		engine: engine,
		srv:    srv,
	}
}

// InitRoutes 集中完成业务路由和基础探活路由的注册。
func (s *Server) InitRoutes() {
	// 注册项目各业务模块的接口。
	SetupRoutes()

	// 提供简单健康检查，供容器、负载均衡等外部组件探测存活状态。
	s.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
}

// logRoutes 在路由注册完成后输出路由表，便于启动时核对接口是否生效。
func (s *Server) logRoutes() {
	routes := s.engine.Routes()
	if len(routes) == 0 {
		global.Log.Warn("没有找到任何注册的路由!")
		return
	}

	global.Log.Info("已注册的路由列表:")
	for _, route := range routes {
		global.Log.Info("路由注册",
			zap.String("method", route.Method),
			zap.String("path", route.Path),
		)
	}
}

// Start 按“注册路由 -> 输出路由 -> 开始监听”的顺序启动 HTTP 服务。
func (s *Server) Start() error {
	global.Log.Info("服务器启动中...", zap.String("addr", s.srv.Addr))

	// 路由必须在监听端口前全部注册完成，否则启动后可能出现接口无法访问。
	s.InitRoutes()

	// 此时路由表已完整，打印的结果可用于确认模块是否成功装载。
	s.logRoutes()

	// 回填到 App，供优雅关闭流程统一持有和关闭 HTTP 服务。
	s.app.Server = s.srv

	// ListenAndServe 会阻塞当前 Goroutine，直到服务关闭或监听发生错误。
	return s.srv.ListenAndServe()
}

// Shutdown 停止接收新连接，并等待现有请求在指定上下文期限内处理完成。
func (s *Server) Shutdown(ctx context.Context) error {
	global.Log.Info("服务器正在关闭...")
	return s.srv.Shutdown(ctx)
}
