package util

import (
	"fmt"
	"strings"
	"xianlv/internal/model"
)

func PlayerPanel(p model.Player) string {
	return fmt.Sprintf("# %s\n> %s · %s\n\n等级：LV%d · 经验%d/%d\n修为：%d / %d\n战力：%d\n气血：%d/%d\n法力：%d/%d\n灵根：%s\n寿元：%d/%d\n位置：%s", p.DaoName, p.RealmName, p.State, p.Level, p.Experience, model.PlayerExperienceRequired(p.Level), p.Cultivation, p.CultivationRequired, p.CombatPower, p.Health, p.MaxHealth, p.Mana, p.MaxMana, p.SpiritualRoot, p.Lifespan, p.MaxLifespan, p.Location)
}
func ListPanel(title string, values []string) string {
	return "# " + title + "\n\n" + strings.Join(values, "\n")
}
