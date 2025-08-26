package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMd5encoder(t *testing.T) {
	result := Md5encoder("test")
	expected := "098f6bcd4621d373cade4e832627b4f6" // "test" 的 MD5 值
	assert.Equal(t, expected, result)
}

func TestMd5StrToUpper(t *testing.T) {
	result := Md5StrToUpper("test")
	expected := "098F6BCD4621D373CADE4E832627B4F6"
	assert.Equal(t, expected, result)
}

func TestSaltPassWord(t *testing.T) {
	password := "mypassword"
	salt := "mysalt"
	result := SaltPassWord(password, salt)

	// 结果应该是 MD5(password) + $ + salt
	expectedMd5 := Md5encoder(password)
	expected := expectedMd5 + "$" + salt
	assert.Equal(t, expected, result)
}

func TestCheckPassWord(t *testing.T) {
	password := "mypassword"
	salt := "mysalt"
	hashedPassword := SaltPassWord(password, salt)

	// 正确密码验证
	result := CheckPassWord(password, salt, hashedPassword)
	assert.True(t, result)

	// 错误密码验证
	result = CheckPassWord("wrongpassword", salt, hashedPassword)
	assert.False(t, result)
}
