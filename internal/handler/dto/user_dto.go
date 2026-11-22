// dto/user_dto.go
package dto

import "time"

const (
	FriendStatusNormal  uint8 = 1
	FriendStatusBlocked uint8 = 2

	FriendRequestStatusPending  uint8 = 0
	FriendRequestStatusAccepted uint8 = 1
	FriendRequestStatusRejected uint8 = 2
)

// FriendListRequest 好友列表查询参数
type FriendListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// FriendRequestListRequest 好友申请列表查询参数
type FriendRequestListRequest struct {
	Cursor uint `form:"cursor"`
	Limit  int  `form:"limit"`
}

// SearchUserRequest 按用户名查找用户
type SearchUserRequest struct {
	Username string `form:"username" binding:"required,max=100"`
}

// SendFriendRequestRequest 发起好友申请
type SendFriendRequestRequest struct {
	ToUserID uint   `json:"to_user_id" binding:"required"`
	Message  string `json:"message" binding:"omitempty,max=200"`
}

// HandleFriendRequestRequest 处理好友申请
type HandleFriendRequestRequest struct {
	RequestID uint `json:"request_id" binding:"required"`
}

// FriendRequestItemVO 好友申请列表项
type FriendRequestItemVO struct {
	RequestID  uint          `json:"request_id"`
	FromUserID uint          `json:"from_user_id"`
	FromUser   *FriendUserVO `json:"from_user,omitempty"`
	Message    string        `json:"message"`
	CreatedAt  time.Time     `json:"created_at"`
}

// FriendRequestListResponse 好友申请列表响应
type FriendRequestListResponse struct {
	List       []FriendRequestItemVO `json:"list"`
	HasMore    bool                  `json:"has_more"`
	NextCursor *uint                 `json:"next_cursor,omitempty"`
}

// FriendUserVO 好友基础信息
type FriendUserVO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Bio      string `json:"bio"`
}

// FriendItemVO 好友列表项
type FriendItemVO struct {
	FriendshipID uint         `json:"friendship_id"`
	Remark       string       `json:"remark"`
	GroupID      uint         `json:"group_id"`
	CreatedAt    time.Time    `json:"created_at"`
	IsOnline     bool         `json:"is_online"`
	User         FriendUserVO `json:"user"`
}

// FriendListResponse 好友列表响应
type FriendListResponse struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	List     []FriendItemVO `json:"list"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string     `json:"token"`
	UserInfo UserInfoVO `json:"user_info"`
}

// UserInfoVO 用户信息展示对象
type UserInfoVO struct {
	ID           uint      `json:"id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Gender       *uint8    `json:"gender,omitempty"`
	Birthday     string    `json:"birthday,omitempty"`
	PresenceMode string    `json:"presence_mode"`
	Avatar       string    `json:"avatar"`
	Bio          string    `json:"bio"`
	CreatedAt    time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,username"`
	Password string `json:"password" binding:"required,password"`
	Nickname string `json:"nickname" binding:"required,max=100"`
	Gender   uint8  `json:"gender" binding:"required,oneof=1 2"`
	Birthday string `json:"birthday" binding:"required,date"`
}

type SetPresenceRequest struct {
	Mode string `json:"mode" binding:"required,oneof=online invisible"`
}

type PresenceVO struct {
	Mode     string `json:"mode"`
	IsOnline bool   `json:"is_online"`
}

type PresenceChangedEvent struct {
	UserID   uint `json:"user_id"`
	IsOnline bool `json:"is_online"`
}

// 登录请求
type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	ApiKey      string `json:"api_key"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}
