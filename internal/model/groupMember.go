package model

import "time"

// 群成员模型
type GroupMember struct {
	ID       uint      `gorm:"primaryKey"`
	GroupID  uint      `gorm:"index:idx_group_member,priority:1;not null" json:"group_id"`
	UserID   uint      `gorm:"index:idx_group_member,priority:2;not null" json:"user_id"`
	Role     uint8     `gorm:"default:0" json:"role"`      // 0普通 1管理员 2群主
	Nickname string    `gorm:"size:100" json:"nickname"`   // 群内昵称
	Muted    bool      `gorm:"default:false" json:"muted"` // 是否被禁言
	JoinedAt time.Time `json:"joined_at"`

	// UNIQUE KEY uk_group_member (group_id, user_id)
}

func (GroupMember) TableName() string {
	return "group_member"
}
