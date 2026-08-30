package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

type pendingImmortalEncounter struct {
	ConfigCode string    `json:"config_code"`
	StartedAt  time.Time `json:"started_at"`
}

type immortalEncounterChoice struct {
	Name        string
	BaseRate    float64
	SuccessText string
	FailureText string
	Success     map[string]any
	Failure     map[string]any
}

var errImmortalEncounterSettled = errors.New("immortal encounter already settled")

func (g *Game) executeImmortalEncounterExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	switch action {
	case "trigger":
		return g.triggerImmortalEncounter(player, command, system)
	case "choose":
		return g.chooseImmortalEncounter(player, command.RawArguments, system)
	case "record":
		return g.extendedOwnedRuntime(player, command, system)
	case "deepen":
		return g.deepenImmortalEncounter(player, system)
	case "transfer":
		return g.transferImmortalEncounter(player, command, system)
	case "awaken":
		return g.awakenImmortalEncounter(player, system)
	default:
		return GameResult{}, false, fmt.Errorf("未知仙缘动作: %s", action)
	}
}

func (g *Game) triggerImmortalEncounter(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	if _, err := g.playerValue(player.ID, "immortal_encounter.pending"); err == nil {
		return g.showPendingImmortalEncounter(player, system)
	}
	config, result, ok, err := g.resolveExtendedRuntimeConfig(player, command, system, "trigger")
	if err != nil || !ok {
		return result, true, err
	}
	requirement, unmet, err := g.prerequisiteStatus(player, config.Prerequisite)
	if err != nil {
		return GameResult{Title: "奇遇因果紊乱", Content: "前置无法解析，本次没有消耗体力。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: "仙缘尚未到来", Content: strings.Join(unmet, "\n"), Actions: append(g.prerequisiteActions(unmet), "仙录")}, true, nil
	}
	remaining, staminaErr := g.useStamina(player.ID, 2)
	if staminaErr != nil {
		return GameResult{Title: "体力不足", Content: staminaErr.Error(), Actions: []string{"状态", "仙录"}}, true, nil
	}
	pending := pendingImmortalEncounter{ConfigCode: config.Code, StartedAt: time.Now()}
	encoded, _ := json.Marshal(pending)
	expires := time.Now().Add(10 * time.Minute)
	if err := g.setPlayerValue(player.ID, "immortal_encounter.pending", string(encoded), &expires); err != nil {
		_ = g.setPlayerValueInt(player.ID, "stamina.value", remaining+2)
		return GameResult{}, true, err
	}
	effect := decodeExtendedEffect(config)
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "仙缘奇遇", ConfigCode: config.Code, ConfigName: config.Name, State: "等待抉择", Level: maxInt(config.Level, 1), Power: effect.Power, MetadataJSON: `{}`}
	if err := upsertExtendedProgressTx(g.store.DB, progress); err != nil {
		return GameResult{}, true, err
	}
	choices := encounterChoices(config)
	lines := []string{config.Description, "━━━━━━━━━━━", fmt.Sprintf("奇遇：%s · %s", config.Name, config.Type), "【可选因果】"}
	actions := []string{"仙录"}
	for _, choice := range choices {
		actual, bonus := probabilityWithLuck(choice.BaseRate, player.Luck, luckEventChoiceBonusCap)
		lines = append(lines, fmt.Sprintf("- %s · 基础%.0f%% · 运气+%.1f%% · 实际%.1f%%\n  成功：%s\n  失败：%s", choice.Name, choice.BaseRate*100, bonus*100, actual*100, eventRewardText(choice.Success), eventRewardText(choice.Failure)))
		actions = append(actions, "仙选 "+choice.Name)
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("体力-2 · 剩余%d · 十分钟内必须选择", remaining), "触发只建立待抉择因果，此刻尚未发放任何奖励。", "前置："+requirement)
	return GameResult{Title: "仙缘奇遇已现", Content: strings.Join(lines, "\n"), ImageURL: config.ImageURL, Actions: actions}, true, nil
}

func encounterChoices(config model.GameplayConfigBase) []immortalEncounterChoice {
	effect := decodeExtendedEffect(config)
	return []immortalEncounterChoice{
		{Name: "迎难承缘", BaseRate: .48, SuccessText: "你直面因果考验，承下完整仙缘。", FailureText: "你未能驾驭异象，气血受到反噬。", Success: map[string]any{"cultivation": effect.Power * 3, "immortal_affinity": max64(effect.Power/12, 2), "merit": max64(effect.Power/25, 1)}, Failure: map[string]any{"health_percent": int64(-12)}},
		{Name: "谨慎查探", BaseRate: .72, SuccessText: "你循道痕稳步查探，取得一份机缘。", FailureText: "因果线突然断裂，只留下少许感悟。", Success: map[string]any{"spirit_stones": effect.Power * 2, "reputation": max64(effect.Power/15, 1)}, Failure: map[string]any{"cultivation": max64(effect.Power/3, 5)}},
		{Name: "守心观照", BaseRate: .9, SuccessText: "你不争外物，只以此缘磨砺道心。", FailureText: "异象散去，你仍守住本心。", Success: map[string]any{"dao_heart": max64(effect.Power/20, 1), "perception": max64(effect.Power/18, 1)}, Failure: map[string]any{"dao_heart": int64(1)}},
	}
}

func (g *Game) pendingEncounterConfig(playerID uint, system extendedSystem) (pendingImmortalEncounter, model.GameplayConfigBase, string, error) {
	value, err := g.playerValue(playerID, "immortal_encounter.pending")
	if err != nil {
		return pendingImmortalEncounter{}, model.GameplayConfigBase{}, "", err
	}
	var pending pendingImmortalEncounter
	if json.Unmarshal([]byte(value), &pending) != nil || pending.ConfigCode == "" {
		return pending, model.GameplayConfigBase{}, value, errors.New("待抉择记录损坏")
	}
	config, err := g.extendedConfig(system.Table, pending.ConfigCode)
	return pending, config, value, err
}

func (g *Game) showPendingImmortalEncounter(player *model.Player, system extendedSystem) (GameResult, bool, error) {
	_, config, _, err := g.pendingEncounterConfig(player.ID, system)
	if err != nil {
		_ = g.store.DB.Where("player_id = ? AND key = ?", player.ID, "immortal_encounter.pending").Delete(&model.PlayerValue{}).Error
		return GameResult{Title: "待抉择奇遇已消散", Content: "旧因果记录无法读取，已经清除。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	lines := []string{fmt.Sprintf("奇遇：%s", config.Name), "十分钟内必须从以下分支选择；重复发送仙遇不会重复扣除体力。", "━━━━━━━━━━━"}
	actions := []string{"仙录"}
	for _, choice := range encounterChoices(config) {
		actual, bonus := probabilityWithLuck(choice.BaseRate, player.Luck, luckEventChoiceBonusCap)
		lines = append(lines, fmt.Sprintf("- %s · 基础%.0f%% · 运气+%.1f%% · 实际%.1f%%", choice.Name, choice.BaseRate*100, bonus*100, actual*100))
		actions = append(actions, "仙选 "+choice.Name)
	}
	return GameResult{Title: "已有待抉择仙缘", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) chooseImmortalEncounter(player *model.Player, raw string, system extendedSystem) (GameResult, bool, error) {
	_, config, pendingValue, err := g.pendingEncounterConfig(player.ID, system)
	if err != nil {
		return GameResult{Title: "没有待决仙缘", Content: "当前没有等待选择的仙缘，或十分钟时限已经结束。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	name := strings.TrimSpace(raw)
	choices := encounterChoices(config)
	var selected *immortalEncounterChoice
	for index := range choices {
		if choices[index].Name == name {
			selected = &choices[index]
			break
		}
	}
	if selected == nil {
		actions := []string{}
		for _, choice := range choices {
			actions = append(actions, "仙选 "+choice.Name)
		}
		return GameResult{Title: "仙缘选项不存在", Content: "请点击当前奇遇给出的完整选项。", Actions: actions}, true, nil
	}
	actual, bonus := probabilityWithLuck(selected.BaseRate, player.Luck, luckEventChoiceBonusCap)
	succeeded := rand.Float64() <= actual
	reward, narrative := selected.Success, selected.SuccessText
	state := "因果圆满"
	if !succeeded {
		reward, narrative, state = selected.Failure, selected.FailureText, "因果反噬"
	}
	err = g.settleImmortalEncounter(player, pendingValue, config, state, reward)
	if errors.Is(err, errImmortalEncounterSettled) {
		return GameResult{Title: "没有待决仙缘", Content: "这段仙缘已经完成结算，请重新触发下一段仙缘。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	luckLine := ""
	if succeeded {
		luckLine, _ = g.tryGrowLuckFromEncounter(player)
	}
	if luckLine != "" {
		luckLine = "\n" + luckLine
	}
	return GameResult{Title: config.Name + "·" + state, Content: fmt.Sprintf("选择：%s\n%s\n成功率：基础%.0f%% · 运气+%.1f%% · 实际%.1f%%\n结算：%s%s\n━━━━━━━━━━━\n结果已经写入个人仙录。", selected.Name, narrative, selected.BaseRate*100, bonus*100, actual*100, eventRewardText(reward), luckLine), Actions: []string{"仙录", "仙深", "仙觉", "状态"}}, true, nil
}

func (g *Game) settleImmortalEncounter(player *model.Player, pendingValue string, config model.GameplayConfigBase, state string, reward map[string]any) error {
	effective := g.playerWithActiveSkillStats(player)
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		consumed := tx.Where("player_id = ? AND key = ? AND value = ?", player.ID, "immortal_encounter.pending", pendingValue).Delete(&model.PlayerValue{})
		if consumed.Error != nil {
			return consumed.Error
		}
		if consumed.RowsAffected != 1 {
			return errImmortalEncounterSettled
		}

		var progress model.PlayerExtendedProgress
		if err := tx.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙缘奇遇", config.Code).First(&progress).Error; err != nil {
			return err
		}
		progress.State = state
		progress.Uses++
		progress.Experience += rewardNumber(reward, "cultivation")
		progress.Mastery += max64(rewardNumber(reward, "immortal_affinity"), 1)

		updates := map[string]any{}
		for key, column := range map[string]string{"cultivation": "cultivation", "spirit_stones": "spirit_stones", "merit": "merit", "reputation": "reputation", "immortal_affinity": "immortal_affinity", "dao_heart": "dao_heart", "perception": "perception"} {
			if value := rewardNumber(reward, key); value != 0 {
				updates[column] = gorm.Expr(column+" + ?", value)
			}
		}
		if percent := rewardNumber(reward, "health_percent"); percent != 0 {
			updates["health"] = min64(max64(effective.Health+effective.MaxHealth*percent/100, 1), effective.MaxHealth)
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		return nil
	})
}

func (g *Game) latestCompletedEncounter(playerID uint) (model.PlayerExtendedProgress, error) {
	var progress model.PlayerExtendedProgress
	err := g.store.DB.Where("player_id = ? AND system = ? AND state <> ?", playerID, "仙缘奇遇", "等待抉择").Order("updated_at DESC,id DESC").First(&progress).Error
	return progress, err
}

func (g *Game) deepenImmortalEncounter(player *model.Player, system extendedSystem) (GameResult, bool, error) {
	progress, err := g.latestCompletedEncounter(player.ID)
	if err != nil {
		return GameResult{Title: "没有可加深仙缘", Content: "先完成一次仙遇抉择。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	tea, itemErr := g.itemByName("灵茶")
	if itemErr != nil || g.itemQuantity(player.ID, tea.ID) < 1 {
		return GameResult{Title: "加深仙缘资源不足", Content: "回溯因果需要灵茶×1。", Actions: []string{"物品 灵茶", "货铺", "背包"}}, true, nil
	}
	gain := max64(progress.Power/20, 1)
	progress.State, progress.Level, progress.Mastery = "仙缘加深", progress.Level+1, progress.Mastery+gain
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var row model.PlayerItem
		if err := tx.Where("player_id = ? AND item_id = ? AND quantity >= 1", player.ID, tea.ID).First(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Update("quantity", gorm.Expr("quantity - 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"immortal_affinity": gorm.Expr("immortal_affinity + ?", gain), "perception": gorm.Expr("perception + 1")}).Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙缘加深", Content: fmt.Sprintf("仙缘：%s\n消耗：灵茶×1\n个人等级：%d\n仙缘+%d · 悟性+1\n该成长已写入角色属性和个人仙录。", progress.ConfigName, progress.Level, gain), Actions: []string{"仙录", "仙觉", "状态"}}, true, nil
}

func (g *Game) transferImmortalEncounter(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	if len(command.Arguments) == 0 {
		return GameResult{Title: "仙缘传承", Content: "请输入：`仙承 @对方`。默认传承最近完成的一段仙缘感悟。", Actions: []string{"仙录", "好友"}}, true, nil
	}
	target, err := g.findPlayer(command.Arguments[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "仙缘传承对象无效", Content: "请选择另一名现存道友。", Actions: []string{"好友", "仙录"}}, true, nil
	}
	progress, err := g.latestCompletedEncounter(player.ID)
	if err != nil {
		return GameResult{Title: "没有可传仙缘", Content: "先完成一次仙遇抉择。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	grant := progress
	grant.ID, grant.PlayerID, grant.State, grant.Level, grant.Mastery, grant.Uses = 0, target.ID, "承接仙缘", maxInt(progress.Level-1, 1), progress.Mastery/2, 0
	if err := upsertExtendedProgressTx(g.store.DB, grant); err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.ID, "仙缘传承", fmt.Sprintf("%s将%s的因果感悟传给你；个人仙录已新增承接记录。", player.DaoName, progress.ConfigName))
	return GameResult{Title: "仙缘传承完成", Content: fmt.Sprintf("传承者：%s\n承接者：%s\n仙缘：%s\n承接等级：%d · 熟练%d\n原记录仍由你保留。", player.DaoName, target.DaoName, grant.ConfigName, grant.Level, grant.Mastery), Actions: []string{"仙录", "通知", "状态"}}, true, nil
}

func (g *Game) awakenImmortalEncounter(player *model.Player, system extendedSystem) (GameResult, bool, error) {
	progress, err := g.latestCompletedEncounter(player.ID)
	if err != nil {
		return GameResult{Title: "没有可觉醒仙缘", Content: "先完成并加深一段仙缘。", Actions: []string{"仙遇", "仙录"}}, true, nil
	}
	if progress.Level < 2 {
		return GameResult{Title: "仙缘尚浅", Content: "至少先发送一次 `仙深`，将该仙缘提升至2级。", Actions: []string{"仙深", "仙录"}}, true, nil
	}
	if progress.State == "仙缘觉醒" {
		return GameResult{Title: "仙缘已经觉醒", Content: "重复觉醒不会叠加永久属性。", Actions: []string{"仙录", "状态"}}, true, nil
	}
	gain := max64(progress.Power/15, 2)
	progress.State, progress.Power, progress.Mastery = "仙缘觉醒", progress.Power+gain*5, progress.Mastery+gain
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"immortal_affinity": gorm.Expr("immortal_affinity + ?", gain), "dao_heart": gorm.Expr("MIN(dao_heart + ?, 100)", max64(gain/2, 1))}).Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙缘觉醒", Content: fmt.Sprintf("仙缘：%s\n永久仙缘+%d · 道心+%d\n觉醒威力：%d\n该条仙缘不会再次重复觉醒。", progress.ConfigName, gain, max64(gain/2, 1), progress.Power), Actions: []string{"仙录", "状态", "全区通报"}}, true, nil
}
