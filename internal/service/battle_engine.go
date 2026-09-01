package service

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"xianlv/internal/model"
)

type combatStats struct {
	Name            string
	Health          int64
	MaxHealth       int64
	Mana            int64
	MaxMana         int64
	PhysicalAttack  int64
	MagicAttack     int64
	PhysicalDefense int64
	MagicDefense    int64
	Agility         int64
	CritRate        float64
	CritDamage      float64
	DodgeRate       float64
	DamageReduction float64
}

type combatRules struct {
	MaxRounds      int
	DefenseFactor  float64
	VarianceMin    float64
	VarianceMax    float64
	DodgeCap       float64
	CritCap        float64
	ReductionCap   float64
	MagicManaCost  int64
	MagicFrequency int
}

type combatOutcome struct {
	PlayerWon       bool
	Draw            bool
	Rounds          int
	PlayerRemaining int64
	EnemyRemaining  int64
	PlayerDamage    int64
	EnemyDamage     int64
	Log             []string
}

func defaultCombatRules() combatRules {
	return combatRules{
		MaxRounds: 20, DefenseFactor: .55, VarianceMin: .9, VarianceMax: 1.1,
		DodgeCap: .45, CritCap: .75, ReductionCap: .8, MagicManaCost: 10, MagicFrequency: 3,
	}
}

func (g *Game) configuredCombatRules() combatRules {
	rules := defaultCombatRules()
	rules.MaxRounds = int(g.settingInt("battle.max_rounds", int64(rules.MaxRounds)))
	rules.DefenseFactor = g.settingFloat("battle.defense_factor", rules.DefenseFactor)
	rules.VarianceMin = g.settingFloat("battle.variance_min", rules.VarianceMin)
	rules.VarianceMax = g.settingFloat("battle.variance_max", rules.VarianceMax)
	rules.DodgeCap = g.settingFloat("battle.dodge_cap", rules.DodgeCap)
	rules.CritCap = g.settingFloat("battle.crit_cap", rules.CritCap)
	rules.ReductionCap = g.settingFloat("battle.reduction_cap", rules.ReductionCap)
	if rules.MaxRounds < 1 {
		rules.MaxRounds = 1
	}
	if rules.MaxRounds > 100 {
		rules.MaxRounds = 100
	}
	if rules.VarianceMax < rules.VarianceMin {
		rules.VarianceMin, rules.VarianceMax = rules.VarianceMax, rules.VarianceMin
	}
	return rules
}

func (g *Game) playerCombatStats(player *model.Player) combatStats {
	adjusted := g.playerWithActiveSkillStats(player)
	stats := combatStats{
		Name: adjusted.DaoName, Health: adjusted.Health, MaxHealth: adjusted.MaxHealth,
		Mana: adjusted.Mana, MaxMana: adjusted.MaxMana, PhysicalAttack: adjusted.PhysicalAttack,
		MagicAttack: adjusted.MagicAttack, PhysicalDefense: adjusted.PhysicalDefense,
		MagicDefense: adjusted.MagicDefense, Agility: adjusted.Agility, CritRate: adjusted.CritRate,
		CritDamage: adjusted.CritDamage, DodgeRate: adjusted.DodgeRate, DamageReduction: adjusted.DamageReduction,
	}
	if stats.Health < 1 {
		stats.Health = 1
	}
	if player.ActivePetID != 0 {
		var pet model.Pet
		if err := g.store.DB.Where("id = ? AND player_id = ? AND active = ?", player.ActivePetID, player.ID, true).First(&pet).Error; err == nil {
			stats.PhysicalAttack += pet.Attack / 3
			stats.MagicAttack += pet.Attack / 3
			stats.PhysicalDefense += pet.Defense / 3
			stats.MagicDefense += pet.Defense / 3
			stats.MaxHealth += pet.Health / 10
			stats.Health += pet.Health / 10
		}
	}
	if _, err := g.playerValue(player.ID, "buff.battle"); err == nil {
		stats.PhysicalAttack = stats.PhysicalAttack * 11 / 10
		stats.MagicAttack = stats.MagicAttack * 11 / 10
	}
	if raw, err := g.playerValue(player.ID, "extended.battle_buff"); err == nil {
		var buff extendedBattleBuff
		if json.Unmarshal([]byte(raw), &buff) == nil {
			stats.PhysicalAttack += stats.PhysicalAttack * buff.AttackPercent / 100
			stats.MagicAttack += stats.MagicAttack * buff.AttackPercent / 100
			stats.PhysicalDefense += stats.PhysicalDefense * buff.DefensePercent / 100
			stats.MagicDefense += stats.MagicDefense * buff.DefensePercent / 100
			stats.Agility += stats.Agility * buff.SpeedPercent / 100
		}
	}
	medicineBonus := g.activeItemBonuses(player.ID)
	if medicineBonus.AgilityRate > 0 {
		stats.Agility += int64(math.Round(float64(stats.Agility) * medicineBonus.AgilityRate))
	}
	if medicineBonus.DefenseRate > 0 {
		stats.PhysicalDefense += int64(math.Round(float64(stats.PhysicalDefense) * medicineBonus.DefenseRate))
		stats.MagicDefense += int64(math.Round(float64(stats.MagicDefense) * medicineBonus.DefenseRate))
	}
	return normalizeCombatStats(stats)
}

func scaledEnemy(name string, player combatStats, ratio float64) combatStats {
	if ratio < .2 {
		ratio = .2
	}
	enemy := combatStats{
		Name:            name,
		MaxHealth:       max64(int64(float64(player.MaxHealth)*ratio), 30),
		MaxMana:         max64(int64(float64(player.MaxMana)*ratio), 10),
		PhysicalAttack:  max64(int64(float64(player.PhysicalAttack)*ratio), 3),
		MagicAttack:     max64(int64(float64(player.MagicAttack)*ratio), 3),
		PhysicalDefense: max64(int64(float64(player.PhysicalDefense)*ratio), 1),
		MagicDefense:    max64(int64(float64(player.MagicDefense)*ratio), 1),
		Agility:         max64(int64(float64(player.Agility)*ratio), 1),
		CritRate:        clampFloat(player.CritRate*ratio, .02, .35),
		CritDamage:      clampFloat(player.CritDamage, 1.2, 2.5),
		DodgeRate:       clampFloat(player.DodgeRate*ratio, 0, .3),
		DamageReduction: clampFloat(player.DamageReduction*ratio, 0, .5),
	}
	enemy.Health = enemy.MaxHealth
	enemy.Mana = enemy.MaxMana
	return normalizeCombatStats(enemy)
}

func normalizeCombatStats(stats combatStats) combatStats {
	stats.MaxHealth = max64(stats.MaxHealth, 1)
	stats.Health = min64(max64(stats.Health, 1), stats.MaxHealth)
	stats.MaxMana = max64(stats.MaxMana, 0)
	stats.Mana = min64(max64(stats.Mana, 0), stats.MaxMana)
	stats.PhysicalAttack = max64(stats.PhysicalAttack, 1)
	stats.MagicAttack = max64(stats.MagicAttack, 1)
	stats.PhysicalDefense = max64(stats.PhysicalDefense, 0)
	stats.MagicDefense = max64(stats.MagicDefense, 0)
	stats.Agility = max64(stats.Agility, 1)
	stats.CritRate = clampFloat(stats.CritRate, 0, 1)
	stats.CritDamage = clampFloat(stats.CritDamage, 1.2, 3)
	stats.DodgeRate = clampFloat(stats.DodgeRate, 0, 1)
	stats.DamageReduction = clampFloat(stats.DamageReduction, 0, 1)
	return stats
}

func resolveCombat(player, enemy combatStats, rules combatRules, rng *rand.Rand) combatOutcome {
	player = normalizeCombatStats(player)
	enemy = normalizeCombatStats(enemy)
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	outcome := combatOutcome{}
	playerFirst := player.Agility >= enemy.Agility
	for round := 1; round <= rules.MaxRounds && player.Health > 0 && enemy.Health > 0; round++ {
		outcome.Rounds = round
		if playerFirst {
			outcome.PlayerDamage += performAttack(&player, &enemy, round, rules, rng, &outcome.Log)
			if enemy.Health > 0 {
				outcome.EnemyDamage += performAttack(&enemy, &player, round, rules, rng, &outcome.Log)
			}
		} else {
			outcome.EnemyDamage += performAttack(&enemy, &player, round, rules, rng, &outcome.Log)
			if player.Health > 0 {
				outcome.PlayerDamage += performAttack(&player, &enemy, round, rules, rng, &outcome.Log)
			}
		}
	}
	outcome.PlayerRemaining = max64(player.Health, 0)
	outcome.EnemyRemaining = max64(enemy.Health, 0)
	switch {
	case enemy.Health <= 0 && player.Health > 0:
		outcome.PlayerWon = true
	case player.Health <= 0 && enemy.Health > 0:
		outcome.PlayerWon = false
	default:
		playerRatio := float64(max64(player.Health, 0)) / float64(player.MaxHealth)
		enemyRatio := float64(max64(enemy.Health, 0)) / float64(enemy.MaxHealth)
		if math.Abs(playerRatio-enemyRatio) < .05 {
			outcome.Draw = true
		} else {
			outcome.PlayerWon = playerRatio > enemyRatio
		}
	}
	return outcome
}

func performAttack(attacker, defender *combatStats, round int, rules combatRules, rng *rand.Rand, logLines *[]string) int64 {
	magic := rules.MagicFrequency > 0 && round%rules.MagicFrequency == 0 && attacker.Mana >= rules.MagicManaCost
	attack := attacker.PhysicalAttack
	defense := defender.PhysicalDefense
	move := "剑诀"
	if magic {
		attacker.Mana -= rules.MagicManaCost
		attack = attacker.MagicAttack
		defense = defender.MagicDefense
		move = "术法"
	}
	dodgeChance := defender.DodgeRate + float64(defender.Agility-attacker.Agility)*.001
	dodgeChance = clampFloat(dodgeChance, 0, rules.DodgeCap)
	if rng.Float64() < dodgeChance {
		appendCombatLog(logLines, fmt.Sprintf("第%d回合 %s闪避%s的%s", round, defender.Name, attacker.Name, move))
		return 0
	}
	variance := rules.VarianceMin + rng.Float64()*(rules.VarianceMax-rules.VarianceMin)
	damage := float64(attack)*variance - float64(defense)*rules.DefenseFactor
	minimum := math.Max(1, float64(attack)*.08)
	if damage < minimum {
		damage = minimum
	}
	critical := rng.Float64() < clampFloat(attacker.CritRate, 0, rules.CritCap)
	if critical {
		damage *= clampFloat(attacker.CritDamage, 1.2, 3)
	}
	damage *= 1 - clampFloat(defender.DamageReduction, 0, rules.ReductionCap)
	value := max64(int64(math.Round(damage)), 1)
	defender.Health -= value
	marker := ""
	if critical {
		marker = "（暴击）"
	}
	appendCombatLog(logLines, fmt.Sprintf("第%d回合 %s以%s造成%d伤害%s", round, attacker.Name, move, value, marker))
	return value
}

func appendCombatLog(lines *[]string, line string) {
	if len(*lines) < 8 {
		*lines = append(*lines, line)
	}
}

func formatCombatOutcome(outcome combatOutcome) string {
	result := "落败"
	if outcome.Draw {
		result = "战平"
	} else if outcome.PlayerWon {
		result = "胜利"
	}
	lines := []string{
		fmt.Sprintf("结果：%s · %d回合", result, outcome.Rounds),
		fmt.Sprintf("造成伤害：%d · 承受伤害：%d", outcome.PlayerDamage, outcome.EnemyDamage),
		fmt.Sprintf("剩余气血：%d · 敌方气血：%d", outcome.PlayerRemaining, outcome.EnemyRemaining),
	}
	if len(outcome.Log) > 0 {
		lines = append(lines, "━━━━━━━━━━━")
		lines = append(lines, outcome.Log...)
	}
	return strings.Join(lines, "\n")
}

func clampFloat(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
