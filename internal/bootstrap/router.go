package bootstrap

import (
	"FastGo/internal/global"
	"FastGo/internal/handler/frontend"
	"FastGo/internal/router"
)

func SetupRoutes() {
	engine := global.Engine
	// 创建路由注册器
	registry := router.NewRouteRegistry()

	// 注册前端路由
	frontend.RegisterFrontendRoutes(registry)
	registry.SetupRoutes(engine)
}
