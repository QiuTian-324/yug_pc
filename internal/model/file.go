package model

import "time"

type File struct {
	ID           string    `gorm:"primaryKey;size:36"`
	UploaderID   uint      `gorm:"not null;index;uniqueIndex:uidx_files_uploader_sha256,priority:1"`
	SHA256       *string   `gorm:"column:sha256;size:64;uniqueIndex:uidx_files_uploader_sha256,priority:2"`
	OriginalName string    `gorm:"size:255;not null"`
	StorageName  string    `gorm:"size:255;not null;uniqueIndex"`
	FilePath     string    `gorm:"type:text;not null"`
	ContentType  string    `gorm:"size:128;not null"`
	Size         int64     `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (File) TableName() string {
	return "files"
}
