package dto

type FileVO struct {
	FileID      string `json:"file_id"`
	MessageType uint8  `json:"message_type"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}
