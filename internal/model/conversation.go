package model

// 会话模型
type Conversation struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `json:"user_id"`      // 所属用户
	TargetID    uint   `json:"target_id"`    // 对方 ID
	TargetType  int    `json:"target_type"`  // 1=单聊 2=群聊
	LastMessage string `json:"last_message"` // 冗余最后消息，避免每次查消息表
	LastTime    int64  `json:"last_time"`    // 冗余最后时间，用于排序
	UnreadCount int    `json:"unread_count"` // 该用户的未读数
	IsPinned    bool   `json:"is_pinned"`    // 该用户的置顶设置
	IsMuted     bool   `json:"is_muted"`     // 该用户的免打扰设置
}

func (Conversation) TableName() string {
	return "conversation"
}
