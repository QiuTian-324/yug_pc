package auth

import (
	"FastGo/internal/global"
	"FastGo/pkg/jwt"
	"FastGo/pkg/response"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

// 自定义错误
var (
	ErrTokenExpired     = errors.New(response.GetMessage(response.TokenExpired))
	ErrTokenNotValidYet = errors.New(response.GetMessage(response.TokenNotValidYet))
	ErrInvalidToken     = errors.New(response.GetMessage(response.InvalidToken))
)

// TokenAuth Token认证中间件
func TokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		println("token-----: ", token)
		if token == "" {
			response.FailMsg(c, response.Unauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		// 移除 Bearer 前缀
		token = strings.TrimPrefix(token, "Bearer ")

		// 解析 token（密钥必须与登录处一致）
		j := jwt.New(global.Config.Jwt.SigningKey)
		claims, err := j.ParseToken(token)

		// 解析失败必须拦截，否则下方 claims 为 nil 会引发空指针 panic
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				response.FailMsg(c, response.Unauthorized, "Token has expired")
			case errors.Is(err, jwt.ErrTokenNotValidYet):
				response.FailMsg(c, response.InvalidToken, "Token is not valid yet")
			default:
				response.FailMsg(c, response.InvalidToken, "Invalid token")
			}
			c.Abort()
			return
		}

		// 双保险：即使 err 为 nil，也确保 claims 非空
		if claims == nil {
			response.FailMsg(c, response.InvalidToken, "Invalid token")
			c.Abort()
			return
		}

		// 将用户信息存入上下文，供后续 Handler 使用（c.GetUint("user_id")）
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		// c.Set("role", claims.Role)

		c.Next()
	}
}
