package jwt

import (
	"FastGo/pkg/response"
	"errors"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v4"
)

var (
	ErrTokenExpired     = errors.New(response.GetMessage(response.TokenExpired))
	ErrTokenNotValidYet = errors.New(response.GetMessage(response.TokenNotValidYet))
	ErrInvalidToken     = errors.New(response.GetMessage(response.InvalidToken))
)

// CustomClaims 精简：只放身份标识，不放权限/角色
type CustomClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwtgo.StandardClaims
}

type JWT struct {
	SigningKey []byte
}

// New 创建 JWT 实例
// 建议传入 global.Config.JWT.Secret，全局唯一来源
func New(signingKey string) *JWT {
	return &JWT{
		SigningKey: []byte(signingKey),
	}
}

// GenerateToken 生成 JWT token —— 去掉 role 参数
func (j *JWT) GenerateToken(userID uint, username string) (string, error) {
	now := time.Now().Unix()
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		StandardClaims: jwtgo.StandardClaims{
			ExpiresAt: now + 86400, // 24h
			IssuedAt:  now,
			NotBefore: now,
		},
	}
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

// ParseToken 解析 token（逻辑不变）
func (j *JWT) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwtgo.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwtgo.Token) (interface{}, error) {
		return j.SigningKey, nil
	})

	if err != nil {
		if ve, ok := err.(*jwtgo.ValidationError); ok {
			switch {
			case ve.Errors&jwtgo.ValidationErrorMalformed != 0:
				return nil, ErrInvalidToken
			case ve.Errors&jwtgo.ValidationErrorExpired != 0:
				return nil, ErrTokenExpired
			case ve.Errors&jwtgo.ValidationErrorNotValidYet != 0:
				return nil, ErrTokenNotValidYet
			}
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}

// RefreshToken 刷新 token —— 去掉 claims 里对 role 的隐式依赖（无需改动逻辑）
func (j *JWT) RefreshToken(tokenString string) (string, error) {
	claims := &CustomClaims{}
	token, err := jwtgo.ParseWithClaims(tokenString, claims, func(t *jwtgo.Token) (interface{}, error) {
		return j.SigningKey, nil
	}, jwtgo.WithoutClaimsValidation())

	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	now := time.Now().Unix()
	claims.StandardClaims.ExpiresAt = now + 86400
	claims.StandardClaims.IssuedAt = now
	claims.StandardClaims.NotBefore = now // ← 顺手补上，否则刷新后 nbf 是旧的，无伤大雅但更严谨

	newToken := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, claims)
	return newToken.SignedString(j.SigningKey)
}

// UserInfo 用户信息 —— 去掉 Role
type UserInfo struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
}

// GetUserInfo 获取用户信息
func (j *JWT) GetUserInfo(tokenString string) (*UserInfo, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return nil, err
	}
	return &UserInfo{UserID: claims.UserID, Username: claims.Username}, nil
}
