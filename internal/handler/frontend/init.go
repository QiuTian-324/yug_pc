package frontend

import (
	"FastGo/internal/realtime"
	"FastGo/internal/router"
	"FastGo/internal/service"
)

// RegisterFrontendRoutes 注册前端路由
func RegisterFrontendRoutes(registry *router.RouteRegistry) {
	hub := realtime.NewHub()
	presenceService := service.NewPresenceService(hub)
	hub.SetConnectionChangeHandler(presenceService.OnConnectionChanged)

	// 注册用户模块的路由
	userHandler := NewUserHandlerWithPresence(presenceService)
	userHandler.RegisterRoutes(registry)

	fileHandler := NewFileHandler()
	fileHandler.RegisterRoutes(registry)

	// 注册会话模块的路由
	conversationHandler := NewConversationHandler()
	conversationHandler.RegisterRoutes(registry)

	messageHandler := NewMessageHandlerWithPresence(hub, presenceService)
	messageHandler.RegisterRoutes(registry)
}
