package service

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeSect(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 101:
		return g.createSect(player, command.RawArguments)
	case 102:
		return g.joinSect(player, command.RawArguments)
	case 103:
		return g.leaveSect(player)
	case 104:
		return g.sectInfo(player)
	case 105:
		return g.sectTask(player)
	case 106:
		return g.sectContribution(player)
	case 107:
		return g.sectStore(player, command.RawArguments)
	case 108:
		return g.sectWar(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) createSect(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "创建宗门", Content: "请输入：`创宗 宗门名`"}, true, nil
	}
	if player.SectName != "" {
		return GameResult{Title: "创建失败", Content: "你已加入宗门。"}, true, nil
	}
	if player.Cultivation < 5000 || player.SpiritStones < 1000 {
		return GameResult{Title: "开宗条件不足", Content: fmt.Sprintf("需要5000修为和1000灵石。\n当前：%d修为，%d灵石。", player.Cultivation, player.SpiritStones)}, true, nil
	}
	sect := model.Sect{Name: name, OwnerID: player.ID, Level: 1, Funds: 1000, Reputation: 0, MemberLimit: 20}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sect).Error; err != nil {
			return err
		}
		member := model.SectMember{SectID: sect.ID, PlayerID: player.ID, Position: "宗主", JoinedAt: time.Now()}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return tx.Model(player).Updates(map[string]any{"sect_name": name, "cultivation": gorm.Expr("cultivation - 5000"), "spirit_stones": gorm.Expr("spirit_stones - 1000")}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "开宗立派", Content: fmt.Sprintf("宗门：**%s**\n宗主：%s\n等级：1\n成员上限：20\n宗门资金：1000", sect.Name, player.DaoName), Actions: []string{"宗门", "宗务"}}, true, nil
}

func (g *Game) joinSect(player *model.Player, argument string) (GameResult, bool, error) {
	if player.SectName != "" {
		return GameResult{Title: "入宗失败", Content: "你已加入" + player.SectName + "。"}, true, nil
	}
	if until, err := g.playerValue(player.ID, "sect.leave_until"); err == nil {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, until); parseErr == nil && parsed.After(time.Now()) {
			return GameResult{Title: "入宗冷却", Content: "退出宗门后还需" + formatDuration(time.Until(parsed)) + "才能加入新宗门。"}, true, nil
		}
	}
	var sect model.Sect
	if err := g.store.DB.Where("name = ?", strings.TrimSpace(argument)).First(&sect).Error; err != nil {
		return GameResult{Title: "宗门不存在", Content: "请输入正确宗门名。"}, true, nil
	}
	var count int64
	g.store.DB.Model(&model.SectMember{}).Where("sect_id = ?", sect.ID).Count(&count)
	if count >= int64(sect.MemberLimit) {
		return GameResult{Title: "宗门已满", Content: "该宗门成员已达上限。"}, true, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		member := model.SectMember{SectID: sect.ID, PlayerID: player.ID, Position: "弟子", JoinedAt: time.Now()}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return tx.Model(player).Update("sect_name", sect.Name).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "加入宗门", Content: fmt.Sprintf("你已通过山门考核，加入 **%s**。\n职位：弟子", sect.Name), Actions: []string{"宗门", "宗务"}}, true, nil
}

func (g *Game) leaveSect(player *model.Player) (GameResult, bool, error) {
	if player.SectName == "" {
		return GameResult{Title: "退出宗门", Content: "你是散修。"}, true, nil
	}
	var member model.SectMember
	if err := g.store.DB.Where("player_id = ?", player.ID).First(&member).Error; err != nil {
		return GameResult{}, true, err
	}
	if member.Position == "宗主" {
		return GameResult{Title: "退出失败", Content: "宗主需先完成宗主传位或解散宗门。"}, true, nil
	}
	oldName := player.SectName
	until := time.Now().Add(24 * time.Hour)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&member).Error; err != nil {
			return err
		}
		return tx.Model(player).Update("sect_name", "").Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.setPlayerValue(player.ID, "sect.leave_until", until.Format(time.RFC3339Nano), &until)
	return GameResult{Title: "退出宗门", Content: fmt.Sprintf("你已离开%s。\n24小时内不可加入其他宗门。", oldName)}, true, nil
}

func (g *Game) playerSect(playerID uint) (model.Sect, model.SectMember, error) {
	var member model.SectMember
	if err := g.store.DB.Where("player_id = ?", playerID).First(&member).Error; err != nil {
		return model.Sect{}, member, err
	}
	var sect model.Sect
	err := g.store.DB.First(&sect, member.SectID).Error
	return sect, member, err
}

func (g *Game) sectInfo(player *model.Player) (GameResult, bool, error) {
	sect, member, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "散修", Content: "你尚未加入宗门。", Actions: []string{"创宗", "入宗"}}, true, nil
	}
	var count int64
	g.store.DB.Model(&model.SectMember{}).Where("sect_id = ?", sect.ID).Count(&count)
	var owner model.Player
	_ = g.store.DB.First(&owner, sect.OwnerID).Error
	return GameResult{Title: sect.Name, Content: fmt.Sprintf("宗主：%s\n等级：%d\n成员：%d/%d\n资金：%d\n声望：%d\n你的职位：%s\n你的贡献：%d", owner.DaoName, sect.Level, count, sect.MemberLimit, sect.Funds, sect.Reputation, member.Position, member.Contribution), Actions: []string{"宗务", "贡献", "宗商"}}, true, nil
}

func (g *Game) sectTask(player *model.Player) (GameResult, bool, error) {
	sect, member, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "宗门任务", Content: "先加入宗门。"}, true, nil
	}
	today := time.Now().Format("2006-01-02")
	if done, _ := g.playerValue(player.ID, "sect.task_date"); done == today {
		return GameResult{Title: "宗务已毕", Content: "今日宗门任务已经完成。\n当前贡献：" + fmt.Sprint(member.Contribution)}, true, nil
	}
	reward := int64(20 + sect.Level*5)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&member).Update("contribution", gorm.Expr("contribution + ?", reward)).Error; err != nil {
			return err
		}
		return tx.Model(&sect).Updates(map[string]any{"funds": gorm.Expr("funds + 100"), "reputation": gorm.Expr("reputation + 5")}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.setPlayerValue(player.ID, "sect.task_date", today, nil)
	_, _ = g.addPlayerValueInt(player.ID, "stats.sect_patrol", 1)
	return GameResult{Title: "宗门任务完成", Content: fmt.Sprintf("你巡查灵脉并修复护山阵纹。\n贡献：+%d\n宗门资金：+100\n宗门声望：+5", reward), Actions: []string{"贡献", "宗商"}}, true, nil
}

func (g *Game) sectContribution(player *model.Player) (GameResult, bool, error) {
	sect, member, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "宗门贡献", Content: "你尚未加入宗门。"}, true, nil
	}
	return GameResult{Title: "宗门贡献", Content: fmt.Sprintf("宗门：%s\n职位：%s\n贡献：%d\n可在宗商兑换灵果（20贡献）或仙露（15贡献）。", sect.Name, member.Position, member.Contribution), Actions: []string{"宗商 灵果", "宗商 仙露"}}, true, nil
}

func (g *Game) sectStore(player *model.Player, argument string) (GameResult, bool, error) {
	_, member, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "宗门商店", Content: "你尚未加入宗门。"}, true, nil
	}
	name := strings.TrimSpace(argument)
	if name == "" {
		return g.sectStorePage(player, member, 1)
	}
	if page := int(parsePositiveInt(name, 0)); page > 0 && strconv.Itoa(page) == name {
		return g.sectStorePage(player, member, page)
	}
	quantity := int64(1)
	parsedName, parsedQuantity, parseErr := parseShopPurchase(strings.Fields(name))
	if parseErr == nil {
		name, quantity = parsedName, parsedQuantity
	}
	var shop model.ShopEntry
	shopErr := g.store.DB.Where("item_name = ? AND currency = ? AND enabled = ?", name, "贡献", true).Order("sort,id").First(&shop).Error
	cost := int64(0)
	if shopErr == nil {
		cost = shop.Price
	} else {
		// 保留宗门基础补给，即使尚未导入贡献货架也可正常使用。
		cost = map[string]int64{"灵果": 20, "仙露": 15, "灵茶": 30}[name]
	}
	item, itemErr := g.itemByName(name)
	if itemErr != nil || cost <= 0 {
		return GameResult{Title: "宗门兑换失败", Content: "宗门商店没有上架“" + name + "”，请从宗商列表蓝字选择。", Actions: []string{"宗商", "贡献"}}, true, nil
	}
	total := cost * quantity
	if quantity <= 0 || total < 0 {
		return GameResult{Title: "兑换数量错误", Content: "数量必须是正整数，可使用 `宗商 物品名*数量`。", Actions: []string{"宗商"}}, true, nil
	}
	if member.Contribution < total {
		return GameResult{Title: "贡献不足", Content: fmt.Sprintf("兑换%s×%d需要%d贡献，当前%d。", name, quantity, total, member.Contribution), Actions: []string{"宗务", "宗商", "贡献"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SectMember{}).Where("id = ? AND contribution >= ?", member.ID, total).Update("contribution", gorm.Expr("contribution - ?", total))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if err == errInsufficientCurrency {
			return GameResult{Title: "贡献不足", Content: "兑换时贡献余额发生变化，请重新查看宗门商店。", Actions: []string{"宗商", "贡献"}}, true, nil
		}
		return GameResult{}, true, err
	}
	return GameResult{Title: "⛩️ 宗门兑换", Content: fmt.Sprintf("获得：%s×%d\n消耗贡献：%d\n剩余贡献：%d\n物品已收入乾坤袋。", item.Name, quantity, total, member.Contribution-total), Actions: []string{"物品 " + item.Name, "背包", "宗商", "贡献"}}, true, nil
}

func (g *Game) sectStorePage(player *model.Player, member model.SectMember, page int) (GameResult, bool, error) {
	const pageSize = 8
	page = maxInt(page, 1)
	rows, total, err := g.shop.ListEnabledPaged(storage.ShopFilter{Currency: "贡献"}, page, pageSize)
	if err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	lines := []string{fmt.Sprintf("当前贡献：%d · 第%d/%d页", member.Contribution, page, pages), "贡献只来自宗务、宗战和宗门协作，用于兑换宗门物资。", "━━━━━━━━━━━"}
	actions := []string{"贡献", "宗务"}
	for _, row := range rows {
		var item model.Item
		_ = g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).First(&item).Error
		lines = append(lines, fmt.Sprintf("- %s · %d贡献 · 常设不限购\n  %s", row.ItemName, row.Price, displayOr(item.Description, "宗门专用物资。")))
		actions = append(actions, "宗商 "+row.ItemName)
	}
	if len(rows) == 0 {
		lines = append(lines, "宗门贡献货架暂未开放，仍可兑换基础补给：灵果20贡献、仙露15贡献、灵茶30贡献。")
		actions = append(actions, "宗商 灵果", "宗商 仙露", "宗商 灵茶")
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("宗商 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("宗商 %d", page+1))
	}
	return GameResult{Title: "⛩️ 宗门商店", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) sectWar(player *model.Player, argument string) (GameResult, bool, error) {
	sect, _, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "宗战失败", Content: "你尚未加入宗门。"}, true, nil
	}
	name := strings.TrimSpace(strings.TrimPrefix(argument, "@"))
	var target model.Sect
	if g.store.DB.Where("name = ?", name).First(&target).Error != nil || target.ID == sect.ID {
		return GameResult{Title: "宗战", Content: "请输入：`宗战 @宗门名`"}, true, nil
	}
	power := g.sectPower(sect.ID)
	targetPower := g.sectPower(target.ID)
	winner := sect.Name
	if power < targetPower {
		winner = target.Name
	}
	if power == targetPower && randomPercent() > 50 {
		winner = target.Name
	}
	return GameResult{Title: "宗门大战", Content: fmt.Sprintf("%s战力：%d\n%s战力：%d\n胜方：**%s**", sect.Name, power, target.Name, targetPower, winner)}, true, nil
}

func (g *Game) sectPower(sectID uint) int64 {
	var total int64
	g.store.DB.Table("sect_members").Select("COALESCE(SUM(players.combat_power),0)").Joins("JOIN players ON players.id = sect_members.player_id").Where("sect_members.sect_id = ?", sectID).Scan(&total)
	return total
}

func (g *Game) executeAlchemy(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 109:
		const pageSize = 6
		page := maxInt(int(parsePositiveInt(strings.TrimSpace(command.RawArguments), 1)), 1)
		query := g.store.DB.Model(&model.AlchemyRecipe{}).Where("enabled = ?", true)
		var total int64
		if err := query.Count(&total).Error; err != nil {
			return GameResult{}, true, err
		}
		pages := maxInt(int((total+pageSize-1)/pageSize), 1)
		if page > pages {
			page = pages
		}
		var rows []model.AlchemyRecipe
		if err := query.Order("id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			return GameResult{}, true, err
		}
		lines := []string{"丹方按用途、材料、产物与成功率独立配置，点击蓝字可学习。", "━━━━━━━━━━━"}
		actions := []string{}
		for _, row := range rows {
			effect := "产物药效尚未登记"
			var output model.Item
			if g.store.DB.Where("id = ? OR name = ?", row.OutputItemID, row.OutputName).Order("id").First(&output).Error == nil {
				effect = itemEffectSummary(output, 1)
			}
			lines = append(lines, fmt.Sprintf("- %s\n  材料：%s\n  产物：%s · 成功率%.0f%%\n  实际药效：%s\n  %s", row.Name, displayConfigText(row.MaterialsJSON), row.OutputName, row.SuccessRate*100, effect, row.Description))
			actions = append(actions, "炼药 "+row.Name, "药效 "+row.OutputName, "物品 "+row.OutputName)
		}
		lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d册丹方", page, pages, total))
		if page > 1 {
			actions = append(actions, fmt.Sprintf("丹方 %d", page-1))
		}
		if page < pages {
			actions = append(actions, fmt.Sprintf("丹方 %d", page+1))
		}
		return GameResult{Title: "丹方", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	case 110:
		name := strings.TrimSpace(command.RawArguments)
		var recipe model.AlchemyRecipe
		if g.store.DB.Where("name = ? AND enabled = ?", name, true).First(&recipe).Error != nil {
			return GameResult{Title: "学习失败", Content: "丹方不存在。"}, true, nil
		}
		if player.Cultivation < 50 {
			return GameResult{Title: "学习失败", Content: "学习丹方需要50修为。"}, true, nil
		}
		if err := g.store.DB.Model(player).Update("cultivation", gorm.Expr("cultivation - 50")).Error; err != nil {
			return GameResult{}, true, err
		}
		_ = g.setPlayerValue(player.ID, "recipe."+recipe.Code, "learned", nil)
		return GameResult{Title: "学会丹方", Content: recipe.Name + "\n" + recipe.Description}, true, nil
	case 111:
		name, quantity, parseErr := parseStackQuantity(command.RawArguments)
		if parseErr != nil || strings.TrimSpace(name) == "" {
			return GameResult{Title: "炼制丹药", Content: "请输入：`炼药 丹方名` 或 `炼药 丹方名*数量`。例如：`炼药 回灵丹*99`。", Actions: []string{"丹方", "背包"}}, true, nil
		}
		return g.brewBasicPill(player, name, quantity)
	case 112:
		return g.consumeItem(player, command.RawArguments)
	case 113:
		if strings.TrimSpace(command.RawArguments) == "" {
			return g.activeMedicineOverview(player)
		}
		item, err := g.itemByName(command.RawArguments)
		if err != nil {
			return GameResult{Title: "药效", Content: "未找到该丹药。"}, true, nil
		}
		return GameResult{Title: "药效 · " + item.Name, Content: fmt.Sprintf("品级：%s · 分类：%s\n━━━━━━━━━━━\n实际药效：%s\n说明：%s\n━━━━━━━━━━━\n药效按物品真实配置结算，临时增益会显示持续时间并被修炼、突破、渡劫或战斗读取。", displayOr(item.RarityName, "凡品"), displayOr(item.CategoryName, "未分类"), itemEffectSummary(item, 1), displayOr(item.Description, "暂无说明")), Actions: []string{"使用 " + item.Name, "物品 " + item.Name, "背包", "丹方"}}, true, nil
	case 114:
		if strings.Contains(command.RawArguments, "*") || strings.Contains(command.RawArguments, "×") {
			name, quantity, parseErr := parseStackQuantity(command.RawArguments)
			if parseErr != nil {
				return GameResult{Title: "批量炼制", Content: "请输入：`批炼 丹方名*数量`。", Actions: []string{"丹方"}}, true, nil
			}
			return g.brewBasicPill(player, name, quantity)
		}
		if len(command.Arguments) < 2 {
			return GameResult{Title: "批量炼制", Content: "请输入：`批炼 丹方名*数量` 或 `批炼 丹方名 数量`。"}, true, nil
		}
		quantity := parsePositiveInt(command.Arguments[len(command.Arguments)-1], 0)
		name := strings.Join(command.Arguments[:len(command.Arguments)-1], " ")
		return g.brewBasicPill(player, name, quantity)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) consumeItem(player *model.Player, argument string) (GameResult, bool, error) {
	name, quantity, parseErr := parseStackQuantity(argument)
	if parseErr != nil {
		return GameResult{Title: "使用失败", Content: parseErr.Error(), Actions: []string{"背包"}}, true, nil
	}
	item, err := g.itemByName(name)
	owned := g.itemQuantity(player.ID, item.ID)
	if err != nil || owned < quantity {
		return GameResult{Title: "使用失败", Content: fmt.Sprintf("背包中的%s不足。\n需要：%d\n持有：%d", name, quantity, owned), Actions: []string{"背包", "物品 " + name}}, true, nil
	}
	if item.EffectFunc == "open_gift_pack" {
		if quantity != 1 {
			return GameResult{Title: "礼包开启", Content: "礼包包含多种独立奖励，请发送 `开启礼包 礼包名` 逐次确认开启，避免背包奖励溢出。", Actions: []string{"开启礼包 " + item.Name, "礼包"}}, true, nil
		}
		return g.openGiftPack(player, item.Name)
	}
	if item.EffectFunc == "tribulation_guard" {
		if quantity != 1 {
			return GameResult{Title: "护劫天灯", Content: "护劫天灯每次只能点燃一盏，请发送 `使用 " + item.Name + "`。", Actions: []string{"备劫", "背包"}}, true, nil
		}
		expires := time.Now().Add(30 * time.Minute)
		err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -1); err != nil {
				return err
			}
			value := model.PlayerValue{PlayerID: player.ID, Key: "buff.tribulation_guard", Value: "12", ExpiresAt: &expires}
			return tx.Where("player_id = ? AND key = ?", player.ID, value.Key).Assign(map[string]any{"value": value.Value, "expires_at": value.ExpiresAt}).FirstOrCreate(&value).Error
		})
		if err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "紫府天灯已燃", Content: fmt.Sprintf("护劫灯火将在%s前有效。\n下一次引劫成功率：+12%%\n规则：引劫后灯火熄灭，无论渡劫成功或失败都只生效一次。\n背包剩余：%d", expires.Format("15:04:05"), owned-1), Actions: []string{"备劫", "引劫", "状态"}}, true, nil
	}
	if item.EffectFunc == "plant_seed" || item.EffectFunc == "pet_loyalty" || item.EffectFunc == "teleport" || item.EffectFunc == "rebirth_guard" || item.EffectFunc == "breakthrough_material" || strings.HasPrefix(item.EffectFunc, "customize_") {
		menu := queryCategoryMenu(map[string]string{"plant_seed": "仙药", "pet_loyalty": "灵兽", "teleport": "地图"}[item.EffectFunc])
		if strings.HasPrefix(item.EffectFunc, "customize_") {
			menu = "定制菜单"
		}
		if item.EffectFunc == "breakthrough_material" {
			menu = "合成菜单"
		}
		return GameResult{Title: "此物需在对应玩法使用", Content: fmt.Sprintf("%s不能直接吞服。\n用途：%s\n说明：%s", item.Name, displayOr(item.EffectType, item.CategoryName), item.Description), Actions: []string{"物品 " + item.Name, menu}}, true, nil
	}
	value := int64(item.EffectValue)
	if value > 0 && quantity > (int64(^uint64(0)>>1)/value) {
		return GameResult{Title: "数量过大", Content: "该数量超过系统可安全计算的整数范围，请拆分为多次使用。"}, true, nil
	}
	totalEffect := value * quantity
	updates := map[string]any{}
	effective := g.playerWithActiveSkillStats(player)
	var synchronizedHealth *int64
	var synchronizedMana *int64
	actualVitalText := ""
	var buffExpires *time.Time
	buffStacks := int64(0)
	switch item.EffectFunc {
	case "heal_hp":
		if effective.Health >= effective.MaxHealth {
			return GameResult{Title: "气血已满", Content: item.Name + "没有消耗。当前气血已经达到上限。", Actions: []string{"状态", "背包"}}, true, nil
		}
		perUse := itemHealingAmount(item, effective.MaxHealth)
		if quantity > 0 && perUse > 0 && quantity > int64(^uint64(0)>>1)/perUse {
			return GameResult{Title: "数量过大", Content: "疗伤总量超过安全计算范围，请拆分使用。"}, true, nil
		}
		totalEffect = perUse * quantity
		newHealth := min64(effective.Health+totalEffect, effective.MaxHealth)
		updates["health"] = newHealth
		synchronizedHealth = &newHealth
		totalEffect = newHealth - effective.Health
		actualVitalText = fmt.Sprintf("\n本次气血：%d → %d/%d（实际+%d，已计主修功法上限）", effective.Health, newHealth, effective.MaxHealth, totalEffect)
	case "restore_mana":
		if effective.Mana >= effective.MaxMana {
			return GameResult{Title: "法力已满", Content: item.Name + "没有消耗。当前法力已经达到上限。", Actions: []string{"状态", "背包"}}, true, nil
		}
		perUse := itemManaRecoveryAmount(item, effective.MaxMana)
		if quantity > 0 && perUse > 0 && quantity > int64(^uint64(0)>>1)/perUse {
			return GameResult{Title: "数量过大", Content: "回灵总量超过安全计算范围，请拆分使用。"}, true, nil
		}
		totalEffect = perUse * quantity
		newMana := min64(effective.Mana+totalEffect, effective.MaxMana)
		updates["mana"] = newMana
		synchronizedMana = &newMana
		totalEffect = newMana - effective.Mana
		actualVitalText = fmt.Sprintf("\n本次法力：%d → %d/%d（实际+%d，已计主修功法上限）", effective.Mana, newMana, effective.MaxMana, totalEffect)
	case "add_cultivation":
		updates["cultivation"] = gorm.Expr("cultivation + ?", totalEffect)
	case "add_perception":
		updates["perception"] = gorm.Expr("perception + ?", totalEffect)
	case "add_spirit":
		updates["spirit"] = gorm.Expr("spirit + ?", totalEffect)
	case "add_lifespan":
		updates["lifespan"] = gorm.Expr("lifespan + ?", totalEffect)
	case "revive":
		if player.Health > 0 {
			return GameResult{Title: "魂火尚存", Content: item.Name + "只在濒死时使用，本次没有消耗。", Actions: []string{"状态", "背包"}}, true, nil
		}
		updates["health"] = effective.MaxHealth
		newHealth := effective.MaxHealth
		synchronizedHealth = &newHealth
	case "root_refine":
		gain := itemRootRefineGain(item)
		if quantity > int64(^uint64(0)>>1)/gain {
			return GameResult{Title: "数量过大", Content: "洗炼药力超过安全计算范围，请拆分使用。"}, true, nil
		}
		newQuality := minInt(player.RootQuality+int(gain*quantity), 100)
		if newQuality <= player.RootQuality {
			return GameResult{Title: "灵根已至无垢", Content: "当前灵根纯度已经达到100，本次没有消耗丹药。", Actions: []string{"灵检", "灵根进化菜单", "背包"}}, true, nil
		}
		refined := g.rebalanceRootQuality(*player, newQuality)
		updates = map[string]any{
			"root_quality": refined.RootQuality, "health": refined.Health, "max_health": refined.MaxHealth,
			"mana": refined.Mana, "max_mana": refined.MaxMana, "physical_attack": refined.PhysicalAttack,
			"magic_attack": refined.MagicAttack, "physical_defense": refined.PhysicalDefense,
			"magic_defense": refined.MagicDefense, "agility": refined.Agility, "crit_rate": refined.CritRate,
			"crit_damage": refined.CritDamage, "damage_reduction": refined.DamageReduction, "combat_power": refined.CombatPower,
		}
		newHealth := refined.Health
		synchronizedHealth = &newHealth
		totalEffect = int64(newQuality - player.RootQuality)
	case "temporary_buff", "breakthrough_bonus", "tribulation_bonus":
		priorStacks := int64(0)
		if value, valueErr := g.playerValue(player.ID, "buff.item."+item.Code); valueErr == nil {
			priorStacks = parseItemBuffStacks(value)
		}
		if priorStacks > int64(^uint64(0)>>1)-quantity {
			return GameResult{Title: "数量过大", Content: "叠加层数超过系统可安全计算的范围，请等待当前药效结束。"}, true, nil
		}
		buffStacks = priorStacks + quantity
		expires := time.Now().Add(itemEffectDuration(item))
		buffExpires = &expires
	default:
		return GameResult{Title: "此物不能直接使用", Content: fmt.Sprintf("%s目前没有可直接触发的服用效果。\n用途：%s\n说明：%s", item.Name, displayOr(item.EffectType, item.CategoryName), item.Description), Actions: []string{"物品 " + item.Name, queryCategoryMenu(item.CategoryName), "背包"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -quantity); err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(player).Updates(updates).Error; err != nil {
				return err
			}
		}
		if buffExpires != nil {
			if err := setPlayerItemBuffTx(tx, player.ID, item, buffStacks, *buffExpires); err != nil {
				return err
			}
		}
		return syncPVEBattleVitalsTx(tx, player.ID, synchronizedHealth, synchronizedMana)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	effectText := itemEffectSummary(item, quantity)
	if buffExpires != nil {
		effectText += fmt.Sprintf("\n当前叠加：%d层 · 有效至%s", buffStacks, buffExpires.Format("15:04:05"))
	}
	if item.EffectFunc == "root_refine" {
		effectText += fmt.Sprintf("\n灵根纯度：%d → %d", player.RootQuality, player.RootQuality+int(totalEffect))
	}
	effectText += actualVitalText
	return GameResult{Title: "使用" + item.Name, Content: fmt.Sprintf("数量：%d\n%s\n━━━━━━━━━━━\n实际生效：%s\n背包剩余：%d", quantity, item.Description, effectText, owned-quantity), Actions: []string{"当前药效", "背包", "物品 " + item.Name, "药效 " + item.Name, "状态"}}, true, nil
}

func (g *Game) executeArtifact(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 115:
		name := strings.TrimSpace(command.RawArguments)
		var row model.ArtifactTemplate
		if g.store.DB.Where("name = ? AND enabled = ?", name, true).First(&row).Error != nil {
			return GameResult{Title: "学习器谱", Content: "器谱不存在。"}, true, nil
		}
		if err := g.adjustNamedItem(player.ID, "功法残卷", -1); err != nil {
			return GameResult{Title: "学习失败", Content: "需要功法残卷×1。"}, true, nil
		}
		_ = g.setPlayerValue(player.ID, "artifact_recipe."+row.Code, "learned", nil)
		return GameResult{Title: "器谱入门", Content: fmt.Sprintf("学会炼制%s。\n槽位：%s · 器型：%s\n所需材料：%s", row.Name, artifactTemplateSlot(row), artifactTemplateArchetype(row), row.MaterialsJSON)}, true, nil
	case 116:
		return g.craftArtifact(player, command.RawArguments)
	case 117:
		return g.viewArtifacts(player)
	case 118:
		return g.equipArtifact(player, command.RawArguments, true)
	case 119:
		return g.equipArtifact(player, command.RawArguments, false)
	case 120:
		return g.upgradeArtifact(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) craftArtifact(player *model.Player, name string) (GameResult, bool, error) {
	var template model.ArtifactTemplate
	if g.store.DB.Where("name = ? AND enabled = ?", strings.TrimSpace(name), true).First(&template).Error != nil {
		return GameResult{Title: "炼器失败", Content: "器谱不存在。"}, true, nil
	}
	if value, err := g.playerValue(player.ID, "artifact_recipe."+template.Code); err != nil || value != "learned" {
		return GameResult{Title: "炼器失败", Content: "尚未学习该器谱。"}, true, nil
	}
	if err := g.adjustNamedItem(player.ID, "仙府材料", -3); err != nil {
		return GameResult{Title: "炼器失败", Content: "需要仙府材料×3。"}, true, nil
	}
	qualities := []string{"凡品", "灵品", "仙品", "神品"}
	roll := randomPercent()
	index := 0
	if roll > 70 {
		index = 1
	}
	if roll > 92 {
		index = 2
	}
	if roll > 99 {
		index = 3
	}
	slot := artifactTemplateSlot(template)
	row := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: qualities[index], Slot: slot}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	result := GameResult{Title: "法宝出炉", Content: fmt.Sprintf("法宝：%s\n槽位：%s · 器型：%s\n品质：%s\n等级：1", row.Name, slot, artifactTemplateArchetype(template), row.Quality), Actions: []string{"法宝", "装备 " + row.Name}}
	if row.Quality == "仙品" || row.Quality == "神品" {
		broadcast := fmt.Sprintf("【至宝出世】道友%s集齐珍材，引玄火反复淬炼，独力炼成%s至宝%s，宝光冲霄！", player.DaoName, row.Quality, row.Name)
		_ = g.publishWorldBroadcast("珍宝", player.DaoName+"炼成"+row.Name, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func (g *Game) viewArtifacts(player *model.Player) (GameResult, bool, error) {
	var rows []model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ?", player.ID).Order("equipped DESC,level DESC").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		var templates []model.ArtifactTemplate
		_ = g.store.DB.Where("enabled = ?", true).Find(&templates).Error
		lines := []string{"尚无法宝。可学习器谱："}
		actions := []string{}
		for _, row := range templates {
			lines = append(lines, fmt.Sprintf("- %s【%s · %s】", row.Name, artifactTemplateSlot(row), artifactTemplateArchetype(row)))
			actions = append(actions, "学器 "+row.Name)
		}
		return GameResult{Title: "法宝", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		mark := ""
		if row.Equipped {
			mark = "【装备】"
		}
		slot := g.ensureArtifactSlot(&row)
		var template model.ArtifactTemplate
		_ = g.store.DB.First(&template, row.TemplateID).Error
		lines = append(lines, fmt.Sprintf("- %s%s【%s · %s】 · %s · +%d", mark, row.Name, slot, artifactTemplateArchetype(template), row.Quality, row.Level))
	}
	return GameResult{Title: "法宝", Content: strings.Join(lines, "\n")}, true, nil
}

func (g *Game) equipArtifact(player *model.Player, name string, equip bool) (GameResult, bool, error) {
	return g.changeEquipment(player, name, equip)
}

func (g *Game) upgradeArtifact(player *model.Player, name string) (GameResult, bool, error) {
	var artifact model.PlayerArtifact
	if g.store.DB.Where("player_id = ? AND name = ?", player.ID, strings.TrimSpace(name)).Order("level DESC").First(&artifact).Error != nil {
		return GameResult{Title: "强化失败", Content: "未找到该法宝。"}, true, nil
	}
	var template model.ArtifactTemplate
	_ = g.store.DB.First(&template, artifact.TemplateID).Error
	if artifact.Level >= template.MaxLevel {
		return GameResult{Title: "强化上限", Content: "已达到最高等级。"}, true, nil
	}
	cost := int64(artifact.Level)
	if err := g.adjustNamedItem(player.ID, "仙府材料", -cost); err != nil {
		return GameResult{Title: "强化失败", Content: fmt.Sprintf("需要仙府材料×%d。", cost)}, true, nil
	}
	rate := 95 - artifact.Level*3
	if rate < 30 {
		rate = 30
	}
	if randomPercent() > rate {
		return GameResult{Title: "强化失败", Content: fmt.Sprintf("材料耗尽，法宝未损坏。\n成功率：%d%%", rate)}, true, nil
	}
	_ = g.store.DB.Model(&artifact).Update("level", gorm.Expr("level + 1")).Error
	return GameResult{Title: "法宝强化", Content: fmt.Sprintf("%s：+%d → +%d\n成功率：%d%%", artifact.Name, artifact.Level, artifact.Level+1, rate), Actions: []string{"强宝 " + artifact.Name, "法宝"}}, true, nil
}

var _ = errors.Is
var _ = rand.Intn
