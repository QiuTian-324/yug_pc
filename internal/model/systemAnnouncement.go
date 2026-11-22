package model

import "time"

const (
	SystemAnnouncementStatusDraft     uint8 = 0
	SystemAnnouncementStatusPublished uint8 = 1
	SystemAnnouncementStatusWithdrawn uint8 = 2
)

// SystemAnnouncement 全员系统公告；不记录用户级已读状态。
type SystemAnnouncement struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"size:200;not null" json:"title"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	Status      uint8      `gorm:"default:0;index:idx_announcements_status_published,priority:1" json:"status"`
	PublisherID *uint      `json:"publisher_id,omitempty"`
	PublishedAt *time.Time `gorm:"index:idx_announcements_status_published,priority:2" json:"published_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SystemAnnouncement) TableName() string {
	return "system_announcements"
}
