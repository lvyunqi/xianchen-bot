package service

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func (g *Game) finishExtendedRuntimeBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	if state.ExtendedCategory == "天地灵脉" {
		return g.finishWorldLeylineExtendedBattle(player, state, won, logLine)
	}
	if state.ExtendedCategory == "宗门战争" {
		return g.finishSectWarExtendedBattle(player, state, won, logLine)
	}
	if state.ExtendedCategory == "仙魔战场" {
		return g.finishBattlefieldExtendedBattle(player, state, won, logLine)
	}
	remainingHP := max64(state.PlayerHP, 1)
	remainingMana := max64(state.PlayerMana, 0)
	if !won {
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("player_id = ? AND key IN ?", player.ID, []string{"pve.battle", "extended.pending.system"}).Delete(&model.PlayerValue{}).Error; err != nil {
				return err
			}
			return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": remainingMana, "state": model.PlayerStateIdle}).Error
		}); err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: state.ExtendedCategory + "试炼失败", Content: fmt.Sprintf("道藏：%s\n%s\n━━━━━━━━━━━\n你在第%d回合失去战斗能力。\n剩余气血：%d/%d · 法力：%d/%d\n本次没有获得熟练度、属性、占领权或奖励；开战消耗不返还。", state.ExtendedConfigName, logLine, state.Round, remainingHP, effective.MaxHealth, remainingMana, effective.MaxMana), Actions: []string{"疗伤", "状态", extendedPrimaryListCommand(state.ExtendedCategory), extendedMenuAction(state.ExtendedCategory)}}, true, nil
	}
	system, ok := extendedSystems[state.ExtendedCategory]
	if !ok {
		return GameResult{}, true, errors.New("道藏战斗分类不存在")
	}
	config, err := g.extendedConfig(system.Table, state.ExtendedConfigCode)
	if err != nil {
		return GameResult{}, true, err
	}
	effect := decodeExtendedEffect(config)
	action := displayOr(state.ExtendedAction, "battle")
	var progress model.PlayerExtendedProgress
	progressErr := g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, state.ExtendedCategory, config.Code).First(&progress).Error
	if progressErr != nil && !errors.Is(progressErr, gorm.ErrRecordNotFound) {
		return GameResult{}, true, progressErr
	}
	beforeLevel, beforeMastery := progress.Level, progress.Mastery
	if progress.ID == 0 {
		progress = model.PlayerExtendedProgress{PlayerID: player.ID, System: state.ExtendedCategory, ConfigCode: config.Code, ConfigName: config.Name, Level: 1, Power: effect.Power, MetadataJSON: `{}`}
	}
	progress.ConfigName = config.Name
	progress.State = "试炼胜利"
	progress.Uses++
	progress.Experience += int64(maxInt(config.Level, 1) * 20)
	progress.Mastery += int64(maxInt(config.Level, 1) * 2)
	if progress.Mastery >= int64(maxInt(progress.Level, 1)*100) {
		progress.Level++
		progress.Power += max64(int64(math.Round(float64(effect.Power)*effect.Growth/5)), 1)
	}
	updates, effectLines := extendedPermanentEffects(state.ExtendedCategory, action, effect, progress.Level)
	cultivation := max64(effect.Power*2+state.EnemyPower/3, 20)
	updates["cultivation"] = gorm.Expr("cultivation + ?", cultivation)
	rewardLines := []string{fmt.Sprintf("修为+%d", cultivation)}
	switch state.ExtendedCategory {
	case "阵法":
		updates["reputation"] = gorm.Expr("reputation + ?", max64(effect.Power/12, 1))
		rewardLines = append(rewardLines, fmt.Sprintf("声望+%d", max64(effect.Power/12, 1)))
	case "傀儡":
		progress.Power += max64(effect.Power/10, 1)
		rewardLines = append(rewardLines, fmt.Sprintf("傀儡威力+%d", max64(effect.Power/10, 1)))
	case "秘境争夺":
		stones, merit := max64(effect.Power, 10), max64(effect.Power/20, 1)
		updates["spirit_stones"] = gorm.Expr("spirit_stones + ?", stones)
		updates["merit"] = gorm.Expr("merit + ?", merit)
		rewardLines = append(rewardLines, fmt.Sprintf("灵石+%d · 功德+%d", stones, merit))
	case "渡劫心魔":
		willGain := max64(effect.Power/15, 1)
		updates["willpower"] = gorm.Expr("willpower + ?", willGain)
		updates["dao_heart"] = gorm.Expr("MIN(dao_heart + ?, 100)", max64(willGain/2, 1))
		rewardLines = append(rewardLines, fmt.Sprintf("意志+%d · 道心+%d", willGain, max64(willGain/2, 1)))
	}
	key := fmt.Sprintf("extended.%s.%s.%s", system.Table, config.Code, action)
	count := g.playerValueInt(player.ID, key, 0) + 1
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key IN ?", player.ID, []string{"pve.battle", "extended.pending.system"}).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		updates["health"] = remainingHP
		updates["mana"] = remainingMana
		updates["state"] = model.PlayerStateIdle
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, key, fmt.Sprintf("%d", count), nil); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, "extended.active."+system.Table, config.Code, nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if len(effectLines) > 0 {
		rewardLines = append(rewardLines, effectLines...)
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	content := fmt.Sprintf("道藏：%s\n%s\n━━━━━━━━━━━\n第%d回合胜出\n等级：%d → %d\n熟练：%d → %d\n威力：%d\n所得：%s\n剩余气血：%d/%d · 法力：%d/%d", config.Name, logLine, state.Round, beforeLevel, progress.Level, beforeMastery, progress.Mastery, progress.Power, strings.Join(rewardLines, " · "), remainingHP, effective.MaxHealth, remainingMana, effective.MaxMana)
	return GameResult{Title: state.ExtendedCategory + "试炼胜利", Content: content, ImageURL: config.ImageURL, Actions: g.extendedProgressActions(state.ExtendedCategory, config.Name)}, true, nil
}
