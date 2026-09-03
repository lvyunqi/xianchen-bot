package storage

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"xianlv/internal/model"
)

type PlayerRepository struct{ db *gorm.DB }

func NewPlayerRepository(db *gorm.DB) *PlayerRepository { return &PlayerRepository{db: db} }
func (r *PlayerRepository) List(query string, offset, limit int) ([]model.Player, int64, error) {
	var players []model.Player
	var total int64
	db := r.db.Model(&model.Player{})
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		db = db.Where("dao_name LIKE ? OR account_id LIKE ? OR realm_name LIKE ?", like, like, like)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&players).Error
	return players, total, err
}
func (r *PlayerRepository) Get(id uint) (model.Player, error) {
	var value model.Player
	err := r.db.First(&value, id).Error
	return value, err
}
func (r *PlayerRepository) GetByAccount(accountID string) (model.Player, error) {
	var value model.Player
	err := r.db.Where("account_id = ?", strings.TrimSpace(accountID)).First(&value).Error
	return value, err
}
func (r *PlayerRepository) Inventory(playerID uint) ([]model.PlayerItem, error) {
	var values []model.PlayerItem
	err := r.db.Where("player_id = ?", playerID).Find(&values).Error
	return values, err
}
func (r *PlayerRepository) Update(id uint, changes map[string]any) error {
	return r.db.Model(&model.Player{}).Where("id = ?", id).Updates(changes).Error
}
func (r *PlayerRepository) AdjustItem(playerID, itemID uint, delta int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var player model.Player
		if err := tx.First(&player, playerID).Error; err != nil {
			return err
		}
		var item model.Item
		if err := tx.First(&item, itemID).Error; err != nil {
			return err
		}
		var row model.PlayerItem
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND item_id = ?", playerID, itemID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if delta < 0 {
				return errors.New("insufficient item quantity")
			}
			row = model.PlayerItem{PlayerID: playerID, ItemID: itemID, Quantity: delta}
			return tx.Create(&row).Error
		}
		if err != nil {
			return err
		}
		if row.Quantity+delta < 0 {
			return errors.New("insufficient item quantity")
		}
		row.Quantity += delta
		if row.Quantity == 0 {
			return tx.Delete(&row).Error
		}
		return tx.Save(&row).Error
	})
}
// CountStrongerThan 战力排行榜前方人数（用于"登顶"判定）：同战力按创建先后排序。
func (r *PlayerRepository) CountStrongerThan(player model.Player) (int64, error) {
	var stronger int64
	err := r.db.Model(&model.Player{}).
		Where("deleted_at IS NULL AND banned = ? AND (combat_power > ? OR (combat_power = ? AND id < ?))",
			false, player.CombatPower, player.CombatPower, player.ID).
		Count(&stronger).Error
	return stronger, err
}

// CountByDaoName 道号占用查重；excludeID>0 时排除自己（改名场景）。
func (r *PlayerRepository) CountByDaoName(daoName string, excludeID uint) (int64, error) {
	var existing int64
	query := r.db.Model(&model.Player{}).Where("dao_name = ?", daoName)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&existing).Error
	return existing, err
}

func (r *PlayerRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var player model.Player
		if err := tx.First(&player, id).Error; err != nil {
			return err
		}
		var couples []model.Couple
		if err := tx.Where("player_a_id = ? OR player_b_id = ?", id, id).Find(&couples).Error; err != nil {
			return err
		}
		for _, couple := range couples {
			partnerID := couple.PlayerAID
			if partnerID == id {
				partnerID = couple.PlayerBID
			}
			if err := tx.Model(&model.Player{}).Where("id = ?", partnerID).Update("couple_id", 0).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Where("player_a_id = ? OR player_b_id = ?", id, id).Delete(&model.Couple{}).Error; err != nil {
			return err
		}
		var mansion model.Mansion
		if err := tx.Where("player_id = ?", id).First(&mansion).Error; err == nil {
			if err := tx.Unscoped().Where("mansion_id = ?", mansion.ID).Delete(&model.MansionCrop{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&mansion).Error; err != nil {
				return err
			}
		}
		var ownedSects []model.Sect
		if err := tx.Where("owner_id = ?", id).Find(&ownedSects).Error; err != nil {
			return err
		}
		for _, sect := range ownedSects {
			if err := tx.Model(&model.Player{}).Where("sect_name = ?", sect.Name).Update("sect_name", "").Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("sect_id = ?", sect.ID).Delete(&model.SectMember{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&sect).Error; err != nil {
				return err
			}
		}
		deletions := []struct {
			model any
			query string
			args  []any
		}{
			{&model.PlayerValue{}, "player_id = ?", []any{id}}, {&model.PlayerExtendedProgress{}, "player_id = ?", []any{id}}, {&model.Pet{}, "player_id = ?", []any{id}},
			{&model.PlayerSkill{}, "player_id = ?", []any{id}}, {&model.PlayerArtifact{}, "player_id = ?", []any{id}},
			{&model.DungeonRun{}, "player_id = ?", []any{id}}, {&model.ArenaRecord{}, "player_id = ?", []any{id}},
			{&model.SectMember{}, "player_id = ?", []any{id}}, {&model.ContentReview{}, "player_id = ?", []any{id}},
			{&model.Friendship{}, "player_id = ? OR friend_id = ?", []any{id, id}},
			{&model.SocialMessage{}, "sender_id = ? OR receiver_id = ?", []any{id, id}},
			{&model.Mentorship{}, "master_id = ? OR disciple_id = ?", []any{id, id}},
			{&model.TradeListing{}, "seller_id = ? OR buyer_id = ?", []any{id, id}},
			{&model.TradeRecord{}, "seller_id = ? OR buyer_id = ?", []any{id, id}},
			{&model.BarterRequest{}, "initiator_id = ? OR recipient_id = ?", []any{id, id}},
			{&model.RankEntry{}, "player_id = ?", []any{id}},
			{&model.AccountMigrationCode{}, "player_id = ?", []any{id}},
		}
		for _, deletion := range deletions {
			if err := tx.Unscoped().Where(deletion.query, deletion.args...).Delete(deletion.model).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("player_id = ?", id).Delete(&model.PlayerItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("player_id = ?", id).Delete(&model.PlayerTask{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&player).Error
	})
}

// adjustAll 全服批量更新（GM 神令），统一排除软删玩家；column 由白名单调用方传入。
func (r *PlayerRepository) adjustAll(column string, value any) error {
	return r.db.Model(&model.Player{}).Where("deleted_at IS NULL").Update(column, value).Error
}

// GrantAllSpiritStones 全服发放灵石（GM"天降灵石"）。
func (r *PlayerRepository) GrantAllSpiritStones(delta int64) error {
	return r.adjustAll("spirit_stones", gorm.Expr("spirit_stones + ?", delta))
}

// ReduceAllCultivationPercent 按百分比削减全服修为（GM"天罚"，percent 为削减量，如 20 表示 -20%）。
func (r *PlayerRepository) ReduceAllCultivationPercent(percent int64) error {
	return r.adjustAll("cultivation", gorm.Expr("cultivation * ? / 100", 100-percent))
}

// ClearAllSpiritStones 清空全服灵石。
func (r *PlayerRepository) ClearAllSpiritStones() error {
	return r.adjustAll("spirit_stones", 0)
}

// ClearAllImmortalJade 清空全服仙金。
func (r *PlayerRepository) ClearAllImmortalJade() error {
	return r.adjustAll("immortal_jade", 0)
}

// BoostAllImmortalAffinity 全服仙缘加成（GM"仙缘令"）。
func (r *PlayerRepository) BoostAllImmortalAffinity(delta int64) error {
	return r.adjustAll("immortal_affinity", gorm.Expr("immortal_affinity + ?", delta))
}

// GrantAllImmortalJade 全服发放仙金。
func (r *PlayerRepository) GrantAllImmortalJade(delta int64) error {
	return r.adjustAll("immortal_jade", gorm.Expr("immortal_jade + ?", delta))
}

// ReduceAllCultivationFixed 全服扣除固定修为（不低于 0）。
func (r *PlayerRepository) ReduceAllCultivationFixed(amount int64) error {
	return r.adjustAll("cultivation", gorm.Expr("CASE WHEN cultivation > ? THEN cultivation - ? ELSE 0 END", amount, amount))
}
