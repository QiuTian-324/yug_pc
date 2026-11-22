package model

// 会话模型
type Conversation struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        uint   `gorm:"not null;uniqueIndex:uk_user_target,priority:1" json:"user_id"`
	TargetID      uint   `gorm:"not null;uniqueIndex:uk_user_target,priority:3" json:"target_id"`
	TargetType    int    `gorm:"not null;uniqueIndex:uk_user_target,priority:2" json:"target_type"` // 1=单聊 2=群聊
	LastMessage   string `json:"last_message"`                                                      // 冗余最后消息，避免每次查消息表
	LastMessageID uint   `gorm:"default:0" json:"-"`
	LastSequence  uint64 `gorm:"default:0" json:"last_sequence"`
	LastTime      int64  `json:"last_time"` // 毫秒时间戳，用于排序
	UnreadCount   int    `gorm:"default:0" json:"unread_count"`
	IsPinned      bool   `gorm:"default:false" json:"is_pinned"`
	IsMuted       bool   `gorm:"default:false" json:"is_muted"`
}

func (Conversation) TableName() string {
	return "conversation"
}
