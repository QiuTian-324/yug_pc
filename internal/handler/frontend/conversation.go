package frontend

import (
	"FastGo/internal/handler"
	"FastGo/internal/handler/dto"
	"FastGo/internal/router"
	"FastGo/internal/service"
	"FastGo/pkg/jwt"
	"FastGo/pkg/response"
	"FastGo/pkg/validator"

	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	*handler.CommonHandler
	ConversationService *service.ConversationService
}

func NewConversationHandler() *ConversationHandler {
	return &ConversationHandler{
		CommonHandler:       handler.NewCommonHandler(),
		ConversationService: service.NewConversationService(),
	}
}

func (h *ConversationHandler) RegisterRoutes(registry *router.RouteRegistry) {
	registry.Register("GET", "conversation", "/list", h.List, jwt.REQUIRE_AUTH, "会话列表")
	registry.Register("POST", "conversation", "/read", h.ClearUnread, jwt.REQUIRE_AUTH, "清空未读")
	registry.Register("POST", "conversation", "/pin-mute", h.SetPinMute, jwt.REQUIRE_AUTH, "置顶/免打扰")
	registry.Register("DELETE", "conversation", "", h.Delete, jwt.REQUIRE_AUTH, "删除会话")
}

func (h *ConversationHandler) List(c *gin.Context) {
	list, err := h.ConversationService.List(c.GetUint("user_id"))
	if err != nil {
		response.FailMsg(c, response.ServerError, "获取会话列表失败")
		return
	}
	response.OK(c, list)
}

func (h *ConversationHandler) ClearUnread(c *gin.Context) {
	var req dto.ConversationTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}
	if err := h.ConversationService.ClearUnread(c.GetUint("user_id"), req); err != nil {
		response.FailMsg(c, response.ServerError, "操作失败")
		return
	}
	response.OKMsg(c, "ok")
}

func (h *ConversationHandler) SetPinMute(c *gin.Context) {
	var req dto.SetPinMuteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}
	if err := h.ConversationService.SetPinMute(c.GetUint("user_id"), req); err != nil {
		response.FailMsg(c, response.ServerError, "设置失败")
		return
	}
	response.OKMsg(c, "ok")
}

func (h *ConversationHandler) Delete(c *gin.Context) {
	var req dto.ConversationTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}
	if err := h.ConversationService.Delete(c.GetUint("user_id"), req); err != nil {
		response.FailMsg(c, response.ServerError, "删除失败")
		return
	}
	response.OKMsg(c, "ok")
}
