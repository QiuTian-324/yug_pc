// dto/user_dto.go
package dto

import "time"

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string     `json:"token"`
	UserInfo UserInfoVO `json:"user_info"`
}

// UserInfoVO 用户信息展示对象
type UserInfoVO struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Bio       string    `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	ApiKey      string `json:"api_key"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}
