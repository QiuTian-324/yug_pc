package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"FastGo/config"
	"FastGo/internal/global"
	"FastGo/internal/model"
)

func TestValidFileSignatures(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		content   string
		sample    []byte
		want      bool
	}{
		{
			name:      "PNG image",
			extension: ".png",
			content:   "image/png",
			sample:    []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
			want:      true,
		},
		{
			name:      "image extension with PDF content",
			extension: ".png",
			content:   "image/png",
			sample:    []byte("%PDF-1.7"),
			want:      false,
		},
		{
			name:      "PDF document",
			extension: ".pdf",
			content:   "application/pdf",
			sample:    []byte("%PDF-1.7"),
			want:      true,
		},
		{
			name:      "binary text file",
			extension: ".txt",
			content:   "text/plain",
			sample:    []byte{'a', 0, 'b'},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if _, isImage := imageFormats[tt.extension]; isImage {
				got = validImageSignature(tt.extension, tt.content, tt.sample)
			} else {
				got = validFileSignature(tt.extension, tt.content, tt.sample)
			}
			if got != tt.want {
				t.Fatalf("signature validation = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "removes path and normalizes whitespace",
			input: `C:\Users\alice\我的 图片.PNG`,
			want:  "我的_图片.png",
		},
		{
			name:  "removes control and header characters",
			input: "hello\nX: bad.PDF",
			want:  "hello_X_bad.pdf",
		},
		{
			name:    "rejects empty name",
			input:   "  ",
			wantErr: true,
		},
		{
			name:  "truncates to filesystem-safe byte length",
			input: strings.Repeat("界", 100) + ".txt",
			want:  strings.Repeat("界", 83) + ".txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeFilename(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeFilename() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("normalizeFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileMessageContent(t *testing.T) {
	file := model.File{
		ID:           "file-id",
		OriginalName: "report.pdf",
		ContentType:  "application/pdf",
		Size:         128,
	}

	content, err := FileMessageContent(file)
	if err != nil {
		t.Fatalf("FileMessageContent() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if got["file_id"] != file.ID || got["file_name"] != file.OriginalName ||
		got["content_type"] != file.ContentType || got["url"] != "/files/"+file.ID ||
		got["size"] != float64(file.Size) {
		t.Fatalf("FileMessageContent() = %s", content)
	}
}

func TestHashUpload(t *testing.T) {
	content := []byte("same file content")
	wantHashBytes := sha256.Sum256(content)

	hash, size, err := hashUpload(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("hashUpload() error = %v", err)
	}
	if hash != hex.EncodeToString(wantHashBytes[:]) || size != int64(len(content)) {
		t.Fatalf("hashUpload() = (%q, %d), want (%q, %d)",
			hash, size, hex.EncodeToString(wantHashBytes[:]), len(content))
	}

	if _, _, err := hashUpload(bytes.NewReader(content), int64(len(content)-1)); err != ErrFileTooLarge {
		t.Fatalf("hashUpload() oversized error = %v, want %v", err, ErrFileTooLarge)
	}
}

func TestFindProjectRootFromCmdDirectory(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot, err := findProjectRoot(workingDir)
	if err != nil {
		t.Fatalf("findProjectRoot() error = %v", err)
	}

	got, err := findProjectRoot(filepath.Join(projectRoot, "cmd"))
	if err != nil {
		t.Fatalf("findProjectRoot(cmd) error = %v", err)
	}
	if got != projectRoot {
		t.Fatalf("findProjectRoot(cmd) = %q, want %q", got, projectRoot)
	}
}

func TestDefaultUploadRootUsesProjectRoot(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot, err := findProjectRoot(workingDir)
	if err != nil {
		t.Fatalf("findProjectRoot() error = %v", err)
	}
	root, err := (&FileService{}).uploadRootDir()
	if err != nil {
		t.Fatalf("uploadRootDir() error = %v", err)
	}
	want := filepath.Join(projectRoot, "uploads", "files")
	if root != want {
		t.Fatalf("uploadRootDir() = %q, want %q", root, want)
	}
}

func TestConfiguredUploadRootUsesProjectRoot(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot, err := findProjectRoot(workingDir)
	if err != nil {
		t.Fatalf("findProjectRoot() error = %v", err)
	}

	root, err := (&FileService{
		RootDir: filepath.Join(string(filepath.Separator), filepath.Base(projectRoot), "uploads", "files"),
	}).uploadRootDir()
	if err != nil {
		t.Fatalf("uploadRootDir() error = %v", err)
	}
	want := filepath.Join(projectRoot, "uploads", "files")
	if root != want {
		t.Fatalf("uploadRootDir() = %q, want %q", root, want)
	}
}

func TestNewFileServiceUsesGlobalUploadDirectory(t *testing.T) {
	previousConfig := global.Config
	global.Config = &config.Options{
		File: config.FileOptions{Upload_dir: "/yug-im-server/uploads/files"},
	}
	defer func() {
		global.Config = previousConfig
	}()

	if got := NewFileService().RootDir; got != "/yug-im-server/uploads/files" {
		t.Fatalf("NewFileService().RootDir = %q, want configured upload directory", got)
	}
}
