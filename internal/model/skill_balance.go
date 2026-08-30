package model

import (
	"encoding/json"
	"math"
)

func CreatedSkillEffectLimits(style string) map[string]float64 {
	limits := map[string]float64{
		"attack": 320, "physical_attack": 480, "magic_attack": 480,
		"defense": 320, "health": 3200, "mana": 1600, "speed": 480,
		"crit_rate": 0.05, "dodge_rate": 0.05, "damage_reduction": 0.05,
	}
	switch style {
	case "剑道":
		limits["physical_attack"], limits["speed"] = 480, 80
	case "术法":
		limits["magic_attack"], limits["mana"] = 480, 1280
	case "炼体":
		limits["defense"], limits["health"] = 320, 3200
	case "神魂":
		limits["attack"], limits["defense"], limits["mana"] = 320, 80, 1600
	case "遁法":
		limits["physical_attack"], limits["speed"] = 160, 480
	case "均衡":
		limits["attack"], limits["defense"], limits["health"], limits["mana"] = 320, 160, 1280, 800
	}
	return limits
}

func ClampCreatedSkillEffectJSON(style, raw string) (string, bool) {
	values := map[string]float64{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return raw, false
	}
	changed := false
	for key, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			values[key] = 0
			changed = true
			continue
		}
		if limit, exists := CreatedSkillEffectLimits(style)[key]; exists && value > limit {
			values[key] = limit
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return raw, false
	}
	return string(encoded), true
}
