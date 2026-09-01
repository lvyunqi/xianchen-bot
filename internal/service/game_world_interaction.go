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

type mapMonsterBattleState struct {
	LocationID         uint   `json:"location_id"`
	DungeonID          uint   `json:"dungeon_id"`
	BattleKind         string `json:"battle_kind"`
	Round              int    `json:"round"`
	EnemyName          string `json:"enemy_name"`
	EnemyPower         int64  `json:"enemy_power"`
	PlayerHP           int64  `json:"player_hp"`
	PlayerMana         int64  `json:"player_mana"`
	EnemyHP            int64  `json:"enemy_hp"`
	EnemyMaxHP         int64  `json:"enemy_max_hp"`
	Team               bool   `json:"team"`
	ExtendedCategory   string `json:"extended_category,omitempty"`
	ExtendedConfigCode string `json:"extended_config_code,omitempty"`
	ExtendedConfigName string `json:"extended_config_name,omitempty"`
	ExtendedAction     string `json:"extended_action,omitempty"`
	Surrendered        bool   `json:"surrendered,omitempty"`
	StartedAt          int64  `json:"started_at"`
}

func (g *Game) talkToLocalNPC(player *model.Player, raw string) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	npcs := decodeTextList(location.NPCJSON)
	name := strings.TrimSpace(raw)
	if name == "" {
		actions := []string{"位置"}
		for _, npc := range npcs {
			actions = append(actions, "对话 "+npc)
		}
		return GameResult{Title: "地图对话", Content: "请输入：`对话 NPC名`，或点击当前地图的NPC蓝字。", Actions: actions}, true, nil
	}
	npcIndex := -1
	for index, npc := range npcs {
		if npc == name {
			npcIndex = index
			break
		}
	}
	// Keep blue links from early map data usable after the generated world names
	// are migrated to their final lore names.
	if npcIndex < 0 && name == location.Region+"巡游使·一" {
		for index, npc := range npcs {
			if strings.Contains(npc, "巡游使") {
				npcIndex = index
				name = npc
				break
			}
		}
	}
	if npcIndex < 0 {
		actions := []string{"位置"}
		for _, npc := range npcs {
			actions = append(actions, "对话 "+npc)
		}
		return GameResult{Title: "NPC不在此地", Content: fmt.Sprintf("你当前在%s，这里没有“%s”。\n请从当前地图NPC中选择，或先按相邻路线前往对方所在地。", location.Name, name), Actions: actions}, true, nil
	}
	firstMeeting := false
	key := fmt.Sprintf("npc.met.%d.%d", location.ID, npcIndex)
	if _, valueErr := g.playerValue(player.ID, key); valueErr != nil {
		firstMeeting = true
		_ = g.setPlayerValue(player.ID, key, "true", nil)
		_ = g.store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("reputation", gorm.Expr("reputation + 1")).Error
	}
	tasks := decodeTextList(location.TasksJSON)
	lines := []string{
		fmt.Sprintf("你向%s见礼。", name),
		fmt.Sprintf("「%s」", localNPCDialogue(name, location)),
		"━━━━━━━",
		fmt.Sprintf("身份：%s的%s", location.Name, npcRole(name)),
		fmt.Sprintf("当前威胁：%s（战力%d）", displayOr(location.MonsterName, "暂无妖患"), location.MonsterPower),
		fmt.Sprintf("区域首领：%s（战力%d）", displayOr(location.BossName, "暂无记录"), location.BossPower),
	}
	if firstMeeting {
		lines = append(lines, "初次见礼：声望+1（每位NPC仅一次）")
		_, _ = g.addPlayerValueInt(player.ID, npcAffinityKey(localNPC{Location: location, Name: name, Index: npcIndex}), 5)
	}
	_ = g.setPlayerValue(player.ID, "npc.last_met", name, nil)
	affinity := g.playerValueInt(player.ID, npcAffinityKey(localNPC{Location: location, Name: name, Index: npcIndex}), 0)
	lines = append(lines, fmt.Sprintf("好感度：%d（%s）", affinity, npcRelationshipName(affinity)))
	actions := []string{"NPC商店 " + name, "NPC赠送 " + name, "NPC关系 " + name, "位置", "寻脉"}
	if len(tasks) > 0 {
		lines = append(lines, "━━━━━━━", "【可承接委托】")
		for _, task := range tasks {
			lines = append(lines, "- "+task)
			actions = append(actions, "接任务 "+task)
		}
	}
	if location.MonsterName != "" {
		actions = append(actions, "挑战 "+location.MonsterName)
	}
	if location.BossName != "" {
		actions = append(actions, "首领", "讨伐")
	}
	return GameResult{Title: "与" + name + "对话", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func localNPCDialogue(name string, location model.WorldLocation) string {
	switch {
	case strings.Contains(name, "巡游"):
		return fmt.Sprintf("此地隶属%s，地脉潮汐已有异动。先查看委托与周边妖患，切勿越过未修满的境界层数强行远行。", location.Region)
	case strings.Contains(name, "守脉"):
		return fmt.Sprintf("我守了%s多年，地底灵息可不只一股。先用“寻脉”定位入口，再查看境界、神识、灵根和护脉材料是否齐备。", location.Name)
	default:
		return fmt.Sprintf("道友既到%s，便是一段因果。先查地图委托，再决定是猎妖、采集、寻脉还是讨伐首领。", location.Name)
	}
}

func npcRole(name string) string {
	switch {
	case strings.Contains(name, "巡游"):
		return "巡境与委托引导人"
	case strings.Contains(name, "守脉"):
		return "地脉看守与灵气潮汐记录者"
	default:
		return "当地修士"
	}
}

func (g *Game) startMapMonsterBattle(player *model.Player, raw string) (GameResult, bool, error) {
	return g.startMapMonsterBattleMode(player, raw, false, 4)
}

func (g *Game) startMapMonsterBattleMode(player *model.Player, raw string, team bool, staminaCost int64) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "挑战妖兽", Content: "请输入：`挑战 妖兽名`，或点击位置面板中的妖兽蓝字。", Actions: []string{"位置"}}, true, nil
	}
	legacyName := location.Region + "·" + location.Name + "妖灵"
	if name == legacyName && location.MonsterName != "" {
		name = location.MonsterName
	}
	if name != location.MonsterName {
		return GameResult{Title: "挑战目标不存在", Content: fmt.Sprintf("当前位置：%s\n此地可挑战的普通妖兽：%s\n区域首领需另行发送“首领”查看后“讨伐”。", location.Name, displayOr(location.MonsterName, "无")), Actions: []string{"挑战 " + location.MonsterName, "位置", "首领"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		if player.State == model.PlayerStateBattling {
			return GameResult{Title: "已在战斗中", Content: "请先完成当前回合，或选择投降。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
		}
		return GameResult{Title: "当前无法战斗", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	if player.Health <= 1 {
		return GameResult{Title: "重伤难战", Content: "当前气血过低，请先疗伤。", Actions: []string{"疗伤", "状态"}}, true, nil
	}
	if remaining := g.playerCooldownRemaining(player.ID, "cooldown.battle.normal_hunt"); remaining > 0 {
		return GameResult{Title: "妖息尚未平复", Content: "上一场猎妖刚刚结束，还需" + formatDuration(remaining) + "方可再次追踪同地妖灵。\n本次没有扣除体力，也没有创建新战局。", Actions: []string{"位置", "状态", "背包"}}, true, nil
	}
	remaining, staminaErr := g.useStamina(player.ID, staminaCost)
	if staminaErr != nil {
		return GameResult{Title: "体力不足", Content: staminaErr.Error(), Actions: []string{"位置"}}, true, nil
	}
	enemyPower := max64(location.MonsterPower, 20)
	enemyHP := max64(enemyPower*2, 60)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{LocationID: location.ID, BattleKind: "地图", Round: 1, EnemyName: name, EnemyPower: enemyPower, PlayerHP: effective.Health, PlayerMana: effective.Mana, EnemyHP: enemyHP, EnemyMaxHP: enemyHP, Team: team, StartedAt: time.Now().UnixMilli()}
	if err := g.beginPVEBattle(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	mode := "单人猎妖"
	if team {
		mode = "仙侣合战 · 造成伤害提升50% · 承受伤害降低20%"
	}
	return GameResult{Title: "妖兽挑战开始", Content: fmt.Sprintf("地点：%s\n敌方：%s（战力%d）\n敌方气血：%d/%d\n你的气血：%d/%d · 法力：%d/%d\n模式：%s\n消耗体力：%d · 剩余体力：%d\n━━━━━━━\n战斗不会自动结算。现在轮到你选择普通攻击、施展已学功法、防御或投降。", location.Name, name, enemyPower, state.EnemyHP, state.EnemyHP, state.PlayerHP, effective.MaxHealth, state.PlayerMana, effective.MaxMana, mode, staminaCost, remaining), Actions: []string{"攻击", "技能", "防御", "投降", "功法"}}, true, nil
}

func (g *Game) pveTurn(player *model.Player, command handler.ParsedCommand, action, argument string) (GameResult, bool, error) {
	raw, err := g.playerValue(player.ID, "pve.battle")
	if err != nil || strings.TrimSpace(raw) == "" {
		return GameResult{}, false, nil
	}
	var state mapMonsterBattleState
	if json.Unmarshal([]byte(raw), &state) != nil || state.EnemyName == "" {
		_ = g.clearMapMonsterBattle(player.ID)
		return GameResult{Title: "战斗记录损坏", Content: "当前战斗已重置，请重新发起挑战。", Actions: []string{"位置"}}, true, nil
	}
	if action == "surrender" {
		if state.ExtendedCategory != "" {
			state.Surrendered = true
			return g.finishMapMonsterBattle(player, state, false, fmt.Sprintf("你主动退出与%s的试炼，本次不会获得胜利结算。", state.EnemyName))
		}
		_ = g.clearMapMonsterBattle(player.ID)
		return GameResult{Title: "已退出战斗", Content: fmt.Sprintf("你与%s拉开距离，本次挑战无奖励。", state.EnemyName), Actions: []string{"位置", "疗伤"}}, true, nil
	}
	if state.EnemyMaxHP <= 0 {
		state.EnemyMaxHP = max64(state.EnemyHP, 1)
	}
	effective := g.playerWithActiveSkillStats(player)
	state.PlayerHP = min64(max64(state.PlayerHP, 0), effective.MaxHealth)
	state.PlayerMana = min64(max64(state.PlayerMana, 0), effective.MaxMana)
	playerStats := g.playerCombatStats(player)
	enemyDefense := max64(state.EnemyPower/12, 2)
	if state.BattleKind == "首领" {
		enemyDefense = max64(state.EnemyPower/18, 2)
	}
	damage, logLine := int64(0), ""
	defending := false
	switch action {
	case "attack":
		damage = pvpDamage(playerStats.PhysicalAttack, enemyDefense, state.Round)
		if state.Team {
			damage = damage * 3 / 2
		}
		logLine = fmt.Sprintf("※ %s御使法器发动普通攻击\n[对%s造成伤害：%d]", player.DaoName, state.EnemyName, damage)
	case "skill":
		var skill model.Skill
		skillName := strings.TrimSpace(argument)
		if skillName == "" && player.CurrentSkillID != 0 {
			_ = g.store.DB.First(&skill, player.CurrentSkillID).Error
		} else if skillName != "" {
			_ = g.store.DB.Where("name = ?", skillName).First(&skill).Error
		}
		if skill.ID == 0 {
			return GameResult{Title: "请选择已学功法", Content: "发送 `功法` 查看已学功法，再发送 `技能 功法名`。", Actions: []string{"功法", "攻击", "防御"}}, true, nil
		}
		var learned model.PlayerSkill
		if g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).First(&learned).Error != nil {
			return GameResult{Title: "功法未学会", Content: "你尚未学会" + skill.Name + "。", Actions: []string{"学功 " + skill.Name, "功法", "攻击"}}, true, nil
		}
		manaCost := int64(10)
		if state.PlayerMana < manaCost {
			return GameResult{Title: "法力不足", Content: fmt.Sprintf("施展%s需要法力%d，当前%d。", skill.Name, manaCost, state.PlayerMana), Actions: []string{"攻击", "防御"}}, true, nil
		}
		state.PlayerMana -= manaCost
		skillAttack := playerStats.MagicAttack + int64(learned.Level*5)
		if skill.ID != player.CurrentSkillID {
			skillAttack += skillOffensiveBonus(decodeSkillStatBonus(skill, learned.Level))
		}
		damage = pvpDamage(skillAttack, enemyDefense, state.Round)
		if state.Team {
			damage = damage * 3 / 2
		}
		logLine = fmt.Sprintf("※ %s施展%s（%d级），消耗法力%d\n[对%s造成法术伤害：%d]", player.DaoName, skill.Name, learned.Level, manaCost, state.EnemyName, damage)
	case "defend":
		defending = true
		logLine = "※ 你收拢真元稳守道心，本回合所受伤害减半。"
	default:
		return GameResult{Title: "战斗指令", Content: "请选择攻击、技能、防御或投降。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
	}
	state.EnemyHP -= damage
	if state.EnemyHP <= 0 {
		return g.finishMapMonsterBattle(player, state, true, logLine)
	}
	enemyAttack := max64(state.EnemyPower/8, 5)
	if state.BattleKind == "首领" {
		enemyAttack = max64(state.EnemyPower/14, 6)
	}
	enemyDamage := pvpDamage(enemyAttack, playerStats.PhysicalDefense, state.Round)
	enemyMove := "妖力反击"
	if state.DungeonID != 0 && state.EnemyHP*100 <= state.EnemyMaxHP*35 {
		enemyDamage = enemyDamage * 3 / 2
		enemyMove = "狂暴技能·碎境"
	} else if state.BattleKind == "首领" && state.EnemyHP*100 <= state.EnemyMaxHP*35 {
		enemyDamage = enemyDamage * 3 / 2
		enemyMove = "首领狂暴·镇域灭法"
	}
	if state.Team {
		enemyDamage = max64(enemyDamage*4/5, 1)
	}
	if defending {
		enemyDamage = max64(enemyDamage/2, 1)
	}
	state.PlayerHP -= enemyDamage
	logLine += fmt.Sprintf("\n\n※ %s：%s\n[对%s造成伤害：%d]", enemyMove, state.EnemyName, player.DaoName, enemyDamage)
	if state.PlayerHP <= 0 {
		return g.finishMapMonsterBattle(player, state, false, logLine)
	}
	state.Round++
	if err := g.persistPVEBattleTurn(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	playerPercent := float64(max64(state.PlayerHP, 0)) * 100 / float64(max64(effective.MaxHealth, 1))
	enemyPercent := float64(max64(state.EnemyHP, 0)) * 100 / float64(max64(state.EnemyMaxHP, 1))
	title := "猎妖战报"
	if state.ExtendedCategory != "" {
		title = state.ExtendedCategory + "试炼战报"
	} else if state.DungeonID != 0 {
		title = "副本战报"
	} else if state.BattleKind == "首领" {
		title = "首领讨伐战报"
	}
	content := fmt.Sprintf("To：%s\n-----【当前第%d回合】-----\n\n%s\n\n-----【当前状况】-----\n『%s』（A1）HP：%.2f%% · MP：%d/%d\n『%s』（D1）HP：%.2f%% · 战力：%d\n\n轮到你选择下一步。", player.DaoName, state.Round-1, logLine, player.DaoName, playerPercent, state.PlayerMana, effective.MaxMana, state.EnemyName, enemyPercent, state.EnemyPower)
	return GameResult{Title: title, Content: content, Actions: []string{"技能", "攻击", "防御", "投降", "功法"}}, true, nil
}

func (g *Game) finishMapMonsterBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	if state.ExtendedCategory != "" {
		return g.finishExtendedRuntimeBattle(player, state, won, logLine)
	}
	if state.DungeonID != 0 {
		return g.finishDungeonBattle(player, state, won, logLine)
	}
	var location model.WorldLocation
	if err := g.store.DB.First(&location, state.LocationID).Error; err != nil {
		return GameResult{}, true, err
	}
	if state.BattleKind == "首领" {
		return g.finishWorldBossBattle(player, state, location, won, logLine)
	}
	settled, err := g.claimNormalMonsterBattleSettlement(player.ID, state)
	if err != nil {
		return GameResult{}, true, err
	}
	if !settled {
		return GameResult{Title: "战斗已经结算", Content: "该战局的胜负与奖励已经处理，本次重复提交不会再次获得奖励或扣除资源。", Actions: []string{"位置", "状态", "背包"}}, true, nil
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.battles", 1)
	remainingHP := max64(state.PlayerHP, 1)
	if !won {
		return GameResult{Title: "猎妖战败", Content: fmt.Sprintf("敌方：%s\n%s\n━━━━━━━\n你的气血已降至%d/%d，本次无奖励。", state.EnemyName, logLine, remainingHP, effective.MaxHealth), Actions: []string{"回城复活", "疗伤", "状态", "位置"}}, true, nil
	}
	var reward map[string]any
	if json.Unmarshal([]byte(location.MonsterRewardJSON), &reward) != nil {
		reward = map[string]any{"cultivation": max64(state.EnemyPower/3, 20), "merit": int64(2)}
	}
	if err := g.applyConfiguredEventReward(player, reward); err != nil {
		return GameResult{}, true, err
	}
	itemText := g.rollBattleDrop(player)
	_, _ = g.addPlayerValueInt(player.ID, "stats.wins", 1)
	content := fmt.Sprintf("敌方：%s\n%s\n━━━━━━━\n战利：%s\n回合数：%d", state.EnemyName, logLine, eventRewardText(reward), state.Round)
	if itemText != "" {
		content += "\n掉落：" + itemText
	}
	cooldownSeconds := max64(g.settingInt("battle.normal_hunt_cooldown_seconds", 8), 1)
	expires := time.Now().Add(time.Duration(cooldownSeconds) * time.Second)
	_ = g.setPlayerValue(player.ID, "cooldown.battle.normal_hunt", expires.Format(time.RFC3339Nano), &expires)
	content += fmt.Sprintf("\n收势：%d秒后可再次猎妖", cooldownSeconds)
	return GameResult{Title: "猎妖胜利", Content: content, Actions: []string{"背包", "物品", "位置", "状态"}}, true, nil
}

func (g *Game) claimNormalMonsterBattleSettlement(playerID uint, state mapMonsterBattleState) (bool, error) {
	settled := false
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var row model.PlayerValue
		if err := tx.Where("player_id = ? AND key = ?", playerID, "pve.battle").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var current mapMonsterBattleState
		if json.Unmarshal([]byte(row.Value), &current) != nil || current.StartedAt != state.StartedAt || current.EnemyName != state.EnemyName {
			return nil
		}
		deleted := tx.Where("id = ?", row.ID).Delete(&model.PlayerValue{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return nil
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", playerID).Updates(map[string]any{
			"health": max64(state.PlayerHP, 1), "mana": max64(state.PlayerMana, 0), "state": model.PlayerStateIdle,
		}).Error; err != nil {
			return err
		}
		settled = true
		return nil
	})
	return settled, err
}

func (g *Game) finishWorldBossBattle(player *model.Player, state mapMonsterBattleState, location model.WorldLocation, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	_ = g.clearMapMonsterBattle(player.ID)
	_, _ = g.addPlayerValueInt(player.ID, "stats.battles", 1)
	remainingHP := max64(state.PlayerHP, 1)
	_ = g.store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": max64(state.PlayerMana, 0)}).Error
	duration := time.Duration(location.BossCooldownMinutes) * time.Minute
	if duration <= 0 {
		duration = time.Hour
	}
	_ = g.setPlayerValue(player.ID, "boss."+location.Code+".cooldown", time.Now().Add(duration).Format(time.RFC3339Nano), nil)
	if !won {
		return GameResult{Title: "首领讨伐失败", Content: fmt.Sprintf("地点：%s\n首领：%s\n%s\n━━━━━━━━━━━\n你在第%d回合失去战斗能力。\n剩余气血：%d/%d\n本次没有获得战利；%s后可再次讨伐。", location.Name, state.EnemyName, logLine, state.Round, remainingHP, effective.MaxHealth, formatDuration(duration)), Actions: []string{"回城复活", "疗伤", "状态", "首领", "位置"}}, true, nil
	}
	reward := make(map[string]any)
	if json.Unmarshal([]byte(location.BossRewardJSON), &reward) != nil {
		reward = map[string]any{"cultivation": float64(300), "spirit_stones": float64(100), "merit": float64(20)}
	}
	if rewardNumber(reward, "cultivation") <= 0 {
		reward["cultivation"] = float64(300)
	}
	if rewardNumber(reward, "spirit_stones") <= 0 {
		reward["spirit_stones"] = float64(100)
	}
	if rewardNumber(reward, "merit") <= 0 {
		reward["merit"] = float64(20)
	}
	if err := g.applyConfiguredEventReward(player, reward); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.boss_wins", 1)
	_, _ = g.addPlayerValueInt(player.ID, "stats.wins", 1)
	rewardText := eventRewardText(reward)
	broadcast := fmt.Sprintf("【诛魔悬赏】%s在%s历经%d回合，亲手斩落镇域首领%s，获得%s。", player.DaoName, location.Name, state.Round, state.EnemyName, rewardText)
	_ = g.publishWorldBroadcast("首领", player.DaoName+"镇域诛魔", broadcast)
	content := fmt.Sprintf("地点：%s\n首领：%s\n%s\n━━━━━━━━━━━\n【战果展示】\n通关回合：%d\n获得：%s\n首领将在%s后重新开放。", location.Name, state.EnemyName, logLine, state.Round, rewardText, formatDuration(duration))
	return GameResult{Title: "首领讨伐成功", Content: content, ImageURL: location.ImageURL, Actions: []string{"背包", "物品", "地图", "首领", "状态"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) clearMapMonsterBattle(playerID uint) error {
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key = ?", playerID, "pve.battle").Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", playerID).Update("state", model.PlayerStateIdle).Error
	})
}

func (g *Game) beginPVEBattle(playerID uint, state mapMonsterBattleState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := upsertPlayerValueTx(tx, playerID, "pve.battle", string(encoded), nil); err != nil {
			return err
		}
		result := tx.Model(&model.Player{}).Where("id = ? AND (state = ? OR state = '')", playerID, model.PlayerStateIdle).Update("state", model.PlayerStateBattling)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("角色状态已经变化，无法创建战局")
		}
		return nil
	})
}

func (g *Game) persistPVEBattleTurn(playerID uint, state mapMonsterBattleState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := upsertPlayerValueTx(tx, playerID, "pve.battle", string(encoded), nil); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", playerID).Updates(map[string]any{
			"health": max64(state.PlayerHP, 1), "mana": max64(state.PlayerMana, 0), "state": model.PlayerStateBattling,
		}).Error
	})
}

// syncPVEBattleVitalsTx keeps consumables and healing commands consistent with
// an already-open map or dungeon battle. Without this, the player row heals
// while the serialized turn state retains the old HP and overwrites it on the
// next action.
func syncPVEBattleVitalsTx(tx *gorm.DB, playerID uint, health, mana *int64) error {
	if health == nil && mana == nil {
		return nil
	}
	var row model.PlayerValue
	err := tx.Where("player_id = ? AND key = ?", playerID, "pve.battle").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var state mapMonsterBattleState
	if json.Unmarshal([]byte(row.Value), &state) != nil || state.EnemyName == "" {
		return nil
	}
	if health != nil {
		state.PlayerHP = *health
	}
	if mana != nil {
		state.PlayerMana = *mana
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return tx.Model(&model.PlayerValue{}).Where("id = ?", row.ID).Update("value", string(encoded)).Error
}

func (g *Game) rollBattleDrop(player *model.Player) string {
	type entry struct {
		model.DropEntry
	}
	var rows []entry
	if g.store.DB.Table("drop_entries").Select("drop_entries.*").Joins("JOIN drop_pools ON drop_pools.id = drop_entries.drop_pool_id").Where("drop_pools.enabled = ? AND drop_pools.source_type = ?", true, "战斗").Find(&rows).Error != nil || len(rows) == 0 {
		return ""
	}
	total := 0
	for _, row := range rows {
		total += maxInt(row.Weight, 1)
	}
	roll := rand.Intn(total)
	selected := rows[0].DropEntry
	for _, row := range rows {
		roll -= maxInt(row.Weight, 1)
		if roll < 0 {
			selected = row.DropEntry
			break
		}
	}
	maximum := max64(selected.Maximum, selected.Minimum)
	quantity := max64(selected.Minimum, 1)
	if maximum > quantity {
		quantity += int64(rand.Intn(int(maximum - quantity + 1)))
	}
	if g.players.AdjustItem(player.ID, selected.ItemID, quantity) != nil {
		return ""
	}
	return fmt.Sprintf("%s×%d", selected.ItemName, quantity)
}
