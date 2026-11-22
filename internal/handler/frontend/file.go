package frontend

import (
	"errors"
	"mime"
	"net/http"

	"FastGo/internal/handler"
	"FastGo/internal/model"
	"FastGo/internal/router"
	"FastGo/internal/service"
	"FastGo/pkg/jwt"
	"FastGo/pkg/response"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	*handler.CommonHandler
	FileService *service.FileService
}

func NewFileHandler() *FileHandler {
	return &FileHandler{
		CommonHandler: handler.NewCommonHandler(),
		FileService:   service.NewFileService(),
	}
}

func (h *FileHandler) RegisterRoutes(registry *router.RouteRegistry) {
	registry.Register("POST", "common", "/upload", h.Upload, jwt.REQUIRE_AUTH, "通用文件上传")
	registry.Register("GET", "files", "/:file_id", h.Download, jwt.REQUIRE_AUTH, "下载文件")
}

func (h *FileHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxUploadRequestSize)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		response.FailMsg(c, response.InvalidParams, "multipart 请求无效或超过大小限制")
		return
	}

	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if c.Request.MultipartForm == nil || len(c.Request.MultipartForm.File) != 1 ||
		len(c.Request.MultipartForm.File["file"]) != 1 {
		response.FailMsg(c, response.InvalidParams, "必须且只能上传一个 file 文件")
		return
	}

	file, err := h.FileService.Upload(
		c.GetUint("user_id"),
		c.Request.MultipartForm.File["file"][0],
	)
	if err != nil {
		h.respondFileError(c, err)
		return
	}
	response.OK(c, file)
}

func (h *FileHandler) Download(c *gin.Context) {
	file, content, err := h.FileService.OpenForUser(c.GetUint("user_id"), c.Param("file_id"))
	if err != nil {
		h.respondFileError(c, err)
		return
	}
	defer content.Close()

	disposition := "attachment"
	if service.MessageTypeForFile(*file) == model.MessageTypeImage {
		disposition = "inline"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{
		"filename": file.OriginalName,
	}))
	c.Header("Content-Type", file.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, file.Size, file.ContentType, content, nil)
}

func (h *FileHandler) respondFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidFile), errors.Is(err, service.ErrFileTooLarge):
		response.FailMsg(c, response.InvalidParams, err.Error())
	case errors.Is(err, service.ErrFileNotFound):
		response.FailMsg(c, response.NotFound, err.Error())
	case errors.Is(err, service.ErrFileForbidden):
		response.FailMsg(c, response.Forbidden, err.Error())
	default:
		response.FailMsg(c, response.ServerError, "文件处理失败")
	}
}
