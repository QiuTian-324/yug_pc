package model

import "time"

// Friend 好友关系模型
type Friendship struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index:idx_user_friend,priority:1;not null" json:"user_id"`
	FriendID  uint   `gorm:"index:idx_user_friend,priority:2;not null" json:"friend_id"`
	Remark    string `gorm:"size:100" json:"remark"`  // 备注名
	GroupID   uint   `gorm:"index" json:"group_id"`   // 好友分组
	Status    uint8  `gorm:"default:1" json:"status"` // 1正常 2已拉黑
	CreatedAt time.Time

	// 联合唯一索引，防止重复添加
	// UNIQUE KEY uk_user_friend (user_id, friend_id)
}

func (Friendship) TableName() string {
	return "user_friends"
}
