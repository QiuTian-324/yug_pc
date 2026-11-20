package model

import "time"

// 消息
type Message struct {
	ID uint `gorm:"primaryKey"`
	// 会话标识：单聊用 "1_100_200"（小ID在前），群聊用 "2_群ID"
	ConvKey string `gorm:"size:64;index:idx_conv_id,priority:1;not null" json:"conv_key"`

	ConversationID uint `gorm:"index" json:"conversation_id"`

	SenderID   uint `gorm:"index;not null" json:"sender_id"`
	ReceiverID uint `json:"receiver_id"`           // 单聊时=接收者，群聊时=0
	GroupID    uint `gorm:"index" json:"group_id"` // 群聊时=群ID，单聊时=0

	Type    uint8  `gorm:"default:1" json:"type"`    // 1文本 2图片 3语音 4文件 5系统 6撤回
	Content string `gorm:"type:text" json:"content"` // 文本内容或 JSON（图片URL等）

	// 冗余发送者信息，避免 JOIN
	SenderNickname string `gorm:"size:100" json:"sender_nickname"`
	SenderAvatar   string `gorm:"size:255" json:"sender_avatar"`

	Status      uint8  `gorm:"default:1" json:"status"`            // 1已发送 2已送达 3已读 4撤回 5失败
	ClientMsgID string `gorm:"size:64;index" json:"client_msg_id"` // 客户端幂等ID，防重复发送

	CreatedAt time.Time `gorm:"index:idx_conv_id,priority:2"`

	// KEY idx_conv_id (conv_key, id) —— 拉历史消息的核心索引
}

func (Message) TableName() string {
	return "message"
}
