package service

import (
	"errors"
	"strings"
	"time"

	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"
	"FastGo/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUserNotFound indicates that the username does not match an active user.
var ErrUserNotFound = errors.New("用户不存在")

// ErrCannotSearchSelf indicates that a user cannot search for their own account.
var ErrCannotSearchSelf = errors.New("不能搜索自己")

var ErrFriendRequestNotFound = errors.New("好友申请不存在")
var ErrFriendRequestNotPending = errors.New("好友申请已处理")

// UserService 提供用户登录相关的业务逻辑
type UserService struct {
	DB *gorm.DB
}

// NewUserService 创建一个新的 UserService 实例
func NewUserService() *UserService {
	return &UserService{DB: global.GetDB()}
}

// service/login.go —— 不再接收 gin.Context，入参用 *dto.LoginRequest
func (s *UserService) Login(req *dto.LoginRequest) (*model.User, error) {
	if req.Username == "" && req.Email == "" && req.PhoneNumber == "" && req.ApiKey == "" {
		return nil, errors.New("必须提供用户名、邮箱、手机号或 API Key 之一")
	}

	var user model.User
	var err error

	switch {
	case req.ApiKey != "":
		if err = s.DB.Where("api_key = ?", req.ApiKey).First(&user).Error; err != nil {
			return nil, errors.New("无效的 API Key")
		}
		return &user, nil // API Key 登录免密
	case req.Username != "":
		err = s.DB.Where("username = ?", req.Username).First(&user).Error
	case req.Email != "":
		err = s.DB.Where("email = ?", req.Email).First(&user).Error
	case req.PhoneNumber != "":
		err = s.DB.Where("phone_number = ?", req.PhoneNumber).First(&user).Error
	default:
		return nil, errors.New("无效的登录方式")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("用户不存在") // 账号密码错统一回一句，不泄露账号是否存在
	}
	if err != nil {
		return nil, err
	}

	if req.Password == "" {
		return nil, errors.New("密码不能为空")
	}
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	return &user, nil
}

// FriendList 获取好友列表
func (s *UserService) FriendList(userID uint, req *dto.FriendListRequest) (*dto.FriendListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	db := s.DB.Model(&model.Friendship{}).
		Where("user_id = ?", userID).
		Where("status = ?", dto.FriendStatusNormal)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	var friendships []model.Friendship
	// 预加载好友用户信息
	err := db.Preload("FriendUser").
		Order("created_at DESC").
		Offset((req.Page - 1) * req.PageSize).
		Limit(req.PageSize).
		Find(&friendships).Error
	if err != nil {
		return nil, err
	}

	list := make([]dto.FriendItemVO, 0, len(friendships))
	for _, friendship := range friendships {
		item := dto.FriendItemVO{
			FriendshipID: friendship.ID,
			Remark:       friendship.Remark,
			GroupID:      friendship.GroupID,
			CreatedAt:    friendship.CreatedAt,
		}

		if friendship.FriendUser != nil {
			item.User = dto.FriendUserVO{
				ID:       friendship.FriendUser.ID,
				Username: friendship.FriendUser.Username,
				Nickname: friendship.FriendUser.Nickname,
				Avatar:   friendship.FriendUser.Avatar,
				Bio:      friendship.FriendUser.Bio,
			}
		}
		list = append(list, item)
	}

	return &dto.FriendListResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     list,
	}, nil
}

// SearchUser 按用户名精确查找可添加的用户
func (s *UserService) SearchUser(currentUserID uint, username string) (*dto.FriendUserVO, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}

	var user model.User
	err := s.DB.Select("id, username, nickname, avatar, bio").
		Where("username = ? AND status = ?", username, 1).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if user.ID == currentUserID {
		return nil, ErrCannotSearchSelf
	}

	return &dto.FriendUserVO{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Bio:      user.Bio,
	}, nil
}

// SendFriendRequest 发起好友申请
func (s *UserService) SendFriendRequest(fromUserID uint, req *dto.SendFriendRequestRequest) error {
	if fromUserID == req.ToUserID {
		return errors.New("不能添加自己为好友")
	}

	// 校验目标用户是否存在
	var toUser model.User
	if err := s.DB.First(&toUser, req.ToUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	// 校验是否已经是好友
	var friendship model.Friendship
	err := s.DB.Where("user_id = ? AND friend_id = ?", fromUserID, req.ToUserID).First(&friendship).Error
	if err == nil {
		if friendship.Status == dto.FriendStatusNormal {
			return errors.New("你们已经是好友了")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 校验是否已有待处理的申请
	var pendingRequest model.FriendRequest
	err = s.DB.Where("from_user_id = ? AND to_user_id = ? AND status = ?", fromUserID, req.ToUserID, dto.FriendRequestStatusPending).First(&pendingRequest).Error
	if err == nil {
		return errors.New("好友申请已发送，请等待对方处理")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.DB.Create(&model.FriendRequest{
		FromUserID: fromUserID,
		ToUserID:   req.ToUserID,
		Message:    req.Message,
		Status:     dto.FriendRequestStatusPending,
	}).Error
}

// FriendRequestList 获取当前用户收到的待处理好友申请
func (s *UserService) FriendRequestList(userID uint, req *dto.FriendRequestListRequest) (*dto.FriendRequestListResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	query := s.DB.
		Where("to_user_id = ? AND status = ?", userID, dto.FriendRequestStatusPending)
	if req.Cursor > 0 {
		query = query.Where("id < ?", req.Cursor)
	}

	var requests []model.FriendRequest
	if err := query.
		Preload("FromUser", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, username, nickname, avatar, bio")
		}).
		Order("id DESC").
		Limit(req.Limit + 1).
		Find(&requests).Error; err != nil {
		return nil, err
	}

	hasMore := len(requests) > req.Limit
	if hasMore {
		requests = requests[:req.Limit]
	}

	list := make([]dto.FriendRequestItemVO, 0, len(requests))
	for _, request := range requests {
		item := dto.FriendRequestItemVO{
			RequestID:  request.ID,
			FromUserID: request.FromUserID,
			Message:    request.Message,
			CreatedAt:  request.CreatedAt,
		}
		if request.FromUser != nil {
			item.FromUser = &dto.FriendUserVO{
				ID:       request.FromUser.ID,
				Username: request.FromUser.Username,
				Nickname: request.FromUser.Nickname,
				Avatar:   request.FromUser.Avatar,
				Bio:      request.FromUser.Bio,
			}
		}
		list = append(list, item)
	}

	result := &dto.FriendRequestListResponse{
		List:    list,
		HasMore: hasMore,
	}
	if hasMore {
		nextCursor := requests[len(requests)-1].ID
		result.NextCursor = &nextCursor
	}
	return result, nil
}

// AcceptFriendRequest 同意好友申请并建立双向好友关系
func (s *UserService) AcceptFriendRequest(userID, requestID uint) error {
	return s.handleFriendRequest(userID, requestID, dto.FriendRequestStatusAccepted)
}

// RejectFriendRequest 拒绝好友申请
func (s *UserService) RejectFriendRequest(userID, requestID uint) error {
	return s.handleFriendRequest(userID, requestID, dto.FriendRequestStatusRejected)
}

func (s *UserService) handleFriendRequest(userID, requestID uint, status uint8) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var request model.FriendRequest
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND to_user_id = ?", requestID, userID).
			First(&request).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendRequestNotFound
		}
		if err != nil {
			return err
		}
		if request.Status != dto.FriendRequestStatusPending {
			return ErrFriendRequestNotPending
		}

		now := time.Now()
		request.Status = status
		request.HandledAt = &now
		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		if status == dto.FriendRequestStatusAccepted {
			if err := upsertFriendship(tx, request.ToUserID, request.FromUserID); err != nil {
				return err
			}
			if err := upsertFriendship(tx, request.FromUserID, request.ToUserID); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertFriendship(db *gorm.DB, userID, friendID uint) error {
	var friendship model.Friendship
	err := db.Where("user_id = ? AND friend_id = ?", userID, friendID).First(&friendship).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&model.Friendship{
			UserID:   userID,
			FriendID: friendID,
			Status:   dto.FriendStatusNormal,
		}).Error
	}
	if err != nil {
		return err
	}

	friendship.Status = dto.FriendStatusNormal
	return db.Save(&friendship).Error
}
