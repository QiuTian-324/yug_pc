package model

import (
	"time"

	"gorm.io/gorm"
)

type Group struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Avatar      string `gorm:"size:255" json:"avatar"`
	OwnerID     uint   `gorm:"index;not null" json:"owner_id"` // 群主
	Notice      string `gorm:"size:1000" json:"notice"`        // 群公告
	MemberCount int    `gorm:"default:0" json:"member_count"`  // 冗余计数，避免 COUNT(*)
	MaxMembers  int    `gorm:"default:500" json:"max_members"`
	Status      uint8  `gorm:"default:1" json:"status"` // 1正常 2已解散
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (Group) TableName() string {
	return "groups"
}
