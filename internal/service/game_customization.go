package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

const (
	rootCustomizationVoucher     = "太初灵根定制玉牒"
	titleCustomizationVoucher    = "九霄尊号玉册"
	mansionCustomizationVoucher  = "洞天幻境地契"
	petCustomizationVoucher      = "山海灵兽血契"
	artifactCustomizationVoucher = "万象器灵铭契"
)

var customTitleStats = equipmentStats{Attack: 20, Defense: 12, Health: 120, Mana: 60}

func (g *Game) customizationMenu(player *model.Player) GameResult {
	lines := []string{
		fmt.Sprintf("道友：%s · 当前仙金%d", player.DaoName, player.ImmortalJade),
		"定制凭证从仙金商城购买，购买后仍由玩家主动确认使用；定制会按固定属性预算真实生效，不允许填写任意数值。",
		"━━━━━━━━━━━",
	}
	options := []struct {
		voucher string
		usage   string
		note    string
	}{
		{rootCustomizationVoucher, "定制灵根 灵根名", "从千种灵根图鉴选择；保留当前纯度，不叠加旧灵根属性"},
		{titleCustomizationVoucher, "定制称号 新称号", "首次定制获得攻击+20、防御+12、气血+120、法力+60；改名不重复叠加"},
		{mansionCustomizationVoucher, "定制仙府 新名称", "首次定制繁荣+200，阵法、兽室、仓库各+1级；重复改名不叠加"},
		{petCustomizationVoucher, "定制灵兽 原灵兽名=新名字", "首次血契使当前攻击、防御、体魄各+10%，进化保留且不可重复叠加"},
		{artifactCustomizationVoucher, "定制法宝 原法宝名=新名字", "首次器魂定制+1星；同一件法宝以后改名不重复加星"},
	}
	for _, option := range options {
		item, _ := g.itemByName(option.voucher)
		owned := int64(0)
		if item.ID != 0 {
			owned = g.itemQuantity(player.ID, item.ID)
		}
		price := int64(0)
		var shop model.ShopEntry
		if g.store.DB.Where("item_name = ? AND currency = ? AND enabled = ?", option.voucher, "仙金", true).Order("sort,id").First(&shop).Error == nil {
			price = shop.Price
		}
		priceText := "仙金商会暂未上架"
		if price > 0 {
			priceText = fmt.Sprintf("%d仙金", price)
		}
		lines = append(lines, fmt.Sprintf("【%s】持有%d · %s\n用途：%s\n说明：%s", option.voucher, owned, priceText, option.usage, option.note), "━━━━━━━━━━━")
	}
	lines = append(lines,
		"定制规则：属性按系统固定预算真实写入，玩家不能填写攻击、倍率、掉率等任意数值；同一对象首次定制才加属性，后续改名不叠加。文本命中敏感词会被拒绝且不扣凭证。",
	)
	return GameResult{Title: "仙途定制", Content: strings.Join(lines, "\n"), Actions: []string{"仙金商城", "灵根图鉴", "定制灵根", "定制称号", "定制仙府", "定制灵兽", "定制法宝", "背包"}}
}

func (g *Game) customizeSpiritualRoot(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "定制灵根", Content: "先发送 `灵根图鉴` 按页查看，再发送 `定制灵根 灵根名`。定制只允许选择已启用图鉴，不接受自造数值。", Actions: []string{"灵根图鉴", "定制菜单", "仙金商城"}}, true, nil
	}
	var root model.SpiritualRootTemplate
	if err := g.store.DB.Where("name = ? AND enabled = ?", name, true).First(&root).Error; err != nil {
		return GameResult{Title: "灵根未收录", Content: "千种灵根图鉴中没有找到“" + name + "”，请从图鉴蓝字选择。", Actions: []string{"灵根图鉴", "定制菜单"}}, true, nil
	}
	if player.SpiritualRoot == root.Name {
		return GameResult{Title: "无需重铸", Content: "你当前已经是" + root.Name + "，本次没有扣除定制玉牒。", Actions: []string{"灵根详情 " + root.Name, "定制菜单"}}, true, nil
	}
	if !g.hasCustomizationVoucher(player.ID, rootCustomizationVoucher) {
		return g.missingCustomizationVoucher(rootCustomizationVoucher), true, nil
	}
	previous := player.SpiritualRoot
	rebalanced := g.rebalanceCustomizedRoot(*player, root.Name)
	skillBonus := g.activeSkillStatBonus(player)
	oldMaximumHealth := max64(player.MaxHealth+skillBonus.Health, 1)
	oldMaximumMana := max64(player.MaxMana+skillBonus.Mana, 0)
	newMaximumHealth := max64(rebalanced.MaxHealth+skillBonus.Health, 1)
	newMaximumMana := max64(rebalanced.MaxMana+skillBonus.Mana, 0)
	err := g.applyCustomization(player, rootCustomizationVoucher, "灵根定制", previous+" → "+root.Name, func(tx *gorm.DB) error {
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"spiritual_root":   root.Name,
			"health":           rebalanceCustomizedCurrent(player.Health, oldMaximumHealth, newMaximumHealth),
			"max_health":       rebalanced.MaxHealth,
			"mana":             rebalanceCustomizedCurrent(player.Mana, oldMaximumMana, newMaximumMana),
			"max_mana":         rebalanced.MaxMana,
			"physical_attack":  rebalanced.PhysicalAttack,
			"magic_attack":     rebalanced.MagicAttack,
			"physical_defense": rebalanced.PhysicalDefense,
			"magic_defense":    rebalanced.MagicDefense,
			"agility":          rebalanced.Agility,
			"crit_rate":        rebalanced.CritRate,
			"crit_damage":      rebalanced.CritDamage,
			"damage_reduction": rebalanced.DamageReduction,
			"combat_power":     rebalanced.CombatPower,
		}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	broadcast := fmt.Sprintf("【太初重铸】%s展开灵根玉牒，将%s重铸为%s；灵根纯度仍为%d，道基未因定制额外叠加。", player.DaoName, previous, root.Name, player.RootQuality)
	_ = g.publishWorldBroadcast("定制", player.DaoName+"重铸灵根", broadcast)
	bonus := g.spiritualRootBonuses(root.Name, player.RootQuality)
	return GameResult{Title: "灵根定制完成", Content: fmt.Sprintf("原灵根：%s\n新灵根：%s · %s\n纯度：%d（保留）\n本源道阶：%s（保留）\n修炼加成：+%s\n主加成：%s\n副加成：%s\n━━━━━━━━━━━\n战力重算：%d → %d\n已消耗：%s×1\n规则：旧灵根初始属性已经移除，新灵根图鉴属性重新写入；进化获得的永久成长不会丢失，也不会重复叠加。", previous, root.Name, root.Grade, player.RootQuality, spiritualRootStageName(spiritualRootStage(g.spiritualRootEvolutionValue(player.ID, "evolve"))), bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, player.CombatPower, rebalanced.CombatPower, rootCustomizationVoucher), Actions: []string{"灵根详情 " + root.Name, "状态", "寻脉", "定制菜单"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) rebalanceCustomizedRoot(player model.Player, newRoot string) model.Player {
	oldDelta := g.initialRootStatDelta(player.SpiritualRoot, player.RootQuality)
	newDelta := g.initialRootStatDelta(newRoot, player.RootQuality)
	floor := model.PlayerLevelStats(player.Level)
	player.SpiritualRoot = newRoot
	player.PhysicalAttack = max64(player.PhysicalAttack-oldDelta.PhysicalAttack+newDelta.PhysicalAttack, floor.PhysicalAttack)
	player.MagicAttack = max64(player.MagicAttack-oldDelta.MagicAttack+newDelta.MagicAttack, floor.MagicAttack)
	player.PhysicalDefense = max64(player.PhysicalDefense-oldDelta.PhysicalDefense+newDelta.PhysicalDefense, floor.PhysicalDefense)
	player.MagicDefense = max64(player.MagicDefense-oldDelta.MagicDefense+newDelta.MagicDefense, floor.MagicDefense)
	player.MaxHealth = max64(player.MaxHealth-oldDelta.MaxHealth+newDelta.MaxHealth, floor.MaxHealth)
	player.MaxMana = max64(player.MaxMana-oldDelta.MaxMana+newDelta.MaxMana, floor.MaxMana)
	player.Agility = max64(player.Agility-oldDelta.Agility+newDelta.Agility, floor.Agility)
	player.CritRate = maxFloat(player.CritRate-oldDelta.CritRate+newDelta.CritRate, 0)
	player.CritDamage = maxFloat(player.CritDamage-oldDelta.CritDamage+newDelta.CritDamage, 1)
	player.DamageReduction = maxFloat(player.DamageReduction-oldDelta.DamageReduction+newDelta.DamageReduction, 0)
	player.CombatPower = calculateCombatPower(player)
	return player
}

func (g *Game) initialRootStatDelta(rootName string, quality int) model.Player {
	baseline := model.Player{
		SpiritualRoot: rootName, RootQuality: quality,
		Health: 100, MaxHealth: 100, Mana: 50, MaxMana: 50,
		PhysicalAttack: 10, MagicAttack: 10, PhysicalDefense: 5, MagicDefense: 5,
		Agility: 10, CritRate: .05, CritDamage: 1.5,
	}
	withRoot := baseline
	g.applyInitialSpiritualRootBonus(&withRoot)
	return model.Player{
		MaxHealth:       withRoot.MaxHealth - baseline.MaxHealth,
		MaxMana:         withRoot.MaxMana - baseline.MaxMana,
		PhysicalAttack:  withRoot.PhysicalAttack - baseline.PhysicalAttack,
		MagicAttack:     withRoot.MagicAttack - baseline.MagicAttack,
		PhysicalDefense: withRoot.PhysicalDefense - baseline.PhysicalDefense,
		MagicDefense:    withRoot.MagicDefense - baseline.MagicDefense,
		Agility:         withRoot.Agility - baseline.Agility,
		CritRate:        withRoot.CritRate - baseline.CritRate,
		CritDamage:      withRoot.CritDamage - baseline.CritDamage,
		DamageReduction: withRoot.DamageReduction - baseline.DamageReduction,
	}
}

func rebalanceCustomizedCurrent(current, oldMaximum, newMaximum int64) int64 {
	if oldMaximum <= 0 {
		return min64(max64(current, 1), newMaximum)
	}
	return min64(max64(current*newMaximum/oldMaximum, 1), newMaximum)
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func (g *Game) customizeTitle(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "定制称号", Content: "请输入：`定制称号 新称号`。称号长度2至12个字符，敏感内容会被拒绝且不扣凭证。", Actions: []string{"定制菜单", "仙金商城"}}, true, nil
	}
	if invalid := validateCustomizationName(name, 2, 12); invalid != "" {
		return GameResult{Title: "称号格式不符", Content: invalid, Actions: []string{"定制菜单"}}, true, nil
	}
	if rejected, blocked, err := g.rejectSensitiveContent("称号定制", player, name); err != nil || blocked {
		return rejected, true, err
	}
	customCode := fmt.Sprintf("custom_title_player_%d", player.ID)
	var existing model.Title
	_ = g.store.DB.Where("code = ?", customCode).First(&existing).Error
	enhanced := g.playerValueExists(player.ID, "custom.title.enhanced")
	legacyUpgrade := !enhanced && (player.Title == name || existing.ID != 0 && existing.Name == name || g.legacyCustomizationReviewMatches(player.ID, "称号定制", name))
	if player.Title == name && enhanced {
		return GameResult{Title: "无需定制", Content: "当前已经使用该定制称号，固定属性也已生效；本次没有扣除玉册。", Actions: []string{"状态", "我的称号", "定制菜单"}}, true, nil
	}
	if !legacyUpgrade && !g.hasCustomizationVoucher(player.ID, titleCustomizationVoucher) {
		return g.missingCustomizationVoucher(titleCustomizationVoucher), true, nil
	}
	var duplicate int64
	if err := g.store.DB.Model(&model.Title{}).Where("name = ? AND id <> ?", name, existing.ID).Count(&duplicate).Error; err != nil {
		return GameResult{}, true, err
	}
	if duplicate > 0 {
		return GameResult{Title: "称号已经存在", Content: "该称号名已经收录于尊号玉册，请更换一个专属名称；本次没有扣除凭证。", Actions: []string{"称号图鉴", "定制菜单"}}, true, nil
	}
	previous := displayOr(player.Title, "无称号")
	applied := equipmentStats{}
	if value, err := g.playerValue(player.ID, "title.applied.stats"); err == nil {
		_ = json.Unmarshal([]byte(value), &applied)
	}
	encodedStats, _ := json.Marshal(customTitleStats)
	title := model.Title{Code: customCode, Name: name, Condition: "专属付费定制，仅本人激活", AttributeBonus: string(encodedStats), Type: "定制", Enabled: true}
	skillBonus := g.activeSkillStatBonus(player)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if !legacyUpgrade {
			var voucher model.Item
			if err := tx.Where("name = ?", titleCustomizationVoucher).First(&voucher).Error; err != nil {
				return err
			}
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, voucher.ID, -1); err != nil {
				return err
			}
		}
		if existing.ID == 0 {
			if err := tx.Create(&title).Error; err != nil {
				return err
			}
		} else {
			title.ID = existing.ID
			if err := tx.Model(&model.Title{}).Where("id = ?", existing.ID).Updates(map[string]any{"name": name, "attribute_bonus": string(encodedStats), "condition": title.Condition, "type": title.Type, "enabled": true}).Error; err != nil {
				return err
			}
		}
		if err := applyEquipmentStatDifferenceTx(tx, player.ID, applied, customTitleStats, skillBonus); err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("title", name).Error; err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "title.applied.stats", string(encodedStats), nil); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, titleUnlockKey(title), "unlocked", nil); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "custom.title.enhanced", "true", nil); err != nil {
			return err
		}
		return createCustomizationReviewTx(tx, player, "称号定制", previous+" → "+name+"；固定属性已生效")
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&latest)
	}
	voucherText := titleCustomizationVoucher + "×1"
	if legacyUpgrade {
		voucherText = "旧版已付费称号补发，不重复扣凭证"
	}
	return GameResult{Title: "称号定制完成", Content: fmt.Sprintf("原称号：%s\n新称号：%s\n真实属性：攻击+20 · 防御+12 · 气血+120 · 法力+60\n结算：%s\n━━━━━━━━━━━\n称号已收入尊号玉册并自动佩戴；旧称号属性已经移除。以后更改该专属称号名称不会重复叠加属性。", previous, name, voucherText), Actions: []string{"状态", "我的称号", "定制菜单"}}, true, nil
}

func (g *Game) legacyCustomizationReviewMatches(playerID uint, kind, target string) bool {
	var reviews []model.ContentReview
	if err := g.store.DB.Where("player_id = ? AND type = ?", playerID, kind).Order("id DESC").Limit(20).Find(&reviews).Error; err != nil {
		return false
	}
	target = strings.TrimSpace(target)
	for _, review := range reviews {
		content := strings.TrimSpace(review.Content)
		if separator := strings.LastIndex(content, "→"); separator >= 0 {
			content = strings.TrimSpace(content[separator+len("→"):])
		}
		for _, separator := range []string{"；", ";", "\r", "\n"} {
			if index := strings.Index(content, separator); index >= 0 {
				content = strings.TrimSpace(content[:index])
			}
		}
		content = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(content, "称号："), "称号:"))
		if content == target {
			return true
		}
	}
	return false
}

func (g *Game) customizeMansion(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "定制仙府", Content: "请输入：`定制仙府 新名称`。首次定制获得繁荣+200、阵法/兽室/仓库各+1级；以后改名不叠加，原等级、灵田和仓库内容全部保留。", Actions: []string{"仙府", "定制菜单"}}, true, nil
	}
	if invalid := validateCustomizationName(name, 2, 16); invalid != "" {
		return GameResult{Title: "仙府名格式不符", Content: invalid, Actions: []string{"定制菜单"}}, true, nil
	}
	if rejected, blocked, err := g.rejectSensitiveContent("仙府定制", player, name); err != nil || blocked {
		return rejected, true, err
	}
	var mansion model.Mansion
	if err := g.store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		return GameResult{Title: "尚未建立仙府", Content: "请先发送 `建府` 建立洞府，再使用地契定制名称。", Actions: []string{"建府", "仙府", "定制菜单"}}, true, nil
	}
	enhanced := g.playerValueExists(player.ID, "custom.mansion.enhanced")
	if mansion.Name == name && enhanced {
		return GameResult{Title: "无需定制", Content: "仙府已经使用该名称，洞天定制属性也已生效；本次没有扣除地契。", Actions: []string{"仙府", "定制菜单"}}, true, nil
	}
	if !g.hasCustomizationVoucher(player.ID, mansionCustomizationVoucher) {
		return g.missingCustomizationVoucher(mansionCustomizationVoucher), true, nil
	}
	firstBonus := !enhanced
	err := g.applyCustomization(player, mansionCustomizationVoucher, "仙府定制", mansion.Name+" → "+name, func(tx *gorm.DB) error {
		updates := map[string]any{"name": name}
		if firstBonus {
			updates["prosperity"] = gorm.Expr("prosperity + 200")
			updates["formation_level"] = gorm.Expr("formation_level + 1")
			updates["beast_room_level"] = gorm.Expr("beast_room_level + 1")
			updates["warehouse_level"] = gorm.Expr("warehouse_level + 1")
			if err := upsertPlayerValueTx(tx, player.ID, "custom.mansion.enhanced", "true", nil); err != nil {
				return err
			}
		}
		return tx.Model(&model.Mansion{}).Where("id = ? AND player_id = ?", mansion.ID, player.ID).Updates(updates).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	effect := "重复改名：原有定制属性保持不变，不重复叠加"
	if firstBonus {
		effect = "首次洞天增益：繁荣+200 · 阵法+1级 · 兽室+1级 · 仓库+1级"
	}
	return customizationSuccessWithEffect("仙府", mansion.Name, name, mansionCustomizationVoucher, effect, []string{"仙府", "我的灵田", "定制菜单"}), true, nil
}

func (g *Game) customizePet(player *model.Player, raw string) (GameResult, bool, error) {
	oldName, newName, ok := parseCustomizationPair(raw)
	if !ok {
		return GameResult{Title: "定制灵兽", Content: "请输入：`定制灵兽 原灵兽名=新名字`。例如：`定制灵兽 青丘灵狐=照月`。", Actions: []string{"灵兽", "定制菜单"}}, true, nil
	}
	if invalid := validateCustomizationName(newName, 1, 12); invalid != "" {
		return GameResult{Title: "灵兽名格式不符", Content: invalid, Actions: []string{"灵兽", "定制菜单"}}, true, nil
	}
	if rejected, blocked, err := g.rejectSensitiveContent("灵兽定制", player, newName); err != nil || blocked {
		return rejected, true, err
	}
	var pet model.Pet
	if err := g.store.DB.Where("player_id = ? AND name = ?", player.ID, oldName).First(&pet).Error; err != nil {
		return GameResult{Title: "灵兽不存在", Content: "你没有名为“" + oldName + "”的灵兽，请先发送 `灵兽` 查看。", Actions: []string{"灵兽", "定制菜单"}}, true, nil
	}
	if !g.hasCustomizationVoucher(player.ID, petCustomizationVoucher) {
		return g.missingCustomizationVoucher(petCustomizationVoucher), true, nil
	}
	bonusKey := fmt.Sprintf("custom.pet.%d.bonus", pet.ID)
	bonus := equipmentStats{}
	enhanced := false
	if value, valueErr := g.playerValue(player.ID, bonusKey); valueErr == nil && json.Unmarshal([]byte(value), &bonus) == nil {
		enhanced = true
	}
	if !enhanced {
		bonus = equipmentStats{Attack: max64(int64(math.Ceil(float64(pet.Attack)*.10)), 1), Defense: max64(int64(math.Ceil(float64(pet.Defense)*.10)), 1), Health: max64(int64(math.Ceil(float64(pet.Health)*.10)), 1)}
	}
	err := g.applyCustomization(player, petCustomizationVoucher, "灵兽定制", oldName+" → "+newName, func(tx *gorm.DB) error {
		updates := map[string]any{"name": newName}
		if !enhanced {
			updates["attack"] = gorm.Expr("attack + ?", bonus.Attack)
			updates["defense"] = gorm.Expr("defense + ?", bonus.Defense)
			updates["health"] = gorm.Expr("health + ?", bonus.Health)
			encoded, _ := json.Marshal(bonus)
			if err := upsertPlayerValueTx(tx, player.ID, bonusKey, string(encoded), nil); err != nil {
				return err
			}
		}
		return tx.Model(&model.Pet{}).Where("id = ? AND player_id = ?", pet.ID, player.ID).Updates(updates).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if pet.Active {
		if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
			_ = g.syncPlayerCombatPower(&latest)
		}
	}
	effect := "重复改名：血契属性保持不变，不重复叠加"
	if !enhanced {
		effect = fmt.Sprintf("首次血契增益：攻击+%d · 防御+%d · 体魄+%d（各按当前值10%%）", bonus.Attack, bonus.Defense, bonus.Health)
	}
	return customizationSuccessWithEffect("灵兽", oldName, newName, petCustomizationVoucher, effect, []string{"灵兽", "出战 " + newName, "定制菜单"}), true, nil
}

func (g *Game) customizeArtifact(player *model.Player, raw string) (GameResult, bool, error) {
	oldName, newName, ok := parseCustomizationPair(raw)
	if !ok {
		return GameResult{Title: "定制法宝", Content: "请输入：`定制法宝 原法宝名=新名字`。例如：`定制法宝 青冥剑=照夜`。", Actions: []string{"装备背包", "定制菜单"}}, true, nil
	}
	if invalid := validateCustomizationName(newName, 2, 16); invalid != "" {
		return GameResult{Title: "法宝名格式不符", Content: invalid, Actions: []string{"装备背包", "定制菜单"}}, true, nil
	}
	if rejected, blocked, err := g.rejectSensitiveContent("法宝定制", player, newName); err != nil || blocked {
		return rejected, true, err
	}
	var artifact model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ? AND name = ?", player.ID, oldName).First(&artifact).Error; err != nil {
		return GameResult{Title: "法宝不存在", Content: "装备背包中没有“" + oldName + "”，请从自己的法宝名称中选择。", Actions: []string{"装备背包", "定制菜单"}}, true, nil
	}
	var duplicates int64
	_ = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND name = ? AND id <> ?", player.ID, newName, artifact.ID).Count(&duplicates).Error
	if duplicates > 0 {
		return GameResult{Title: "法宝名重复", Content: "你的装备背包中已有同名法宝，请换一个名称，避免穿戴与强化指令发生歧义。", Actions: []string{"装备背包", "定制菜单"}}, true, nil
	}
	if !g.hasCustomizationVoucher(player.ID, artifactCustomizationVoucher) {
		return g.missingCustomizationVoucher(artifactCustomizationVoucher), true, nil
	}
	bonusKey := fmt.Sprintf("custom.artifact.%d.enhanced", artifact.ID)
	enhanced := g.playerValueExists(player.ID, bonusKey)
	beforeStats := g.equipmentStats(artifact)
	updatedArtifact := artifact
	if !enhanced {
		updatedArtifact.StarLevel = minInt(updatedArtifact.StarLevel+1, 20)
	}
	afterStats := g.equipmentStats(updatedArtifact)
	skillBonus := g.activeSkillStatBonus(player)
	err := g.applyCustomization(player, artifactCustomizationVoucher, "法宝定制", oldName+" → "+newName, func(tx *gorm.DB) error {
		updates := map[string]any{"name": newName}
		if !enhanced {
			updates["star_level"] = updatedArtifact.StarLevel
			if err := upsertPlayerValueTx(tx, player.ID, bonusKey, "true", nil); err != nil {
				return err
			}
			if artifact.Equipped {
				if err := applyEquipmentStatDifferenceTx(tx, player.ID, beforeStats, afterStats, skillBonus); err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.PlayerArtifact{}).Where("id = ? AND player_id = ?", artifact.ID, player.ID).Updates(updates).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&latest)
	}
	effect := "重复改名：器魂属性保持不变，不重复叠加"
	if !enhanced {
		effect = fmt.Sprintf("首次器魂觉醒：星阶%d → %d，装备基础属性额外提高15%%", artifact.StarLevel, updatedArtifact.StarLevel)
	}
	return customizationSuccessWithEffect("法宝", oldName, newName, artifactCustomizationVoucher, effect, []string{"装备背包", "穿戴 " + newName, "定制菜单"}), true, nil
}

func (g *Game) hasCustomizationVoucher(playerID uint, voucherName string) bool {
	item, err := g.itemByName(voucherName)
	return err == nil && g.itemQuantity(playerID, item.ID) > 0
}

func (g *Game) applyCustomization(player *model.Player, voucherName, kind, content string, mutate func(*gorm.DB) error) error {
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		var voucher model.Item
		if err := tx.Where("name = ?", voucherName).First(&voucher).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, voucher.ID, -1); err != nil {
			return err
		}
		if err := mutate(tx); err != nil {
			return err
		}
		return createCustomizationReviewTx(tx, player, kind, content)
	})
}

func createCustomizationReviewTx(tx *gorm.DB, player *model.Player, kind, content string) error {
	review := model.ContentReview{Type: kind, PlayerID: player.ID, PlayerName: player.DaoName, Content: content, Status: "已通过", Reason: "付费定制指令已自动审核并完成"}
	return tx.Create(&review).Error
}

func (g *Game) missingCustomizationVoucher(voucherName string) GameResult {
	return GameResult{Title: "缺少定制凭证", Content: "本次定制需要“" + voucherName + "”×1。未持有凭证时不会修改数据，也不会扣除仙金。", Actions: []string{"仙金商城", "物品 " + voucherName, "定制菜单", "充值菜单"}}
}

func customizationSuccessWithEffect(kind, oldName, newName, voucher, effect string, actions []string) GameResult {
	return GameResult{Title: kind + "定制完成", Content: fmt.Sprintf("原%s：%s\n新%s：%s\n已消耗：%s×1\n实际属性：%s\n━━━━━━━━━━━\n原有等级、品质、强化、进度与归属全部保留；同一对象的定制属性只在首次绑定时增加一次。", kind, oldName, kind, newName, voucher, effect), Actions: actions}
}

func parseCustomizationPair(raw string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
	if len(parts) != 2 {
		parts = strings.SplitN(strings.TrimSpace(raw), "＝", 2)
	}
	if len(parts) != 2 {
		return "", "", false
	}
	oldName, newName := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	return oldName, newName, oldName != "" && newName != "" && oldName != newName
}

func validateCustomizationName(name string, minimum, maximum int) string {
	length := utf8.RuneCountInString(strings.TrimSpace(name))
	if length < minimum || length > maximum {
		return fmt.Sprintf("名称长度必须为%d至%d个字符，当前%d个字符。", minimum, maximum, length)
	}
	if strings.ContainsAny(name, "\r\n\t`[](){}<>") {
		return "名称不能包含换行、链接或模板控制符。"
	}
	return ""
}
