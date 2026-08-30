package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

type itemEffectParameters struct {
	MaxHealthPercent      float64 `json:"max_health_percent"`
	MaxManaPercent        float64 `json:"max_mana_percent"`
	CultivationMultiplier float64 `json:"cultivation_multiplier"`
	DaoHeart              float64 `json:"dao_heart"`
	Rate                  float64 `json:"rate"`
	Minutes               int64   `json:"minutes"`
	DurationMinutes       int64   `json:"duration_minutes"`
}

type activeItemBonus struct {
	CultivationMultiplier float64
	DaoHeart              int64
	BreakthroughRate      float64
	TribulationRate       float64
	AgilityRate           float64
	DefenseRate           float64
}

type activeMedicineEffect struct {
	Item      model.Item
	Stacks    int64
	ExpiresAt time.Time
}

func decodeItemEffectParameters(item model.Item) itemEffectParameters {
	var parameters itemEffectParameters
	_ = json.Unmarshal([]byte(item.EffectParams), &parameters)
	return parameters
}

func itemEffectDuration(item model.Item) time.Duration {
	parameters := decodeItemEffectParameters(item)
	minutes := parameters.Minutes
	if minutes <= 0 {
		minutes = parameters.DurationMinutes
	}
	if minutes <= 0 {
		minutes = 30
	}
	return time.Duration(minutes) * time.Minute
}

func itemHealingAmount(item model.Item, maximumHealth int64) int64 {
	parameters := decodeItemEffectParameters(item)
	if parameters.MaxHealthPercent > 0 {
		percentage := clampFloat(parameters.MaxHealthPercent, 1, 100)
		return max64(int64(math.Ceil(float64(maximumHealth)*percentage/100)), 1)
	}
	return max64(int64(math.Round(item.EffectValue)), 1)
}

func itemManaRecoveryAmount(item model.Item, maximumMana int64) int64 {
	parameters := decodeItemEffectParameters(item)
	if parameters.MaxManaPercent > 0 {
		percentage := clampFloat(parameters.MaxManaPercent, 1, 100)
		return max64(int64(math.Ceil(float64(maximumMana)*percentage/100)), 1)
	}
	return max64(int64(math.Round(item.EffectValue)), 1)
}

func itemRootRefineGain(item model.Item) int64 {
	gain := int64(2 + math.Floor(item.EffectValue/250))
	return min64(max64(gain, 2), 10)
}

func itemTemporaryStatValue(item model.Item) int64 {
	value := int64(math.Round(item.EffectValue / 10))
	return min64(max64(value, 5), 50)
}

func itemTemporaryStatRate(item model.Item) float64 {
	return clampFloat(item.EffectValue/1000, .05, .50)
}

func itemChanceBonus(item model.Item) float64 {
	parameters := decodeItemEffectParameters(item)
	if parameters.Rate > 0 {
		return clampFloat(parameters.Rate, .01, .50)
	}
	return clampFloat(item.EffectValue/1000, .03, .25)
}

func itemEffectSummary(item model.Item, quantity int64) string {
	if quantity < 1 {
		quantity = 1
	}
	parameters := decodeItemEffectParameters(item)
	duration := itemEffectDuration(item)
	minutes := int64(duration / time.Minute)
	switch item.EffectFunc {
	case "heal_hp":
		if parameters.MaxHealthPercent > 0 {
			return fmt.Sprintf("每份恢复最大气血%.0f%%；连续服用%d份时逐份结算，气血不会超过上限", parameters.MaxHealthPercent, quantity)
		}
		return fmt.Sprintf("每份恢复气血%.0f点；%d份理论药力%.0f点", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "restore_mana":
		if parameters.MaxManaPercent > 0 {
			return fmt.Sprintf("每颗恢复最大法力%.0f%%；连续服用%d颗时逐颗结算，法力不会超过上限", parameters.MaxManaPercent, quantity)
		}
		return fmt.Sprintf("每颗恢复法力%.0f点；%d颗理论药力%.0f点", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "add_cultivation":
		return fmt.Sprintf("每颗修为+%.0f；%d颗合计修为+%.0f", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "add_spirit":
		return fmt.Sprintf("每份神识+%.0f；%d份合计神识+%.0f", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "add_perception":
		return fmt.Sprintf("每份悟性+%.0f；%d份合计悟性+%.0f", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "add_lifespan":
		return fmt.Sprintf("每份寿元+%.0f年；%d份合计寿元+%.0f年", item.EffectValue, quantity, item.EffectValue*float64(quantity))
	case "revive":
		return "濒死时恢复全部气血，并同步当前回合战斗状态"
	case "root_refine":
		gain := itemRootRefineGain(item)
		return fmt.Sprintf("每份灵根纯度+%d；%d份最多提升%d点，纯度上限100", gain, quantity, gain*quantity)
	case "breakthrough_bonus":
		rate := itemChanceBonus(item) * 100
		return fmt.Sprintf("每份小境突破成功率+%.1f%%，持续%d分钟；%d份叠加+%.1f%%", rate, minutes, quantity, rate*float64(quantity))
	case "tribulation_bonus":
		rate := itemChanceBonus(item) * 100
		return fmt.Sprintf("每份渡劫成功率+%.1f%%，持续%d分钟；%d份叠加+%.1f%%", rate, minutes, quantity, rate*float64(quantity))
	case "temporary_buff":
		if parameters.CultivationMultiplier > 1 {
			combined := 1 + (parameters.CultivationMultiplier-1)*float64(quantity)
			return fmt.Sprintf("每份提供闭关倍率×%.2f，持续%d分钟；%d份同时使用后的总倍率×%.2f", parameters.CultivationMultiplier, minutes, quantity, combined)
		}
		if parameters.DaoHeart > 0 {
			return fmt.Sprintf("每份临时道心+%.0f，持续%d分钟；%d份叠加+%.0f", parameters.DaoHeart, minutes, quantity, parameters.DaoHeart*float64(quantity))
		}
		switch item.EffectType {
		case "道心":
			value := itemTemporaryStatValue(item)
			return fmt.Sprintf("每份临时道心+%d，持续%d分钟；%d份叠加+%d", value, minutes, quantity, value*quantity)
		case "身法":
			rate := itemTemporaryStatRate(item) * 100
			return fmt.Sprintf("每份战斗身法+%.1f%%，持续%d分钟；%d份叠加+%.1f%%", rate, minutes, quantity, rate*float64(quantity))
		case "防御":
			rate := itemTemporaryStatRate(item) * 100
			return fmt.Sprintf("每份物防与法抗+%.1f%%，持续%d分钟；%d份叠加+%.1f%%", rate, minutes, quantity, rate*float64(quantity))
		}
	}
	if item.EffectValue != 0 {
		return fmt.Sprintf("%s %.0f（%d份累计%.0f）", displayOr(item.EffectType, "药力"), item.EffectValue, quantity, item.EffectValue*float64(quantity))
	}
	return displayOr(item.Description, "该物品没有可直接触发的药效")
}

func parseItemBuffStacks(value string) int64 {
	marker := "stacks="
	index := strings.LastIndex(value, marker)
	if index < 0 {
		return 1
	}
	text := value[index+len(marker):]
	if end := strings.IndexAny(text, "|;, "); end >= 0 {
		text = text[:end]
	}
	stacks, err := strconv.ParseInt(text, 10, 64)
	if err != nil || stacks < 1 {
		return 1
	}
	return stacks
}

func setPlayerItemBuffTx(tx *gorm.DB, playerID uint, item model.Item, stacks int64, expiresAt time.Time) error {
	key := "buff.item." + item.Code
	row := model.PlayerValue{PlayerID: playerID, Key: key, Value: fmt.Sprintf("stacks=%d", stacks), ExpiresAt: &expiresAt}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "player_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "expires_at", "updated_at"}),
	}).Create(&row).Error
}

func (g *Game) activeItemBonuses(playerID uint) activeItemBonus {
	bonus := activeItemBonus{CultivationMultiplier: 1}
	type buffRow struct {
		Value string
		Code  string
	}
	var rows []buffRow
	now := time.Now()
	err := g.store.DB.Table("player_values").
		Select("player_values.value, SUBSTR(player_values.key, 11) AS code").
		Where("player_values.player_id = ? AND player_values.key LIKE ? AND (player_values.expires_at IS NULL OR player_values.expires_at > ?)", playerID, "buff.item.%", now).
		Find(&rows).Error
	if err != nil {
		return bonus
	}
	for _, row := range rows {
		var item model.Item
		if g.store.DB.Where("code = ?", row.Code).First(&item).Error != nil {
			continue
		}
		stacks := parseItemBuffStacks(row.Value)
		parameters := decodeItemEffectParameters(item)
		switch item.EffectFunc {
		case "breakthrough_bonus":
			bonus.BreakthroughRate += itemChanceBonus(item) * float64(stacks)
		case "tribulation_bonus":
			bonus.TribulationRate += itemChanceBonus(item) * float64(stacks)
		case "temporary_buff":
			if parameters.CultivationMultiplier > 1 {
				bonus.CultivationMultiplier += (parameters.CultivationMultiplier - 1) * float64(stacks)
			}
			if parameters.DaoHeart > 0 {
				bonus.DaoHeart += int64(math.Round(parameters.DaoHeart * float64(stacks)))
			}
			switch item.EffectType {
			case "道心":
				bonus.DaoHeart += itemTemporaryStatValue(item) * stacks
			case "身法":
				bonus.AgilityRate += itemTemporaryStatRate(item) * float64(stacks)
			case "防御":
				bonus.DefenseRate += itemTemporaryStatRate(item) * float64(stacks)
			}
		}
	}
	bonus.CultivationMultiplier = clampFloat(bonus.CultivationMultiplier, 1, 1000)
	bonus.BreakthroughRate = clampFloat(bonus.BreakthroughRate, 0, .50)
	bonus.TribulationRate = clampFloat(bonus.TribulationRate, 0, .50)
	bonus.AgilityRate = clampFloat(bonus.AgilityRate, 0, 3)
	bonus.DefenseRate = clampFloat(bonus.DefenseRate, 0, 3)
	return bonus
}

func (g *Game) activeMedicineEffects(playerID uint) ([]activeMedicineEffect, error) {
	var values []model.PlayerValue
	now := time.Now()
	if err := g.store.DB.Where("player_id = ? AND key LIKE ? AND expires_at > ?", playerID, "buff.item.%", now).Order("expires_at, id").Find(&values).Error; err != nil {
		return nil, err
	}
	effects := make([]activeMedicineEffect, 0, len(values))
	for _, value := range values {
		if value.ExpiresAt == nil || !value.ExpiresAt.After(now) {
			continue
		}
		code := strings.TrimPrefix(value.Key, "buff.item.")
		var item model.Item
		if code == value.Key || g.store.DB.Where("code = ?", code).First(&item).Error != nil {
			continue
		}
		effects = append(effects, activeMedicineEffect{Item: item, Stacks: parseItemBuffStacks(value.Value), ExpiresAt: *value.ExpiresAt})
	}
	return effects, nil
}

func activeMedicineBonusText(bonus activeItemBonus) string {
	parts := make([]string, 0, 6)
	if bonus.CultivationMultiplier > 1 {
		parts = append(parts, fmt.Sprintf("闭关×%.2f", bonus.CultivationMultiplier))
	}
	if bonus.DaoHeart > 0 {
		parts = append(parts, fmt.Sprintf("道心+%d", bonus.DaoHeart))
	}
	if bonus.BreakthroughRate > 0 {
		parts = append(parts, fmt.Sprintf("突破+%.1f%%", bonus.BreakthroughRate*100))
	}
	if bonus.TribulationRate > 0 {
		parts = append(parts, fmt.Sprintf("渡劫+%.1f%%", bonus.TribulationRate*100))
	}
	if bonus.AgilityRate > 0 {
		parts = append(parts, fmt.Sprintf("身法+%.1f%%", bonus.AgilityRate*100))
	}
	if bonus.DefenseRate > 0 {
		parts = append(parts, fmt.Sprintf("双防+%.1f%%", bonus.DefenseRate*100))
	}
	return strings.Join(parts, " · ")
}

func medicineAdjustedDisplayStats(player *model.Player, bonus activeItemBonus) (physicalDefense, magicDefense, agility, daoHeart int64) {
	physicalDefense, magicDefense, agility, daoHeart = player.PhysicalDefense, player.MagicDefense, player.Agility, player.DaoHeart+bonus.DaoHeart
	if bonus.DefenseRate > 0 {
		physicalDefense += int64(math.Round(float64(physicalDefense) * bonus.DefenseRate))
		magicDefense += int64(math.Round(float64(magicDefense) * bonus.DefenseRate))
	}
	if bonus.AgilityRate > 0 {
		agility += int64(math.Round(float64(agility) * bonus.AgilityRate))
	}
	return
}

func (g *Game) activeMedicineOverview(player *model.Player) (GameResult, bool, error) {
	effects, err := g.activeMedicineEffects(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if len(effects) == 0 {
		return GameResult{Title: "🧪 当前药效", Content: "当前没有仍在持续的丹药增益。\n━━━━━━━━━━━\n回血、回蓝、修为、神识、悟性、寿元与洗髓类丹药会在服用时立即写入道籍，因此不会留在持续药效列表；服用结果会显示生效前后数值。", Actions: []string{"丹药图鉴", "背包搜索 丹药", "状态", "丹方"}}, true, nil
	}
	bonus := g.activeItemBonuses(player.ID)
	adjusted := g.playerWithActiveSkillStats(player)
	physicalDefense, magicDefense, agility, daoHeart := medicineAdjustedDisplayStats(&adjusted, bonus)
	lines := []string{
		fmt.Sprintf("道友：%s · 生效中%d种", player.DaoName, len(effects)),
		"总药力：" + activeMedicineBonusText(bonus),
		"━━━━━━━━━━━",
	}
	for _, effect := range effects {
		lines = append(lines, fmt.Sprintf("【%s】×%d · 还剩%s\n%s", effect.Item.Name, effect.Stacks, formatDuration(time.Until(effect.ExpiresAt)), itemEffectSummary(effect.Item, effect.Stacks)), "━━━━━━━")
	}
	lines = append(lines,
		fmt.Sprintf("实战双防：%d/%d → %d/%d", adjusted.PhysicalDefense, adjusted.MagicDefense, physicalDefense, magicDefense),
		fmt.Sprintf("实战身法：%d → %d", adjusted.Agility, agility),
		fmt.Sprintf("有效道心：%d → %d", adjusted.DaoHeart, daoHeart),
		"突破、渡劫、闭关与战斗会读取以上有效值；效果到期后自动移除，不会残留到永久面板。",
	)
	return GameResult{Title: "🧪 当前药效", Content: strings.Join(lines, "\n"), Actions: []string{"状态", "备劫", "突破", "修炼", "背包搜索 丹药", "丹药图鉴"}}, true, nil
}

func (g *Game) rebalanceRootQuality(player model.Player, quality int) model.Player {
	quality = minInt(maxInt(quality, 1), 100)
	oldDelta := g.initialRootStatDelta(player.SpiritualRoot, player.RootQuality)
	newDelta := g.initialRootStatDelta(player.SpiritualRoot, quality)
	floor := model.PlayerLevelStats(player.Level)
	skillBonus := g.activeSkillStatBonus(&player)
	oldMaximumHealth := max64(player.MaxHealth+skillBonus.Health, 1)
	oldMaximumMana := max64(player.MaxMana+skillBonus.Mana, 0)
	player.RootQuality = quality
	player.PhysicalAttack = max64(player.PhysicalAttack-oldDelta.PhysicalAttack+newDelta.PhysicalAttack, floor.PhysicalAttack)
	player.MagicAttack = max64(player.MagicAttack-oldDelta.MagicAttack+newDelta.MagicAttack, floor.MagicAttack)
	player.PhysicalDefense = max64(player.PhysicalDefense-oldDelta.PhysicalDefense+newDelta.PhysicalDefense, floor.PhysicalDefense)
	player.MagicDefense = max64(player.MagicDefense-oldDelta.MagicDefense+newDelta.MagicDefense, floor.MagicDefense)
	player.MaxHealth = max64(player.MaxHealth-oldDelta.MaxHealth+newDelta.MaxHealth, floor.MaxHealth)
	player.MaxMana = max64(player.MaxMana-oldDelta.MaxMana+newDelta.MaxMana, floor.MaxMana)
	player.Health = rebalanceCustomizedCurrent(player.Health, oldMaximumHealth, max64(player.MaxHealth+skillBonus.Health, 1))
	player.Mana = rebalanceCustomizedCurrent(player.Mana, oldMaximumMana, max64(player.MaxMana+skillBonus.Mana, 0))
	player.Agility = max64(player.Agility-oldDelta.Agility+newDelta.Agility, floor.Agility)
	player.CritRate = maxFloat(player.CritRate-oldDelta.CritRate+newDelta.CritRate, 0)
	player.CritDamage = maxFloat(player.CritDamage-oldDelta.CritDamage+newDelta.CritDamage, 1)
	player.DamageReduction = maxFloat(player.DamageReduction-oldDelta.DamageReduction+newDelta.DamageReduction, 0)
	player.CombatPower = calculateCombatPower(player)
	return player
}
