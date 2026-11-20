package auth

import (
	"FastGo/pkg/response"
	"crypto/subtle"
	"errors"

	"github.com/gin-gonic/gin"
)

func ApiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			response.FailMsg(c, response.Unauthorized, "API Key is required")
			c.Abort()
			return
		}

		userID, err := validateApiKey(apiKey)
		if err != nil {
			// 验证失败
			response.FailMsg(c, response.Unauthorized, "Invalid API Key")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		// 注意：ApiKeyAuth 不设置 role，若后续链中有 TeamAuth，
		// TeamAuth 的安全断言会捕获此情况并返回错误，符合预期
		c.Next()
	}
}

// validateApiKey 示例实现
func validateApiKey(apiKey string) (uint, error) {
	// 【防时序攻击】使用常量时间比较，防止通过响应时间推测有效 Key
	// 实际项目中应从数据库/Redis 查询，并对查询结果做常量时间比较
	validKey := "your-valid-api-key" // 示例
	if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
		return 1, nil // 返回对应 userID
	}
	return 0, errors.New("invalid api key")
}
