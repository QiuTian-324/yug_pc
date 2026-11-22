package model

import "time"

// FriendRequest 好友请求
type FriendRequest struct {
	ID         uint   `gorm:"primaryKey"`
	FromUserID uint   `gorm:"index;not null" json:"from_user_id"`
	ToUserID   uint   `gorm:"index;not null" json:"to_user_id"`
	Message    string `gorm:"size:200" json:"message"`
	Status     uint8  `gorm:"default:0" json:"status"` // 0待处理 1已同意 2已拒绝 3已忽略
	CreatedAt  time.Time
	HandledAt  *time.Time `json:"handled_at"`

	// 关联申请人
	FromUser *User `gorm:"foreignKey:FromUserID;references:ID" json:"from_user,omitempty"`
}

func (FriendRequest) TableName() string {
	return "friend_requests"
}
