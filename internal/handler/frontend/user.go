package frontend

import (
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
	LoginService *service.LoginService
}

// NewUserHandler 创建并返回一个 UserHandler 实例
func NewUserHandler() *UserHandler {
	return &UserHandler{
		CommonHandler: handler.NewCommonHandler(),
		LoginService:  service.NewLoginService(),
	}
}

// RegisterRoutes 注册用户模块的路由
func (h *UserHandler) RegisterRoutes(registry *router.RouteRegistry) {
	// 公开接口：保持 1（PUBLIC）
	registry.Register("POST", "user", "/login", h.Login, jwt.PUBLIC, "用户登录")
	registry.Register("POST", "user", "/register", h.Register, jwt.PUBLIC, "用户注册")
	// 需要认证的接口：保持 2（REQUIRE_AUTH）
	// registry.Register("GET", "user", "/profile", h.Profile, jwt.REQUIRE_AUTH, "获取用户信息")
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	// 绑定 JSON 请求体到 LoginRequest 结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailMsg(c, response.InvalidParams, validator.TranslateError(err))
		return
	}

	// 调用 LoginService 的 Login 方法
	user, err := h.LoginService.Login(&req)
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
