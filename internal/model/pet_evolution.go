package model

import (
	"encoding/json"
	"math"
)

type PetEvolutionRequirement struct {
	Loyalty int `json:"loyalty"`
	Level   int `json:"level"`
}

func PetEvolutionRequirementFor(template PetTemplate) PetEvolutionRequirement {
	requirement := PetEvolutionRequirement{Loyalty: 80, Level: 5}
	_ = json.Unmarshal([]byte(template.EvolutionCondition), &requirement)
	if requirement.Loyalty < 1 {
		requirement.Loyalty = 80
	}
	if requirement.Level < 1 {
		requirement.Level = 5
	}
	return requirement
}

func PetStatsAtLevel(template PetTemplate, level int, evolved bool) (attack, defense, health int64) {
	initial := template.InitialPower
	if initial < 1 {
		initial = 1
	}
	growth := template.GrowthPerLevel
	if growth < 1 {
		growth = 1
	}
	levels := int64(level - 1)
	if levels < 0 {
		levels = 0
	}
	attack = saturatingAdd(initial, saturatingMultiply(growth, levels))
	defense = saturatingAdd(maxModelInt64(initial/2, 1), saturatingMultiply(maxModelInt64(growth/2, 1), levels))
	health = saturatingAdd(saturatingMultiply(initial, 10), saturatingMultiply(saturatingMultiply(growth, 10), levels))
	if evolved {
		attack = scalePetEvolutionStat(attack)
		defense = scalePetEvolutionStat(defense)
		health = scalePetEvolutionStat(health)
	}
	return attack, defense, health
}

func scalePetEvolutionStat(value int64) int64 {
	if value <= 0 {
		return 1
	}
	if value > math.MaxInt64/3 {
		return math.MaxInt64
	}
	return value * 3 / 2
}

func saturatingMultiply(left, right int64) int64 {
	if left <= 0 || right <= 0 {
		return 0
	}
	if left > math.MaxInt64/right {
		return math.MaxInt64
	}
	return left * right
}

func saturatingAdd(left, right int64) int64 {
	if right > 0 && left > math.MaxInt64-right {
		return math.MaxInt64
	}
	return left + right
}

func maxModelInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
