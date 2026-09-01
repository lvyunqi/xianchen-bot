package service

import (
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"xianlv/internal/model"
	"xianlv/internal/storage"
	"xianlv/internal/util"
)

type Admin struct {
	*Base
	Players *storage.PlayerRepository
	Couples *storage.CoupleRepository
	Items   *storage.ItemRepository
	Events  *storage.EventRepository
	Tasks   *storage.TaskRepository
	Ranks   *storage.RankRepository
}

func NewAdmin(base *Base) *Admin {
	return &Admin{Base: base, Players: storage.NewPlayerRepository(base.Store.DB), Couples: storage.NewCoupleRepository(base.Store.DB), Items: storage.NewItemRepository(base.Store.DB), Events: storage.NewEventRepository(base.Store.DB), Tasks: storage.NewTaskRepository(base.Store.DB), Ranks: storage.NewRankRepository(base.Store.DB)}
}

func (a *Admin) ListRealms() (rows []model.Realm, err error) {
	err = a.Store.DB.Order("sequence").Find(&rows).Error
	return
}
func (a *Admin) CreateRealm(row *model.Realm) error { return a.Store.DB.Create(row).Error }
func (a *Admin) UpdateRealm(id uint, changes map[string]any) error {
	return a.Store.DB.Model(&model.Realm{}).Where("id = ?", id).Updates(changes).Error
}
func (a *Admin) DeleteRealm(id uint) error {
	var count int64
	if err := a.Store.DB.Model(&model.Player{}).Where("realm_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该境界仍有玩家，不能删除")
	}
	return a.Store.DB.Delete(&model.Realm{}, id).Error
}
func (a *Admin) Setting(key string) (model.SystemSetting, error) {
	var row model.SystemSetting
	err := a.Store.DB.Where("key = ?", key).First(&row).Error
	return row, err
}
func (a *Admin) Settings() (rows []model.SystemSetting, err error) {
	err = a.Store.DB.Order("key").Find(&rows).Error
	return
}
func (a *Admin) SetSetting(key, value, valueType, description, gm string) (model.SystemSetting, error) {
	var row model.SystemSetting
	err := a.Store.DB.Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.SystemSetting{Key: key}
		err = nil
	}
	if err != nil {
		return row, err
	}
	row.Value = value
	row.ValueType = valueType
	row.Description = description
	row.UpdatedBy = gm
	err = a.Store.DB.Save(&row).Error
	return row, err
}

func (a *Admin) Categories() (rows []model.ItemCategory, err error) {
	err = a.Store.DB.Order("sort,name").Find(&rows).Error
	return
}
func (a *Admin) SaveCategory(row *model.ItemCategory) error { return a.Store.DB.Save(row).Error }
func (a *Admin) DeleteCategory(id uint) error {
	var count int64
	a.Store.DB.Model(&model.Item{}).Where("category_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("分类仍包含物品")
	}
	return a.Store.DB.Delete(&model.ItemCategory{}, id).Error
}
func (a *Admin) Rarities() (rows []model.Rarity, err error) {
	err = a.Store.DB.Order("level").Find(&rows).Error
	return
}
func (a *Admin) SaveRarity(row *model.Rarity) error { return a.Store.DB.Save(row).Error }
func (a *Admin) DeleteRarity(id uint) error {
	var count int64
	a.Store.DB.Model(&model.Item{}).Where("rarity_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("稀有度仍被物品使用")
	}
	return a.Store.DB.Delete(&model.Rarity{}, id).Error
}
func (a *Admin) DropPools() (rows []model.DropPool, err error) {
	err = a.Store.DB.Order("id DESC").Find(&rows).Error
	return
}
func (a *Admin) SaveDropPool(row *model.DropPool, entries []model.DropEntry) error {
	return a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		if err := tx.Where("drop_pool_id = ?", row.ID).Delete(&model.DropEntry{}).Error; err != nil {
			return err
		}
		for i := range entries {
			entries[i].ID = 0
			entries[i].DropPoolID = row.ID
			if err := tx.Create(&entries[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (a *Admin) DeleteDropPool(id uint) error {
	return a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("drop_pool_id = ?", id).Delete(&model.DropEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DropPool{}, id).Error
	})
}

func (a *Admin) PlayerDetail(id uint) (map[string]any, error) {
	player, err := a.Players.Get(id)
	if err != nil {
		return nil, err
	}
	inventory, err := a.Players.Inventory(id)
	if err != nil {
		return nil, err
	}
	var tasks []model.PlayerTask
	a.Store.DB.Where("player_id = ?", id).Find(&tasks)
	return map[string]any{"player": player, "inventory": inventory, "tasks": tasks}, nil
}
func (a *Admin) BanPlayer(id uint, banned bool, reason string) error {
	return a.Players.Update(id, map[string]any{"banned": banned, "ban_reason": reason})
}
func (a *Admin) ResetCultivation(id uint) error {
	var realm model.Realm
	if err := a.Store.DB.Order("sequence").First(&realm).Error; err != nil {
		return err
	}
	var next model.Realm
	_ = a.Store.DB.Where("sequence > ?", realm.Sequence).Order("sequence").First(&next).Error
	return a.Players.Update(id, map[string]any{"realm_id": realm.ID, "realm_name": realm.Name, "realm_level": 1, "cultivation": 0, "cultivation_required": realmStageCost(realm, next), "level": 1, "experience": 0, "state": model.PlayerStateIdle})
}
func (a *Admin) UpdatePlayer(id uint, changes map[string]any) error {
	delete(changes, "id")
	delete(changes, "account_id")
	delete(changes, "created_at")
	return a.Players.Update(id, changes)
}

func (a *Admin) CoupleDetail(id uint) (map[string]any, error) {
	couple, err := a.Couples.Get(id)
	if err != nil {
		return nil, err
	}
	aPlayer, _ := a.Players.Get(couple.PlayerAID)
	bPlayer, _ := a.Players.Get(couple.PlayerBID)
	return map[string]any{"couple": couple, "player_a": aPlayer, "player_b": bPlayer}, nil
}
func (a *Admin) CoupleRanking() (rows []model.Couple, err error) {
	err = a.Store.DB.Where("status = ?", model.CoupleStatusActive).Order("affinity DESC").Limit(100).Find(&rows).Error
	return
}

func (a *Admin) Stats() (map[string]int64, error) {
	tables := map[string]any{"players": &model.Player{}, "online_players": &model.Player{}, "couples": &model.Couple{}, "items": &model.Item{}, "events": &model.Event{}, "tasks": &model.TaskTemplate{}}
	out := map[string]int64{}
	for key, table := range tables {
		db := a.Store.DB.Model(table)
		if key == "online_players" {
			db = db.Where("online = ?", true)
		}
		if key == "couples" {
			db = db.Where("status = ?", model.CoupleStatusActive)
		}
		var count int64
		if err := db.Count(&count).Error; err != nil {
			return nil, err
		}
		out[key] = count
	}
	return out, nil
}
func (a *Admin) CreateGM(name, token, permissions string) (model.GMAccount, error) {
	row := model.GMAccount{Name: strings.TrimSpace(name), TokenHash: util.TokenHash(token), Permissions: permissions, Enabled: true}
	err := a.Store.DB.Create(&row).Error
	return row, err
}
func (a *Admin) UpdateGM(id uint, changes map[string]any) error {
	if token, ok := changes["token"].(string); ok && token != "" {
		changes["token_hash"] = util.TokenHash(token)
	}
	delete(changes, "token")
	delete(changes, "id")
	return a.Store.DB.Model(&model.GMAccount{}).Where("id = ?", id).Updates(changes).Error
}
func (a *Admin) DeleteGM(id uint) error { return a.Store.DB.Delete(&model.GMAccount{}, id).Error }
func (a *Admin) ListGMs() (rows []model.GMAccount, err error) {
	err = a.Store.DB.Order("id").Find(&rows).Error
	return
}
func ParseUint(value string) (uint, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	return uint(parsed), err
}
