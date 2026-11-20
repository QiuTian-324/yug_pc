package service

import (
	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"
	"FastGo/utils"
	"errors"

	"gorm.io/gorm"
)

// LoginService 提供用户登录相关的业务逻辑
type LoginService struct {
	DB *gorm.DB
}

// NewLoginService 创建一个新的 LoginService 实例
func NewLoginService() *LoginService {
	return &LoginService{DB: global.GetDB()}
}

// service/login.go —— 不再接收 gin.Context，入参用 *dto.LoginRequest
func (s *LoginService) Login(req *dto.LoginRequest) (*model.User, error) {
	if req.Username == "" && req.Email == "" && req.PhoneNumber == "" && req.ApiKey == "" {
		return nil, errors.New("必须提供用户名、邮箱、手机号或 API Key 之一")
	}

	var user model.User
	var err error

	switch {
	case req.ApiKey != "":
		if err = s.DB.Where("api_key = ?", req.ApiKey).First(&user).Error; err != nil {
			return nil, errors.New("无效的 API Key")
		}
		return &user, nil // API Key 登录免密
	case req.Username != "":
		err = s.DB.Where("username = ?", req.Username).First(&user).Error
	case req.Email != "":
		err = s.DB.Where("email = ?", req.Email).First(&user).Error
	case req.PhoneNumber != "":
		err = s.DB.Where("phone_number = ?", req.PhoneNumber).First(&user).Error
	default:
		return nil, errors.New("无效的登录方式")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("用户不存在") // 账号密码错统一回一句，不泄露账号是否存在
	}
	if err != nil {
		return nil, err
	}

	if req.Password == "" {
		return nil, errors.New("密码不能为空")
	}
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	return &user, nil
}
