package bootstrap

import (
	"FastGo/internal/global"
	"FastGo/internal/model"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func (app *App) setupMySQL() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		app.Config.MySQL.Username,
		app.Config.MySQL.Password,
		app.Config.MySQL.Host,
		app.Config.MySQL.Port,
		app.Config.MySQL.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		global.Log.Error("MySQL 连接失败",
			zap.Error(err),
		)
		return fmt.Errorf("连接MySQL失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		global.Log.Error("获取 *sql.DB 失败",
			zap.Error(err),
		)
		return err
	}

	// 设置连接池
	sqlDB.SetMaxOpenConns(app.Config.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(app.Config.MySQL.MaxIdleConns)

	// 使用单例模式设置数据库实例
	global.SetDB(db)

	global.Log.Info("MySQL 连接成功")

	return nil
}

// Migrate 执行数据库迁移
func Migrate() {
	db := global.GetDB()
	if db == nil {
		global.Log.Error("数据库未初始化")
		return
	}

	// Nullable IDs let legacy messages without a client id coexist with the
	// unique sender/client idempotency index added by AutoMigrate.
	if db.Migrator().HasTable(&model.Message{}) && db.Migrator().HasColumn(&model.Message{}, "client_msg_id") {
		if err := db.Exec("UPDATE `message` SET `client_msg_id` = NULL WHERE `client_msg_id` = ''").Error; err != nil {
			global.Log.Error("清理旧消息客户端ID失败", zap.Error(err))
			return
		}
	}

	err := db.AutoMigrate(
		&model.User{},               // 用户表
		&model.FriendRequest{},      // 好友请求表
		&model.Friendship{},         //	好友关系表
		&model.Group{},              //	群组表
		&model.GroupMember{},        //	群组成员表
		&model.Notification{},       // 用户级好友和群组通知表
		&model.SystemAnnouncement{}, // 全员系统公告表
		&model.ConversationStream{}, // 会话服务端消息序列表
		&model.Conversation{},       //	会话表
		&model.Message{},            //	消息表
	)
	if err != nil {
		global.Log.Error("数据库迁移失败", zap.Error(err))
		return
	}

	if err := backfillZeroSequenceMessages(db); err != nil {
		global.Log.Error("回填历史消息序号失败", zap.Error(err))
		return
	}
	global.Log.Info("数据库迁移成功")
}

func backfillZeroSequenceMessages(db *gorm.DB) error {
	var convKeys []string
	if err := db.Model(&model.Message{}).
		Where("sequence = 0").
		Distinct().
		Pluck("conv_key", &convKeys).Error; err != nil {
		return err
	}

	for _, convKey := range convKeys {
		err := db.Transaction(func(tx *gorm.DB) error {
			var stream model.ConversationStream
			streamErr := tx.Where("conv_key = ?", convKey).First(&stream).Error
			if streamErr != nil && streamErr != gorm.ErrRecordNotFound {
				return streamErr
			}

			var maxMessageSequence uint64
			if err := tx.Model(&model.Message{}).
				Where("conv_key = ?", convKey).
				Select("COALESCE(MAX(sequence), 0)").
				Scan(&maxMessageSequence).Error; err != nil {
				return err
			}
			if maxMessageSequence > stream.LastSequence {
				stream.LastSequence = maxMessageSequence
			}

			var lastID uint
			for {
				var rows []struct {
					ID uint
				}
				if err := tx.Model(&model.Message{}).
					Select("id").
					Where("conv_key = ? AND sequence = 0 AND id > ?", convKey, lastID).
					Order("id ASC").
					Limit(500).
					Find(&rows).Error; err != nil {
					return err
				}
				if len(rows) == 0 {
					break
				}

				var caseSQL strings.Builder
				caseSQL.WriteString("CASE `id`")
				args := make([]any, 0, len(rows)*3)
				ids := make([]any, 0, len(rows))
				for _, row := range rows {
					stream.LastSequence++
					caseSQL.WriteString(" WHEN ? THEN ?")
					args = append(args, row.ID, stream.LastSequence)
					ids = append(ids, row.ID)
					lastID = row.ID
				}
				caseSQL.WriteString(" END")
				args = append(args, ids...)
				placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
				query := "UPDATE `message` SET `sequence` = " + caseSQL.String() +
					" WHERE `id` IN (" + placeholders + ")"
				if err := tx.Exec(query, args...).Error; err != nil {
					return err
				}
			}

			if stream.ID == 0 {
				stream.ConvKey = convKey
				if err := tx.Create(&stream).Error; err != nil {
					return err
				}
			} else if err := tx.Model(&stream).Update("last_sequence", stream.LastSequence).Error; err != nil {
				return err
			}

			return updateConversationSequence(tx, convKey, stream.LastSequence)
		})
		if err != nil {
			return fmt.Errorf("会话 %q: %w", convKey, err)
		}
	}
	return nil
}

func updateConversationSequence(tx *gorm.DB, convKey string, sequence uint64) error {
	parts := strings.Split(convKey, "_")
	if len(parts) == 3 && parts[0] == "1" {
		userA, errA := strconv.ParseUint(parts[1], 10, 64)
		userB, errB := strconv.ParseUint(parts[2], 10, 64)
		if errA != nil || errB != nil {
			return nil
		}
		return tx.Model(&model.Conversation{}).
			Where("target_type = 1 AND ((user_id = ? AND target_id = ?) OR (user_id = ? AND target_id = ?))",
				userA, userB, userB, userA).
			Update("last_sequence", sequence).Error
	}
	if len(parts) == 2 && parts[0] == "2" {
		groupID, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil
		}
		return tx.Model(&model.Conversation{}).
			Where("target_type = 2 AND target_id = ?", groupID).
			Update("last_sequence", sequence).Error
	}
	return nil
}
