package model

import "time"

// 好友请求
type FriendRequest struct {
	ID         uint   `gorm:"primaryKey"`
	FromUserID uint   `gorm:"index;not null" json:"from_user_id"`
	ToUserID   uint   `gorm:"index;not null" json:"to_user_id"`
	Message    string `gorm:"size:200" json:"message"` // 验证消息
	Status     uint8  `gorm:"default:0" json:"status"` // 0待处理 1已同意 2已拒绝 3已忽略
	CreatedAt  time.Time
	HandledAt  *time.Time `json:"handled_at"`
}

func (FriendRequest) TableName() string {
	return "friend_requests"
}
