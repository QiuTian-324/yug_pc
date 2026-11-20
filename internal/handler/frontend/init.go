package frontend

import "FastGo/internal/router"

// RegisterFrontendRoutes 注册前端路由
func RegisterFrontendRoutes(registry *router.RouteRegistry) {
	// 注册用户模块的路由
	userHandler := NewUserHandler()
	userHandler.RegisterRoutes(registry)

	// 注册会话模块的路由
	conversationHandler := NewConversationHandler()
	conversationHandler.RegisterRoutes(registry)
}
