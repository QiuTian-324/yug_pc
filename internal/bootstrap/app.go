package bootstrap

import (
	"FastGo/config"
	"FastGo/pkg/validator"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"FastGo/internal/global"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用核心结构体，持有全局依赖实例
type App struct {
	Config *config.Options // 配置信息
	DB     *gorm.DB        // 数据库连接实例
	Redis  *redis.Client   // Redis 客户端实例
	Server *http.Server    // 原生 HTTP Server 实例
	srv    *Server         // 自定义的 Server 包装类（可能包含路由引擎等）
}

// Setup 应用装配根（Assembly Root），负责按顺序初始化所有依赖
func Setup() (*App, error) {
	// 1. 加载配置（通常从 .env 或 yaml 读取）
	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("load config failed: %v", err)
	}
	global.Config = cfg // 将配置存入全局变量，供其他包直接调用

	app := &App{
		Config: cfg,
	}

	// 2. 初始化日志组件（如 zap）
	if err := setupLogger(); err != nil {
		return nil, fmt.Errorf("setup logger failed: %v", err)
	}

	// 3. 初始化 MySQL 连接池
	if err := app.setupMySQL(); err != nil {
		return nil, fmt.Errorf("setup mysql failed: %v", err)
	}

	// 4. 初始化 Redis 连接池
	if err := app.setupRedis(); err != nil {
		return nil, fmt.Errorf("setup redis failed: %v", err)
	}

	// 5. 初始化 HTTP Server 实例
	server := NewServer(app)
	app.Server = server.srv // 绑定原生 http.Server
	app.srv = server        // 绑定自定义 Server

	// 6. 初始化参数验证器（注册自定义验证规则）
	validator.Setup()

	return app, nil
}

// Run 启动应用主逻辑
func (app *App) Run() error {
	// 使用 Goroutine 异步启动 HTTP 服务，防止阻塞主线程
	go func() {
		if err := app.srv.Start(); err != nil {
			// http.ErrServerClosed 是调用 Shutdown 时的正常返回，忽略该错误
			if err != http.ErrServerClosed {
				global.Log.Error("服务器启动失败", zap.Error(err))
			}
		}
	}()

	// 主线程进入阻塞状态，监听系统退出信号
	app.GracefulShutdown()
	return nil
}

// GracefulShutdown 优雅关闭逻辑，确保资源安全释放
func (app *App) GracefulShutdown() {
	// 1. 创建信号通道，监听 Ctrl+C (SIGINT) 和 终止命令 (SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 2. 阻塞等待信号，接收到信号后继续向下执行
	<-quit
	global.Log.Info("开始优雅关闭...")

	// 3. 设置 5 秒超时上下文，防止关闭过程无限期挂起
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. 关闭 HTTP Server（停止接收新请求，等待现有请求处理完毕）
	if app.Server != nil {
		global.Log.Info("正在关闭 HTTP 服务器...")
		if err := app.Server.Shutdown(ctx); err != nil {
			global.Log.Error("HTTP 服务器关闭错误", zap.Error(err))
		}
	}

	// 5. 关闭 MySQL 连接池
	if app.DB != nil {
		global.Log.Info("正在关闭数据库连接...")
		sqlDB, err := app.DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				global.Log.Error("数据库关闭错误", zap.Error(err))
			}
		}
	}

	// 6. 关闭 Redis 连接池
	if app.Redis != nil {
		global.Log.Info("正在关闭 Redis 连接...")
		if err := app.Redis.Close(); err != nil {
			global.Log.Error("Redis 关闭错误", zap.Error(err))
		}
	}

	global.Log.Info("服务器已成功关闭")
}
