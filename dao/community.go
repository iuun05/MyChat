package dao

import (
	"MyChat/global"
	"MyChat/models"
	"errors"
)

// CommunityDAO 群组数据访问对象
type CommunityDAO struct {
}

// NewCommunityDAO 创建CommunityDAO实例
func NewCommunityDAO() *CommunityDAO {
	return &CommunityDAO{}
}

// ===== CommunityDAO 方法实现 =====

// CreateCommunity 新建群
func (c *CommunityDAO) CreateCommunity(community models.Community) (int, error) {
	com := models.Community{}
	//查询群是否已经存在
	if tx := global.DB.Where("name = ?", community.Name).First(&com); tx.RowsAffected == 1 {
		return -1, errors.New("当前群记录已存在")
	}

	tx := global.DB.Begin()
	if t := tx.Create(&community); t.RowsAffected == 0 {
		tx.Rollback()
		return -1, errors.New("群记录创建失败")
	}

	relation := models.Relation{}
	relation.OwnerId = community.OwnerId //群主id
	relation.TargetId = community.ID     //群id
	relation.Type = 2                    //群
	if t := tx.Create(&relation); t.RowsAffected == 0 {
		tx.Rollback()
		return -1, errors.New("群记录创建失败")
	}

	tx.Commit()
	return 0, nil
}

// GetCommunityList 获取群列表
func (c *CommunityDAO) GetCommunityList(ownerId uint) (*[]models.Community, error) {
	//获取我加入的群
	relation := make([]models.Relation, 0)

	if tx := global.DB.Where("owner_id = ? and type = 2", ownerId).Find(&relation); tx.RowsAffected == 0 {
		return nil, errors.New("不存在群记录")
	}

	communityID := make([]uint, 0)
	for _, v := range relation {
		cid := v.TargetId
		communityID = append(communityID, cid)
	}

	community := make([]models.Community, 0)
	if tx := global.DB.Where("id in ?", communityID).Find(&community); tx.RowsAffected == 0 {
		return nil, errors.New("获取群数据失败")
	}

	return &community, nil
}

// JoinCommunity 根据群昵称搜索并加入群
func (c *CommunityDAO) JoinCommunity(ownerId uint, cname string) (int, error) {
	community := models.Community{}
	if tx := global.DB.Where("name = ?", cname).First(&community); tx.RowsAffected == 0 {
		return -1, errors.New("群记录不存在")
	}

	//重复加群
	relation := models.Relation{}
	if tx := global.DB.Where("owner_id = ? and target_id = ? and type = 2", ownerId, community.ID).First(&relation); tx.RowsAffected == 1 {
		return -1, errors.New("该群已经加入")
	}

	relation = models.Relation{}
	relation.OwnerId = ownerId
	relation.TargetId = community.ID
	relation.Type = 2

	if tx := global.DB.Create(&relation); tx.RowsAffected == 0 {
		return -1, errors.New("加入失败")
	}

	return 0, nil
}

// FindUsers 获取群成员id
func (c *CommunityDAO) FindUsers(groupId uint) (*[]uint, error) {
	relation := make([]models.Relation, 0)
	if tx := global.DB.Where("target_id = ? and type = 2", groupId).Find(&relation); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到成员信息")
	}

	userIDs := make([]uint, 0)
	for _, v := range relation {
		userId := v.OwnerId
		userIDs = append(userIDs, userId)
	}
	return &userIDs, nil
}

// ===== 向后兼容的全局函数（委托给defaultCommunityDAO） =====
var defaultCommunityDAO *CommunityDAO

func init() {
	defaultCommunityDAO = NewCommunityDAO()
}

// CreateCommunity 新建群（向后兼容）
func CreateCommunity(community models.Community) (int, error) {
	return defaultCommunityDAO.CreateCommunity(community)
}

// GetCommunityList 获取群列表（向后兼容）
func GetCommunityList(ownerId uint) (*[]models.Community, error) {
	return defaultCommunityDAO.GetCommunityList(ownerId)
}

// JoinCommunity 根据群昵称搜索并加入群（向后兼容）
func JoinCommunity(ownerId uint, cname string) (int, error) {
	return defaultCommunityDAO.JoinCommunity(ownerId, cname)
}

// FindUsers 获取群成员id（向后兼容）
func FindUsers(groupId uint) (*[]uint, error) {
	return defaultCommunityDAO.FindUsers(groupId)
}
