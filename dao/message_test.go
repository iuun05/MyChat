package dao

import (
	"MyChat/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgTableName(t *testing.T) {
	msg := &models.Message{}
	tableName := msg.MsgTableName()
	assert.Equal(t, "message", tableName)
}

func TestUserTableName(t *testing.T) {
	user := &models.UserBasic{}
	tableName := user.UserTableName()
	assert.Equal(t, "user_basic", tableName)
}

func TestRelTableName(t *testing.T) {
	relation := &models.Relation{}
	tableName := relation.RelTableName()
	assert.Equal(t, "relation", tableName)
}
