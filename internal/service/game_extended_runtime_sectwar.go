package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func sectWarProgressCode(sourceID, targetID uint) string {
	return fmt.Sprintf("sect_war:%d:%d", sourceID, targetID)
}

func (g *Game) executeSectWarExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	sect, member, err := g.playerSect(player.ID)
	if err != nil {
		return GameResult{Title: "宗门战争", Content: "散修不能发起宗门战争，请先加入宗门。", Actions: []string{"宗门列表", "加入宗门", "创建宗门"}}, true, nil
	}
	if action == "territory" {
		return g.sectWarTerritory(player, sect)
	}
	if member.Position != "宗主" && member.Position != "长老" {
		return GameResult{Title: "宗战权限不足", Content: fmt.Sprintf("你在%s的职位是%s。只有宗主或长老可以宣战、备战、议和与结盟。", sect.Name, member.Position), Actions: []string{"宗门", "成员列表"}}, true, nil
	}
	switch action {
	case "declare":
		return g.declareSectWar(player, sect, command.RawArguments)
	case "prepare":
		return g.prepareSectWar(player, sect)
	case "battle":
		return g.beginSectWarBattle(player, sect, command.RawArguments)
	case "negotiate":
		return g.resolveSectDiplomacy(player, sect, command.RawArguments, "议和")
	case "ally":
		return g.resolveSectDiplomacy(player, sect, command.RawArguments, "结盟")
	default:
		return GameResult{}, false, fmt.Errorf("未知宗门战争动作: %s (%s)", action, system.Table)
	}
}

func (g *Game) sectWarTarget(source model.Sect, raw string) (model.Sect, error) {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "@"))
	var target model.Sect
	err := g.store.DB.Where("name = ? AND id <> ?", name, source.ID).First(&target).Error
	return target, err
}

func (g *Game) declareSectWar(player *model.Player, sect model.Sect, raw string) (GameResult, bool, error) {
	target, err := g.sectWarTarget(sect, raw)
	if err != nil {
		return GameResult{Title: "宣战目标无效", Content: "请发送 `宣战 宗门完整名称`，不能向本宗宣战。", Actions: []string{"宗门列表", "领地"}}, true, nil
	}
	code := sectWarProgressCode(sect.ID, target.ID)
	var existing model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ? AND state IN ?", player.ID, "宗门战争", code, []string{"已经宣战", "宗门备战", "交战中"}).First(&existing).Error == nil {
		return GameResult{Title: "战书已经送达", Content: fmt.Sprintf("%s与%s的战事仍在进行。重复宣战不会再次消耗宗门资金。", sect.Name, target.Name), Actions: []string{"备战", "宗战 " + target.Name, "议和 " + target.Name, "领地"}}, true, nil
	}
	if sect.Funds < 100 {
		return GameResult{Title: "宗门资金不足", Content: fmt.Sprintf("发布战书需要宗门资金100，当前%d。", sect.Funds), Actions: []string{"宗门捐献", "宗务", "宗门状态"}}, true, nil
	}
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "宗门战争", ConfigCode: code, ConfigName: sect.Name + "征讨" + target.Name, State: "已经宣战", Level: maxInt(sect.Level, 1), Power: g.sectPower(sect.ID), MetadataJSON: fmt.Sprintf(`{"source_sect_id":%d,"target_sect_id":%d}`, sect.ID, target.ID)}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Sect{}).Where("id = ? AND funds >= 100", sect.ID).Update("funds", gorm.Expr("funds - 100"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("宗门资金不足")
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.OwnerID, "宗门战书", fmt.Sprintf("%s宗主%s向%s正式宣战。对方需完成备战并通过逐回合宗战才能结算领地。", sect.Name, player.DaoName, target.Name))
	return GameResult{Title: "宗门战书已发", Content: fmt.Sprintf("宣战方：%s\n守战方：%s\n战书：%s\n宗门资金：-100\n━━━━━━━━━━━\n当前只完成宣战，尚未自动判定胜负。请先备战，再进入逐回合宗战。", sect.Name, target.Name, progress.ConfigName), Actions: []string{"备战", "宗战 " + target.Name, "议和 " + target.Name, "领地"}}, true, nil
}

func (g *Game) activeSectWar(playerID, sectID uint, targetID uint) (model.PlayerExtendedProgress, error) {
	query := g.store.DB.Where("player_id = ? AND system = ? AND config_code LIKE ? AND state IN ?", playerID, "宗门战争", fmt.Sprintf("sect_war:%d:%%", sectID), []string{"已经宣战", "宗门备战", "交战中"})
	if targetID != 0 {
		query = query.Where("config_code = ?", sectWarProgressCode(sectID, targetID))
	}
	var progress model.PlayerExtendedProgress
	err := query.Order("updated_at DESC").First(&progress).Error
	return progress, err
}

func (g *Game) prepareSectWar(player *model.Player, sect model.Sect) (GameResult, bool, error) {
	progress, err := g.activeSectWar(player.ID, sect.ID, 0)
	if err != nil {
		return GameResult{Title: "当前没有战事", Content: "请先发送 `宣战 宗门名`。", Actions: []string{"宗门列表", "领地"}}, true, nil
	}
	if progress.Uses >= 5 {
		return GameResult{Title: "宗门备战圆满", Content: "本场战争已经完成五轮备战，不会继续扣除宗门资金。", Actions: []string{"宗战 " + sectWarTargetName(progress.ConfigName), "领地"}}, true, nil
	}
	cost := int64(100 + progress.Uses*50)
	if sect.Funds < cost {
		return GameResult{Title: "备战资金不足", Content: fmt.Sprintf("第%d轮备战需要宗门资金%d，当前%d。", progress.Uses+1, cost, sect.Funds), Actions: []string{"宗门捐献", "宗务", "宗门状态"}}, true, nil
	}
	progress.State = "宗门备战"
	progress.Uses++
	progress.Mastery += int64(sect.Level * 10)
	progress.Power = g.sectPower(sect.ID) + progress.Uses*100
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Sect{}).Where("id = ? AND funds >= ?", sect.ID, cost).Update("funds", gorm.Expr("funds - ?", cost))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("宗门资金不足")
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "宗门备战完成", Content: fmt.Sprintf("宗门：%s\n备战轮次：%d/5\n消耗宗门资金：%d\n集结战力：%d\n━━━━━━━━━━━\n备战只提高本场宗战基础，不会直接判胜。", sect.Name, progress.Uses, cost, progress.Power), Actions: []string{"备战", "宗战 " + sectWarTargetName(progress.ConfigName), "领地"}}, true, nil
}

func sectWarTargetName(configName string) string {
	parts := strings.SplitN(configName, "征讨", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "目标宗门"
}

func (g *Game) beginSectWarBattle(player *model.Player, sect model.Sect, raw string) (GameResult, bool, error) {
	target, err := g.sectWarTarget(sect, raw)
	if err != nil {
		return GameResult{Title: "宗战目标无效", Content: "请输入：`宗战 宗门完整名称`。", Actions: []string{"宗门列表", "领地"}}, true, nil
	}
	progress, err := g.activeSectWar(player.ID, sect.ID, target.ID)
	if err != nil {
		return GameResult{Title: "尚未宣战", Content: fmt.Sprintf("必须先向%s宣战并完成至少一轮备战。", target.Name), Actions: []string{"宣战 " + target.Name, "备战"}}, true, nil
	}
	if progress.State != "宗门备战" || progress.Uses < 1 {
		return GameResult{Title: "宗门尚未备战", Content: "宣战后至少完成一轮备战才能出征。", Actions: []string{"备战", "宗门状态"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "当前无法宗战", Content: "请先结束当前修行或战斗。", Actions: []string{"状态", "投降"}}, true, nil
	}
	var memberCount int64
	_ = g.store.DB.Model(&model.SectMember{}).Where("sect_id = ?", target.ID).Count(&memberCount).Error
	enemyPower := max64(g.sectPower(target.ID)/max64(memberCount, 1), 80)
	preparedReduction := min64(progress.Uses*5, 25)
	enemyPower = max64(enemyPower*(100-preparedReduction)/100, 50)
	enemyHP := max64(enemyPower*2, 100)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{BattleKind: "道藏试炼", Round: 1, EnemyName: target.Name + "护山法相", EnemyPower: enemyPower, PlayerHP: effective.Health, PlayerMana: effective.Mana, EnemyHP: enemyHP, EnemyMaxHP: enemyHP, ExtendedCategory: "宗门战争", ExtendedConfigCode: progress.ConfigCode, ExtendedConfigName: progress.ConfigName, ExtendedAction: "battle", StartedAt: time.Now().UnixMilli()}
	if err := g.beginPVEBattle(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Model(&progress).Update("state", "交战中").Error
	return GameResult{Title: "宗门攻防战开启", Content: fmt.Sprintf("攻方：%s\n守方：%s\n守山法相战力：%d\n备战削弱：%d%%\n敌方气血：%d/%d\n━━━━━━━━━━━\n必须逐回合击破护山法相，胜负不会提前自动结算。", sect.Name, target.Name, enemyPower, preparedReduction, enemyHP, enemyHP), Actions: []string{"攻击", "技能", "防御", "投降", "功法"}}, true, nil
}

func parseSectWarCode(code string) (uint, uint) {
	parts := strings.Split(code, ":")
	if len(parts) != 3 {
		return 0, 0
	}
	source, _ := strconv.ParseUint(parts[1], 10, 64)
	target, _ := strconv.ParseUint(parts[2], 10, 64)
	return uint(source), uint(target)
}

func (g *Game) finishSectWarExtendedBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	sourceID, targetID := parseSectWarCode(state.ExtendedConfigCode)
	var source, target model.Sect
	if g.store.DB.First(&source, sourceID).Error != nil || g.store.DB.First(&target, targetID).Error != nil {
		return GameResult{}, true, errors.New("宗战宗门记录不存在")
	}
	remainingHP, remainingMana := max64(state.PlayerHP, 1), max64(state.PlayerMana, 0)
	var progress model.PlayerExtendedProgress
	if err := g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "宗门战争", state.ExtendedConfigCode).First(&progress).Error; err != nil {
		return GameResult{}, true, err
	}
	resultState := "宗战失利"
	if won {
		resultState = "宗战胜利"
	}
	progress.State, progress.Experience, progress.Mastery = resultState, progress.Experience+int64(state.EnemyPower), progress.Mastery+int64(state.Round)
	rewardFunds, rewardReputation := max64(state.EnemyPower/2, 100), max64(state.EnemyPower/20, 10)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key = ?", player.ID, "pve.battle").Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": remainingMana, "state": model.PlayerStateIdle}).Error; err != nil {
			return err
		}
		if won {
			if err := tx.Model(&model.Sect{}).Where("id = ?", source.ID).Updates(map[string]any{"funds": gorm.Expr("funds + ?", rewardFunds), "reputation": gorm.Expr("reputation + ?", rewardReputation)}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Sect{}).Where("id = ?", target.ID).Update("funds", gorm.Expr("MAX(funds - ?, 0)", rewardFunds/2)).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.SectMember{}).Where("player_id = ?", player.ID).Update("contribution", gorm.Expr("contribution + ?", rewardReputation)).Error; err != nil {
				return err
			}
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.OwnerID, "宗门战果", fmt.Sprintf("%s对%s发起的宗门攻防战已经结算：%s。", source.Name, target.Name, map[bool]string{true: "守山法相被攻破", false: "守山成功"}[won]))
	if !won {
		return GameResult{Title: "宗门攻防失败", Content: fmt.Sprintf("攻方：%s\n守方：%s\n%s\n━━━━━━━━━━━\n领地未转移，宗门没有获得资金、声望或贡献奖励。", source.Name, target.Name, logLine), Actions: []string{"疗伤", "宣战 " + target.Name, "领地", "宗门状态"}}, true, nil
	}
	broadcast := fmt.Sprintf("【仙盟战报】%s由%s领阵，历经%d回合攻破%s护山法相，夺得战争声望%d。", source.Name, player.DaoName, state.Round, target.Name, rewardReputation)
	_ = g.publishWorldBroadcast("宗战", source.Name+"攻破"+target.Name, broadcast)
	return GameResult{Title: "宗门攻防胜利", Content: fmt.Sprintf("攻方：%s\n守方：%s\n%s\n━━━━━━━━━━━\n宗门资金+%d · 宗门声望+%d · 个人贡献+%d\n战果已进入本宗领地记录。", source.Name, target.Name, logLine, rewardFunds, rewardReputation, rewardReputation), Actions: []string{"领地", "宗门状态", "贡献", "通知"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) resolveSectDiplomacy(player *model.Player, sect model.Sect, raw, kind string) (GameResult, bool, error) {
	target, err := g.sectWarTarget(sect, raw)
	if err != nil {
		return GameResult{Title: "宗门" + kind, Content: fmt.Sprintf("请输入：`%s 宗门完整名称`。", kind), Actions: []string{"宗门列表", "领地"}}, true, nil
	}
	progress, progressErr := g.activeSectWar(player.ID, sect.ID, target.ID)
	if kind == "议和" && progressErr != nil {
		return GameResult{Title: "没有可议和战事", Content: "双方当前没有宣战记录。", Actions: []string{"宣战 " + target.Name, "领地"}}, true, nil
	}
	if kind == "结盟" && progressErr != nil {
		progress = model.PlayerExtendedProgress{PlayerID: player.ID, System: "宗门战争", ConfigCode: sectWarProgressCode(sect.ID, target.ID), ConfigName: sect.Name + "与" + target.Name + "仙盟", Level: 1, MetadataJSON: fmt.Sprintf(`{"source_sect_id":%d,"target_sect_id":%d}`, sect.ID, target.ID)}
	}
	progress.State = kind + "生效"
	progress.Uses++
	if err := upsertExtendedProgressTx(g.store.DB, progress); err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.OwnerID, "宗门"+kind, fmt.Sprintf("%s已与%s完成%s，道籍外交状态已经更新。", sect.Name, target.Name, kind))
	return GameResult{Title: "宗门" + kind + "完成", Content: fmt.Sprintf("本宗：%s\n对方：%s\n状态：%s生效\n━━━━━━━━━━━\n当前战事状态已经修改，不会继续把旧宣战记录判作交战中。", sect.Name, target.Name, kind), Actions: []string{"领地", "宗门状态", "通知"}}, true, nil
}

func (g *Game) sectWarTerritory(player *model.Player, sect model.Sect) (GameResult, bool, error) {
	var rows []model.PlayerExtendedProgress
	if err := g.store.DB.Where("system = ? AND config_code LIKE ?", "宗门战争", fmt.Sprintf("sect_war:%d:%%", sect.ID)).Order("updated_at DESC").Limit(30).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("宗门：%s · 等级%d · 资金%d · 声望%d", sect.Name, sect.Level, sect.Funds, sect.Reputation), "以下只列本宗真实战争与外交记录。", "━━━━━━━━━━━"}
	actions := []string{"宗门状态", "宗门列表"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  备战%d轮 · 战功%d · 集结战力%d", row.ConfigName, row.State, row.Uses, row.Mastery, row.Power))
		target := sectWarTargetName(row.ConfigName)
		if strings.Contains(row.State, "宣战") || strings.Contains(row.State, "备战") || strings.Contains(row.State, "交战") {
			actions = append(actions, "备战", "宗战 "+target, "议和 "+target)
		}
	}
	if len(rows) == 0 {
		lines = append(lines, "本宗尚无战争、领地或结盟记录。")
		actions = append(actions, "宣战 宗门名", "结盟 宗门名")
	}
	return GameResult{Title: "宗门领地与战事", Content: strings.Join(lines, "\n"), Actions: uniqueExtendedActions(actions)}, true, nil
}
