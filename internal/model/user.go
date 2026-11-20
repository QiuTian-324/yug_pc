package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Username  string `gorm:"uniqueIndex;size:100;not null" json:"username"`
	Password  string `gorm:"size:255;not null" json:"-"`
	Nickname  string `gorm:"size:100" json:"nickname"`
	Avatar    string `gorm:"size:255" json:"avatar"`
	Bio       string `gorm:"size:500" json:"bio"`
	Status    uint8  `gorm:"default:1" json:"status"` // 1正常 0禁用
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
