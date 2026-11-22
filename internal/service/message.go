package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"FastGo/internal/global"
	"FastGo/internal/handler/dto"
	"FastGo/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidMessage          = errors.New("消息参数无效")
	ErrMessageUserNotFound     = errors.New("用户不存在或不可用")
	ErrMessageNotFriend        = errors.New("只能向正常好友发送消息")
	ErrMessageBlocked          = errors.New("好友关系已被拉黑")
	ErrConversationAccess      = errors.New("无权访问该会话")
	ErrClientMessageIDConflict = errors.New("client_msg_id 已用于其他消息")
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
	defaultSyncLimit    = 100
	maxSyncLimit        = 200
	maxTextRunes        = 4000
)

type MessageService struct {
	DB *gorm.DB
}

func NewMessageService() *MessageService {
	return &MessageService{DB: global.GetDB()}
}

// SendDirectMessage persists a direct message before returning it for delivery.
func (s *MessageService) SendDirectMessage(senderID uint, req dto.SendMessageRequest) (*dto.MessageVO, bool, error) {
	if senderID == 0 || req.TargetID == 0 || senderID == req.TargetID ||
		strings.TrimSpace(req.ClientMsgID) == "" || len(req.ClientMsgID) > 64 {
		return nil, false, ErrInvalidMessage
	}
	if req.MessageType == model.MessageTypeText {
		if strings.TrimSpace(req.FileID) != "" ||
			strings.TrimSpace(req.Content) == "" || utf8.RuneCountInString(req.Content) > maxTextRunes {
			return nil, false, ErrInvalidMessage
		}
	} else if (req.MessageType != model.MessageTypeImage && req.MessageType != model.MessageTypeFile) ||
		strings.TrimSpace(req.FileID) == "" || strings.TrimSpace(req.Content) != "" {
		return nil, false, ErrInvalidMessage
	}

	clientMsgID := strings.TrimSpace(req.ClientMsgID)
	convKey := directConvKey(senderID, req.TargetID)
	var saved model.Message
	created := false

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		messageContent := req.Content
		if req.MessageType != model.MessageTypeText {
			var file model.File
			if err := tx.Where("id = ? AND uploader_id = ?", req.FileID, senderID).First(&file).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrFileNotFound
				}
				return err
			}
			if MessageTypeForFile(file) != req.MessageType {
				return ErrInvalidMessage
			}
			var err error
			messageContent, err = FileMessageContent(file)
			if err != nil {
				return err
			}
		}

		existing, err := findMessageByClientID(tx, senderID, clientMsgID)
		if err == nil {
			if !sameMessagePayload(existing, convKey, req.TargetID, req.MessageType, messageContent) {
				return ErrClientMessageIDConflict
			}
			saved = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var sender, receiver model.User
		if err := tx.Select("id, username, nickname, avatar").
			Where("status = ?", 1).First(&sender, senderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMessageUserNotFound
			}
			return err
		}
		if err := tx.Select("id, username, nickname, avatar").
			Where("status = ?", 1).First(&receiver, req.TargetID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMessageUserNotFound
			}
			return err
		}
		if err := validateDirectFriendship(tx, senderID, req.TargetID); err != nil {
			return err
		}

		sequence, err := nextConversationSequence(tx, convKey)
		if err != nil {
			return err
		}
		saved = model.Message{
			ConvKey:        convKey,
			Sequence:       sequence,
			SenderID:       senderID,
			ReceiverID:     req.TargetID,
			Type:           req.MessageType,
			Content:        messageContent,
			SenderNickname: sender.Nickname,
			SenderAvatar:   sender.Avatar,
			Status:         1,
			ClientMsgID:    &clientMsgID,
			CreatedAt:      time.Now(),
		}
		if saved.SenderNickname == "" {
			saved.SenderNickname = sender.Username
		}
		if err := tx.Create(&saved).Error; err != nil {
			return err
		}

		participants := []struct {
			userID   uint
			targetID uint
			unread   bool
		}{
			{userID: senderID, targetID: req.TargetID},
			{userID: req.TargetID, targetID: senderID, unread: true},
		}
		if participants[0].userID > participants[1].userID {
			participants[0], participants[1] = participants[1], participants[0]
		}
		for _, participant := range participants {
			if err := touchConversationTx(tx, participant.userID, participant.targetID, saved, participant.unread); err != nil {
				return err
			}
		}

		created = true
		return nil
	})
	if err != nil {
		// A concurrent retry may win the unique client-message index while this
		// transaction is inserting. Resolve it as an idempotent retry.
		existing, lookupErr := findMessageByClientID(s.DB, senderID, clientMsgID)
		if lookupErr == nil {
			expectedContent := req.Content
			if req.MessageType != model.MessageTypeText {
				var file model.File
				fileErr := s.DB.Where("id = ? AND uploader_id = ?", req.FileID, senderID).First(&file).Error
				if fileErr != nil {
					return nil, false, fileErr
				}
				if MessageTypeForFile(file) != req.MessageType {
					return nil, false, ErrInvalidMessage
				}
				expectedContent, fileErr = FileMessageContent(file)
				if fileErr != nil {
					return nil, false, fileErr
				}
			}
			if sameMessagePayload(existing, convKey, req.TargetID, req.MessageType, expectedContent) {
				result := messageToVO(existing)
				return &result, false, nil
			}
			return nil, false, ErrClientMessageIDConflict
		}
		return nil, false, err
	}

	result := messageToVO(saved)
	return &result, created, nil
}

// History returns the latest or older messages for a direct conversation in chronological order.
func (s *MessageService) History(userID uint, req dto.MessageHistoryRequest) (*dto.MessagePage, error) {
	if userID == 0 || req.TargetID == 0 || req.TargetID == userID {
		return nil, ErrInvalidMessage
	}
	if req.Limit <= 0 {
		req.Limit = defaultHistoryLimit
	}
	if req.Limit > maxHistoryLimit {
		req.Limit = maxHistoryLimit
	}

	convKey := directConvKey(userID, req.TargetID)
	if err := s.ensureCanReadDirect(userID, req.TargetID); err != nil {
		return nil, err
	}

	query := s.DB.Where("conv_key = ?", convKey)
	if req.BeforeSequence > 0 {
		query = query.Where("sequence < ?", req.BeforeSequence)
	}

	var messages []model.Message
	if err := query.Order("sequence DESC").Limit(req.Limit + 1).Find(&messages).Error; err != nil {
		return nil, err
	}

	hasMore := len(messages) > req.Limit
	if hasMore {
		messages = messages[:req.Limit]
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	page := &dto.MessagePage{
		List:    make([]dto.MessageVO, 0, len(messages)),
		HasMore: hasMore,
	}
	for _, message := range messages {
		page.List = append(page.List, messageToVO(message))
	}
	if hasMore && len(messages) > 0 {
		page.NextCursor = messages[0].Sequence
	}
	return page, nil
}

// Sync returns direct messages in one conversation after its sequence cursor.
func (s *MessageService) Sync(userID uint, req dto.MessageSyncRequest) (*dto.MessagePage, error) {
	if userID == 0 || req.TargetID == 0 || req.TargetID == userID {
		return nil, ErrInvalidMessage
	}
	if req.Limit <= 0 {
		req.Limit = defaultSyncLimit
	}
	if req.Limit > maxSyncLimit {
		req.Limit = maxSyncLimit
	}

	if err := s.ensureCanReadDirect(userID, req.TargetID); err != nil {
		return nil, err
	}

	var messages []model.Message
	if err := s.DB.
		Where("conv_key = ? AND sequence > ?", directConvKey(userID, req.TargetID), req.AfterSequence).
		Order("sequence ASC").
		Limit(req.Limit + 1).
		Find(&messages).Error; err != nil {
		return nil, err
	}

	hasMore := len(messages) > req.Limit
	if hasMore {
		messages = messages[:req.Limit]
	}
	page := &dto.MessagePage{
		List:    make([]dto.MessageVO, 0, len(messages)),
		HasMore: hasMore,
	}
	for _, message := range messages {
		page.List = append(page.List, messageToVO(message))
	}
	page.NextCursor = req.AfterSequence
	if len(messages) > 0 {
		page.NextCursor = messages[len(messages)-1].Sequence
	}
	return page, nil
}

func (s *MessageService) ensureCanReadDirect(userID, targetID uint) error {
	var conversation model.Conversation
	err := s.DB.Where("user_id = ? AND target_id = ? AND target_type = 1", userID, targetID).
		First(&conversation).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return validateDirectFriendship(s.DB, userID, targetID)
}

func validateDirectFriendship(db *gorm.DB, userID, targetID uint) error {
	var blocked int64
	if err := db.Model(&model.Friendship{}).
		Where("((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)) AND status = ?",
			userID, targetID, targetID, userID, dto.FriendStatusBlocked).
		Count(&blocked).Error; err != nil {
		return err
	}
	if blocked > 0 {
		return ErrMessageBlocked
	}

	for _, pair := range [][2]uint{{userID, targetID}, {targetID, userID}} {
		var friendship model.Friendship
		if err := db.Where("user_id = ? AND friend_id = ? AND status = ?",
			pair[0], pair[1], dto.FriendStatusNormal).First(&friendship).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMessageNotFriend
			}
			return err
		}
	}
	return nil
}

func touchConversationTx(tx *gorm.DB, userID, targetID uint, message model.Message, incrementUnread bool) error {
	placeholder := model.Conversation{
		UserID:     userID,
		TargetID:   targetID,
		TargetType: 1,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&placeholder).Error; err != nil {
		return err
	}

	var conversation model.Conversation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, 1).
		First(&conversation).Error; err != nil {
		return err
	}

	updates := map[string]any{}
	if message.Sequence > conversation.LastSequence {
		updates["last_message_id"] = message.ID
		updates["last_sequence"] = message.Sequence
		updates["last_message"] = messagePreview(message)
		updates["last_time"] = message.CreatedAt.UnixMilli()
	}
	if incrementUnread {
		updates["unread_count"] = gorm.Expr("unread_count + ?", 1)
	}
	if len(updates) == 0 {
		return nil
	}
	return tx.Model(&conversation).Updates(updates).Error
}

func nextConversationSequence(tx *gorm.DB, convKey string) (uint64, error) {
	stream := model.ConversationStream{ConvKey: convKey}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&stream).Error; err != nil {
		return 0, err
	}

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conv_key = ?", convKey).
		First(&stream).Error; err != nil {
		return 0, err
	}

	stream.LastSequence++
	if err := tx.Model(&stream).Update("last_sequence", stream.LastSequence).Error; err != nil {
		return 0, err
	}
	return stream.LastSequence, nil
}

func findMessageByClientID(db *gorm.DB, senderID uint, clientMsgID string) (model.Message, error) {
	var message model.Message
	err := db.Where("sender_id = ? AND client_msg_id = ?", senderID, clientMsgID).First(&message).Error
	return message, err
}

func sameMessagePayload(message model.Message, convKey string, targetID uint, messageType uint8, content string) bool {
	return message.ConvKey == convKey &&
		message.ReceiverID == targetID &&
		message.Type == messageType &&
		message.Content == content
}

func directConvKey(userA, userB uint) string {
	if userA > userB {
		userA, userB = userB, userA
	}
	return fmt.Sprintf("1_%d_%d", userA, userB)
}

func messagePreview(message model.Message) string {
	switch message.Type {
	case model.MessageTypeImage:
		return "[图片]"
	case model.MessageTypeFile:
		return "[文件]"
	}
	runes := []rune(message.Content)
	if len(runes) > 100 {
		return string(runes[:100])
	}
	return message.Content
}

func messageToVO(message model.Message) dto.MessageVO {
	clientMsgID := ""
	if message.ClientMsgID != nil {
		clientMsgID = *message.ClientMsgID
	}
	return dto.MessageVO{
		ID:             message.ID,
		ConvKey:        message.ConvKey,
		Sequence:       message.Sequence,
		SenderID:       message.SenderID,
		ReceiverID:     message.ReceiverID,
		MessageType:    message.Type,
		Content:        message.Content,
		SenderNickname: message.SenderNickname,
		SenderAvatar:   message.SenderAvatar,
		Status:         message.Status,
		ClientMsgID:    clientMsgID,
		CreatedAt:      message.CreatedAt,
	}
}
