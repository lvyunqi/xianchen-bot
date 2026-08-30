package service

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const (
	farmWeatherAppliedKey = "farm.weather.applied"
	farmDisasterLogKey    = "farm.disaster.log"
)

type farmWeather struct {
	Name          string
	Description   string
	GrowthMinutes int
	YieldDelta    int64
	DisasterRisk  int
}

var farmWeatherCatalog = []farmWeather{
	{Name: "灵雨润壤", Description: "温和灵雨渗入田垄，灵植生长加快并略有增产。", GrowthMinutes: 20, YieldDelta: 1},
	{Name: "星辉甘露", Description: "星辉凝成甘露，成熟时间提前并滋养药性。", GrowthMinutes: 30, YieldDelta: 1},
	{Name: "和风晴日", Description: "天象平稳，灵植按原有速度生长。"},
	{Name: "烈阳灼脉", Description: "烈阳蒸腾灵壤，防护不足的田垄可能迟滞生长。", GrowthMinutes: -20, DisasterRisk: 30},
	{Name: "噬灵虫潮", Description: "虫潮沿地脉突入，除虫、灵肥抗灾与护田灵兽可以抵消损失。", GrowthMinutes: -15, YieldDelta: -1, DisasterRisk: 45},
	{Name: "地脉震荡", Description: "洞天地脉短暂震荡，未受护持的灵植可能减产并延迟成熟。", GrowthMinutes: -30, YieldDelta: -1, DisasterRisk: 55},
}

func deterministicFarmRoll(parts ...string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(strings.Join(parts, "|")))
	return hasher.Sum64()
}

func dailyFarmWeather(playerID uint, day string) farmWeather {
	index := deterministicFarmRoll(strconv.FormatUint(uint64(playerID), 10), day, "weather") % uint64(len(farmWeatherCatalog))
	return farmWeatherCatalog[index]
}

func cropWeatherResistance(crop model.MansionCrop) int {
	resistance := crop.DisasterResistance
	if crop.Protected {
		resistance += 30
	}
	if crop.Watered {
		resistance += 10
	}
	if crop.Weeded {
		resistance += 10
	}
	if crop.PestFree {
		resistance += 15
	}
	return minInt(resistance, 95)
}

func (g *Game) applyDailyFarmWeather(player *model.Player, mansion model.Mansion) (farmWeather, []string, error) {
	now := time.Now()
	day := now.Format("2006-01-02")
	weather := dailyFarmWeather(player.ID, day)
	if value, err := g.playerValue(player.ID, farmWeatherAppliedKey); err == nil && value == day {
		return weather, nil, nil
	}
	var crops []model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND harvested = ? AND ready_at > ?", mansion.ID, false, now).Order("plot").Find(&crops).Error; err != nil {
		return weather, nil, err
	}
	events := make([]string, 0, len(crops))
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var marker model.PlayerValue
		if err := tx.Where("player_id = ? AND key = ?", player.ID, farmWeatherAppliedKey).First(&marker).Error; err == nil && marker.Value == day {
			return nil
		}
		for _, crop := range crops {
			affected := weather.DisasterRisk == 0
			if weather.DisasterRisk > 0 {
				resistance := cropWeatherResistance(crop)
				effectiveRisk := weather.DisasterRisk * (100 - resistance) / 100
				roll := int(deterministicFarmRoll(day, strconv.FormatUint(uint64(player.ID), 10), strconv.Itoa(crop.Plot), weather.Name) % 100)
				affected = roll < effectiveRisk
			}
			if !affected || weather.GrowthMinutes == 0 && weather.YieldDelta == 0 {
				continue
			}
			readyAt := crop.ReadyAt
			if weather.GrowthMinutes > 0 {
				readyAt = readyAt.Add(-time.Duration(weather.GrowthMinutes) * time.Minute)
				// Viewing the field must not instantly mature a newly planted
				// crop and remove every chance to water, weed or fertilize it.
				minimumCareWindow := now.Add(5 * time.Minute)
				if !crop.ReadyAt.After(minimumCareWindow) {
					readyAt = crop.ReadyAt
				} else if readyAt.Before(minimumCareWindow) {
					readyAt = minimumCareWindow
				}
			} else if weather.GrowthMinutes < 0 {
				readyAt = readyAt.Add(time.Duration(-weather.GrowthMinutes) * time.Minute)
			}
			quantity := max64(crop.Quantity+weather.YieldDelta, 1)
			if err := tx.Model(&model.MansionCrop{}).Where("id = ? AND harvested = ?", crop.ID, false).Updates(map[string]any{"ready_at": readyAt, "quantity": quantity}).Error; err != nil {
				return err
			}
			change := []string{}
			if weather.GrowthMinutes > 0 {
				actualMinutes := maxInt(int(crop.ReadyAt.Sub(readyAt)/time.Minute), 0)
				change = append(change, fmt.Sprintf("成熟提前%d分钟", actualMinutes))
			} else if weather.GrowthMinutes < 0 {
				change = append(change, fmt.Sprintf("成熟延后%d分钟", -weather.GrowthMinutes))
			}
			if weather.YieldDelta > 0 {
				change = append(change, fmt.Sprintf("预计增产+%d", weather.YieldDelta))
			} else if weather.YieldDelta < 0 {
				change = append(change, fmt.Sprintf("预计减产%d", weather.YieldDelta))
			}
			events = append(events, fmt.Sprintf("地块%d：%s", crop.Plot, strings.Join(change, "、")))
		}
		marker = model.PlayerValue{PlayerID: player.ID, Key: farmWeatherAppliedKey, Value: day}
		if err := tx.Where("player_id = ? AND key = ?", player.ID, farmWeatherAppliedKey).Assign(map[string]any{"value": day, "expires_at": nil}).FirstOrCreate(&marker).Error; err != nil {
			return err
		}
		if len(events) > 0 {
			logLine := fmt.Sprintf("%s【%s】%s", now.Format("01-02 15:04"), weather.Name, strings.Join(events, "；"))
			var logValue model.PlayerValue
			old := ""
			if err := tx.Where("player_id = ? AND key = ?", player.ID, farmDisasterLogKey).First(&logValue).Error; err == nil {
				old = logValue.Value
			}
			lines := append([]string{logLine}, strings.Split(strings.TrimSpace(old), "\n")...)
			if len(lines) > 30 {
				lines = lines[:30]
			}
			logValue = model.PlayerValue{PlayerID: player.ID, Key: farmDisasterLogKey, Value: strings.TrimSpace(strings.Join(lines, "\n"))}
			if err := tx.Where("player_id = ? AND key = ?", player.ID, farmDisasterLogKey).Assign(map[string]any{"value": logValue.Value, "expires_at": nil}).FirstOrCreate(&logValue).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return weather, events, err
}

func (g *Game) farmWeatherOverview(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	weather, events, err := g.applyDailyFarmWeather(player, mansion)
	if err != nil {
		return GameResult{}, true, err
	}
	result := "今日没有田垄受到额外影响。"
	if len(events) > 0 {
		result = strings.Join(events, "\n")
	}
	return GameResult{Title: "灵田天象", Content: fmt.Sprintf("今日：%s\n%s\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n灾异只会每日结算一次。浇水、除草、除虫、灵肥抗灾、护田灵兽和主动护持都会降低灾害风险。", weather.Name, weather.Description, result), Actions: []string{"我的灵田", "护持灵田 全部", "灵田灾异录", "一键施肥 灵壤肥"}}, true, nil
}

func (g *Game) protectFarmCrops(player *model.Player, raw string) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GameResult{Title: "护持灵田", Content: "请输入：`护持灵田 地块号` 或 `护持灵田 全部`。\n每块消耗2点体力，永久为本轮作物增加15点抗灾并启用守护；收获后新一轮需要重新布置。", Actions: []string{"护持灵田 全部", "我的灵田", "灵田天象"}}, true, nil
	}
	query := g.store.DB.Where("mansion_id = ? AND harvested = ? AND ready_at > ?", mansion.ID, false, time.Now())
	if raw != "全部" {
		plot, parseErr := strconv.Atoi(raw)
		if parseErr != nil || plot <= 0 {
			return GameResult{Title: "护持目标错误", Content: "地块号必须是正整数，也可以发送“护持灵田 全部”。", Actions: []string{"我的灵田"}}, true, nil
		}
		query = query.Where("plot = ?", plot)
	}
	var crops []model.MansionCrop
	if err := query.Order("plot").Find(&crops).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(crops) == 0 {
		return GameResult{Title: "没有可护持田垄", Content: "目标田垄为空闲或作物已经成熟，本次没有消耗体力。", Actions: []string{"我的灵田", "种植"}}, true, nil
	}
	cost := int64(len(crops) * 2)
	remaining, staminaErr := g.useStamina(player.ID, cost)
	if staminaErr != nil {
		return GameResult{Title: "护持体力不足", Content: fmt.Sprintf("护持%d块田垄需要%d体力。%s", len(crops), cost, staminaErr.Error()), Actions: []string{"体力", "我的灵田"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		for _, crop := range crops {
			resistance := minInt(crop.DisasterResistance+15, 100)
			if err := tx.Model(&model.MansionCrop{}).Where("id = ? AND harvested = ?", crop.ID, false).Updates(map[string]any{"protected": true, "disaster_resistance": resistance}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_, _ = g.addPlayerValueInt(player.ID, "stamina.value", cost)
		return GameResult{}, true, err
	}
	plots := make([]string, 0, len(crops))
	for _, crop := range crops {
		plots = append(plots, strconv.Itoa(crop.Plot))
	}
	return GameResult{Title: "护田阵成", Content: fmt.Sprintf("护持田垄：第%s垄\n每垄抗灾：+15\n消耗体力：%d\n剩余体力：%d", strings.Join(plots, "、第"), cost, remaining), Actions: []string{"我的灵田", "灵田天象", "灵田灾异录"}}, true, nil
}

func (g *Game) farmDisasterLog(player *model.Player, raw string) (GameResult, bool, error) {
	value, err := g.playerValue(player.ID, farmDisasterLogKey)
	if err != nil || strings.TrimSpace(value) == "" {
		return GameResult{Title: "灵田灾异录", Content: "当前没有灵田天象造成的增益或灾害记录。", Actions: []string{"灵田天象", "我的灵田", "护持灵田 全部"}}, true, nil
	}
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	entries := strings.Split(strings.TrimSpace(value), "\n")
	const pageSize = 6
	pages := maxInt((len(entries)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(entries))
	lines := []string{fmt.Sprintf("第%d/%d页 · 最近%d条", page, pages, len(entries)), "━━━━━━━━━━━"}
	lines = append(lines, entries[start:end]...)
	actions := []string{"灵田天象", "我的灵田", "护持灵田 全部"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("灵田灾异录 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("灵田灾异录 %d", page+1))
	}
	return GameResult{Title: "灵田灾异录", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}
