package middlewear

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	userID := uint(123)
	issuer := "testapp"

	token, err := GenerateToken(userID, issuer)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// 验证生成的token可以被解析
	claims, err := ParseToken(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, issuer, claims.Issuer)
}

func TestParseToken(t *testing.T) {
	userID := uint(456)
	issuer := "testapp"

	// 生成token
	token, err := GenerateToken(userID, issuer)
	assert.NoError(t, err)

	// 解析token
	claims, err := ParseToken(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, issuer, claims.Issuer)
	assert.True(t, time.Now().Unix() < claims.ExpiresAt)
}

func TestParseInvalidToken(t *testing.T) {
	invalidToken := "invalid.token.here"

	claims, err := ParseToken(invalidToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}
