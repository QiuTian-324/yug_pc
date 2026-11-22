package frontend

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"FastGo/internal/handler"
	"FastGo/internal/handler/dto"
	"FastGo/internal/realtime"
	"FastGo/internal/router"
	"FastGo/internal/service"
	"FastGo/pkg/jwt"
	"FastGo/pkg/response"
	"FastGo/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	webSocketTicketPrefix = "ws:ticket:"
	webSocketTicketTTL    = time.Minute
)

type MessageHandler struct {
	*handler.CommonHandler
	MessageService  *service.MessageService
	PresenceService *service.PresenceService
	Hub             *realtime.Hub
}

type webSocketTicket struct {
	UserID uint   `json:"user_id"`
	Origin string `json:"origin"`
}

func NewMessageHandler() *MessageHandler {
	hub := realtime.NewHub()
	presenceService := service.NewPresenceService(hub)
	hub.SetConnectionChangeHandler(presenceService.OnConnectionChanged)
	return NewMessageHandlerWithPresence(hub, presenceService)
}

func NewMessageHandlerWithPresence(hub *realtime.Hub, presenceService *service.PresenceService) *MessageHandler {
	return &MessageHandler{
		CommonHandler:   handler.NewCommonHandler(),
		MessageService:  service.NewMessageService(),
		PresenceService: presenceService,
		Hub:             hub,
	}
}

func (h *MessageHandler) RegisterRoutes(registry *router.RouteRegistry) {
	registry.Register("POST", "ws", "/ticket", h.IssueWebSocketTicket, jwt.REQUIRE_AUTH, "申请WebSocket连接票据")
	registry.Register("GET", "ws", "", h.ConnectWebSocket, jwt.PUBLIC, "WebSocket连接")
	registry.Register("GET", "message", "/history", h.History, jwt.REQUIRE_AUTH, "获取单聊历史消息")
	registry.Register("GET", "message", "/sync", h.Sync, jwt.REQUIRE_AUTH, "增量同步单聊消息")
}

func (h *MessageHandler) IssueWebSocketTicket(c *gin.Context) {
	if h.Redis == nil {
		response.FailMsg(c, response.ServerError, "WebSocket认证服务不可用")
		return
	}

	ticket := uuid.NewString()
	payload, err := json.Marshal(webSocketTicket{
		UserID: c.GetUint("user_id"),
		Origin: c.GetHeader("Origin"),
	})
	if err != nil {
		response.FailMsg(c, response.ServerError, "创建WebSocket票据失败")
		return
	}
	if err := h.Redis.SetNX(c.Request.Context(), webSocketTicketPrefix+ticket, payload, webSocketTicketTTL).Err(); err != nil {
		response.FailMsg(c, response.ServerError, "创建WebSocket票据失败")
		return
	}

	response.OK(c, dto.WebSocketTicketVO{
		Ticket:    ticket,
		ExpiresIn: int(webSocketTicketTTL.Seconds()),
	})
}

func (h *MessageHandler) ConnectWebSocket(c *gin.Context) {
	if h.Redis == nil {
		response.FailMsg(c, response.ServerError, "WebSocket认证服务不可用")
		return
	}
	ticket := c.Query("ticket")
	if ticket == "" {
		response.FailMsg(c, response.Unauthorized, "WebSocket票据不能为空")
		return
	}

	payload, err := h.Redis.GetDel(c.Request.Context(), webSocketTicketPrefix+ticket).Bytes()
	if errors.Is(err, redis.Nil) {
		response.FailMsg(c, response.Unauthorized, "WebSocket票据无效或已过期")
		return
	}
	if err != nil {
		response.FailMsg(c, response.ServerError, "校验WebSocket票据失败")
		return
	}
	var claims webSocketTicket
	if err := json.Unmarshal(payload, &claims); err != nil || claims.UserID == 0 {
		response.FailMsg(c, response.Unauthorized, "WebSocket票据无效")
		return
	}
	if c.GetHeader("Origin") != claims.Origin {
		response.FailMsg(c, response.Forbidden, "WebSocket来源不匹配")
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == claims.Origin
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := h.Hub.Register(claims.UserID, conn)
	go client.WritePump()
	client.ReadPump(func(payload []byte) {
		h.handleWebSocketMessage(client, payload)
	})
}

func (h *MessageHandler) History(c *gin.Context) {
	var req dto.MessageHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	page, err := h.MessageService.History(c.GetUint("user_id"), req)
	if err != nil {
		h.respondMessageError(c, err)
		return
	}
	response.OK(c, page)
}

func (h *MessageHandler) Sync(c *gin.Context) {
	var req dto.MessageSyncRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	page, err := h.MessageService.Sync(c.GetUint("user_id"), req)
	if err != nil {
		h.respondMessageError(c, err)
		return
	}
	response.OK(c, page)
}

func (h *MessageHandler) handleWebSocketMessage(client *realtime.Client, payload []byte) {
	var envelope dto.WSClientEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		h.sendWebSocketError(client, "", response.InvalidParams, "消息格式错误")
		return
	}
	if envelope.Type != "message.send" {
		h.sendWebSocketError(client, envelope.RequestID, response.InvalidParams, "不支持的消息类型")
		return
	}

	var req dto.SendMessageRequest
	if err := json.Unmarshal(envelope.Data, &req); err != nil {
		h.sendWebSocketError(client, envelope.RequestID, response.InvalidParams, "消息参数格式错误")
		return
	}
	message, created, err := h.MessageService.SendDirectMessage(client.UserID, req)
	if err != nil {
		code, messageText := messageError(err)
		h.sendWebSocketError(client, envelope.RequestID, code, messageText)
		return
	}

	if created {
		event := dto.WSServerEnvelope{Type: "message.new", Data: message}
		if err := h.Hub.SendToUser(req.TargetID, nil, event); err != nil {
			h.sendWebSocketError(client, envelope.RequestID, response.ServerError, "消息推送失败")
			return
		}
		if err := h.Hub.SendToUser(client.UserID, client, event); err != nil {
			h.sendWebSocketError(client, envelope.RequestID, response.ServerError, "多端消息同步失败")
			return
		}
	}

	if err := client.SendJSON(dto.WSServerEnvelope{
		Type:      "message.ack",
		RequestID: envelope.RequestID,
		Data:      message,
	}); err != nil {
		return
	}
}

func (h *MessageHandler) sendWebSocketError(client *realtime.Client, requestID string, code int, message string) {
	_ = client.SendJSON(dto.WSServerEnvelope{
		Type:      "message.error",
		RequestID: requestID,
		Code:      code,
		Message:   message,
	})
}

func (h *MessageHandler) respondMessageError(c *gin.Context, err error) {
	code, message := messageError(err)
	response.FailMsg(c, code, message)
}

func messageError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrInvalidMessage):
		return response.InvalidParams, err.Error()
	case errors.Is(err, service.ErrClientMessageIDConflict):
		return response.InvalidParams, err.Error()
	case errors.Is(err, service.ErrFileNotFound):
		return response.NotFound, err.Error()
	case errors.Is(err, service.ErrMessageUserNotFound):
		return response.NotFound, err.Error()
	case errors.Is(err, service.ErrMessageNotFriend), errors.Is(err, service.ErrMessageBlocked):
		return response.Forbidden, err.Error()
	case errors.Is(err, service.ErrConversationAccess):
		return response.Forbidden, err.Error()
	default:
		return response.ServerError, "消息处理失败"
	}
}
