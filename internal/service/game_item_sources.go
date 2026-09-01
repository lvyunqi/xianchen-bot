package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"xianlv/internal/model"
)

type itemSource struct {
	Text    string
	Actions []string
}

func (g *Game) itemAcquisitionGuide(itemName string) (string, []string) {
	itemName = strings.TrimSpace(itemName)
	if itemName == "" {
		return "暂无可查询来源", nil
	}
	sources := make([]itemSource, 0, 12)
	like := "%\"" + itemName + "\"%"

	var resourceMaps []model.WorldLocation
	_ = g.store.DB.Where("enabled = ? AND resource_name = ?", true, itemName).Order("minimum_realm_sequence,minimum_realm_level,sort_order,id").Limit(3).Find(&resourceMaps).Error
	for _, row := range resourceMaps {
		sources = append(sources, itemSource{
			Text:    fmt.Sprintf("地图采集：%s·%s（%s，发送“采集 %s”）", row.Region, row.Name, g.locationRealmRequirement(row), itemName),
			Actions: []string{"前往 " + row.Name, "采集 " + itemName},
		})
	}

	var monsterMaps []model.WorldLocation
	_ = g.store.DB.Where("enabled = ? AND monster_reward_json LIKE ?", true, like).Order("minimum_realm_sequence,minimum_realm_level,sort_order,id").Limit(3).Find(&monsterMaps).Error
	for _, row := range monsterMaps {
		sources = append(sources, itemSource{
			Text:    fmt.Sprintf("妖兽掉落：%s的%s（%s，逐回合挑战）", row.Name, row.MonsterName, g.locationRealmRequirement(row)),
			Actions: []string{"前往 " + row.Name, "挑战 " + row.MonsterName},
		})
	}

	var bossMaps []model.WorldLocation
	_ = g.store.DB.Where("enabled = ? AND boss_reward_json LIKE ?", true, like).Order("minimum_realm_sequence,minimum_realm_level,sort_order,id").Limit(2).Find(&bossMaps).Error
	for _, row := range bossMaps {
		sources = append(sources, itemSource{
			Text:    fmt.Sprintf("首领掉落：%s的%s（战力%d，逐回合讨伐）", row.Name, row.BossName, row.BossPower),
			Actions: []string{"前往 " + row.Name, "首领", "讨伐"},
		})
	}

	var dungeons []model.Dungeon
	_ = g.store.DB.Where("enabled = ? AND reward_pool_json LIKE ?", true, like).Order("recommended_power,id").Limit(3).Find(&dungeons).Error
	for _, row := range dungeons {
		sources = append(sources, itemSource{
			Text:    fmt.Sprintf("副本掉落：%s【%s】（推荐战力%d，进入后逐回合战斗）", row.Name, row.Difficulty, row.RecommendedPower),
			Actions: []string{"进入 " + row.Name, "副本"},
		})
	}

	var seeds []model.Item
	_ = g.store.DB.Where("category_name = ? AND effect_params LIKE ?", "种子", like).Order("base_value,id").Limit(2).Find(&seeds).Error
	for _, seed := range seeds {
		sources = append(sources, itemSource{
			Text:    fmt.Sprintf("灵田培育：种子商店购买%s，种植成熟后收获%s", seed.Name, itemName),
			Actions: []string{"种子商店", "购买种子 " + seed.Name, "灵田"},
		})
	}

	var shops []model.ShopEntry
	_ = g.store.DB.Where("enabled = ? AND item_name = ?", true, itemName).Order("price,id").Limit(2).Find(&shops).Error
	for _, shop := range shops {
		command := "购入 " + itemName
		menu := "货铺"
		switch shop.Currency {
		case "银币", "仙金":
			command = shop.Currency + "购买 " + itemName
			menu = shop.Currency + "商城"
		case "竞技币":
			command = "竞商 " + itemName
			menu = "竞技商店"
		case "贡献":
			menu = "宗门菜单"
		}
		sources = append(sources, itemSource{Text: fmt.Sprintf("常设购买：%s，单价%d%s，不限购", menu, shop.Price, shop.Currency), Actions: []string{menu, command}})
	}

	var poolNames []string
	_ = g.store.DB.Table("drop_entries").Distinct("drop_pools.name").Joins("JOIN drop_pools ON drop_pools.id = drop_entries.drop_pool_id").Where("drop_entries.item_name = ? AND drop_pools.enabled = ?", itemName, true).Limit(4).Pluck("drop_pools.name", &poolNames).Error
	for _, name := range poolNames {
		sources = append(sources, itemSource{Text: "掉落池：" + name, Actions: []string{"查询 " + itemName}})
	}

	if len(sources) == 0 {
		return "当前数据没有配置可执行来源，请主人在地图、妖兽、Boss、副本、任务或商店中至少配置一处。", []string{"查询 " + itemName, "地图", "副本", "货铺"}
	}
	seenText := make(map[string]struct{})
	seenAction := make(map[string]struct{})
	lines := make([]string, 0, len(sources))
	actions := make([]string, 0, len(sources)*2)
	for _, source := range sources {
		if _, exists := seenText[source.Text]; !exists {
			seenText[source.Text] = struct{}{}
			lines = append(lines, "- "+source.Text)
		}
		for _, action := range source.Actions {
			if action == "" {
				continue
			}
			if _, exists := seenAction[action]; exists {
				continue
			}
			seenAction[action] = struct{}{}
			actions = append(actions, action)
		}
	}
	return strings.Join(lines, "\n"), actions
}

func (g *Game) craftingMaterialGuide(raw string) (string, []string) {
	materials := make(map[string]int64)
	if json.Unmarshal([]byte(raw), &materials) != nil || len(materials) == 0 {
		return "材料来源尚未配置。", nil
	}
	names := make([]string, 0, len(materials))
	for name := range materials {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	actions := make([]string, 0, len(names)*2)
	seen := make(map[string]struct{})
	for _, name := range names {
		guide, sourceActions := g.itemAcquisitionGuide(name)
		lines = append(lines, "【"+name+"】\n"+guide)
		for _, action := range append([]string{"物品 " + name}, sourceActions...) {
			if _, exists := seen[action]; exists {
				continue
			}
			seen[action] = struct{}{}
			actions = append(actions, action)
		}
	}
	return strings.Join(lines, "\n"), actions
}

func (g *Game) locationRealmRequirement(location model.WorldLocation) string {
	if location.MinimumRealmSequence <= 0 {
		return "无境界前置"
	}
	var realm model.Realm
	if g.store.DB.Where("sequence = ?", location.MinimumRealmSequence).First(&realm).Error == nil {
		return fmt.Sprintf("需%s·%d层", realm.Name, maxInt(location.MinimumRealmLevel, 1))
	}
	return fmt.Sprintf("需第%d境·%d层", location.MinimumRealmSequence, maxInt(location.MinimumRealmLevel, 1))
}
