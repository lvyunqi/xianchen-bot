package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

type pvpBattleState struct {
	OpponentID   uint  `json:"opponent_id"`
	Round        int   `json:"round"`
	PlayerHP     int64 `json:"player_hp"`
	OpponentHP   int64 `json:"opponent_hp"`
	PlayerMana   int64 `json:"player_mana"`
	OpponentMana int64 `json:"opponent_mana"`
	Defending    bool  `json:"defending"`
}

func (g *Game) executePVPTurn(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	action := map[int]string{253: "attack", 254: "skill", 255: "defend", 256: "surrender"}[command.Spec.ID]
	if raw, err := g.playerValue(player.ID, "pve.battle"); err == nil && strings.TrimSpace(raw) != "" {
		return g.pveTurn(player, command, action, command.RawArguments)
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		actions := []string{"状态"}
		if player.State == model.PlayerStateCultivating {
			actions = []string{"出关", "修记", "状态"}
		}
		return GameResult{Title: "当前无法出手", Content: "当前状态：" + player.State + "。请先结束当前行动，再发起猎妖、副本或竞技战斗。", Actions: actions}, true, nil
	}
	return g.pvpTurn(player, action, command.RawArguments)
}

func (g *Game) pvpTurn(player *model.Player, action, argument string) (GameResult, bool, error) {
	targetID := g.playerValueInt(player.ID, "arena.target", 0)
	if targetID <= 0 {
		return GameResult{Title: "尚未匹配", Content: "先发送 `竞技` 匹配对手，再选择攻击方式。", Actions: []string{"竞技"}}, true, nil
	}
	target, err := g.players.Get(uint(targetID))
	if err != nil {
		return GameResult{Title: "对手已离开", Content: "对手数据已失效，请重新匹配。", Actions: []string{"竞技"}}, true, nil
	}
	effectivePlayer := g.playerWithActiveSkillStats(player)
	effectiveTarget := g.playerWithActiveSkillStats(&target)
	state := pvpBattleState{}
	if raw, rawErr := g.playerValue(player.ID, "arena.battle"); rawErr == nil {
		_ = json.Unmarshal([]byte(raw), &state)
	}
	if state.OpponentID == 0 {
		state = pvpBattleState{OpponentID: target.ID, Round: 1, PlayerHP: effectivePlayer.Health, OpponentHP: effectiveTarget.MaxHealth, PlayerMana: effectivePlayer.Mana, OpponentMana: effectiveTarget.MaxMana}
	}
	state.PlayerHP = min64(max64(state.PlayerHP, 0), effectivePlayer.MaxHealth)
	state.PlayerMana = min64(max64(state.PlayerMana, 0), effectivePlayer.MaxMana)
	state.OpponentHP = min64(max64(state.OpponentHP, 0), effectiveTarget.MaxHealth)
	state.OpponentMana = min64(max64(state.OpponentMana, 0), effectiveTarget.MaxMana)
	if action == "surrender" {
		return g.finishPVP(player, target, state, false, "你主动投降，保留道心但失去本场竞技资格。")
	}
	if state.PlayerHP <= 0 || state.OpponentHP <= 0 {
		return g.finishPVP(player, target, state, state.OpponentHP <= 0, "本场战斗已经结束，请重新匹配。")
	}
	playerStats := g.playerCombatStats(player)
	targetStats := g.playerCombatStats(&target)
	logLine := ""
	damage := int64(0)
	switch action {
	case "attack":
		damage = pvpDamage(playerStats.PhysicalAttack, targetStats.PhysicalDefense, state.Round)
		state.OpponentHP -= damage
		logLine = fmt.Sprintf("你发动普通攻击，造成%d点伤害。", damage)
	case "skill":
		skillName := strings.TrimSpace(argument)
		var skill model.Skill
		if skillName == "" && player.CurrentSkillID != 0 {
			_ = g.store.DB.First(&skill, player.CurrentSkillID).Error
		} else if skillName != "" {
			_ = g.store.DB.Where("name = ?", skillName).First(&skill).Error
		}
		if skill.ID == 0 {
			return GameResult{Title: "功法未选择", Content: "你尚未指定已学功法。发送 `功法` 查看，或发送 `技能 功法名`。", Actions: []string{"功法", "技能"}}, true, nil
		}
		var learned model.PlayerSkill
		if err := g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).First(&learned).Error; err != nil {
			return GameResult{Title: "功法未学会", Content: fmt.Sprintf("你没有学会%s，请先发送 `学功 %s`。", skill.Name, skill.Name), Actions: []string{"功法", "学功 " + skill.Name}}, true, nil
		}
		const manaCost int64 = 10
		if state.PlayerMana < manaCost {
			return GameResult{Title: "法力不足", Content: fmt.Sprintf("施展%s需要法力%d，当前%d。", skill.Name, manaCost, state.PlayerMana), Actions: []string{"疗伤", "状态"}}, true, nil
		}
		state.PlayerMana -= manaCost
		skillAttack := playerStats.MagicAttack + int64(learned.Level*5)
		if skill.ID != player.CurrentSkillID {
			skillAttack += skillOffensiveBonus(decodeSkillStatBonus(skill, learned.Level))
		}
		damage = pvpDamage(skillAttack, targetStats.MagicDefense, state.Round)
		state.OpponentHP -= damage
		logLine = fmt.Sprintf("你施展%s（%d级），消耗法力%d，造成%d点法术伤害。", skill.Name, learned.Level, manaCost, damage)
	case "defend":
		state.Defending = true
		logLine = "你收拢灵力进入防御姿态，本回合受到的伤害降低50%。"
	default:
		return GameResult{Title: "战斗指令", Content: "请选择普通攻击、已学功法技能或防御。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
	}
	if state.OpponentHP <= 0 {
		return g.finishPVP(player, target, state, true, logLine+"\n对手气血归零。")
	}
	enemyDamage := pvpDamage(targetStats.PhysicalAttack, playerStats.PhysicalDefense, state.Round)
	if state.Defending {
		enemyDamage /= 2
		state.Defending = false
	}
	state.PlayerHP -= enemyDamage
	logLine += fmt.Sprintf("\n对手发动反击，造成%d点伤害。", enemyDamage)
	if state.PlayerHP <= 0 {
		return g.finishPVP(player, target, state, false, logLine+"\n你的气血归零。")
	}
	state.Round++
	data, _ := json.Marshal(state)
	if err := g.setPlayerValue(player.ID, "arena.battle", string(data), nil); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: fmt.Sprintf("竞技回合%d", state.Round-1), Content: fmt.Sprintf("对手：%s\n%s\n你的气血：%d/%d · 法力：%d/%d\n对手气血：%d/%d\n轮到你选择下一步。", target.DaoName, logLine, state.PlayerHP, effectivePlayer.MaxHealth, state.PlayerMana, effectivePlayer.MaxMana, state.OpponentHP, effectiveTarget.MaxHealth), Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
}

func pvpDamage(attack, defense int64, round int) int64 {
	damage := attack + int64(rand.Intn(int(max64(attack/4, 1)))) - defense/2 + int64(round%3)
	if damage < 1 {
		damage = 1
	}
	return damage
}

func (g *Game) finishPVP(player *model.Player, target model.Player, state pvpBattleState, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	delta := int64(-12)
	result := "失败"
	if won {
		delta = 20
		result = "胜利"
	}
	mine, _ := g.arenaRecord(player.ID)
	theirs, _ := g.arenaRecord(target.ID)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&mine).Update("rating", gorm.Expr("MAX(rating + ?, 0)", delta)).Error; err != nil {
			return err
		}
		if won {
			if err := tx.Model(&mine).Update("wins", gorm.Expr("wins + 1")).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("arena_coins", gorm.Expr("arena_coins + 20")).Error; err != nil {
				return err
			}
			return tx.Model(&theirs).Updates(map[string]any{"rating": gorm.Expr("MAX(rating - 12, 0)"), "losses": gorm.Expr("losses + 1")}).Error
		}
		if err := tx.Model(&mine).Update("losses", gorm.Expr("losses + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("arena_coins", gorm.Expr("arena_coins + 5")).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.setPlayerValueInt(player.ID, "arena.target", 0)
	_ = g.setPlayerValue(player.ID, "arena.battle", "", nil)
	if won {
		_, _ = g.addPlayerValueInt(player.ID, "stats.arena_wins", 1)
	}
	coins := int64(5)
	if won {
		coins = 20
	}
	targetOutcome := "你胜"
	if won {
		targetOutcome = "你败"
	}
	targetRecord, _ := g.arenaRecord(target.ID)
	_ = g.createPlayerNotification(target.ID, "演武", fmt.Sprintf("%s在问剑台匹配到你并完成挑战：%s。你当前竞技积分%d；本次未消耗你的每日场次。", player.DaoName, targetOutcome, targetRecord.Rating))
	return GameResult{Title: "竞技" + result, Content: fmt.Sprintf("对手：%s\n%s\n段位积分：%+d\n竞技币：+%d\n本场回合：%d\n最终气血：%d/%d", target.DaoName, logLine, delta, coins, state.Round, max64(state.PlayerHP, 0), effective.MaxHealth), Actions: []string{"竞技", "竞技档案", "竞技商店", "竞榜", "状态"}}, true, nil
}
