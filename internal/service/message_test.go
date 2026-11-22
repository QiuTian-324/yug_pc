package service

import (
	"testing"

	"FastGo/internal/model"
)

func TestMessagePreview(t *testing.T) {
	tests := []struct {
		name    string
		message model.Message
		want    string
	}{
		{
			name:    "text",
			message: model.Message{Type: model.MessageTypeText, Content: "hello"},
			want:    "hello",
		},
		{
			name:    "image",
			message: model.Message{Type: model.MessageTypeImage, Content: `{"file_id":"id"}`},
			want:    "[图片]",
		},
		{
			name:    "file",
			message: model.Message{Type: model.MessageTypeFile, Content: `{"file_id":"id"}`},
			want:    "[文件]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messagePreview(tt.message); got != tt.want {
				t.Fatalf("messagePreview() = %q, want %q", got, tt.want)
			}
		})
	}
}
