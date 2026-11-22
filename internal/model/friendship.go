package model

import "time"

// Friendship 好友关系模型
type Friendship struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index:idx_user_friend,priority:1;not null" json:"user_id"`
	FriendID  uint   `gorm:"index:idx_user_friend,priority:2;not null" json:"friend_id"`
	Remark    string `gorm:"size:100" json:"remark"`
	GroupID   uint   `gorm:"index" json:"group_id"`
	Status    uint8  `gorm:"default:1" json:"status"` // 1正常 2已拉黑
	CreatedAt time.Time

	// 关联用户表，用于预加载好友信息
	FriendUser *User `gorm:"foreignKey:FriendID;references:ID" json:"friend_user,omitempty"`
}

func (Friendship) TableName() string {
	return "user_friends"
}
