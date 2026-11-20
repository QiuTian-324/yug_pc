package response

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构，字段与前端 ApiResponse 对齐
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"` // ← 原 Msg/“msg” 改为 Message/“message”
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success"` // ← 新增：业务是否成功
}

// 成功
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: SUCCESS, Message: GetMessage(SUCCESS), Data: data, Success: true})
}

// 成功，只带提示语
func OKMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{Code: SUCCESS, Message: msg, Success: true})
}

// 失败：用预定义码，消息自动取
func Fail(c *gin.Context, code int) {
	c.JSON(http.StatusOK, Response{Code: code, Message: GetMessage(code), Success: false})
}

// 失败：用预定义码 + 自定义详情（详情放 error 字段）
func FailDetail(c *gin.Context, code int, detail string) {
	c.JSON(http.StatusOK, Response{Code: code, Message: GetMessage(code), Error: detail, Success: false})
}

// 失败：直接给一个错误码 + 自定义消息（最灵活）
func FailMsg(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{Code: code, Message: msg, Success: false})
}

// 兜底：service 返回 error，一行处理
func Err(c *gin.Context, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "1062"), strings.Contains(msg, "Duplicate entry"):
		FailMsg(c, UserExists, "User already exists")
		return
	case strings.Contains(msg, "record not found"):
		FailMsg(c, UserNotFound, "User not found")
		return
	}
	FailDetail(c, ServerError, msg)
}
