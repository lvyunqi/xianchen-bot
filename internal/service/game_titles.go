package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

func (g *Game) titleEligible(player *model.Player, title model.Title) (bool, string) {
	condition := strings.TrimSpace(title.Condition)
	if condition == "" || strings.Contains(condition, "入道即获得") {
		return true, "已完成入道"
	}
	if strings.HasPrefix(condition, "达到") {
		name := strings.TrimSpace(strings.TrimPrefix(condition, "达到"))
		var required model.Realm
		if g.store.DB.Where("name = ?", name).First(&required).Error == nil {
			sequence, _ := g.playerRealmSequence(player)
			return sequence >= required.Sequence, fmt.Sprintf("需要达到%s，当前%s", name, player.RealmName)
		}
	}
	if strings.Contains(condition, "结为仙侣") {
		return player.CoupleID > 0, "需要先结为仙侣"
	}
	if strings.Contains(condition, "道缘深度达到") {
		var couple model.Couple
		_ = g.store.DB.First(&couple, player.CoupleID).Error
		return couple.Affinity >= 500, fmt.Sprintf("需要道缘500，当前%d", couple.Affinity)
	}
	if strings.Contains(condition, "运气达到50") {
		return player.Luck >= 50, fmt.Sprintf("需要运气50，当前%d", player.Luck)
	}
	if strings.Contains(condition, "战斗100次") {
		value := g.playerValueInt(player.ID, "stats.wins", 0)
		return value >= 100, fmt.Sprintf("需要胜利100场，当前%d", value)
	}
	if strings.Contains(condition, "丹道达到50") {
		value := g.playerValueInt(player.ID, "stats.alchemy", 0)
		return value >= 50, fmt.Sprintf("需要成功炼丹50次，当前%d", value)
	}
	if strings.Contains(condition, "捕获灵兽5只") {
		var count int64
		_ = g.store.DB.Model(&model.Pet{}).Where("player_id = ?", player.ID).Count(&count).Error
		return count >= 5, fmt.Sprintf("需要拥有5只灵兽，当前%d", count)
	}
	if start := strings.Index(condition, "“"); start >= 0 {
		if end := strings.Index(condition[start+len("“"):], "”"); end >= 0 {
			taskName := condition[start+len("“") : start+len("“")+end]
			var count int64
			_ = g.store.DB.Table("player_tasks").Joins("JOIN task_templates ON task_templates.id = player_tasks.task_template_id").Where("player_tasks.player_id = ? AND player_tasks.status = ? AND task_templates.name = ?", player.ID, "已完成", taskName).Count(&count).Error
			return count > 0, "需要完成成就任务“" + taskName + "”"
		}
	}
	if strings.Contains(condition, "渡劫成功") {
		value := g.playerValueInt(player.ID, "stats.tribulations", 0)
		return value > 0, fmt.Sprintf("需要至少成功渡劫一次，当前%d", value)
	}
	if strings.Contains(condition, "飞升成功") {
		value := g.playerValueInt(player.ID, "stats.ascensions", 0)
		return value > 0, "需要完成飞升"
	}
	return false, "尚未满足：“" + condition + "”"
}

func titleUnlockKey(title model.Title) string { return "title.unlocked." + title.Code }

func (g *Game) unlockTitle(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	var title model.Title
	if name == "" || g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&title).Error != nil {
		return GameResult{Title: "🏅 激活称号", Content: "请输入：`激活称号 称号名`，可先查看称号图鉴。", Actions: []string{"称号图鉴", "成就"}}, true, nil
	}
	if g.playerValueExists(player.ID, titleUnlockKey(title)) {
		return GameResult{Title: "🏅 称号已激活", Content: title.Name + "已经收入你的尊号玉册。", Actions: []string{"佩戴称号 " + title.Name, "我的称号"}}, true, nil
	}
	eligible, reason := g.titleEligible(player, title)
	if !eligible {
		return GameResult{Title: "🏅 称号前置未满", Content: fmt.Sprintf("称号：%s\n解锁条件：%s\n当前判定：%s", title.Name, title.Condition, reason), Actions: []string{"图鉴详情 称号 " + title.Name, "成就", "任务菜单"}}, true, nil
	}
	if err := g.setPlayerValue(player.ID, titleUnlockKey(title), "unlocked", nil); err != nil {
		return GameResult{}, true, err
	}
	result := GameResult{Title: "🏅 成就称号激活", Content: fmt.Sprintf("称号：%s【%s】\n解锁：%s\n佩戴属性：%s\n━━━━━━━━━━━\n称号已经收入尊号玉册，需手动佩戴后属性才生效。", title.Name, title.Type, reason, displayConfigText(title.AttributeBonus)), Actions: []string{"佩戴称号 " + title.Name, "我的称号", "称号图鉴"}}
	if isHighTitle(title) && !g.playerValueExists(player.ID, "title.announced."+title.Code) {
		broadcast := fmt.Sprintf("【尊号昭世】恭贺道友%s完成%s，获仙盟敕封高阶称号【%s】，尊号光耀诸界！", player.DaoName, title.Condition, title.Name)
		_ = g.setPlayerValue(player.ID, "title.announced."+title.Code, "unlock", nil)
		_ = g.publishWorldBroadcast("称号", player.DaoName+"获封"+title.Name, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func isHighTitle(title model.Title) bool {
	if title.Type == "隐藏" || title.Type == "渡劫" || title.Type == "飞升" {
		return true
	}
	var values map[string]float64
	_ = json.Unmarshal([]byte(title.AttributeBonus), &values)
	return values["all_percent"] >= 15
}

func (g *Game) myTitles(player *model.Player, raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 6
	var values []model.PlayerValue
	if err := g.store.DB.Where("player_id = ? AND key LIKE ?", player.ID, "title.unlocked.%").Order("id").Find(&values).Error; err != nil {
		return GameResult{}, true, err
	}
	codes := make([]string, 0, len(values))
	for _, value := range values {
		codes = append(codes, strings.TrimPrefix(value.Key, "title.unlocked."))
	}
	var rows []model.Title
	if len(codes) > 0 {
		_ = g.store.DB.Where("code IN ? AND enabled = ?", codes, true).Order("type,id").Find(&rows).Error
	}
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start, end := minInt((page-1)*pageSize, len(rows)), minInt(page*pageSize, len(rows))
	lines := []string{fmt.Sprintf("当前佩戴：%s · 第%d/%d页 · 已激活%d个", displayOr(player.Title, "无"), page, pages, len(rows)), "━━━━━━━━━━━"}
	actions := []string{"称号图鉴", "卸下称号"}
	for _, row := range rows[start:end] {
		mark := ""
		if row.Name == player.Title {
			mark = "【佩戴中】"
		}
		lines = append(lines, fmt.Sprintf("%s%s【%s】\n属性：%s", mark, row.Name, row.Type, displayConfigText(row.AttributeBonus)), "━━━━━━━")
		actions = append(actions, "佩戴称号 "+row.Name, "图鉴详情 称号 "+row.Name)
	}
	if len(rows) == 0 {
		lines = append(lines, "尚未激活成就称号。完成成就后从称号图鉴手动激活。")
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("我的称号 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("我的称号 %d", page+1))
	}
	return GameResult{Title: "🏅 尊号玉册", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) titleStatsForPlayer(player model.Player, title model.Title) equipmentStats {
	var values map[string]float64
	_ = json.Unmarshal([]byte(title.AttributeBonus), &values)
	percent := values["all_percent"] / 100
	return equipmentStats{
		Attack:  int64(values["attack"] + math.Round(float64(player.PhysicalAttack)*percent)),
		Defense: int64(values["defense"] + math.Round(float64(player.PhysicalDefense)*percent)),
		Health:  int64(values["health"] + math.Round(float64(player.MaxHealth)*percent)),
		Mana:    int64(values["mana"] + math.Round(float64(player.MaxMana)*percent)),
		Speed:   int64(values["speed"] + math.Round(float64(player.Agility)*percent)),
		Power:   int64(values["power"]),
	}
}

func (g *Game) wearTitle(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	var title model.Title
	if name == "" || g.store.DB.Where("enabled = ? AND name = ?", true, name).First(&title).Error != nil {
		return GameResult{Title: "🏅 佩戴称号", Content: "请输入：`佩戴称号 称号名`。", Actions: []string{"我的称号", "称号图鉴"}}, true, nil
	}
	if !g.playerValueExists(player.ID, titleUnlockKey(title)) {
		return GameResult{Title: "🏅 称号尚未激活", Content: "请先完成条件并发送“激活称号 " + title.Name + "”。", Actions: []string{"激活称号 " + title.Name, "图鉴详情 称号 " + title.Name}}, true, nil
	}
	skillBonus := g.activeSkillStatBonus(player)
	stats := equipmentStats{}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, player.ID).Error; err != nil {
			return err
		}
		var realm model.Realm
		if err := tx.First(&realm, current.RealmID).Error; err != nil {
			return err
		}
		applied := equipmentStats{}
		var ledger model.PlayerValue
		ledgerErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", current.ID, "title.applied.stats").First(&ledger).Error
		if ledgerErr == nil {
			if err := json.Unmarshal([]byte(ledger.Value), &applied); err != nil {
				return fmt.Errorf("title stats ledger is invalid: %w", err)
			}
		} else if !errors.Is(ledgerErr, gorm.ErrRecordNotFound) {
			return ledgerErr
		}
		base := playerAfterEquipmentStatDifference(current, realm, applied, equipmentStats{}, skillBonus)
		stats = g.titleStatsForPlayer(base, title)
		updated := playerAfterEquipmentStatDifference(base, realm, equipmentStats{}, stats, skillBonus)
		updates := equipmentPlayerStatUpdates(updated)
		updates["title"] = title.Name
		if err := tx.Model(&model.Player{}).Where("id = ?", current.ID).Updates(updates).Error; err != nil {
			return err
		}
		encoded, _ := json.Marshal(stats)
		return upsertPlayerValueTx(tx, current.ID, "title.applied.stats", string(encoded), nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	latest, _ = g.players.Get(player.ID)
	result := GameResult{Title: "🏅 尊号佩戴完成", Content: fmt.Sprintf("原称号：%s\n当前称号：%s【%s】\n属性生效：攻击+%d · 防御+%d · 气血+%d · 法力+%d · 身法+%d\n当前战力：%d\n━━━━━━━━━━━\n旧称号属性已经移除，不会叠加残留。", displayOr(player.Title, "无"), title.Name, title.Type, stats.Attack+stats.Power, stats.Defense, stats.Health, stats.Mana, stats.Speed, latest.CombatPower), Actions: []string{"状态", "我的称号", "卸下称号", "图鉴详情 称号 " + title.Name}}
	if isHighTitle(title) && !g.playerValueExists(player.ID, "title.announced."+title.Code) {
		broadcast := fmt.Sprintf("【尊号临世】高阶称号【%s】择主，%s佩印昭告诸界，当前战力%d！", title.Name, player.DaoName, latest.CombatPower)
		_ = g.setPlayerValue(player.ID, "title.announced."+title.Code, "wear", nil)
		_ = g.publishWorldBroadcast("称号", player.DaoName+"佩戴"+title.Name, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func (g *Game) removeTitle(player *model.Player) (GameResult, bool, error) {
	if strings.TrimSpace(player.Title) == "" {
		return GameResult{Title: "🏅 当前无称号", Content: "没有佩戴称号。", Actions: []string{"我的称号", "称号图鉴"}}, true, nil
	}
	skillBonus := g.activeSkillStatBonus(player)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, player.ID).Error; err != nil {
			return err
		}
		var realm model.Realm
		if err := tx.First(&realm, current.RealmID).Error; err != nil {
			return err
		}
		applied := equipmentStats{}
		var ledger model.PlayerValue
		ledgerErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", current.ID, "title.applied.stats").First(&ledger).Error
		if ledgerErr == nil {
			if err := json.Unmarshal([]byte(ledger.Value), &applied); err != nil {
				return fmt.Errorf("title stats ledger is invalid: %w", err)
			}
		} else if !errors.Is(ledgerErr, gorm.ErrRecordNotFound) {
			return ledgerErr
		}
		updated := playerAfterEquipmentStatDifference(current, realm, applied, equipmentStats{}, skillBonus)
		updates := equipmentPlayerStatUpdates(updated)
		updates["title"] = ""
		if err := tx.Model(&model.Player{}).Where("id = ?", current.ID).Updates(updates).Error; err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, current.ID, "title.applied.stats", "{}", nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	latest, loadErr := g.players.Get(player.ID)
	if loadErr == nil {
		_ = g.syncPlayerCombatPower(&latest)
	}
	return GameResult{Title: "🏅 尊号卸下", Content: fmt.Sprintf("已卸下：%s\n对应属性已经移除，称号仍保留在尊号玉册。", player.Title), Actions: []string{"我的称号", "状态"}}, true, nil
}

func titleAllPercent(title model.Title) float64 {
	var values map[string]float64
	_ = json.Unmarshal([]byte(title.AttributeBonus), &values)
	return values["all_percent"]
}

var _ = strconv.Itoa
