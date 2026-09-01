package service

import (
	"encoding/json"
	"math"
	"strings"

	"xianlv/internal/model"
)

const maximumTitleGameplayPercent = 100

func (g *Game) activeTitleGameplayPercent(player *model.Player, keys ...string) float64 {
	if player == nil || player.ID == 0 || strings.TrimSpace(player.Title) == "" {
		return 0
	}
	var title model.Title
	if err := g.store.DB.Where("name = ? AND enabled = ?", player.Title, true).First(&title).Error; err != nil {
		return 0
	}
	values := map[string]float64{}
	if json.Unmarshal([]byte(title.AttributeBonus), &values) != nil {
		return 0
	}
	bonus := 0.0
	for _, key := range keys {
		value := values[key]
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			continue
		}
		if value > bonus {
			bonus = value
		}
	}
	return math.Min(bonus, maximumTitleGameplayPercent)
}

func (g *Game) activeTitleProbabilityBonus(player *model.Player, keys ...string) float64 {
	return g.activeTitleGameplayPercent(player, keys...) / 100
}
