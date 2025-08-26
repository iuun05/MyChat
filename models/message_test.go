package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgTableName(t *testing.T) {
	msg := &Message{}
	tableName := msg.MsgTableName()
	assert.Equal(t, "message", tableName)
}

func TestUserTableName(t *testing.T) {
	user := &UserBasic{}
	tableName := user.UserTableName()
	assert.Equal(t, "user_basic", tableName)
}

func TestRelTableName(t *testing.T) {
	relation := &Relation{}
	tableName := relation.RelTableName()
	assert.Equal(t, "relation", tableName)
}
