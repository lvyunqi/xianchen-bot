package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type GMCommand struct {
	Name     string
	Raw      string
	Args     []string
	MinLevel int
}

type gmDefinition struct {
	Name     string
	MinLevel int
}

type GMCommandInfo struct {
	Name     string `json:"name"`
	MinLevel int    `json:"min_level"`
	MinRole  string `json:"min_role"`
}

var gmDefinitions = []gmDefinition{
	{"天赐灵玉", 1}, {"天赐灵石", 1}, {"赐仙位", 2}, {"天降灵玉", 3}, {"天降灵石", 3},
	{"天赐仙物", 1}, {"天降福泽", 3}, {"净化仙物", 3}, {"净仙物", 2}, {"净全物", 2},
	{"净灵玉", 4}, {"净灵石", 4}, {"灵域重置", 3}, {"唤魔令", 3}, {"仙缘令", 3},
	{"赐神通", 2}, {"赐副神通", 2}, {"改灵根", 2}, {"改道体", 2}, {"卸仙宝", 2},
	{"清神通", 2}, {"天眼通", 1}, {"天赐修为", 2}, {"天罚扣", 3}, {"天罚全服", 4},
	{"封仙使", 3}, {"天罚禁", 5}, {"天罚解", 5}, {"乾坤令", 5}, {"仙启令", 4},
	{"充值", 1}, {"发放道具", 1}, {"封号", 3}, {"解封", 3}, {"删号", 5},
	{"群审核", 1},
}

func GMCommandCatalog() []GMCommandInfo {
	result := make([]GMCommandInfo, 0, len(gmDefinitions))
	for _, definition := range gmDefinitions {
		result = append(result, GMCommandInfo{Name: definition.Name, MinLevel: definition.MinLevel, MinRole: gmRoleName(definition.MinLevel)})
	}
	return result
}

func ParseGMCommand(message string) (GMCommand, bool) {
	message = strings.TrimSpace(strings.ReplaceAll(message, "　", " "))
	if message == "" {
		return GMCommand{}, false
	}
	parts := strings.SplitN(message, " ", 2)
	name := parts[0]
	raw := ""
	if len(parts) == 2 {
		raw = strings.TrimSpace(parts[1])
	}
	for _, definition := range gmDefinitions {
		if definition.Name == name {
			return GMCommand{Name: name, Raw: raw, Args: splitGMArguments(name, raw), MinLevel: definition.MinLevel}, true
		}
	}
	return GMCommand{}, false
}

func splitGMArguments(commandName, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if gmSingleArgumentCommands[commandName] {
		return []string{raw}
	}
	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return strings.Fields(raw)
	}
	return strings.FieldsFunc(raw, func(r rune) bool { return r == '-' })
}

var gmSingleArgumentCommands = map[string]bool{
	"天降灵玉": true, "天降灵石": true, "净化仙物": true, "灵域重置": true, "唤魔令": true,
	"仙缘令": true, "卸仙宝": true, "清神通": true, "天眼通": true, "净灵玉": true,
	"净灵石": true, "封仙使": true, "天罚禁": true, "天罚解": true, "乾坤令": true, "仙启令": true,
}

func (g *Game) ExecuteGM(actorID string, command GMCommand) (GameResult, bool, error) {
	actorName, level, authorized := g.gmAuthority(actorID)
	if !authorized {
		return GameResult{}, false, nil
	}
	if level < command.MinLevel {
		result := GameResult{Title: "神令权限不足", Content: fmt.Sprintf("%s无权施展“%s”。需要%s及以上权限。", actorName, command.Name, gmRoleName(command.MinLevel))}
		g.writeGMLog(actorName, command, "permission_denied", result.Content)
		return result, true, nil
	}
	result, target, err := g.runGMCommand(actorName, level, command)
	if err != nil {
		g.writeGMLog(actorName, command, target, "error: "+err.Error())
		return GameResult{}, true, err
	}
	g.writeGMLog(actorName, command, target, result.Content)
	return result, true, nil
}

func (g *Game) gmAuthority(actorID string) (string, int, bool) {
	var ownerID string
	_ = g.store.DB.Table("system_settings").Select("value").Where("key = ?", "owner.user_id").Scan(&ownerID).Error
	if strings.TrimSpace(ownerID) != "" && strings.TrimSpace(ownerID) == strings.TrimSpace(actorID) {
		return "主人·道祖", 5, true
	}
	var manager model.ManagerAccount
	if err := g.store.DB.Where("user_id = ? AND enabled = ?", strings.TrimSpace(actorID), true).First(&manager).Error; err != nil {
		return "", 0, false
	}
	level := map[string]int{"护法": 1, "长老": 2, "宗主": 3, "仙尊": 4, "道祖": 5}[manager.Role]
	if level == 0 {
		return "", 0, false
	}
	name := strings.TrimSpace(manager.Name)
	if name == "" {
		name = manager.UserID
	}
	return name + "·" + manager.Role, level, true
}

func gmRoleName(level int) string {
	names := []string{"", "护法", "长老", "宗主", "仙尊", "道祖"}
	if level < 1 || level >= len(names) {
		return "未知"
	}
	return names[level]
}

func (g *Game) runGMCommand(actor string, actorLevel int, command GMCommand) (GameResult, string, error) {
	args := command.Args
	require := func(count int, format string) (GameResult, string, bool) {
		if len(args) >= count {
			return GameResult{}, "", false
		}
		return GameResult{Title: command.Name, Content: "格式：`" + format + "`"}, "format_error", true
	}
	switch command.Name {
	case "群审核":
		return g.reviewGroupAccess(actor, command.Raw)
	case "删号":
		if result, target, stop := require(1, "删号 道号"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		name, accountID := player.DaoName, player.AccountID
		if err := storage.NewPlayerRepository(g.store.DB).Delete(player.ID); err != nil {
			return GameResult{}, accountID, err
		}
		return GameResult{Title: "删号完成", Content: fmt.Sprintf("玩家【%s】的角色及关联动态数据已永久删除，道号已经释放，可被重新注册。此操作已写入审计。", name)}, accountID, nil

	case "充值":
		if result, target, stop := require(3, "充值 道号 灵石/仙金/银币 数量"); stop {
			return result, target, nil
		}
		player, amount, err := g.gmPlayerAmount(args[0], args[2])
		if err != nil {
			return GameResult{}, args[0], err
		}
		currency := args[1]
		column := ""
		requiredLevel := 2
		yuan := int64(0)
		switch currency {
		case "银币":
			column = "silver_coins"
		case "灵石":
			column = "spirit_stones"
			requiredLevel = 4
			yuan = amount / max64(g.settingInt("recharge.spirit_stones_per_yuan", 2_000_000), 1)
		case "仙金", "仙币":
			currency = "仙金"
			column = "immortal_jade"
			requiredLevel = 4
			yuan = amount / max64(g.settingInt("recharge.jade_per_yuan", 2_000), 1)
		default:
			return GameResult{Title: "充值", Content: "货币只能填写灵石、仙金或银币。\n格式：`充值 道号 仙金 2000`"}, player.AccountID, nil
		}
		if actorLevel < requiredLevel {
			return GameResult{Title: "充值权限不足", Content: fmt.Sprintf("充值%s需要%s权限。", currency, gmRoleName(requiredLevel))}, player.AccountID, nil
		}
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update(column, gorm.Expr(column+" + ?", amount)).Error; err != nil {
				return err
			}
			if yuan > 0 {
				total := playerValueIntTx(tx, player.ID, "recharge.total_yuan", 0) + yuan
				return upsertPlayerValueTx(tx, player.ID, "recharge.total_yuan", fmt.Sprintf("%d", total), nil)
			}
			return nil
		}); err != nil {
			return GameResult{}, player.AccountID, err
		}
		broadcast := ""
		if currency == "仙金" {
			broadcast = fmt.Sprintf("【仙金到账】道友%s获得仙金%d，充值已经入账。", player.DaoName, amount)
			_ = g.publishWorldBroadcast("充值", player.DaoName+"仙金到账", broadcast)
		}
		cumulativeText := "本次不计累充（银币或不足1元折算单位）"
		if yuan > 0 {
			cumulativeText = fmt.Sprintf("本次累充：+%d元", yuan)
		}
		return GameResult{Title: "充值成功", Content: fmt.Sprintf("玩家：%s\n货币：%s\n到账：%d\n%s\n操作人：%s\n本次操作已经写入神令审计。", player.DaoName, currency, amount, cumulativeText, actor), BroadcastContent: broadcast}, player.AccountID, nil

	case "天赐灵玉", "天赐灵石", "赐仙位", "天赐修为":
		if result, target, stop := require(2, command.Name+" ID 数量"); stop {
			return result, target, nil
		}
		player, amount, err := g.gmPlayerAmount(args[0], args[1])
		if err != nil {
			return GameResult{}, args[0], err
		}
		levelText := ""
		switch command.Name {
		case "天赐灵玉":
			err = g.store.DB.Model(&player).Update("immortal_jade", gorm.Expr("immortal_jade + ?", amount)).Error
		case "天赐灵石":
			err = g.store.DB.Model(&player).Update("spirit_stones", gorm.Expr("spirit_stones + ?", amount)).Error
		case "赐仙位":
			_, err = g.addPlayerValueInt(player.ID, "immortal_rank.experience", amount)
		case "天赐修为":
			var progress model.PlayerLevelProgress
			progress, err = g.grantCultivationExperience(player.ID, amount)
			if err == nil {
				levelText = fmt.Sprintf("\n角色经验同步：+%d · 等级LV%d→LV%d", amount, progress.BeforeLevel, progress.AfterLevel)
				if refreshed, loadErr := g.players.Get(player.ID); loadErr == nil {
					_ = g.syncPlayerCombatPower(&refreshed)
				}
			}
		}
		if err != nil {
			return GameResult{}, player.AccountID, err
		}
		return GameResult{Title: command.Name, Content: fmt.Sprintf("神令生效，修士【%s】获得%s × %d。%s", player.DaoName, gmResourceName(command.Name), amount, levelText)}, player.AccountID, nil

	case "天降灵玉", "天降灵石":
		if result, target, stop := require(1, command.Name+" 数量"); stop {
			return result, target, nil
		}
		amount, err := positiveInt64(args[0])
		if err != nil {
			return GameResult{}, "all_players", err
		}
		if command.Name == "天降灵石" {
			err = g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("spirit_stones", gorm.Expr("spirit_stones + ?", amount)).Error
		} else {
			err = g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("immortal_jade", gorm.Expr("immortal_jade + ?", amount)).Error
		}
		if err != nil {
			return GameResult{}, "all_players", err
		}
		return GameResult{Title: command.Name, Content: fmt.Sprintf("福泽遍及全服，所有修士获得%s × %d。", gmResourceName(command.Name), amount)}, "all_players", nil

	case "天赐仙物", "发放道具", "净仙物":
		if result, target, stop := require(3, command.Name+" ID-仙物名-数量"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		item, err := g.itemByName(args[1])
		if err != nil {
			return GameResult{}, player.AccountID, err
		}
		amount, err := positiveInt64(args[2])
		if err != nil {
			return GameResult{}, player.AccountID, err
		}
		if command.Name == "净仙物" {
			amount = -amount
		}
		if err := g.players.AdjustItem(player.ID, item.ID, amount); err != nil {
			return GameResult{}, player.AccountID, err
		}
		verb := "获得"
		if amount < 0 {
			verb, amount = "扣除", -amount
		}
		return GameResult{Title: command.Name, Content: fmt.Sprintf("修士【%s】%s%s × %d。", player.DaoName, verb, item.Name, amount)}, player.AccountID, nil

	case "天降福泽":
		if result, target, stop := require(2, "天降福泽 仙物名-数量"); stop {
			return result, target, nil
		}
		item, err := g.itemByName(args[0])
		if err != nil {
			return GameResult{}, "all_players", err
		}
		amount, err := positiveInt64(args[1])
		if err != nil {
			return GameResult{}, "all_players", err
		}
		var players []model.Player
		if err := g.store.DB.Find(&players).Error; err != nil {
			return GameResult{}, "all_players", err
		}
		for _, player := range players {
			if err := g.players.AdjustItem(player.ID, item.ID, amount); err != nil {
				return GameResult{}, "all_players", err
			}
		}
		return GameResult{Title: "天降福泽", Content: fmt.Sprintf("全服修士获得%s × %d。", item.Name, amount)}, "all_players", nil

	case "净化仙物":
		if result, target, stop := require(1, "净化仙物 仙物名"); stop {
			return result, target, nil
		}
		item, err := g.itemByName(args[0])
		if err != nil {
			return GameResult{}, "all_players", err
		}
		err = g.store.DB.Where("item_id = ?", item.ID).Delete(&model.PlayerItem{}).Error
		return GameResult{Title: "净化仙物", Content: "全服持有的“" + item.Name + "”已清空。"}, "all_players", err

	case "净全物":
		if result, target, stop := require(2, "净全物 ID-仙物名"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		item, err := g.itemByName(args[1])
		if err == nil {
			err = g.store.DB.Where("player_id = ? AND item_id = ?", player.ID, item.ID).Delete(&model.PlayerItem{}).Error
		}
		return GameResult{Title: "净全物", Content: fmt.Sprintf("修士【%s】的%s已全部清空。", player.DaoName, args[1])}, player.AccountID, err

	case "净灵玉", "净灵石":
		if command.Name == "净灵石" {
			if err := g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("spirit_stones", 0).Error; err != nil {
				return GameResult{}, "all_players", err
			}
		} else if err := g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("immortal_jade", 0).Error; err != nil {
			return GameResult{}, "all_players", err
		}
		return GameResult{Title: command.Name, Content: "神令已清空全服对应货币。"}, "all_players", nil

	case "灵域重置", "仙缘令":
		key := "world.sect_war_reset"
		content := "灵域已重置，各宗门可重新争夺领地。"
		if command.Name == "仙缘令" {
			key, content = "world.encounter_trigger", "仙缘令已触发，全服修士的仙缘奇遇进入活跃期。"
			_ = g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("immortal_affinity", gorm.Expr("immortal_affinity + 5")).Error
		}
		if err := g.setSystemSetting(key, time.Now().Format(time.RFC3339), content); err != nil {
			return GameResult{}, "world", err
		}
		return GameResult{Title: command.Name, Content: content}, "world", nil

	case "唤魔令":
		if result, target, stop := require(1, "唤魔令 BOSS名"); stop {
			return result, target, nil
		}
		boss := strings.Join(args, " ")
		if err := g.setSystemSetting("world.boss.current", boss, "当前世界BOSS"); err != nil {
			return GameResult{}, "world", err
		}
		return GameResult{Title: "唤魔令", Content: boss + "已降临修仙界，速速集结讨伐。"}, "world", nil

	case "赐神通", "赐副神通":
		if result, target, stop := require(3, command.Name+" ID-神通名-等级"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		level, err := positiveInt64(args[2])
		if err != nil {
			return GameResult{}, player.AccountID, err
		}
		key := "gm.main_skill"
		if command.Name == "赐副神通" {
			key = "gm.sub_skill"
		}
		err = g.setPlayerValue(player.ID, key, fmt.Sprintf("%s|%d", args[1], level), nil)
		return GameResult{Title: command.Name, Content: fmt.Sprintf("修士【%s】获得%s %s（%d级）。", player.DaoName, command.Name, args[1], level)}, player.AccountID, err

	case "改灵根", "改道体":
		if result, target, stop := require(2, command.Name+" ID-名称"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		if command.Name == "改灵根" {
			err = g.store.DB.Model(&player).Update("spiritual_root", args[1]).Error
		} else {
			err = g.setPlayerValue(player.ID, "gm.dao_body", args[1], nil)
		}
		return GameResult{Title: command.Name, Content: fmt.Sprintf("修士【%s】的%s已改为%s。", player.DaoName, strings.TrimPrefix(command.Name, "改"), args[1])}, player.AccountID, err

	case "卸仙宝", "清神通", "天眼通", "封仙使", "天罚禁", "天罚解", "封号", "解封":
		if result, target, stop := require(1, command.Name+" ID"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		switch command.Name {
		case "卸仙宝":
			err = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ?", player.ID).Update("equipped", false).Error
		case "清神通":
			err = g.store.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("player_id = ?", player.ID).Delete(&model.PlayerSkill{}).Error; err != nil {
					return err
				}
				if err := tx.Where("player_id = ? AND key IN ?", player.ID, []string{"gm.main_skill", "gm.sub_skill"}).Delete(&model.PlayerValue{}).Error; err != nil {
					return err
				}
				return tx.Model(&player).Update("current_skill_id", 0).Error
			})
		case "天眼通":
			return g.archive(&player), player.AccountID, nil
		case "封仙使":
			err = g.setPlayerValueInt(player.ID, "authorized_speaker", 1)
		case "天罚禁", "封号":
			err = g.store.DB.Model(&player).Updates(map[string]any{"banned": true, "ban_reason": "GM天罚禁"}).Error
		case "天罚解", "解封":
			err = g.store.DB.Model(&player).Updates(map[string]any{"banned": false, "ban_reason": ""}).Error
		}
		return GameResult{Title: command.Name, Content: fmt.Sprintf("神令已对修士【%s】生效。", player.DaoName)}, player.AccountID, err

	case "天罚扣":
		if result, target, stop := require(3, "天罚扣 ID-属性-数值"); stop {
			return result, target, nil
		}
		player, err := g.findPlayer(args[0])
		if err != nil {
			return GameResult{}, args[0], err
		}
		amount, err := positiveInt64(args[2])
		if err != nil {
			return GameResult{}, player.AccountID, err
		}
		column := map[string]string{"修为": "cultivation", "灵石": "spirit_stones", "功德": "merit", "声望": "reputation", "气运": "luck", "运气": "luck", "仙缘": "immortal_affinity", "气血": "health", "法力": "mana"}[args[1]]
		if column == "" {
			return GameResult{}, player.AccountID, fmt.Errorf("不支持扣除属性: %s", args[1])
		}
		expression := "CASE WHEN " + column + " > ? THEN " + column + " - ? ELSE 0 END"
		err = g.store.DB.Model(&player).Update(column, gorm.Expr(expression, amount, amount)).Error
		return GameResult{Title: "天罚扣", Content: fmt.Sprintf("修士【%s】的%s扣除%d。", player.DaoName, args[1], amount)}, player.AccountID, err

	case "天罚全服":
		if result, target, stop := require(2, "天罚全服 百分比/固定值-数值"); stop {
			return result, target, nil
		}
		amount, err := positiveInt64(args[1])
		if err != nil {
			return GameResult{}, "all_players", err
		}
		if args[0] == "百分比" {
			if amount > 100 {
				amount = 100
			}
			err = g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("cultivation", gorm.Expr("cultivation * ? / 100", 100-amount)).Error
		} else {
			err = g.store.DB.Model(&model.Player{}).Where("deleted_at IS NULL").Update("cultivation", gorm.Expr("CASE WHEN cultivation > ? THEN cultivation - ? ELSE 0 END", amount, amount)).Error
		}
		return GameResult{Title: "天罚全服", Content: fmt.Sprintf("全服修为已按%s扣除%d。", args[0], amount)}, "all_players", err

	case "乾坤令":
		if strings.TrimSpace(command.Raw) == "" {
			return GameResult{Title: "乾坤令", Content: "格式：`乾坤令 内容`"}, "format_error", nil
		}
		now := time.Now()
		row := model.Notice{Code: "gm_notice_" + strconv.FormatInt(now.UnixNano(), 10), Title: "乾坤令", Content: command.Raw, Type: "系统", Pinned: true, Published: true, PublishedAt: &now}
		if err := g.store.DB.Create(&row).Error; err != nil {
			return GameResult{}, "world", err
		}
		_ = g.store.DB.Create(&model.Broadcast{Content: command.Raw, Level: "GM", CreatedBy: actor}).Error
		return GameResult{Title: "乾坤令", Content: "全服公告：" + command.Raw, BroadcastContent: "【世界公告】" + command.Raw}, "world", nil

	case "仙启令":
		if strings.TrimSpace(command.Raw) == "" {
			return GameResult{Title: "仙启令", Content: "格式：`仙启令 活动名`"}, "format_error", nil
		}
		start, end := time.Now(), time.Now().Add(7*24*time.Hour)
		row := model.Activity{Code: "gm_activity_" + strconv.FormatInt(start.UnixNano(), 10), Name: command.Raw, Type: "全服", StartsAt: start, EndsAt: end, Effect: "GM神令开启", EffectJSON: `{}`, Status: "进行中"}
		if err := g.store.DB.Create(&row).Error; err != nil {
			return GameResult{}, "world", err
		}
		return GameResult{Title: "仙启令", Content: "全服活动“" + command.Raw + "”已开启。"}, "world", nil
	}
	return GameResult{}, "unknown", fmt.Errorf("神令未实现: %s", command.Name)
}

func (g *Game) gmPlayerAmount(target, rawAmount string) (model.Player, int64, error) {
	player, err := g.findPlayer(target)
	if err != nil {
		return player, 0, err
	}
	amount, err := positiveInt64(rawAmount)
	return player, amount, err
}

func positiveInt64(value string) (int64, error) {
	amount, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("数值必须是正整数")
	}
	return amount, nil
}

func gmResourceName(command string) string {
	switch command {
	case "天赐灵玉", "天降灵玉":
		return "灵玉"
	case "天赐灵石", "天降灵石":
		return "灵石"
	case "赐仙位":
		return "仙位经验"
	default:
		return "修为"
	}
}

func (g *Game) setSystemSetting(key, value, description string) error {
	row := model.SystemSetting{Key: key, Value: value, ValueType: "string", Description: description}
	return g.store.DB.Where("key = ?", key).Assign(map[string]any{"value": value, "description": description}).FirstOrCreate(&row).Error
}

func (g *Game) writeGMLog(actor string, command GMCommand, target, result string) {
	before, _ := json.Marshal(map[string]any{"command": command.Name, "arguments": command.Raw})
	after, _ := json.Marshal(map[string]any{"result": result})
	_ = g.store.DB.Create(&model.OperationLog{GMName: actor, Action: command.Name, TargetType: "gm_command", TargetID: target, BeforeJSON: string(before), AfterJSON: string(after)}).Error
}
