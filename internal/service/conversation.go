package service

import (
	"errors"
	"sort"

	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"

	"gorm.io/gorm"
)

type ConversationService struct {
	DB *gorm.DB
}

var ErrConversationNotFound = errors.New("会话不存在")

func NewConversationService() *ConversationService {
	return &ConversationService{DB: global.GetDB()}
}

// List 会话列表：单聊联 users，群聊联 groups，内存合并后按 置顶优先 + 时间倒序
func (s *ConversationService) List(userID uint) ([]dto.ConversationVO, error) {
	var convs []model.Conversation
	if err := s.DB.Where("user_id = ?", userID).Find(&convs).Error; err != nil {
		return nil, err
	}

	// 按类型分组收集 target_id，批量查资料，避免 N+1
	var singleIDs, groupIDs []uint
	for _, c := range convs {
		if c.TargetType == 1 {
			singleIDs = append(singleIDs, c.TargetID)
		} else {
			groupIDs = append(groupIDs, c.TargetID)
		}
	}

	type profile struct {
		Name   string
		Avatar string
	}
	singleMap := map[uint]profile{}
	groupMap := map[uint]profile{}

	if len(singleIDs) > 0 {
		var rows []struct {
			ID       uint
			Nickname string
			Username string
			Avatar   string
		}
		s.DB.Model(&model.User{}).
			Select("id, nickname, username, avatar").
			Where("id IN ?", singleIDs).Find(&rows)
		for _, r := range rows {
			name := r.Nickname
			if name == "" {
				name = r.Username
			}
			singleMap[r.ID] = profile{Name: name, Avatar: r.Avatar}
		}
	}
	if len(groupIDs) > 0 {
		var rows []struct {
			ID     uint
			Name   string
			Avatar string
		}
		s.DB.Model(&model.Group{}).
			Select("id, name, avatar").
			Where("id IN ?", groupIDs).Find(&rows)
		for _, r := range rows {
			groupMap[r.ID] = profile{Name: r.Name, Avatar: r.Avatar}
		}
	}

	list := make([]dto.ConversationVO, 0, len(convs))
	for _, c := range convs {
		vo := dto.ConversationVO{
			ID:           c.ID,
			TargetID:     c.TargetID,
			TargetType:   c.TargetType,
			LastMessage:  c.LastMessage,
			LastSequence: c.LastSequence,
			LastTime:     c.LastTime,
			UnreadCount:  c.UnreadCount,
			IsPinned:     c.IsPinned,
			IsMuted:      c.IsMuted,
		}
		if c.TargetType == 1 {
			if p, ok := singleMap[c.TargetID]; ok {
				vo.ShowName, vo.Avatar = p.Name, p.Avatar
			}
		} else if p, ok := groupMap[c.TargetID]; ok {
			vo.ShowName, vo.Avatar = p.Name, p.Avatar
		}
		list = append(list, vo)
	}

	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsPinned != list[j].IsPinned {
			return list[i].IsPinned
		}
		return list[i].LastTime > list[j].LastTime
	})
	return list, nil
}

// Touch 取出（不存在则创建）某条会话并更新摘要。
// incUnread：收到对方消息时传 true，自己发消息给对方那条传 false。
func (s *ConversationService) Touch(userID, targetID uint, targetType int, lastMsg string, ts int64, incUnread bool) error {
	var conv model.Conversation
	err := s.DB.Where("user_id = ? AND target_id = ? AND target_type = ?",
		userID, targetID, targetType).First(&conv).Error

	if err == nil {
		upd := map[string]any{"last_message": lastMsg, "last_time": ts}
		if incUnread {
			upd["unread_count"] = gorm.Expr("unread_count + 1")
		}
		return s.DB.Model(&conv).Updates(upd).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	conv = model.Conversation{
		UserID:      userID,
		TargetID:    targetID,
		TargetType:  targetType,
		LastMessage: lastMsg,
		LastTime:    ts,
	}
	if incUnread {
		conv.UnreadCount = 1
	}
	return s.DB.Create(&conv).Error
}

// ClearUnread 进入会话时清空未读
func (s *ConversationService) ClearUnread(userID uint, req dto.ConversationTargetReq) error {
	return s.DB.Model(&model.Conversation{}).
		Where("user_id = ? AND target_id = ? AND target_type = ?",
			userID, req.TargetID, req.TargetType).
		Update("unread_count", 0).Error
}

// SetPinMute 设置置顶/免打扰，只更新显式传入的字段
func (s *ConversationService) SetPinMute(userID uint, req dto.SetPinMuteReq) (*dto.ConversationPinMuteVO, error) {
	if req.IsPinned == nil && req.IsMuted == nil {
		return nil, errors.New("is_pinned 或 is_muted 至少需要传一个")
	}

	var conv model.Conversation
	err := s.DB.Where("user_id = ? AND target_id = ? AND target_type = ?",
		userID, req.TargetID, req.TargetType).First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	upd := map[string]any{}
	if req.IsPinned != nil {
		upd["is_pinned"] = *req.IsPinned
		conv.IsPinned = *req.IsPinned
	}
	if req.IsMuted != nil {
		upd["is_muted"] = *req.IsMuted
		conv.IsMuted = *req.IsMuted
	}
	if err := s.DB.Model(&conv).Updates(upd).Error; err != nil {
		return nil, err
	}

	return &dto.ConversationPinMuteVO{
		TargetID:   conv.TargetID,
		TargetType: conv.TargetType,
		IsPinned:   conv.IsPinned,
		IsMuted:    conv.IsMuted,
	}, nil
}

// Delete 删除本用户的会话记录（不影响对方）
func (s *ConversationService) Delete(userID uint, req dto.ConversationTargetReq) error {
	return s.DB.Where("user_id = ? AND target_id = ? AND target_type = ?",
		userID, req.TargetID, req.TargetType).Delete(&model.Conversation{}).Error
}
