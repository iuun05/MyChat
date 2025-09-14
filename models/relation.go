package models

type Relation struct {
	Model
	OwnerId  uint   // 谁的关系信息
	TargetId uint   // 对应的谁
	Type     int    // 关系描述： 1. 好友关系；2. 群关系
	Desc     string // 描述
}

func (r *Relation) RelTableName() string {
	return "relation"
}
