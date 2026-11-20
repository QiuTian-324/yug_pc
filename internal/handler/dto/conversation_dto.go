package dto

// 会话列表返回项：联表补好展示信息，前端直接渲染
type ConversationVO struct {
	ID          uint   `json:"id"`
	TargetID    uint   `json:"target_id"`
	TargetType  int    `json:"target_type"`
	ShowName    string `json:"show_name"`
	Avatar      string `json:"avatar"`
	LastMessage string `json:"last_message"`
	LastTime    int64  `json:"last_time"`
	UnreadCount int    `json:"unread_count"`
	IsPinned    bool   `json:"is_pinned"`
	IsMuted     bool   `json:"is_muted"`
}

// 定位某条会话的通用入参
type ConversationTargetReq struct {
	TargetID   uint `json:"target_id"   binding:"required"`
	TargetType int  `json:"target_type" binding:"required,oneof=1 2"`
}

// 设置置顶/免打扰
type SetPinMuteReq struct {
	ConversationTargetReq
	IsPinned *bool `json:"is_pinned"` // 指针：区分“未传”与“传 false”
	IsMuted  *bool `json:"is_muted"`
}
