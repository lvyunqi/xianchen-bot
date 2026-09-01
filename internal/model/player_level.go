package model

import "math"

const (
	PlayerLevelBaseHealth  int64 = 100
	PlayerLevelBaseMana    int64 = 50
	PlayerLevelBaseAttack  int64 = 10
	PlayerLevelBaseDefense int64 = 5
	PlayerLevelBaseAgility int64 = 10
	PlayerHealthPerLevel   int64 = 24
	PlayerAttackPerLevel   int64 = 7
	PlayerDefensePerLevel  int64 = 5
)

// PlayerLevelBaseStats is the permanent minimum earned from character levels.
// Equipment, titles, roots, realms and inheritances are added independently.
type PlayerLevelBaseStats struct {
	MaxHealth       int64
	MaxMana         int64
	PhysicalAttack  int64
	MagicAttack     int64
	PhysicalDefense int64
	MagicDefense    int64
	Agility         int64
	Strength        int64
	Constitution    int64
	Spirit          int64
	Perception      int64
	Willpower       int64
}

func PlayerLevelStats(level int) PlayerLevelBaseStats {
	if level < 1 {
		level = 1
	}
	if int64(level) > math.MaxInt32 {
		level = math.MaxInt32
	}
	steps := int64(level - 1)
	mul := func(base, growth int64) int64 {
		if growth > 0 && steps > (math.MaxInt64-base)/growth {
			return math.MaxInt64
		}
		return base + steps*growth
	}
	// Mana keeps the established milestone curve; the explicitly published
	// combat growth is +24 HP, +7 to both attacks and +5 to both defenses.
	q, r := int64(level/20), int64(level%20)
	stepMana := steps * 4
	milestoneMana := q * (10*(q-1) + r + 1)
	mana := PlayerLevelBaseMana + stepMana + milestoneMana
	return PlayerLevelBaseStats{
		MaxHealth:       mul(PlayerLevelBaseHealth, PlayerHealthPerLevel),
		MaxMana:         mana,
		PhysicalAttack:  mul(PlayerLevelBaseAttack, PlayerAttackPerLevel),
		MagicAttack:     mul(PlayerLevelBaseAttack, PlayerAttackPerLevel),
		PhysicalDefense: mul(PlayerLevelBaseDefense, PlayerDefensePerLevel),
		MagicDefense:    mul(PlayerLevelBaseDefense, PlayerDefensePerLevel),
		Agility:         PlayerLevelBaseAgility + int64(level/2),
		Strength:        10 + int64(level/5),
		Constitution:    10 + int64(level/5),
		Spirit:          10 + int64(level/5),
		Perception:      10 + int64(level/10),
		Willpower:       10 + int64(level/10),
	}
}

type PlayerLevelProgress struct {
	ExperienceGain   int64
	BeforeLevel      int
	AfterLevel       int
	BeforeExp        int64
	AfterExp         int64
	NextRequired     int64
	HealthGain       int64
	ManaGain         int64
	AttackGain       int64
	DefenseGain      int64
	AgilityGain      int64
	StrengthGain     int64
	ConstitutionGain int64
	SpiritGain       int64
	PerceptionGain   int64
	WillpowerGain    int64
}

func PlayerExperienceRequired(level int) int64 {
	if level < 1 {
		level = 1
	}
	value := int64(level)
	if value > 303700049 {
		return math.MaxInt64
	}
	squared := value * value
	if squared > math.MaxInt64/100 {
		return math.MaxInt64
	}
	return squared * 100
}

func ApplyPlayerExperience(player *Player, gain int64) PlayerLevelProgress {
	if player.Level < 1 {
		player.Level = 1
	}
	if player.Experience < 0 {
		player.Experience = 0
	}
	if gain < 0 {
		gain = 0
	}
	progress := PlayerLevelProgress{
		ExperienceGain: gain,
		BeforeLevel:    player.Level,
		AfterLevel:     player.Level,
		BeforeExp:      player.Experience,
	}
	if gain > math.MaxInt64-player.Experience {
		player.Experience = math.MaxInt64
	} else {
		player.Experience += gain
	}
	for player.Experience >= PlayerExperienceRequired(player.Level) && player.Level < math.MaxInt32 {
		required := PlayerExperienceRequired(player.Level)
		if required == math.MaxInt64 {
			break
		}
		player.Experience -= required
		player.Level++
		newLevel := player.Level
		health := PlayerHealthPerLevel
		mana := int64(4 + newLevel/20)
		attack := PlayerAttackPerLevel
		defense := PlayerDefensePerLevel
		progress.HealthGain += health
		progress.ManaGain += mana
		progress.AttackGain += attack
		progress.DefenseGain += defense
		player.MaxHealth += health
		player.Health += health
		player.MaxMana += mana
		player.Mana += mana
		player.PhysicalAttack += attack
		player.MagicAttack += attack
		player.PhysicalDefense += defense
		player.MagicDefense += defense
		if newLevel%2 == 0 {
			progress.AgilityGain++
			player.Agility++
		}
		if newLevel%5 == 0 {
			progress.StrengthGain++
			progress.ConstitutionGain++
			progress.SpiritGain++
			player.Strength++
			player.Constitution++
			player.Spirit++
		}
		if newLevel%10 == 0 {
			progress.PerceptionGain++
			progress.WillpowerGain++
			player.Perception++
			player.Willpower++
		}
	}
	progress.AfterLevel = player.Level
	progress.AfterExp = player.Experience
	progress.NextRequired = PlayerExperienceRequired(player.Level)
	return progress
}
