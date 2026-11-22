package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxImageSize         = int64(10 << 20)
	maxFileSize          = int64(50 << 20)
	MaxUploadRequestSize = maxFileSize + (1 << 20)
)

var (
	ErrInvalidFile   = errors.New("图片格式无效或文件无效")
	ErrFileTooLarge  = errors.New("文件超过大小限制")
	ErrFileNotFound  = errors.New("文件不存在")
	ErrFileForbidden = errors.New("无权下载该文件")
)

type FileService struct {
	DB      *gorm.DB
	RootDir string
}

type fileFormat struct {
	contentType string
	maxSize     int64
	isImage     bool
}

var imageFormats = map[string]fileFormat{
	".jpg":  {contentType: "image/jpeg", maxSize: maxImageSize, isImage: true},
	".jpeg": {contentType: "image/jpeg", maxSize: maxImageSize, isImage: true},
	".png":  {contentType: "image/png", maxSize: maxImageSize, isImage: true},
	".gif":  {contentType: "image/gif", maxSize: maxImageSize, isImage: true},
	".webp": {contentType: "image/webp", maxSize: maxImageSize, isImage: true},
}

var validatedFileFormats = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".zip":  "application/zip",
}

func NewFileService() *FileService {
	rootDir := ""
	if global.Config != nil {
		rootDir = strings.TrimSpace(global.Config.File.Upload_dir)
	}
	return &FileService{
		DB:      global.GetDB(),
		RootDir: rootDir,
	}
}

func (s *FileService) Upload(userID uint, header *multipart.FileHeader) (*dto.FileVO, error) {
	if userID == 0 || header == nil {
		return nil, ErrInvalidFile
	}

	originalName, err := normalizeFilename(header.Filename)
	if err != nil {
		return nil, err
	}
	extension := strings.ToLower(filepath.Ext(originalName))
	format, ok := imageFormats[extension]
	if !ok {
		contentType, knownFormat := validatedFileFormats[extension]
		if !knownFormat {
			contentType = mime.TypeByExtension(extension)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}
		format = fileFormat{contentType: contentType, maxSize: maxFileSize}
	}
	if header.Size <= 0 {
		return nil, ErrInvalidFile
	}
	if header.Size > format.maxSize {
		return nil, ErrFileTooLarge
	}

	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	sample := make([]byte, 512)
	n, readErr := io.ReadFull(src, sample)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, readErr
	}
	if format.isImage && !validImageSignature(extension, format.contentType, sample[:n]) {
		return nil, ErrInvalidFile
	}
	if contentType, knownFormat := validatedFileFormats[extension]; knownFormat &&
		!validFileSignature(extension, contentType, sample[:n]) {
		return nil, ErrInvalidFile
	}
	fileHash, hashedSize, err := hashUpload(src, format.maxSize)
	if err != nil {
		return nil, err
	}

	var existing model.File
	err = s.DB.Where("uploader_id = ? AND sha256 = ?", userID, fileHash).First(&existing).Error
	if err == nil {
		return s.fileVOIfPresent(existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	rootDir, err := s.uploadRootDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建文件目录失败: %w", err)
	}
	id := uuid.NewString()
	storageName := id + extension
	storagePath := filepath.Join(rootDir, storageName)
	dst, err := os.OpenFile(storagePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}

	size, copyErr := io.Copy(dst, io.LimitReader(src, format.maxSize+1))
	if copyErr == nil && size > format.maxSize {
		copyErr = ErrFileTooLarge
	}
	if copyErr == nil && size == 0 {
		copyErr = ErrInvalidFile
	}
	if copyErr == nil && size != hashedSize {
		copyErr = errors.New("上传文件内容在校验后发生变化")
	}
	if copyErr == nil {
		copyErr = dst.Sync()
	}
	closeErr := dst.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return nil, cleanupFile(storagePath, copyErr)
	}

	file := model.File{
		ID:           id,
		UploaderID:   userID,
		SHA256:       &fileHash,
		OriginalName: originalName,
		StorageName:  storageName,
		FilePath:     storagePath,
		ContentType:  format.contentType,
		Size:         size,
	}
	if err := s.DB.Create(&file).Error; err != nil {
		if duplicate, lookupErr := s.findFileByHash(userID, fileHash); lookupErr == nil {
			if cleanupErr := os.Remove(storagePath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return nil, errors.Join(err, fmt.Errorf("清理重复文件失败: %w", cleanupErr))
			}
			return s.fileVOIfPresent(duplicate)
		}
		return nil, cleanupFile(storagePath, err)
	}

	return fileToVO(file), nil
}

func (s *FileService) uploadRootDir() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	projectRoot, err := findProjectRoot(workingDir)
	if err != nil {
		return "", err
	}
	configuredDir := strings.TrimSpace(s.RootDir)
	if configuredDir == "" {
		return filepath.Join(projectRoot, "uploads", "files"), nil
	}

	configuredDir = filepath.Clean(strings.TrimLeft(configuredDir, `/\`))
	if configuredDir == "." || configuredDir == "" {
		return "", errors.New("file.upload_dir 不能为空")
	}
	if firstComponent := strings.SplitN(configuredDir, string(filepath.Separator), 2)[0]; firstComponent == filepath.Base(projectRoot) {
		configuredDir = strings.TrimPrefix(configuredDir, firstComponent+string(filepath.Separator))
	}
	if configuredDir == "" || configuredDir == "." {
		return "", errors.New("file.upload_dir 必须指定项目内的存储目录")
	}

	rootDir := filepath.Join(projectRoot, configuredDir)
	relativePath, err := filepath.Rel(projectRoot, rootDir)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", errors.New("file.upload_dir 必须位于项目目录内")
	}
	return rootDir, nil
}

func findProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("解析工作目录失败: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("检查项目根目录失败: %w", statErr)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("无法定位项目根目录，请配置 file.upload_dir")
		}
		dir = parent
	}
}

func (s *FileService) OpenForUser(userID uint, fileID string) (*model.File, *os.File, error) {
	if userID == 0 {
		return nil, nil, ErrFileForbidden
	}
	parsedID, idErr := uuid.Parse(fileID)
	if idErr != nil || parsedID.String() != fileID {
		return nil, nil, ErrFileNotFound
	}

	var file model.File
	if err := s.DB.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, err
	}
	if file.UploaderID != userID {
		var messageCount int64
		fileRef := fmt.Sprintf(`%%"file_id":"%s"%%`, file.ID)
		if err := s.DB.Model(&model.Message{}).
			Where("sender_id = ? AND receiver_id = ? AND type IN ? AND content LIKE ?",
				file.UploaderID, userID, []uint8{model.MessageTypeImage, model.MessageTypeFile}, fileRef).
			Count(&messageCount).Error; err != nil {
			return nil, nil, err
		}
		if messageCount == 0 {
			return nil, nil, ErrFileForbidden
		}
	}
	if err := validateStoredFilePath(file); err != nil {
		return nil, nil, ErrInvalidFile
	}

	handle, err := os.Open(file.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrFileNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &file, handle, nil
}

func (s *FileService) findFileByHash(userID uint, fileHash string) (model.File, error) {
	var file model.File
	err := s.DB.Where("uploader_id = ? AND sha256 = ?", userID, fileHash).First(&file).Error
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		return file, err
	}

	var unindexedFiles []model.File
	if err := s.DB.Where("uploader_id = ? AND sha256 IS NULL", userID).
		Order("created_at ASC").Find(&unindexedFiles).Error; err != nil {
		return model.File{}, err
	}
	for _, candidate := range unindexedFiles {
		candidateHash, size, hashErr := hashStoredFile(candidate)
		if errors.Is(hashErr, os.ErrNotExist) {
			continue
		}
		if hashErr != nil {
			return model.File{}, hashErr
		}
		if size != candidate.Size {
			continue
		}

		if err := s.DB.Model(&model.File{}).
			Where("id = ? AND sha256 IS NULL", candidate.ID).
			Update("sha256", candidateHash).Error; err != nil {
			var winner model.File
			if lookupErr := s.DB.Where("uploader_id = ? AND sha256 = ?", userID, candidateHash).
				First(&winner).Error; lookupErr == nil {
				if candidateHash == fileHash {
					return winner, nil
				}
				continue
			}
			return model.File{}, err
		}

		if candidateHash == fileHash {
			return candidate, nil
		}
	}
	return model.File{}, gorm.ErrRecordNotFound
}

func (s *FileService) fileVOIfPresent(file model.File) (*dto.FileVO, error) {
	if err := validateStoredFilePath(file); err != nil {
		return nil, ErrInvalidFile
	}
	info, err := os.Stat(file.FilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalidFile
	}
	return fileToVO(file), nil
}

func hashStoredFile(file model.File) (string, int64, error) {
	if err := validateStoredFilePath(file); err != nil {
		return "", 0, err
	}
	handle, err := os.Open(file.FilePath)
	if err != nil {
		return "", 0, err
	}
	defer handle.Close()
	return hashUpload(handle, file.Size)
}

func validateStoredFilePath(file model.File) error {
	expectedStorageName := file.ID + strings.ToLower(filepath.Ext(file.OriginalName))
	if (file.StorageName != file.ID && file.StorageName != expectedStorageName) ||
		filepath.Base(file.FilePath) != file.StorageName || !filepath.IsAbs(file.FilePath) {
		return ErrInvalidFile
	}
	return nil
}

func fileToVO(file model.File) *dto.FileVO {
	return &dto.FileVO{
		FileID:      file.ID,
		MessageType: MessageTypeForFile(file),
		FileName:    file.OriginalName,
		ContentType: file.ContentType,
		Size:        file.Size,
		URL:         "/files/" + file.ID,
	}
}

func hashUpload(src io.ReadSeeker, maxSize int64) (string, int64, error) {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(src, maxSize+1))
	if err != nil {
		return "", 0, err
	}
	if size > maxSize {
		return "", 0, ErrFileTooLarge
	}
	if size == 0 {
		return "", 0, ErrInvalidFile
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func MessageTypeForFile(file model.File) uint8 {
	switch file.ContentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return model.MessageTypeImage
	default:
		return model.MessageTypeFile
	}
}

func FileMessageContent(file model.File) (string, error) {
	content, err := json.Marshal(struct {
		FileID      string `json:"file_id"`
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
		URL         string `json:"url"`
	}{
		FileID:      file.ID,
		FileName:    file.OriginalName,
		ContentType: file.ContentType,
		Size:        file.Size,
		URL:         "/files/" + file.ID,
	})
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func normalizeFilename(filename string) (string, error) {
	filename = strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, "\\", "/")))
	if filename == "" || !utf8.ValidString(filename) {
		return "", ErrInvalidFile
	}

	rawExtension := filepath.Ext(filename)
	extension := strings.ToLower(sanitizeFilenamePart(rawExtension))
	name := sanitizeFilenamePart(strings.TrimSuffix(filename, rawExtension))
	baseName := strings.Trim(name, " ._-")
	if baseName == "" {
		baseName = "file"
	}
	if len(extension) > 255 {
		return "", ErrInvalidFile
	}
	maxNameBytes := 255 - len(extension)
	baseName = truncateUTF8(baseName, maxNameBytes)
	if baseName == "" {
		return "", ErrInvalidFile
	}
	return baseName + extension, nil
}

func sanitizeFilenamePart(value string) string {
	var safeName strings.Builder
	lastUnderscore := false
	for _, character := range value {
		allowed := unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsMark(character) ||
			strings.ContainsRune("._-()[] ", character)
		if !allowed || unicode.IsControl(character) {
			character = '_'
		}
		if unicode.IsSpace(character) {
			character = '_'
		}
		if character == '_' {
			if lastUnderscore {
				continue
			}
			lastUnderscore = true
		} else {
			lastUnderscore = false
		}
		safeName.WriteRune(character)
	}
	return safeName.String()
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	for maxBytes > 0 && !utf8.RuneStart(value[maxBytes]) {
		maxBytes--
	}
	return value[:maxBytes]
}

func cleanupFile(path string, primaryErr error) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(primaryErr, fmt.Errorf("清理未完成的文件失败: %w", err))
	}
	return primaryErr
}

func validImageSignature(extension, contentType string, sample []byte) bool {
	if extension == ".webp" {
		return len(sample) >= 12 &&
			string(sample[:4]) == "RIFF" &&
			string(sample[8:12]) == "WEBP"
	}
	return http.DetectContentType(sample) == contentType
}

func validFileSignature(extension, contentType string, sample []byte) bool {
	switch extension {
	case ".pdf":
		return bytes.HasPrefix(sample, []byte("%PDF-"))
	case ".zip", ".docx", ".xlsx", ".pptx":
		return bytes.HasPrefix(sample, []byte("PK\x03\x04")) ||
			(extension == ".zip" && bytes.HasPrefix(sample, []byte("PK\x05\x06")))
	case ".doc", ".xls", ".ppt":
		return bytes.HasPrefix(sample, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	case ".txt", ".csv":
		return !bytes.Contains(sample, []byte{0})
	default:
		return contentType != ""
	}
}
