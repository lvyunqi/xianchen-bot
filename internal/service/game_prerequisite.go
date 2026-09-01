package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"xianlv/internal/model"
)

type gameplayPrerequisite struct {
	MinimumRealm         string `json:"minimum_realm"`
	MinimumRealmSequence int    `json:"minimum_realm_sequence"`
	MinimumRealmLevel    int    `json:"minimum_realm_level"`
	MinimumLevel         int    `json:"minimum_level"`
	MinimumCombatPower   int64  `json:"minimum_combat_power"`
	MinimumReputation    int64  `json:"minimum_reputation"`
	MinimumMerit         int64  `json:"minimum_merit"`
	MinimumSpirit        int64  `json:"minimum_spirit"`
	MinimumPerception    int64  `json:"minimum_perception"`
	MinimumWillpower     int64  `json:"minimum_willpower"`
	MinimumLuck          int64  `json:"minimum_luck"`
	MinimumDaoHeart      int64  `json:"minimum_dao_heart"`
	MinimumAffinity      int64  `json:"minimum_immortal_affinity"`
	MinimumRootQuality   int    `json:"minimum_root_quality"`
	MinimumMana          int64  `json:"minimum_mana"`
	RequiredRootElement  string `json:"required_root_element"`
	Location             string `json:"location"`
	SectRequired         bool   `json:"sect_required"`
	CoupleRequired       bool   `json:"couple_required"`
	MansionRequired      bool   `json:"mansion_required"`
	MinimumFarmLevel     int    `json:"minimum_farm_level"`
	PreviousTask         string `json:"previous_task"`
	Item                 string `json:"item"`
	ItemCount            int64  `json:"item_count"`
}

func decodeGameplayPrerequisite(raw string) (gameplayPrerequisite, error) {
	var requirement gameplayPrerequisite
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return requirement, nil
	}
	if err := json.Unmarshal([]byte(raw), &requirement); err != nil {
		return requirement, err
	}
	return requirement, nil
}

func (g *Game) prerequisiteStatus(player *model.Player, raw string) (string, []string, error) {
	requirement, err := decodeGameplayPrerequisite(raw)
	if err != nil {
		return "前置道纹无法解析", []string{"请主人检查玩法前置配置"}, err
	}
	var required []string
	var unmet []string
	sequence, sequenceErr := g.playerRealmSequence(player)
	if sequenceErr != nil {
		sequence = 1
	}
	minimumSequence := requirement.MinimumRealmSequence
	if requirement.MinimumRealm != "" {
		var realm model.Realm
		if g.store.DB.Where("name = ?", requirement.MinimumRealm).First(&realm).Error == nil && realm.Sequence > minimumSequence {
			minimumSequence = realm.Sequence
		}
	}
	if minimumSequence > 0 {
		name := fmt.Sprintf("第%d境", minimumSequence)
		var realm model.Realm
		if g.store.DB.Where("sequence = ?", minimumSequence).First(&realm).Error == nil {
			name = realm.Name
		}
		required = append(required, "境界达到"+name)
		if sequence < minimumSequence {
			unmet = append(unmet, fmt.Sprintf("境界不足：需要%s，当前%s", name, player.RealmName))
		}
	}
	if requirement.MinimumRealmLevel > 0 {
		required = append(required, fmt.Sprintf("当前大境达到%d层", requirement.MinimumRealmLevel))
		// 已进入更高大境时，不应再被低境界的层数反向拦截。
		if (minimumSequence == 0 || sequence == minimumSequence) && player.RealmLevel < requirement.MinimumRealmLevel {
			unmet = append(unmet, fmt.Sprintf("境界层数不足：需要%d层，当前%d层", requirement.MinimumRealmLevel, player.RealmLevel))
		}
	}
	if requirement.MinimumLevel > 0 {
		required = append(required, fmt.Sprintf("角色等级%d", requirement.MinimumLevel))
		if player.Level < requirement.MinimumLevel {
			unmet = append(unmet, fmt.Sprintf("角色等级不足：需要%d，当前%d", requirement.MinimumLevel, player.Level))
		}
	}
	if requirement.MinimumCombatPower > 0 {
		required = append(required, fmt.Sprintf("战力%d", requirement.MinimumCombatPower))
		if player.CombatPower < requirement.MinimumCombatPower {
			unmet = append(unmet, fmt.Sprintf("战力不足：需要%d，当前%d", requirement.MinimumCombatPower, player.CombatPower))
		}
	}
	if requirement.MinimumReputation > 0 {
		required = append(required, fmt.Sprintf("声望%d", requirement.MinimumReputation))
		if player.Reputation < requirement.MinimumReputation {
			unmet = append(unmet, fmt.Sprintf("声望不足：需要%d，当前%d", requirement.MinimumReputation, player.Reputation))
		}
	}
	if requirement.MinimumMerit > 0 {
		required = append(required, fmt.Sprintf("功德%d", requirement.MinimumMerit))
		if player.Merit < requirement.MinimumMerit {
			unmet = append(unmet, fmt.Sprintf("功德不足：需要%d，当前%d", requirement.MinimumMerit, player.Merit))
		}
	}
	attributeRequirements := []struct {
		label    string
		required int64
		current  int64
	}{
		{"神识", requirement.MinimumSpirit, player.Spirit},
		{"悟性", requirement.MinimumPerception, player.Perception},
		{"意志", requirement.MinimumWillpower, player.Willpower},
		{"运气", requirement.MinimumLuck, player.Luck},
		{"道心", requirement.MinimumDaoHeart, player.DaoHeart},
		{"仙缘", requirement.MinimumAffinity, player.ImmortalAffinity},
		{"灵根纯度", int64(requirement.MinimumRootQuality), int64(player.RootQuality)},
		{"法力", requirement.MinimumMana, player.Mana},
	}
	for _, attribute := range attributeRequirements {
		if attribute.required <= 0 {
			continue
		}
		required = append(required, fmt.Sprintf("%s%d", attribute.label, attribute.required))
		if attribute.current < attribute.required {
			unmet = append(unmet, fmt.Sprintf("%s不足：需要%d，当前%d", attribute.label, attribute.required, attribute.current))
		}
	}
	if requirement.RequiredRootElement != "" {
		required = append(required, "灵根契合"+requirement.RequiredRootElement+"本源")
		if !rootElementMatches(player.SpiritualRoot, requirement.RequiredRootElement) {
			unmet = append(unmet, fmt.Sprintf("灵根不契合：需要%s本源，当前%s", requirement.RequiredRootElement, player.SpiritualRoot))
		}
	}
	if requirement.Location != "" {
		required = append(required, "身处"+requirement.Location)
		if player.Location != requirement.Location {
			unmet = append(unmet, fmt.Sprintf("位置不符：需要前往%s，当前在%s", requirement.Location, player.Location))
		}
	}
	if requirement.SectRequired {
		required = append(required, "已经加入宗门")
		if strings.TrimSpace(player.SectName) == "" {
			unmet = append(unmet, "尚未加入宗门")
		}
	}
	if requirement.CoupleRequired {
		required = append(required, "已经结为仙侣")
		if player.CoupleID == 0 {
			unmet = append(unmet, "尚未结为仙侣")
		}
	}
	if requirement.MansionRequired {
		required = append(required, "已开辟仙府")
		if player.MansionID == 0 {
			unmet = append(unmet, "尚未开辟仙府")
		}
	}
	if requirement.MinimumFarmLevel > 0 {
		required = append(required, fmt.Sprintf("灵田达到%d阶", requirement.MinimumFarmLevel))
		var mansion model.Mansion
		if g.store.DB.Where("player_id = ?", player.ID).First(&mansion).Error != nil || mansion.FarmLevel < requirement.MinimumFarmLevel {
			current := 0
			if mansion.ID != 0 {
				current = mansion.FarmLevel
			}
			unmet = append(unmet, fmt.Sprintf("灵田田阶不足：需要%d阶，当前%d阶", requirement.MinimumFarmLevel, current))
		}
	}
	if requirement.PreviousTask != "" {
		required = append(required, "完成前序任务“"+requirement.PreviousTask+"”")
		var completed int64
		g.store.DB.Table("player_tasks").Joins("JOIN task_templates ON task_templates.id = player_tasks.task_template_id").Where("player_tasks.player_id = ? AND player_tasks.status = ? AND task_templates.name = ?", player.ID, "已完成", requirement.PreviousTask).Count(&completed)
		if completed == 0 {
			unmet = append(unmet, "前序任务未完成：“"+requirement.PreviousTask+"”")
		}
	}
	if requirement.Item != "" {
		count := max64(requirement.ItemCount, 1)
		required = append(required, fmt.Sprintf("携带%s×%d", requirement.Item, count))
		item, itemErr := g.itemByName(requirement.Item)
		if itemErr != nil || g.itemQuantity(player.ID, item.ID) < count {
			unmet = append(unmet, fmt.Sprintf("物品不足：需要%s×%d", requirement.Item, count))
		}
	}
	if len(required) == 0 {
		return "无额外前置", unmet, nil
	}
	return strings.Join(required, "、"), unmet, nil
}

func (g *Game) prerequisiteActions(unmet []string) []string {
	actions := []string{}
	text := strings.Join(unmet, "\n")
	if strings.Contains(text, "境界") || strings.Contains(text, "等级") {
		actions = append(actions, "修炼", "突破")
	}
	if strings.Contains(text, "战力") {
		actions = append(actions, "装备系统", "功法", "灵兽")
	}
	if strings.Contains(text, "神识") || strings.Contains(text, "悟性") || strings.Contains(text, "意志") || strings.Contains(text, "道心") {
		actions = append(actions, "状态", "悟道")
	}
	if strings.Contains(text, "运气") || strings.Contains(text, "气运") || strings.Contains(text, "仙缘") {
		actions = append(actions, "探索", "奇遇菜单")
	}
	if strings.Contains(text, "灵根") {
		actions = append(actions, "灵检", "灵根进化菜单")
	}
	if strings.Contains(text, "法力") {
		actions = append(actions, "状态", "使用 仙露")
	}
	if strings.Contains(text, "位置") {
		actions = append(actions, "地图", "位置")
	}
	if strings.Contains(text, "宗门") {
		actions = append(actions, "宗门菜单")
	}
	if strings.Contains(text, "仙侣") {
		actions = append(actions, "寻缘")
	}
	if strings.Contains(text, "仙府") {
		actions = append(actions, "开辟仙府", "仙府菜单")
	}
	if strings.Contains(text, "灵田") || strings.Contains(text, "田阶") {
		actions = append(actions, "灵田", "升级灵田")
	}
	if strings.Contains(text, "任务") {
		actions = append(actions, "日常", "悬赏")
	}
	if strings.Contains(text, "物品") {
		actions = append(actions, "背包", "查询")
	}
	return actions
}
