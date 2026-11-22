package frontend

import (
	"errors"

	"FastGo/internal/global"
	"FastGo/internal/handler"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"
	"FastGo/internal/router"
	"FastGo/internal/service"
	"FastGo/pkg/jwt"
	"FastGo/pkg/response"
	"FastGo/pkg/validator"
	"FastGo/utils"

	"github.com/gin-gonic/gin"
)

// UserHandler 处理用户相关的请求
type UserHandler struct {
	*handler.CommonHandler
	UserService *service.UserService
}

// NewUserHandler 创建并返回一个 UserHandler 实例
func NewUserHandler() *UserHandler {
	return &UserHandler{
		CommonHandler: handler.NewCommonHandler(),
		UserService:   service.NewUserService(),
	}
}

// RegisterRoutes 注册用户模块的路由
func (h *UserHandler) RegisterRoutes(registry *router.RouteRegistry) {
	// 公开接口：保持 1（PUBLIC）
	registry.Register("POST", "user", "/login", h.Login, jwt.PUBLIC, "用户登录")
	registry.Register("POST", "user", "/register", h.Register, jwt.PUBLIC, "用户注册")
	// 需要认证的接口：保持 2（REQUIRE_AUTH）
	// registry.Register("GET", "user", "/profile", h.Profile, jwt.REQUIRE_AUTH, "获取用户信息")
	registry.Register("GET", "user", "/search", h.SearchUser, jwt.REQUIRE_AUTH, "按用户名搜索用户")
	// 获取好友列表
	registry.Register("GET", "friend", "/list", h.FriendList, jwt.REQUIRE_AUTH, "好友列表")

	// 发起好友申请
	registry.Register("POST", "friend", "/request", h.SendFriendRequest, jwt.REQUIRE_AUTH, "发起好友申请")
	registry.Register("GET", "friend", "/request/list", h.FriendRequestList, jwt.REQUIRE_AUTH, "好友申请列表")
	registry.Register("POST", "friend", "/request/accept", h.AcceptFriendRequest, jwt.REQUIRE_AUTH, "同意好友申请")
	registry.Register("POST", "friend", "/request/reject", h.RejectFriendRequest, jwt.REQUIRE_AUTH, "拒绝好友申请")
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	// 绑定 JSON 请求体到 LoginRequest 结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	// 调用 UserService 的 Login 方法
	user, err := h.UserService.Login(&req)
	if err != nil {
		response.FailMsg(c, response.InvalidParams, err.Error())
		return
	}

	// 创建 JWT 服务

	jwtService := jwt.New(global.Config.Jwt.SigningKey)
	token, err := jwtService.GenerateToken(user.ID, user.Username) // 去掉 role 参数
	if err != nil {
		response.FailMsg(c, response.ServerError, "Token 生成失败")
		return
	}
	response.OK(c, dto.LoginResponse{
		Token: token,
		UserInfo: dto.UserInfoVO{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			Avatar:    user.Avatar,
			Bio:       user.Bio,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *UserHandler) Register(c *gin.Context) {
	var registerData struct {
		Username string `json:"username" binding:"required,username"`
		Password string `json:"password" binding:"required,password"`
	}

	if err := c.ShouldBindJSON(&registerData); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	hashedPassword, err := utils.HashPassword(registerData.Password)
	if err != nil {
		response.FailMsg(c, response.ServerError, "密码加密失败")
		return
	}

	user := model.User{
		Username: registerData.Username,
		Password: hashedPassword,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		response.FailMsg(c, response.ServerError, "注册失败")
		return
	}

	response.OKMsg(c, "注册成功")
}

// 获取用户信息
func (h *UserHandler) GetUserInfo(c *gin.Context) {

}

// SearchUser 按用户名搜索可添加的用户
func (h *UserHandler) SearchUser(c *gin.Context) {
	var req dto.SearchUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	user, err := h.UserService.SearchUser(c.GetUint("user_id"), req.Username)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.FailMsg(c, response.NotFound, err.Error())
		case errors.Is(err, service.ErrCannotSearchSelf):
			response.FailMsg(c, response.InvalidParams, err.Error())
		default:
			response.FailMsg(c, response.ServerError, "搜索用户失败")
		}
		return
	}

	response.OK(c, user)
}

// FriendList 获取好友列表
func (h *UserHandler) FriendList(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req dto.FriendListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailMsg(c, response.ServerError, "参数错误")
		return
	}

	h.UserService.FriendList(userID, &req)

	res, err := h.UserService.FriendList(userID, &req)
	if err != nil {
		response.FailMsg(c, response.ServerError, "获取好友列表失败")
		return
	}

	response.OK(c, res)
}

// SendFriendRequest 发起好友申请
func (h *UserHandler) SendFriendRequest(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req dto.SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	if err := h.UserService.SendFriendRequest(userID, &req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}
	response.OKMsg(c, "申请发送成功")
}

// FriendRequestList 获取当前用户收到的待处理好友申请
func (h *UserHandler) FriendRequestList(c *gin.Context) {
	var req dto.FriendRequestListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	list, err := h.UserService.FriendRequestList(c.GetUint("user_id"), &req)
	if err != nil {
		response.FailMsg(c, response.ServerError, "获取好友申请列表失败")
		return
	}
	response.OK(c, list)
}

// AcceptFriendRequest 同意好友申请
func (h *UserHandler) AcceptFriendRequest(c *gin.Context) {
	h.handleFriendRequest(c, true)
}

// RejectFriendRequest 拒绝好友申请
func (h *UserHandler) RejectFriendRequest(c *gin.Context) {
	h.handleFriendRequest(c, false)
}

// handleFriendRequest 处理好友申请
func (h *UserHandler) handleFriendRequest(c *gin.Context, accept bool) {
	var req dto.HandleFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	var err error
	if accept {
		err = h.UserService.AcceptFriendRequest(c.GetUint("user_id"), req.RequestID)
	} else {
		err = h.UserService.RejectFriendRequest(c.GetUint("user_id"), req.RequestID)
	}
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFriendRequestNotFound):
			response.FailMsg(c, response.NotFound, err.Error())
		case errors.Is(err, service.ErrFriendRequestNotPending):
			response.FailMsg(c, response.InvalidParams, err.Error())
		default:
			response.FailMsg(c, response.ServerError, "处理好友申请失败")
		}
		return
	}

	if accept {
		response.OKMsg(c, "已同意好友申请")
		return
	}
	response.OKMsg(c, "已拒绝好友申请")
}
