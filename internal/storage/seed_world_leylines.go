package storage

import (
	"fmt"
	"strconv"
	"strings"

	"xianlv/internal/model"
)

var worldLeylineOrigins = []string{"青丘", "赤霄", "玄都", "沧海", "昆仑", "幽冥", "九天", "太虚", "鸿蒙", "混沌"}
var worldLeylineElements = []string{"庚金", "乙木", "玄水", "离火", "厚土", "风灵", "雷灵", "冰魄", "时空", "轮回"}
var worldLeylineForms = []string{"龙脊", "凤巢", "天河", "地心", "剑渊", "药泉", "雷池", "月窟", "星井", "道眼"}
var worldLeylineGrades = []string{"微型灵脉", "下品灵脉", "中品灵脉", "上品灵脉", "极品灵脉", "地仙灵脉", "天仙灵脉", "神品灵脉", "混沌祖脉", "大道源脉"}
var worldLeylineSpecials = []string{"破甲剑意", "青木生息", "沧海回元", "丹火共鸣", "护山玄罡", "御风神行", "九霄雷罡", "太阴凝神", "虚空延展", "六道悟真"}
var worldLeylineMaterials = []string{"玄铁", "凝露草", "仙露", "赤焰草", "阵基石", "灵茶", "雷灵晶", "月华花", "星辰砂", "灵果"}

func (s *Store) seedWorldLeylineCatalog() error {
	for index := 1; index <= contentSeedLimit(); index++ {
		row := worldLeylineProfile(index)
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return s.migrateWorldLeylineCatalog()
}

// worldLeylineProfile distributes all ten spiritual-root origins at every
// cultivation hub. The first hub is available from the opening realm, while
// later hubs follow the world-map realm curve instead of hiding rare roots at
// the far end of the catalogue.
func worldLeylineProfile(index int) model.WorldLeyline {
	if index < 1 {
		index = 1
	}
	i := index - 1
	elementIndex := i % len(worldLeylineElements)
	tier := i / len(worldLeylineElements)
	originIndex := (tier / len(worldLeylineForms)) % len(worldLeylineOrigins)
	formIndex := tier % len(worldLeylineForms)
	locationIndex := tier*10 + 1
	location, region := seededWorldLocationName(locationIndex)
	element := worldLeylineElements[elementIndex]
	name := worldLeylineOrigins[originIndex] + element + worldLeylineForms[formIndex] + "灵脉"
	multiplier := 2.0 + float64(index)*0.00991
	aura := int64(180 + index*17)
	minimumPower := int64(45 + tier*tier*90 + elementIndex*3)
	minimumSpirit := int64(5 + tier*3 + elementIndex/2)
	requiredCount := int64(1 + tier/10 + elementIndex%3)
	bonusPower := 1000 + index*113
	return model.WorldLeyline{
		Code: indexWorldLeylineCode(index), Name: name, Region: region, LocationName: location,
		Element: element, Grade: worldLeylineGrades[originIndex], AuraPerMinute: aura,
		CultivationMultiplier: multiplier, MeditationSlots: 1 + (index*3)%8,
		DiscoveryManaCost: int64(2 + elementIndex + tier%7), MinimumRealmSequence: tier + 1, MinimumRealmLevel: 1,
		MinimumCombatPower: minimumPower, MinimumSpirit: minimumSpirit,
		RequiredRootElement: element, RequiredItem: worldLeylineMaterials[elementIndex], RequiredItemCount: requiredCount,
		BonusJSON:   fmt.Sprintf(`{"special_effect":"%s","unique_bonus_power":%d,"attack_basis_points":%d,"defense_basis_points":%d,"aura_control":%d,"breakthrough_insight":%d}`, worldLeylineSpecials[elementIndex], bonusPower, 100+index*2, 120+index*3, 50+index*5, 20+index*7),
		Description: fmt.Sprintf("%s深藏于%s的%s地下，以%s本源贯穿%s道相。入脉后修炼倍率%.3f倍，每分钟涌出灵气%d；其独有道韵为“%s”，与其他灵脉的成长数值和修炼侧重均不相同。", name, region, location, element, worldLeylineForms[formIndex], multiplier, aura, worldLeylineSpecials[elementIndex]),
		Enabled:     true, SortOrder: index,
	}
}

func indexWorldLeylineCode(index int) string {
	return fmt.Sprintf("world_leyline_%d", index)
}

// migrateWorldLeylineCatalog updates only fields that still equal the former
// generated defaults. This keeps operator edits while repairing old catalogues
// whose roots used 风雷、太阴、太阳、星辰 and contained no 轮回灵脉.
func (s *Store) migrateWorldLeylineCatalog() error {
	var rows []model.WorldLeyline
	if err := s.DB.Where("code LIKE ?", "world_leyline_%").Order("id").Find(&rows).Error; err != nil {
		return err
	}
	type migration struct {
		row         model.WorldLeyline
		legacy      model.WorldLeyline
		target      model.WorldLeyline
		migrateName bool
	}
	migrations := make([]migration, 0, len(rows))
	for _, row := range rows {
		index, ok := parseWorldLeylineIndex(row.Code)
		if !ok || index > 1000 {
			continue
		}
		legacy := legacyWorldLeylineProfile(index)
		target := worldLeylineProfile(index)
		migrations = append(migrations, migration{row: row, legacy: legacy, target: target, migrateName: row.Name == legacy.Name && legacy.Name != target.Name})
	}
	// Both catalogues contain the same set of names in a different order. Move
	// legacy names aside first so SQLite's unique index cannot fail mid-swap.
	for index := range migrations {
		entry := &migrations[index]
		if !entry.migrateName {
			continue
		}
		temporary := fmt.Sprintf("迁脉中%d", entry.row.ID)
		if err := s.DB.Model(&model.WorldLeyline{}).Where("id = ? AND name = ?", entry.row.ID, entry.legacy.Name).Update("name", temporary).Error; err != nil {
			return err
		}
		entry.row.Name = temporary
	}
	for _, entry := range migrations {
		updates := map[string]any{}
		if entry.migrateName {
			updates["name"] = entry.target.Name
		}
		addLeylineMigrationField(updates, "region", entry.row.Region, entry.legacy.Region, entry.target.Region)
		addLeylineMigrationField(updates, "location_name", entry.row.LocationName, entry.legacy.LocationName, entry.target.LocationName)
		addLeylineMigrationField(updates, "element", entry.row.Element, entry.legacy.Element, entry.target.Element)
		addLeylineMigrationField(updates, "required_root_element", entry.row.RequiredRootElement, entry.legacy.RequiredRootElement, entry.target.RequiredRootElement)
		addLeylineMigrationField(updates, "required_item", entry.row.RequiredItem, entry.legacy.RequiredItem, entry.target.RequiredItem)
		addLeylineMigrationField(updates, "bonus_json", entry.row.BonusJSON, entry.legacy.BonusJSON, entry.target.BonusJSON)
		addLeylineMigrationField(updates, "description", entry.row.Description, entry.legacy.Description, entry.target.Description)
		addLeylineMigrationInt(updates, "minimum_realm_sequence", entry.row.MinimumRealmSequence, entry.legacy.MinimumRealmSequence, entry.target.MinimumRealmSequence)
		addLeylineMigrationInt(updates, "minimum_realm_level", entry.row.MinimumRealmLevel, entry.legacy.MinimumRealmLevel, entry.target.MinimumRealmLevel)
		addLeylineMigrationInt64(updates, "minimum_combat_power", entry.row.MinimumCombatPower, entry.legacy.MinimumCombatPower, entry.target.MinimumCombatPower)
		addLeylineMigrationInt64(updates, "minimum_spirit", entry.row.MinimumSpirit, entry.legacy.MinimumSpirit, entry.target.MinimumSpirit)
		addLeylineMigrationInt64(updates, "discovery_mana_cost", entry.row.DiscoveryManaCost, entry.legacy.DiscoveryManaCost, entry.target.DiscoveryManaCost)
		addLeylineMigrationInt64(updates, "required_item_count", entry.row.RequiredItemCount, entry.legacy.RequiredItemCount, entry.target.RequiredItemCount)
		if len(updates) > 0 {
			if err := s.DB.Model(&model.WorldLeyline{}).Where("id = ?", entry.row.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func parseWorldLeylineIndex(code string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(code), "world_leyline_"))
	return value, err == nil && value > 0 && strings.HasPrefix(strings.TrimSpace(code), "world_leyline_")
}

func addLeylineMigrationField(updates map[string]any, column, current, legacy, target string) {
	if current == legacy && current != target {
		updates[column] = target
	}
}

func addLeylineMigrationInt(updates map[string]any, column string, current, legacy, target int) {
	if current == legacy && current != target {
		updates[column] = target
	}
}

func addLeylineMigrationInt64(updates map[string]any, column string, current, legacy, target int64) {
	if current == legacy && current != target {
		updates[column] = target
	}
}

func legacyWorldLeylineProfile(index int) model.WorldLeyline {
	if index < 1 {
		index = 1
	}
	origins := []string{"青丘", "赤霄", "玄都", "沧海", "昆仑", "幽冥", "九天", "太虚", "鸿蒙", "混沌"}
	elements := []string{"庚金", "乙木", "玄水", "离火", "厚土", "风雷", "太阴", "太阳", "星辰", "时空"}
	forms := []string{"龙脊", "凤巢", "天河", "地心", "剑渊", "药泉", "雷池", "月窟", "星井", "道眼"}
	grades := []string{"微型灵脉", "下品灵脉", "中品灵脉", "上品灵脉", "极品灵脉", "地仙灵脉", "天仙灵脉", "神品灵脉", "混沌祖脉", "大道源脉"}
	specials := []string{"破甲剑意", "灵植丰饶", "法力回潮", "丹火共鸣", "护体山势", "风雷神行", "神魂清明", "气血涅槃", "星命悟性", "时空延展"}
	materials := []string{"玄铁", "凝露草", "仙露", "赤焰草", "阵基石", "雷灵晶", "月华花", "龙血芝", "星辰砂", "灵茶"}
	i := index - 1
	originIndex := (i / 100) % len(origins)
	elementIndex := (i / 10) % len(elements)
	formIndex := i % len(forms)
	locationIndex := ((i % 334) * 3) + 1
	if locationIndex > 1000 {
		locationIndex = 1
	}
	location, region := seededWorldLocationName(locationIndex)
	name := origins[originIndex] + elements[elementIndex] + forms[formIndex] + "灵脉"
	multiplier := 2.0 + float64(index)*0.00991
	aura := int64(180 + index*17)
	bonusPower := 1000 + index*113
	return model.WorldLeyline{
		Code: indexWorldLeylineCode(index), Name: name, Region: region, LocationName: location,
		Element: elements[elementIndex], Grade: grades[originIndex], AuraPerMinute: aura,
		CultivationMultiplier: multiplier, MeditationSlots: 1 + (index*3)%8,
		DiscoveryManaCost: int64(3 + index%37), MinimumRealmSequence: 1 + i%1000, MinimumRealmLevel: 1 + (i*7)%10,
		MinimumCombatPower: int64(80 + index*index*9), MinimumSpirit: int64(8 + index*3),
		RequiredRootElement: elements[elementIndex], RequiredItem: materials[(index*7)%len(materials)], RequiredItemCount: int64(1 + index%9),
		BonusJSON:   fmt.Sprintf(`{"special_effect":"%s","unique_bonus_power":%d,"attack_basis_points":%d,"defense_basis_points":%d,"aura_control":%d,"breakthrough_insight":%d}`, specials[formIndex], bonusPower, 100+index*2, 120+index*3, 50+index*5, 20+index*7),
		Description: fmt.Sprintf("%s深藏于%s的%s地下，以%s本源贯穿%s道相。入脉后修炼倍率%.3f倍，每分钟涌出灵气%d；其独有道韵为“%s”，与其他灵脉的成长数值和修炼侧重均不相同。", name, region, location, elements[elementIndex], forms[formIndex], multiplier, aura, specials[formIndex]),
		Enabled:     true, SortOrder: index,
	}
}

func seededWorldLocationName(index int) (string, string) {
	regions := []string{"东洲", "南疆", "西漠", "北原", "中天域", "沧海", "幽冥界", "九霄天", "太虚境", "星河界"}
	region := regions[((index-1)/10)%len(regions)]
	if index == 1 {
		return "青云山脚", "东洲"
	}
	return region + "·" + seedPlaceName(index), region
}
