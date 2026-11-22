package dto

import (
	"encoding/json"
	"time"
)

type SendMessageRequest struct {
	TargetID    uint   `json:"target_id"`
	ClientMsgID string `json:"client_msg_id"`
	MessageType uint8  `json:"message_type"`
	Content     string `json:"content"`
}

type MessageHistoryRequest struct {
	TargetID       uint   `form:"target_id" binding:"required"`
	BeforeSequence uint64 `form:"before_sequence"`
	Limit          int    `form:"limit"`
}

type MessageSyncRequest struct {
	TargetID      uint   `form:"target_id" binding:"required"`
	AfterSequence uint64 `form:"after_sequence"`
	Limit         int    `form:"limit"`
}

type MessageVO struct {
	ID             uint      `json:"id"`
	ConvKey        string    `json:"conv_key"`
	Sequence       uint64    `json:"sequence"`
	SenderID       uint      `json:"sender_id"`
	ReceiverID     uint      `json:"receiver_id"`
	MessageType    uint8     `json:"message_type"`
	Content        string    `json:"content"`
	SenderNickname string    `json:"sender_nickname"`
	SenderAvatar   string    `json:"sender_avatar"`
	Status         uint8     `json:"status"`
	ClientMsgID    string    `json:"client_msg_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type MessagePage struct {
	List       []MessageVO `json:"list"`
	HasMore    bool        `json:"has_more"`
	NextCursor uint64      `json:"next_cursor"`
}

type WebSocketTicketVO struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

type WSClientEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type WSServerEnvelope struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
	Code      int    `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}
