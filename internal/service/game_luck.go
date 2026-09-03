package service

import (
	"fmt"
	"math/rand"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const (
	initialPlayerLuck = int64(10)
	maximumPlayerLuck = int64(50)

	luckEventTriggerBonusCap = 0.12
	luckEventChoiceBonusCap  = 0.15
	luckTreasureBonusCap     = 0.20
	luckPetCaptureBonusCap   = 0.15
	luckAlchemyBonusCap      = 0.08
	luckSynthesisBonusCap    = 0.08
	luckMeetImmortalBonusCap = 0.05
)

func clampInt64(value, minimum, maximum int64) int64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func normalizedPlayerLuck(value int64) int64 {
	return clampInt64(value, 0, maximumPlayerLuck)
}

func luckProbabilityBonus(luck int64, maximumBonus float64) float64 {
	if maximumBonus <= 0 {
		return 0
	}
	progress := float64(normalizedPlayerLuck(luck)-initialPlayerLuck) / float64(maximumPlayerLuck-initialPlayerLuck)
	if progress <= 0 {
		return 0
	}
	if progress > 1 {
		progress = 1
	}
	return maximumBonus * progress
}

func probabilityWithLuck(base float64, luck int64, maximumBonus float64) (actual, bonus float64) {
	if base < 0 {
		base = 0
	}
	if base > 1 {
		base = 1
	}
	actual = base + luckProbabilityBonus(luck, maximumBonus)
	if actual > 1 {
		actual = 1
	}
	bonus = actual - base
	return actual, bonus
}

func luckEffectSummary(luck int64) string {
	luck = normalizedPlayerLuck(luck)
	return fmt.Sprintf(
		"运气：%d/%d\n当前概率加成：奇遇触发+%.1f%% · 奇遇抉择+%.1f%% · 寻宝+%.1f%% · 灵兽捕获+%.1f%%\n炼丹+%.1f%% · 合成+%.1f%% · 遇仙+%.1f%%\n规则：运气是永久属性，不会因寻宝或抽签消耗；达到%d后不再增长。",
		luck, maximumPlayerLuck,
		luckProbabilityBonus(luck, luckEventTriggerBonusCap)*100,
		luckProbabilityBonus(luck, luckEventChoiceBonusCap)*100,
		luckProbabilityBonus(luck, luckTreasureBonusCap)*100,
		luckProbabilityBonus(luck, luckPetCaptureBonusCap)*100,
		luckProbabilityBonus(luck, luckAlchemyBonusCap)*100,
		luckProbabilityBonus(luck, luckSynthesisBonusCap)*100,
		luckProbabilityBonus(luck, luckMeetImmortalBonusCap)*100,
		maximumPlayerLuck,
	)
}

func eventReceivesLuckBonus(eventType string) bool {
	switch eventType {
	case "劫难", "心魔", "惩罚":
		return false
	default:
		return true
	}
}

func isLuckGrowthEncounter(eventType string) bool {
	return eventType == "仙缘" || eventType == "奇遇"
}

// tryGrowLuckFromEncounter performs the permanent growth roll used by genuine
// 仙缘/奇遇 outcomes. The conditional update keeps concurrent rewards below 50.
func (g *Game) tryGrowLuckFromEncounter(player *model.Player) (string, error) {
	return g.tryGrowLuckFromEncounterRoll(player, rand.Float64())
}

func (g *Game) tryGrowLuckFromEncounterRoll(player *model.Player, roll float64) (string, error) {
	var latest model.Player
	if err := g.store.DB.Select("id", "luck", "immortal_affinity").First(&latest, player.ID).Error; err != nil {
		return "", err
	}
	before := normalizedPlayerLuck(latest.Luck)
	if before >= maximumPlayerLuck {
		return fmt.Sprintf("天缘气数：%d/%d（已达上限）", before, maximumPlayerLuck), nil
	}
	baseRate := g.settingFloat("luck.encounter_growth_rate", .10)
	if baseRate < 0 {
		baseRate = 0
	}
	if baseRate > .50 {
		baseRate = .50
	}
	// Existing luck gently improves the chance of condensing the next point,
	// while the hard cap prevents runaway growth.
	rate := baseRate + luckProbabilityBonus(before, .08)
	if rate > .60 {
		rate = .60
	}
	if roll > rate {
		return fmt.Sprintf("天缘气数：本次未凝成永久运气（%.1f%%） · 当前%d/%d", rate*100, before, maximumPlayerLuck), nil
	}
	hit, err := g.players.UpdateColumnWhere(player.ID, "luck", gorm.Expr("luck + 1"), "luck < ?", maximumPlayerLuck)
	if err != nil {
		return "", err
	}
	if !hit {
		return fmt.Sprintf("天缘气数：%d/%d（已达上限）", maximumPlayerLuck, maximumPlayerLuck), nil
	}
	after := min64(before+1, maximumPlayerLuck)
	return fmt.Sprintf("天缘垂青：运气 %d → %d/%d（%.1f%%判定成功）", before, after, maximumPlayerLuck, rate*100), nil
}
