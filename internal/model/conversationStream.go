package model

// ConversationStream 为每个会话维护单调递增的服务端消息序号。
type ConversationStream struct {
	ID           uint   `gorm:"primaryKey"`
	ConvKey      string `gorm:"size:64;not null;uniqueIndex"`
	LastSequence uint64 `gorm:"not null;default:0"`
}

func (ConversationStream) TableName() string {
	return "conversation_streams"
}
