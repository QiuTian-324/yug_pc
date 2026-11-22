package service

import (
	"errors"
	"sync"

	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"
	"FastGo/internal/realtime"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrInvalidPresenceMode = errors.New("在线状态无效")

type PresenceService struct {
	DB  *gorm.DB
	Hub *realtime.Hub

	mu                sync.Mutex
	lastVisibleStatus map[uint]bool
}

func NewPresenceService(hub *realtime.Hub) *PresenceService {
	return &PresenceService{
		DB:                global.GetDB(),
		Hub:               hub,
		lastVisibleStatus: make(map[uint]bool),
	}
}

func (s *PresenceService) SetMode(userID uint, mode string) (dto.PresenceVO, error) {
	var value uint8
	switch mode {
	case "online":
		value = model.PresenceModeOnline
	case "invisible":
		value = model.PresenceModeInvisible
	default:
		return dto.PresenceVO{}, ErrInvalidPresenceMode
	}

	var user model.User
	if err := s.DB.Select("id, presence_mode").First(&user, userID).Error; err != nil {
		return dto.PresenceVO{}, err
	}
	if user.PresenceMode != value {
		if err := s.DB.Model(&user).Update("presence_mode", value).Error; err != nil {
			return dto.PresenceVO{}, err
		}
	}

	s.broadcastCurrentStatus(userID)
	statuses, err := s.Statuses([]uint{userID})
	if err != nil {
		return dto.PresenceVO{}, err
	}
	return dto.PresenceVO{
		Mode:     mode,
		IsOnline: statuses[userID],
	}, nil
}

func (s *PresenceService) Statuses(userIDs []uint) (map[uint]bool, error) {
	statuses := make(map[uint]bool, len(userIDs))
	if len(userIDs) == 0 {
		return statuses, nil
	}

	onlineConnections := s.Hub.OnlineUserIDs()
	var users []struct {
		ID           uint
		PresenceMode uint8
		Status       uint8
	}
	if err := s.DB.Model(&model.User{}).
		Select("id, presence_mode, status").
		Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		_, connected := onlineConnections[user.ID]
		statuses[user.ID] = connected &&
			user.PresenceMode == model.PresenceModeOnline &&
			user.Status == 1
	}
	return statuses, nil
}

func (s *PresenceService) OnConnectionChanged(userID uint) {
	s.broadcastCurrentStatus(userID)
}

func (s *PresenceService) broadcastCurrentStatus(userID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var user model.User
	if err := s.DB.Select("id, presence_mode, status").First(&user, userID).Error; err != nil {
		global.Log.Error("读取用户在线状态失败", zap.Error(err))
		return
	}
	isOnline := s.Hub.IsOnline(userID) &&
		user.PresenceMode == model.PresenceModeOnline &&
		user.Status == 1
	if s.lastVisibleStatus[userID] == isOnline {
		return
	}

	var friendIDs []uint
	if err := s.DB.Model(&model.Friendship{}).
		Where("user_id = ? AND status = ?", userID, dto.FriendStatusNormal).
		Pluck("friend_id", &friendIDs).Error; err != nil {
		global.Log.Error("查询好友在线状态推送目标失败", zap.Error(err))
		return
	}

	if isOnline {
		s.lastVisibleStatus[userID] = true
	} else {
		delete(s.lastVisibleStatus, userID)
	}
	event := dto.WSServerEnvelope{
		Type: "presence.changed",
		Data: dto.PresenceChangedEvent{UserID: userID, IsOnline: isOnline},
	}
	for _, friendID := range friendIDs {
		if err := s.Hub.SendToUser(friendID, nil, event); err != nil {
			global.Log.Error("推送好友在线状态失败", zap.Error(err))
		}
	}
}
