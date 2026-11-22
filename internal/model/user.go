package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PresenceModeOnline    uint8 = 1
	PresenceModeInvisible uint8 = 2
)

// User 用户模型
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"uniqueIndex;size:100;not null" json:"username"`
	Password     string     `gorm:"size:255;not null" json:"-"`
	Nickname     string     `gorm:"size:100" json:"nickname"`
	Gender       *uint8     `gorm:"type:tinyint unsigned" json:"gender,omitempty"`
	Birthday     *time.Time `gorm:"type:date" json:"birthday,omitempty"`
	Avatar       string     `gorm:"size:255" json:"avatar"`
	Bio          string     `gorm:"size:500" json:"bio"`
	PresenceMode uint8      `gorm:"not null;default:1" json:"-"`
	Status       uint8      `gorm:"default:1" json:"status"` // 1正常 0禁用
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
