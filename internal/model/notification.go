package model

import "time"

const (
	NotificationTypeFriendRequestReceived uint8 = 1 // 好友请求
	NotificationTypeFriendRequestAccepted uint8 = 2 // 好友请求被接受
	NotificationTypeFriendRequestRejected uint8 = 3 // 好友请求被拒绝

	NotificationTypeGroupInvitationReceived uint8 = 10 // 群组邀请
	NotificationTypeGroupInvitationAccepted uint8 = 11 //	群组邀请被接受
	NotificationTypeGroupInvitationRejected uint8 = 12 // 群组邀请被拒绝
	NotificationTypeGroupJoinAccepted       uint8 = 13 //	群组加入被接受
	NotificationTypeGroupJoinRejected       uint8 = 14 //	群组加入被拒绝
	NotificationTypeGroupRemoved            uint8 = 15 //	群组被移除
	NotificationTypeGroupRoleChanged        uint8 = 16 // 群组角色改变
)

// Notification 用户级好友和群组通知；Payload 使用 JSON 字符串保存事件扩展数据。
// 系统公告单独存储，不按用户复制。
type Notification struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	RecipientUserID uint       `gorm:"not null;index:idx_notifications_user_created,priority:1;index:idx_notifications_user_read,priority:1" json:"recipient_user_id"`
	Type            uint8      `gorm:"not null" json:"type"`
	ActorUserID     *uint      `gorm:"index" json:"actor_user_id,omitempty"`
	FriendRequestID *uint      `gorm:"index" json:"friend_request_id,omitempty"`
	GroupID         *uint      `gorm:"index" json:"group_id,omitempty"`
	Payload         string     `gorm:"type:text" json:"payload,omitempty"`
	ReadAt          *time.Time `gorm:"index:idx_notifications_user_read,priority:2" json:"read_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index:idx_notifications_user_created,priority:2" json:"created_at"`
}

func (Notification) TableName() string {
	return "notifications"
}
