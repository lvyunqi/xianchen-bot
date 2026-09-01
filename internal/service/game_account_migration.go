package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

const migrationCodeLifetime = 24 * time.Hour

var (
	errMigrationCodeInvalid = errors.New("migration code invalid")
	errMigrationCodeUsed    = errors.New("migration code used")
	errMigrationTargetBusy  = errors.New("migration target account already exists")
	errMigrationFrozen      = errors.New("migration target is frozen")
)

func migrationTokenHash(token string) string {
	digest := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(token))))
	return hex.EncodeToString(digest[:])
}

func newMigrationToken() (string, error) {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	return "XC-" + encoded, nil
}

func (g *Game) migrationGuide(player *model.Player) (GameResult, bool, error) {
	if player == nil {
		return GameResult{Title: "🔐 道籍迁移", Content: "新官机尚未找到当前OpenID对应的角色。\n━━━━━━━━━━━\n正确顺序：\n一、所有待迁玩家先在旧官机发送“生成迁移码”。\n二、停止旧官机写入，再把生成凭证后的同一份仙尘数据库完整复制到新官机。\n三、只启动新官机，玩家使用新OpenID发送“迁入道籍 凭证”。\n━━━━━━━━━━━\n凭证有效24小时且只能使用一次。若先复制数据库、后在旧官机生成凭证，新官机数据库不会凭空拥有该记录，必须重新同步数据库。迁入只更换OpenID和群服标识，原道号、境界、背包、装备、任务、灵兽、仙侣、宗门与全部进度保持不变。", Actions: []string{"迁入道籍 凭证", "运行状态"}}, true, nil
	}
	return GameResult{Title: "🔐 道籍迁移", Content: "正确顺序：先让所有待迁玩家在旧官机生成迁移码，再停止旧官机，把生成凭证后的数据库完整复制到新官机，最后只启动新官机执行迁入。\n━━━━━━━━━━━\n不要先复制、后生成：迁移凭证记录必须跟随数据库进入新官机。凭证有效24小时且只能使用一次；迁移只更换OpenID和新群服标识，不会重置角色数据。已经在新官机误入道的账号不会被自动覆盖，请先联系主人处理。", Actions: []string{"生成迁移码", "帮助 角色", "运行状态"}}, true, nil
}

func (g *Game) createAccountMigrationCode(player *model.Player) (GameResult, bool, error) {
	if player == nil {
		return g.migrationGuide(nil)
	}
	if player.Banned {
		return GameResult{Title: "🔐 道籍冻结", Content: "冻结道籍不能生成迁移凭证。", Actions: []string{"状态"}}, true, nil
	}
	token, err := newMigrationToken()
	if err != nil {
		return GameResult{}, true, err
	}
	expiresAt := time.Now().Add(migrationCodeLifetime)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AccountMigrationCode{}).Where("player_id = ? AND status = ?", player.ID, "active").Updates(map[string]any{"status": "replaced", "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AccountMigrationCode{
			PlayerID: player.ID, OldAccountID: player.AccountID, TokenHash: migrationTokenHash(token),
			Status: "active", ExpiresAt: expiresAt,
		}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🔐 迁移凭证已生成", Content: fmt.Sprintf("原道号：%s\n迁移凭证：%s\n有效至：%s\n━━━━━━━━━━━\n下一步：等所有玩家生成完毕后，停止旧官机写入，再把当前这份数据库完整复制到新官机。只启动新官机后，使用新OpenID发送“迁入道籍 %s”。\n━━━━━━━━━━━\n不能在复制数据库之后才生成本码，否则新官机查不到凭证记录。迁入成功后旧OpenID自动释放，新OpenID接管原角色；凭证仅使用一次，请勿交给其他人。再次生成会立即替换旧凭证。", player.DaoName, token, expiresAt.Format("2006-01-02 15:04"), token), Actions: []string{"道籍迁移", "运行状态"}}, true, nil
}

func (g *Game) importAccountMigration(groupID, accountID, raw string) (GameResult, bool, error) {
	token := strings.TrimSpace(raw)
	if fields := strings.Fields(token); len(fields) > 0 {
		token = fields[0]
	}
	if token == "" {
		return g.migrationGuide(nil)
	}
	if len(token) < 12 {
		return GameResult{Title: "🔐 迁移凭证无效", Content: "凭证格式不正确，请完整复制旧官机生成的迁移凭证。", Actions: []string{"道籍迁移", "迁入道籍 凭证"}}, true, nil
	}
	hash := migrationTokenHash(token)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var migration model.AccountMigrationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", hash).First(&migration).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errMigrationCodeInvalid
			}
			return err
		}
		if migration.Status != "active" || migration.UsedAt != nil {
			return errMigrationCodeUsed
		}
		if migration.ExpiresAt.Before(time.Now()) {
			_ = tx.Model(&migration).Update("status", "expired").Error
			return errMigrationCodeInvalid
		}
		var occupant model.Player
		occupantErr := tx.Where("account_id = ?", accountID).First(&occupant).Error
		if occupantErr == nil && occupant.ID != migration.PlayerID {
			return errMigrationTargetBusy
		}
		if occupantErr != nil && !errors.Is(occupantErr, gorm.ErrRecordNotFound) {
			return occupantErr
		}
		var player model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, migration.PlayerID).Error; err != nil {
			return err
		}
		if player.Banned {
			return errMigrationFrozen
		}
		if player.AccountID == accountID {
			return errMigrationTargetBusy
		}
		if player.AccountID != migration.OldAccountID {
			return errMigrationCodeUsed
		}
		if err := migrateReferralAccountTx(tx, player.ID, player.AccountID, accountID); err != nil {
			return err
		}
		changes := map[string]any{"account_id": accountID}
		if strings.TrimSpace(groupID) != "" && groupID != "私信" {
			changes["server_name"] = groupID
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(changes).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&migration).Updates(map[string]any{"status": "used", "used_at": &now, "new_account_id": accountID}).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errMigrationCodeInvalid):
			return GameResult{Title: "🔐 新官机未找到迁移凭证", Content: "当前数据库中不存在该凭证，或凭证已经超过24小时。\n━━━━━━━━━━━\n最常见原因：新官机数据库复制于凭证生成之前。请停止两边官机写入，把旧官机中生成凭证后的完整数据库重新同步到新官机，再重试同一凭证。\n本次没有创建、覆盖或改动任何角色数据。", Actions: []string{"道籍迁移", "运行状态"}}, true, nil
		case errors.Is(err, errMigrationCodeUsed):
			return GameResult{Title: "🔐 迁移凭证已失效", Content: "该凭证已经使用、被替换，或原角色已完成过其他迁移。本次没有改动数据。", Actions: []string{"道籍迁移"}}, true, nil
		case errors.Is(err, errMigrationTargetBusy):
			return GameResult{Title: "🔐 新OpenID已有道籍", Content: "当前官机账号已经绑定另一个角色，系统不会自动覆盖或删除该角色。请先联系主人确认，再重新迁移。", Actions: []string{"状态", "道籍迁移"}}, true, nil
		case errors.Is(err, errMigrationFrozen):
			return GameResult{Title: "🔐 原道籍已冻结", Content: "冻结道籍不能通过迁移绕过处罚。", Actions: []string{"道籍迁移"}}, true, nil
		default:
			return GameResult{}, true, err
		}
	}
	return GameResult{Title: "🔐 道籍迁移成功", Content: "新官机OpenID已经接管原道籍。\n━━━━━━━━━━━\n道号、境界、灵根、修为、气血、法力、背包、装备、灵兽、仙侣、宗门、任务、活动、通知和所有历史进度均已保留。\n旧官机OpenID已释放；该迁移凭证已立即失效。", Actions: []string{"状态", "背包", "位置", "道籍迁移"}}, true, nil
}

func migrateReferralAccountTx(tx *gorm.DB, playerID uint, oldAccountID, newAccountID string) error {
	var conflicts int64
	if err := tx.Model(&model.ReferralCode{}).
		Where("account_id = ? AND current_player_id <> ?", newAccountID, playerID).
		Count(&conflicts).Error; err != nil {
		return err
	}
	if conflicts > 0 {
		return errMigrationTargetBusy
	}
	if err := tx.Model(&model.ReferralBinding{}).
		Where("invitee_account_id = ? AND invitee_player_id <> ?", newAccountID, playerID).
		Count(&conflicts).Error; err != nil {
		return err
	}
	if conflicts > 0 {
		return errMigrationTargetBusy
	}
	if err := tx.Model(&model.ReferralClaim{}).Where("account_id = ?", newAccountID).Count(&conflicts).Error; err != nil {
		return err
	}
	if conflicts > 0 {
		return errMigrationTargetBusy
	}
	if err := tx.Model(&model.ReferralCode{}).
		Where("current_player_id = ? OR account_id = ?", playerID, oldAccountID).
		Updates(map[string]any{"account_id": newAccountID, "current_player_id": playerID}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ReferralBinding{}).
		Where("invitee_player_id = ? OR invitee_account_id = ?", playerID, oldAccountID).
		Update("invitee_account_id", newAccountID).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ReferralBinding{}).
		Where("inviter_player_id = ? OR inviter_account_id = ?", playerID, oldAccountID).
		Update("inviter_account_id", newAccountID).Error; err != nil {
		return err
	}
	return tx.Model(&model.ReferralClaim{}).Where("account_id = ?", oldAccountID).Update("account_id", newAccountID).Error
}
