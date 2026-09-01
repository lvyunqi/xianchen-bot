package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"xianlv/internal/model"
)

type skillStatBonus struct {
	Name            string  `json:"-"`
	Level           int     `json:"-"`
	Attack          int64   `json:"attack,omitempty"`
	PhysicalAttack  int64   `json:"physical_attack,omitempty"`
	MagicAttack     int64   `json:"magic_attack,omitempty"`
	Defense         int64   `json:"defense,omitempty"`
	Health          int64   `json:"health,omitempty"`
	Mana            int64   `json:"mana,omitempty"`
	Speed           int64   `json:"speed,omitempty"`
	CritRate        float64 `json:"crit_rate,omitempty"`
	DodgeRate       float64 `json:"dodge_rate,omitempty"`
	DamageReduction float64 `json:"damage_reduction,omitempty"`
}

func decodeSkillStatBonus(skill model.Skill, level int) skillStatBonus {
	bonus := skillStatBonus{Name: skill.Name, Level: maxInt(level, 1)}
	_ = json.Unmarshal([]byte(skill.EffectJSON), &bonus)
	multiplier := int64(100 + maxInt(level-1, 0)*20)
	bonus.Attack = bonus.Attack * multiplier / 100
	bonus.PhysicalAttack = bonus.PhysicalAttack * multiplier / 100
	bonus.MagicAttack = bonus.MagicAttack * multiplier / 100
	bonus.Defense = bonus.Defense * multiplier / 100
	bonus.Health = bonus.Health * multiplier / 100
	bonus.Mana = bonus.Mana * multiplier / 100
	bonus.Speed = bonus.Speed * multiplier / 100
	bonus.CritRate = bonus.CritRate * float64(multiplier) / 100
	bonus.DodgeRate = bonus.DodgeRate * float64(multiplier) / 100
	bonus.DamageReduction = bonus.DamageReduction * float64(multiplier) / 100
	return bonus
}

func (g *Game) activeSkillStatBonus(player *model.Player) skillStatBonus {
	if player == nil || player.ID == 0 || player.CurrentSkillID == 0 {
		return skillStatBonus{}
	}
	var learned model.PlayerSkill
	if err := g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, player.CurrentSkillID).First(&learned).Error; err != nil {
		return skillStatBonus{}
	}
	var skill model.Skill
	if err := g.store.DB.First(&skill, learned.SkillID).Error; err != nil {
		return skillStatBonus{}
	}
	return decodeSkillStatBonus(skill, learned.Level)
}

func applySkillBonusToPlayer(player *model.Player, bonus skillStatBonus) {
	if player == nil {
		return
	}
	player.PhysicalAttack += bonus.Attack + bonus.PhysicalAttack
	player.MagicAttack += bonus.Attack + bonus.MagicAttack
	player.PhysicalDefense += bonus.Defense
	player.MagicDefense += bonus.Defense
	player.MaxHealth += bonus.Health
	player.MaxMana += bonus.Mana
	player.Agility += bonus.Speed
	player.CritRate += bonus.CritRate
	player.DodgeRate += bonus.DodgeRate
	player.DamageReduction += bonus.DamageReduction
}

func (g *Game) playerWithActiveSkillStats(player *model.Player) model.Player {
	if player == nil {
		return model.Player{}
	}
	adjusted := *player
	applySkillBonusToPlayer(&adjusted, g.activeSkillStatBonus(player))
	adjusted.Health = min64(max64(player.Health, 0), max64(adjusted.MaxHealth, 1))
	adjusted.Mana = min64(max64(player.Mana, 0), max64(adjusted.MaxMana, 0))
	return adjusted
}

func (g *Game) activeSkillCombatPower(player *model.Player) int64 {
	bonus := g.activeSkillStatBonus(player)
	return skillCombatPowerContribution(player, bonus)
}

func skillCombatPowerContribution(player *model.Player, bonus skillStatBonus) int64 {
	if player == nil || bonus.Name == "" {
		return 0
	}
	preview := *player
	applySkillBonusToPlayer(&preview, bonus)
	return max64(calculateCombatPower(preview)-calculateCombatPower(*player), 0)
}

func skillBonusText(bonus skillStatBonus) string {
	parts := make([]string, 0, 5)
	if bonus.Attack != 0 {
		parts = append(parts, fmt.Sprintf("攻法+%d", bonus.Attack))
	}
	if bonus.PhysicalAttack != 0 {
		parts = append(parts, fmt.Sprintf("物攻+%d", bonus.PhysicalAttack))
	}
	if bonus.MagicAttack != 0 {
		parts = append(parts, fmt.Sprintf("法强+%d", bonus.MagicAttack))
	}
	if bonus.Defense != 0 {
		parts = append(parts, fmt.Sprintf("双防+%d", bonus.Defense))
	}
	if bonus.Health != 0 {
		parts = append(parts, fmt.Sprintf("气血+%d", bonus.Health))
	}
	if bonus.Mana != 0 {
		parts = append(parts, fmt.Sprintf("法力+%d", bonus.Mana))
	}
	if bonus.Speed != 0 {
		parts = append(parts, fmt.Sprintf("身法+%d", bonus.Speed))
	}
	if bonus.CritRate != 0 {
		parts = append(parts, fmt.Sprintf("暴击+%.1f%%", bonus.CritRate*100))
	}
	if bonus.DodgeRate != 0 {
		parts = append(parts, fmt.Sprintf("闪避+%.1f%%", bonus.DodgeRate*100))
	}
	if bonus.DamageReduction != 0 {
		parts = append(parts, fmt.Sprintf("减伤+%.1f%%", bonus.DamageReduction*100))
	}
	if len(parts) == 0 {
		return "无战斗属性"
	}
	return strings.Join(parts, " · ")
}

func skillOffensiveBonus(bonus skillStatBonus) int64 {
	return max64(bonus.Attack+bonus.PhysicalAttack, bonus.Attack+bonus.MagicAttack)
}
