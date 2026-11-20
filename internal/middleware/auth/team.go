package auth

import (
	"FastGo/pkg/response"

	"github.com/gin-gonic/gin"
)

// TeamAuth 团队权限验证中间件
func TeamAuth(requiredLevel uint8) gin.HandlerFunc {
	return func(c *gin.Context) {
		TokenAuth()(c)
		if c.IsAborted() {
			return
		}

		role, exists := c.Get("role")
		if !exists {
			response.FailMsg(c, response.Unauthorized, "Role is required")
			c.Abort()
			return
		}

		userRole := role.(uint8)
		if userRole < requiredLevel {
			response.FailMsg(c, response.Forbidden, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
