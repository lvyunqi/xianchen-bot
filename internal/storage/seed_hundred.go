package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

// seedHundredContent guarantees a substantial, unique starter catalogue for
// every editable static-data page. Existing operator edits are never replaced.
func (s *Store) seedHundredContent() error {
	staticLimit := contentSeedLimit()
	if !s.hasHundredContent(int64(staticLimit)) {
		for i := 1; i <= staticLimit; i++ {
			if err := s.seedStaticNumber(i); err != nil {
				return err
			}
		}
	}
	worldLimit := worldLocationSeedLimit()
	legacyLimit := minInt(staticLimit, worldLimit)
	for i := 1; i <= legacyLimit; i++ {
		if err := s.seedWorldLocationNumber(i, worldLimit); err != nil {
			return err
		}
	}
	if worldLimit > legacyLimit {
		return s.seedExpandedWorldLocations(legacyLimit+1, worldLimit)
	}
	return nil
}

func (s *Store) hasHundredContent(limit int64) bool {
	models := []any{
		&model.SystemSetting{}, &model.ItemCategory{}, &model.Rarity{}, &model.Item{}, &model.Event{}, &model.TaskTemplate{}, &model.Skill{},
		&model.PetTemplate{}, &model.Dungeon{}, &model.AlchemyRecipe{}, &model.ArtifactTemplate{}, &model.Title{},
		&model.Activity{}, &model.Mail{}, &model.CheckinReward{}, &model.ShopEntry{}, &model.RedemptionCode{},
		&model.Notice{}, &model.DropPool{}, &model.SensitiveWord{},
	}
	for _, target := range models {
		var count int64
		if s.DB.Model(target).Count(&count).Error != nil || count < limit {
			return false
		}
	}
	return true
}

const productionWorldLocationLimit = 10000

var cultivationWorldRegions = []string{"东洲", "南疆", "西漠", "北原", "中天域", "沧海", "幽冥界", "九霄天", "太虚境", "星河界"}

type generatedWorldLocationSeed struct {
	Location           model.WorldLocation
	Tasks              []model.TaskTemplate
	LegacyTaskRewards  []string
	LegacyResourceName string
}

func worldLocationSeedLimit() int {
	if strings.Contains(strings.ToLower(os.Args[0]), ".test") {
		return contentSeedLimit()
	}
	return productionWorldLocationLimit
}

func worldLocationIdentity(index int) (region string, regionIndex, localPosition int) {
	index = maxInt(index, 1)
	row := (index - 1) / 10
	regionIndex = row % len(cultivationWorldRegions)
	column := (index - 1) % 10
	localPosition = row/len(cultivationWorldRegions)*10 + column + 1
	return cultivationWorldRegions[regionIndex], regionIndex, localPosition
}

func worldLocationGlobalIndex(regionIndex, localPosition int) int {
	localPosition = maxInt(localPosition, 1)
	worldRow := (localPosition - 1) / 10
	column := (localPosition - 1) % 10
	return worldRow*100 + regionIndex*10 + column + 1
}

func generatedWorldLocationName(index int) string {
	region, _, _ := worldLocationIdentity(index)
	if index == 1 {
		return "青云山脚"
	}
	return region + "·" + seedPlaceName(index)
}

func generatedWorldNeighborNames(index, limit int) []string {
	_, regionIndex, localPosition := worldLocationIdentity(index)
	neighbors := make([]string, 0, 4)
	column := (localPosition - 1) % 10
	appendLocal := func(candidate int) {
		if candidate < 1 {
			return
		}
		global := worldLocationGlobalIndex(regionIndex, candidate)
		if global > limit {
			return
		}
		neighbors = append(neighbors, generatedWorldLocationName(global))
	}
	if column > 0 {
		appendLocal(localPosition - 1)
	}
	if column < 9 {
		appendLocal(localPosition + 1)
	}
	appendLocal(localPosition - 10)
	appendLocal(localPosition + 10)
	return neighbors
}

func legacyWorldNeighborNames(index, limit int) []string {
	row := (index - 1) / 10
	column := (index - 1) % 10
	neighbors := make([]string, 0, 4)
	if column > 0 {
		neighbors = append(neighbors, generatedWorldLocationName(index-1))
	}
	if column < 9 && index+1 <= limit {
		neighbors = append(neighbors, generatedWorldLocationName(index+1))
	}
	if row > 0 {
		neighbors = append(neighbors, generatedWorldLocationName(index-10))
	}
	if index+10 <= limit {
		neighbors = append(neighbors, generatedWorldLocationName(index+10))
	}
	return neighbors
}

func isWorldEntryLocation(index int) bool {
	_, _, localPosition := worldLocationIdentity(index)
	return localPosition == 1
}

func buildGeneratedWorldLocationSeed(index, limit int) (generatedWorldLocationSeed, error) {
	region, regionIndex, localPosition := worldLocationIdentity(index)
	row := (index - 1) / 10
	column := (index - 1) % 10
	neighbors := generatedWorldNeighborNames(index, limit)
	encodedNeighbors, err := json.Marshal(neighbors)
	if err != nil {
		return generatedWorldLocationSeed{}, err
	}
	properName := cultivationSeedName(index)
	npcNames := []string{properName + "巡游使", seedPlaceName(index) + "守脉人"}
	locationLabel := generatedWorldLocationName(index)
	legacyResourceName := []string{"青冥草", "赤霞花", "玄霜藤", "星砂矿"}[index%4]
	resourceName := worldResourceForIndex(index)
	bossMaterial := worldBossMaterialForIndex(index)
	taskNames := []string{"探访" + locationLabel + "的灵脉", "平息" + locationLabel + "的妖患"}
	tasks := make([]model.TaskTemplate, 0, len(taskNames))
	legacyRewards := make([]string, 0, len(taskNames))
	for taskIndex, taskName := range taskNames {
		prerequisiteJSON := fmt.Sprintf(`{"minimum_realm_sequence":%d,"minimum_realm_level":%d,"minimum_combat_power":%d,"location":"%s"}`, row+1, column+1, 60+index*15, locationLabel)
		oldRewardJSON := fmt.Sprintf(`{"cultivation":%d,"merit":%d,"reputation":%d}`, 60+index*2, 2+index%6, 3+index%4)
		rewardJSON := fmt.Sprintf(`{"cultivation":%d,"merit":%d,"reputation":%d,"silver_coins":%d,"items":{"%s":%d}}`, 60+index*2, 2+index%6, 3+index%4, 40+(row+1)*8+taskIndex*12, resourceName, 1+taskIndex+index%2)
		tasks = append(tasks, model.TaskTemplate{
			Name: taskName, Type: "地图", Description: []string{"勘察当地灵脉，完成一次探索。", "击败当地妖灵，恢复一方安宁。"}[taskIndex],
			PrerequisiteJSON: prerequisiteJSON,
			ObjectiveJSON:    fmt.Sprintf(`{"type":"%s","count":1,"location":"%s"}`, []string{"explore", "hunt"}[taskIndex], locationLabel),
			RewardJSON:       rewardJSON,
			Weight:           80 + index%20, Enabled: true,
		})
		legacyRewards = append(legacyRewards, oldRewardJSON)
	}
	npcJSON, err := json.Marshal(npcNames)
	if err != nil {
		return generatedWorldLocationSeed{}, err
	}
	tasksJSON, err := json.Marshal(taskNames)
	if err != nil {
		return generatedWorldLocationSeed{}, err
	}
	entry := localPosition == 1
	location := model.WorldLocation{
		Code:                 fmt.Sprintf("world_location_%d", index),
		Name:                 locationLabel,
		Region:               region,
		Description:          fmt.Sprintf("%s·%s道域内的%s，山川灵脉、古修遗迹与当地奇遇皆有独立来历。", region, cultivationSeedName(localPosition), seedPlaceName(index)),
		NPCJSON:              string(npcJSON),
		TasksJSON:            string(tasksJSON),
		ResourceName:         resourceName,
		ResourceQuantity:     1 + index%3,
		ResourceCooldownMin:  10 + index%8,
		TeleportEnabled:      entry || index%5 != 0,
		CrossRegionEnabled:   entry || index%25 == 0,
		MinimumRealmSequence: row + 1,
		MinimumRealmLevel:    column + 1,
		MinimumLevel:         1,
		StaminaCost:          int64(1 + row/2),
		MonsterName:          properName + "妖灵",
		MonsterPower:         int64(70 + index*18),
		MonsterEncounterRate: 0.75,
		MonsterRewardJSON:    fmt.Sprintf(`{"cultivation":%d,"merit":%d,"items":{"妖兽内丹":%d}}`, 30+index*4, 2+index%5, 1+index%2),
		BossName:             properName + "镇域妖王",
		BossPower:            int64(180 + index*45),
		BossRewardJSON:       fmt.Sprintf(`{"cultivation":%d,"spirit_stones":%d,"merit":%d,"items":{"%s":%d}}`, 300+index*25, 100+index*8, 20+index, bossMaterial, 1+index%3),
		BossCooldownMinutes:  60,
		NeighborsJSON:        string(encodedNeighbors),
		Enabled:              true,
		SortOrder:            regionIndex*productionWorldLocationLimit + localPosition*10,
	}
	return generatedWorldLocationSeed{Location: location, Tasks: tasks, LegacyTaskRewards: legacyRewards, LegacyResourceName: legacyResourceName}, nil
}

func (s *Store) seedWorldLocationNumber(index, limit int) error {
	seed, err := buildGeneratedWorldLocationSeed(index, limit)
	if err != nil {
		return err
	}
	for taskIndex, task := range seed.Tasks {
		if err := s.DB.Where("name = ?", task.Name).FirstOrCreate(&task).Error; err != nil {
			return err
		}
		if err := s.DB.Model(&model.TaskTemplate{}).Where("name = ? AND (prerequisite_json = '' OR prerequisite_json = '{}' OR prerequisite_json IS NULL)", task.Name).Update("prerequisite_json", task.PrerequisiteJSON).Error; err != nil {
			return err
		}
		if err := s.DB.Model(&model.TaskTemplate{}).Where("name = ? AND reward_json = ?", task.Name, seed.LegacyTaskRewards[taskIndex]).Update("reward_json", task.RewardJSON).Error; err != nil {
			return err
		}
	}
	rowData := seed.Location
	var existing model.WorldLocation
	err = s.DB.Where("code = ?", rowData.Code).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.DB.Create(&rowData).Error
	}
	if err != nil {
		return err
	}
	if existing.MinimumRealmLevel == 0 {
		if err := s.DB.Model(&existing).Update("minimum_realm_level", rowData.MinimumRealmLevel).Error; err != nil {
			return err
		}
	}
	if existing.ResourceName == seed.LegacyResourceName || isGeneratedWorldResource(existing.ResourceName) && existing.ResourceName != rowData.ResourceName {
		if err := s.DB.Model(&existing).Update("resource_name", rowData.ResourceName).Error; err != nil {
			return err
		}
	}
	legacyLimit := minInt(limit, 1000)
	legacyNeighbors, _ := json.Marshal(legacyWorldNeighborNames(index, legacyLimit))
	if strings.TrimSpace(existing.NeighborsJSON) == "" || existing.NeighborsJSON == string(legacyNeighbors) {
		if err := s.DB.Model(&existing).Update("neighbors_json", rowData.NeighborsJSON).Error; err != nil {
			return err
		}
	}
	if isWorldEntryLocation(index) && (!existing.TeleportEnabled || !existing.CrossRegionEnabled) {
		if err := s.DB.Model(&existing).Updates(map[string]any{"teleport_enabled": true, "cross_region_enabled": true}).Error; err != nil {
			return err
		}
	}
	if !strings.Contains(existing.MonsterRewardJSON, `"items"`) {
		if err := s.DB.Model(&existing).Update("monster_reward_json", rowData.MonsterRewardJSON).Error; err != nil {
			return err
		}
	}
	if !strings.Contains(existing.BossRewardJSON, `"items"`) {
		if err := s.DB.Model(&existing).Update("boss_reward_json", rowData.BossRewardJSON).Error; err != nil {
			return err
		}
	}
	if strings.TrimSpace(existing.MonsterName) == "" || strings.TrimSpace(existing.NPCJSON) == "" || strings.TrimSpace(existing.TasksJSON) == "" || strings.TrimSpace(existing.ResourceName) == "" {
		return s.DB.Model(&existing).Updates(map[string]any{
			"npc_json": rowData.NPCJSON, "tasks_json": rowData.TasksJSON, "resource_name": rowData.ResourceName,
			"resource_quantity": rowData.ResourceQuantity, "resource_cooldown_min": rowData.ResourceCooldownMin,
			"teleport_enabled": rowData.TeleportEnabled, "cross_region_enabled": rowData.CrossRegionEnabled,
			"monster_name": rowData.MonsterName, "monster_power": rowData.MonsterPower,
			"monster_encounter_rate": rowData.MonsterEncounterRate, "monster_reward_json": rowData.MonsterRewardJSON,
			"boss_name": rowData.BossName, "boss_power": rowData.BossPower, "boss_reward_json": rowData.BossRewardJSON,
			"boss_cooldown_minutes": rowData.BossCooldownMinutes,
		}).Error
	}
	return nil
}

func (s *Store) seedExpandedWorldLocations(start, limit int) error {
	var generatedCount int64
	if err := s.DB.Model(&model.WorldLocation{}).Where("code LIKE ?", "world_location_%").Count(&generatedCount).Error; err != nil {
		return err
	}
	if generatedCount >= int64(limit) {
		return nil
	}
	const batchSize = 100
	for batchStart := start; batchStart <= limit; batchStart += batchSize {
		batchEnd := minInt(batchStart+batchSize-1, limit)
		locations := make([]model.WorldLocation, 0, batchEnd-batchStart+1)
		tasks := make([]model.TaskTemplate, 0, (batchEnd-batchStart+1)*2)
		for index := batchStart; index <= batchEnd; index++ {
			seed, err := buildGeneratedWorldLocationSeed(index, limit)
			if err != nil {
				return err
			}
			locations = append(locations, seed.Location)
			tasks = append(tasks, seed.Tasks...)
		}
		if err := s.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&tasks, batchSize).Error; err != nil {
			return err
		}
		if err := s.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&locations, batchSize).Error; err != nil {
			return err
		}
	}
	return nil
}

func worldResourceForIndex(index int) string {
	lowRealm := []string{"凝露草", "灵茶", "赤焰草", "月华花", "玄铁"}
	if index <= 40 {
		return lowRealm[(maxInt(index, 1)-1)%len(lowRealm)]
	}
	all := []string{"凝露草", "灵茶", "赤焰草", "月华花", "玄铁", "星辰砂", "雷灵晶", "阵基石", "龙血芝"}
	return all[(index-1)%len(all)]
}

func worldBossMaterialForIndex(index int) string {
	materials := []string{"妖兽内丹", "星辰砂", "阵基石", "雷灵晶", "龙血芝"}
	return materials[(maxInt(index, 1)-1)%len(materials)]
}

func isGeneratedWorldResource(name string) bool {
	for _, candidate := range []string{"凝露草", "灵茶", "赤焰草", "月华花", "玄铁", "星辰砂", "雷灵晶", "阵基石", "龙血芝"} {
		if name == candidate {
			return true
		}
	}
	return false
}

func contentSeedLimit() int {
	if strings.Contains(strings.ToLower(os.Args[0]), ".test") {
		return 3
	}
	return 1000
}

type dungeonAccess struct {
	Stamina int
	Limit   int
}

func dungeonAccessProfile(difficulty string) dungeonAccess {
	switch difficulty {
	case "困难":
		return dungeonAccess{Stamina: 6, Limit: 12}
	case "噩梦":
		return dungeonAccess{Stamina: 9, Limit: 8}
	case "地狱":
		return dungeonAccess{Stamina: 12, Limit: 5}
	default:
		return dungeonAccess{Stamina: 3, Limit: 20}
	}
}

func cultivationSeedName(value int) string {
	heavens := []string{"太虚", "青冥", "玄霜", "赤霄", "沧溟", "紫府", "金阙", "玉京", "幽都", "星罗"}
	essences := []string{"青莲", "扶桑", "龙渊", "凤鸣", "月魄", "日曜", "雷泽", "风墟", "云海", "归元"}
	forms := []string{"流光", "照夜", "问心", "镇岳", "逐星", "藏锋", "御灵", "化劫", "长生", "无相"}
	cycles := []string{"", "太初", "鸿蒙", "混元", "无极", "乾元", "坤灵", "紫微", "天衍", "万象"}
	index := maxInt(value-1, 0)
	return cycles[(index/1000)%len(cycles)] + heavens[(index/100)%len(heavens)] + essences[(index/10)%len(essences)] + forms[index%len(forms)]
}

func seedPlaceName(value int) string {
	prefixes := []string{"青云", "栖霞", "听风", "玄月", "赤霄", "万松", "落星", "长生", "归墟", "问道"}
	landforms := []string{"山门", "剑台", "古渡", "灵谷", "天门", "道场", "湖畔", "石林", "驿站", "峰顶"}
	suffixes := []string{"云境", "月庭", "松涧", "鹤洲", "龙潭", "星野", "霜原", "竹海", "雷泽", "天池"}
	cycles := []string{"", "太初", "鸿蒙", "混元", "无极", "乾元", "坤灵", "紫微", "天衍", "万象"}
	index := maxInt(value-1, 0)
	return cycles[(index/1000)%len(cycles)] + prefixes[(index/100)%len(prefixes)] + landforms[(index/10)%len(landforms)] + suffixes[index%len(suffixes)]
}

func (s *Store) seedStaticNumber(i int) error {
	n := cultivationSeedName(i)
	rarities := []string{"凡品", "灵品", "仙品", "神品"}
	difficulties := []string{"普通", "困难", "噩梦", "地狱"}
	coreCategories := []string{
		"丹药", "材料", "灵草", "功法", "残卷", "法宝", "灵兽", "阵图", "符箓", "傀儡",
		"仙药", "矿石", "装备", "任务物品", "活动道具", "兑换道具", "仙府资源", "宗门资源", "战场物资", "星河奇珍",
	}
	categoryName := n + "仙物"
	if i <= len(coreCategories) {
		categoryName = coreCategories[i-1]
	}
	category := model.ItemCategory{Name: categoryName, Description: "收录" + n + "一脉的丹材、灵物与传承之物。", Sort: i * 10}
	if err := s.DB.Where("name = ?", category.Name).FirstOrCreate(&category).Error; err != nil {
		return err
	}

	coreRarities := []string{
		"凡品", "灵品", "仙品", "神品", "黄阶", "玄阶", "地阶", "天阶", "下品", "中品",
		"上品", "极品", "珍品", "绝品", "圣品", "帝品", "混沌", "鸿蒙", "起源", "超脱",
	}
	rarityName := n + "品"
	if i <= len(coreRarities) {
		rarityName = coreRarities[i-1]
	}
	colorValue := uint32(i) * uint32(2654435761)
	rarity := model.Rarity{
		Name: rarityName, Level: i, ValueMultiplier: 1 + float64(i-1)*0.25,
		DropWeight: maxInt(1, 101-i), Color: fmt.Sprintf("#%06X", colorValue&0xFFFFFF),
	}
	if err := s.DB.Where("name = ?", rarity.Name).FirstOrCreate(&rarity).Error; err != nil {
		return err
	}

	setting := model.SystemSetting{Key: "custom.rule." + n, Value: fmt.Sprintf("%d", i), ValueType: "int", Description: n + "规则"}
	if err := s.DB.Where("key = ?", setting.Key).FirstOrCreate(&setting).Error; err != nil {
		return err
	}
	feature := model.SystemSetting{Key: "feature.module." + n, Value: fmt.Sprintf("%t", i <= 40), ValueType: "bool", Description: n + "功能开关"}
	if err := s.DB.Where("key = ?", feature.Key).FirstOrCreate(&feature).Error; err != nil {
		return err
	}
	constant := model.SystemSetting{Key: "constant.game." + n, Value: fmt.Sprintf("%d", i*10), ValueType: "int", Description: n + "游戏常量"}
	if err := s.DB.Where("key = ?", constant.Key).FirstOrCreate(&constant).Error; err != nil {
		return err
	}
	cooldown := model.SystemSetting{Key: "cooldown.action." + n, Value: fmt.Sprintf("%d", 3+i%58), ValueType: "int", Description: n + "操作冷却"}
	if err := s.DB.Where("key = ?", cooldown.Key).FirstOrCreate(&cooldown).Error; err != nil {
		return err
	}
	alert := model.SystemSetting{Key: "alert.rule." + n, Value: fmt.Sprintf("%d", 50+i), ValueType: "int", Description: n + "监控告警阈值"}
	if err := s.DB.Where("key = ?", alert.Key).FirstOrCreate(&alert).Error; err != nil {
		return err
	}
	word := sensitiveWordSeed(i)
	legacyWord := n + "禁语"
	var existingWord model.SensitiveWord
	if err := s.DB.Where("word IN ?", []string{word.Word, legacyWord}).Order("CASE WHEN word = '" + legacyWord + "' THEN 0 ELSE 1 END").First(&existingWord).Error; err == nil {
		if existingWord.Word == legacyWord {
			if err := s.DB.Model(&existingWord).Updates(map[string]any{"word": word.Word, "replacement": word.Replacement, "enabled": true}).Error; err != nil {
				return err
			}
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.DB.Create(&word).Error; err != nil {
			return err
		}
	} else {
		return err
	}

	medicine := cultivationMedicineProfile(i, n)
	item := model.Item{
		Code: "catalog_item_" + n, Name: medicine.OutputName, CategoryName: "丹药",
		RarityName: rarities[(i-1)%len(rarities)], Description: medicine.ItemDescription,
		EffectType: medicine.EffectType, EffectFunc: medicine.EffectFunc, EffectParams: medicine.EffectParams, EffectValue: medicine.EffectValue,
		BaseValue: int64(50 + i*25), StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: int64(80 + i*30),
	}
	if err := s.firstOrCreateCodeName(&item, item.Code, item.Name); err != nil {
		return err
	}

	event := cultivationEventProfile(i)
	if err := s.DB.Where("name = ?", event.Name).FirstOrCreate(&event).Error; err != nil {
		return err
	}

	task := cultivationTaskTemplate(i)
	if err := s.DB.Where("name = ?", task.Name).FirstOrCreate(&task).Error; err != nil {
		return err
	}

	skill := cultivationSkillProfile(i, n)
	if err := s.DB.Where("name = ?", skill.Name).FirstOrCreate(&skill).Error; err != nil {
		return err
	}

	petName, petTarget := cultivationPetNames(i, n)
	pet := model.PetTemplate{
		Code: "catalog_pet_" + n, Name: petName, InitialPower: int64(10 + i*4), GrowthPerLevel: int64(2 + i%12),
		LoyaltyDecay: 1 + i%4, EvolutionCondition: fmt.Sprintf(`{"loyalty":%d,"level":%d}`, 60+i%31, 5+i%30),
		EvolutionTarget: petTarget, Enabled: true,
	}
	if err := s.firstOrCreateCodeName(&pet, pet.Code, pet.Name); err != nil {
		return err
	}

	dungeon := model.Dungeon{
		Code: "catalog_dungeon_" + n, Name: cultivationDungeonName(i, n), Difficulty: difficulties[(i-1)%len(difficulties)],
		RecommendedPower: int64(100 + i*i*20), StaminaCost: dungeonAccessProfile(difficulties[(i-1)%len(difficulties)]).Stamina,
		RewardPoolJSON: fmt.Sprintf(`{"cultivation":%d,"item":"%s"}`, 100+i*25, item.Name), DailyLimit: dungeonAccessProfile(difficulties[(i-1)%len(difficulties)]).Limit, Enabled: true,
	}
	if err := s.firstOrCreateCodeName(&dungeon, dungeon.Code, dungeon.Name); err != nil {
		return err
	}

	recipe := model.AlchemyRecipe{
		Code: "catalog_recipe_" + n, Name: medicine.RecipeName, MaterialsJSON: medicine.MaterialsJSON,
		OutputItemID: item.ID, OutputName: item.Name, SuccessRate: 0.45 + float64(i%51)/100,
		Description: medicine.RecipeDescription, Enabled: true,
	}
	if err := s.firstOrCreateCodeName(&recipe, recipe.Code, recipe.Name); err != nil {
		return err
	}

	artifactProfile := cultivationArtifactProfile(i, n)
	artifact := model.ArtifactTemplate{
		Code: "catalog_artifact_" + n, Name: artifactProfile.Name, Type: artifactProfile.Archetype,
		Slot: artifactProfile.Slot, Archetype: artifactProfile.Archetype, Positioning: artifactProfile.Positioning,
		SetName: artifactProfile.SetName, SetBonusJSON: artifactProfile.SetBonusJSON,
		MaterialsJSON: artifactProfile.MaterialsJSON, AttributeJSON: artifactProfile.AttributeJSON,
		MinimumRealmSequence: artifactProfile.MinimumRealmSequence, MinimumRealmLevel: artifactProfile.MinimumRealmLevel,
		MinimumCombatPower: artifactProfile.MinimumCombatPower, Description: artifactProfile.Description,
		SourceJSON: artifactProfile.SourceJSON, MaxLevel: artifactProfile.MaxLevel, Enabled: true,
	}
	if err := s.firstOrCreateCodeName(&artifact, artifact.Code, artifact.Name); err != nil {
		return err
	}

	title := model.Title{
		Code: "catalog_title_" + n, Name: cultivationTitleName(i, n), Condition: "完成“" + task.Name + "”并达成对应修行前置",
		AttributeBonus: fmt.Sprintf(`{"all_percent":%d}`, 1+i%20), Type: []string{"境界", "仙侣", "战斗", "隐藏"}[(i-1)%4], Enabled: true,
	}
	if err := s.firstOrCreateCodeName(&title, title.Code, title.Name); err != nil {
		return err
	}

	now := time.Now().Truncate(time.Second)
	activityName, activityEffect := cultivationActivity(i, n)
	activity := model.Activity{
		Code: "catalog_activity_" + n, Name: activityName, Type: []string{"修炼", "探索", "战斗", "渡劫"}[(i-1)%4],
		StartsAt: now.AddDate(0, 0, i-1), EndsAt: now.AddDate(0, 0, i+6), Effect: activityEffect,
		EffectJSON: fmt.Sprintf(`{"multiplier":%.2f}`, 1+float64(5+i%46)/100), Status: "未开始",
	}
	if err := s.DB.Where("code = ?", activity.Code).FirstOrCreate(&activity).Error; err != nil {
		return err
	}

	expires := now.AddDate(1, 0, i)
	mailTitle, mailContent, mailSender := cultivationMail(i, n)
	mail := model.Mail{
		Code: "catalog_mail_" + n, Title: mailTitle, Content: mailContent, Sender: mailSender,
		RewardJSON: fmt.Sprintf(`[{"item":"%s","count":%d}]`, item.Name, 1+i%5), TargetType: "全部", ExpiresAt: &expires,
	}
	if err := s.DB.Where("code = ?", mail.Code).FirstOrCreate(&mail).Error; err != nil {
		return err
	}

	checkin := model.CheckinReward{Day: i, ItemName: item.Name, Quantity: int64(1 + i%8), SpecialReward: n + "修为礼匣"}
	if err := s.DB.Where("day = ?", i).FirstOrCreate(&checkin).Error; err != nil {
		return err
	}

	shopCurrencies := []string{"灵石", "银币", "仙金", "贡献", "竞技币"}
	shopCurrency := shopCurrencies[(i-1)%len(shopCurrencies)]
	shopPrice := int64(100 + i*35)
	if shopCurrency == "仙金" {
		shopPrice = int64(1 + i%98)
	}
	shop := model.ShopEntry{
		Code: "catalog_shop_" + n, ItemID: item.ID, ItemName: item.Name, Currency: shopCurrency,
		Price: shopPrice, PurchaseLimit: 0, RefreshCycle: "永不", Sort: i, Enabled: true,
	}
	if err := s.DB.Where("code = ?", shop.Code).FirstOrCreate(&shop).Error; err != nil {
		return err
	}

	cdk := model.RedemptionCode{Code: "XLYQ" + fmt.Sprintf("%06d", 202600+i), RewardJSON: fmt.Sprintf(`[{"item":"%s","count":%d}]`, item.Name, 1+i%5), MaxUses: 1000 + i*10, ExpiresAt: &expires, Status: "有效"}
	if err := s.DB.Where("code = ?", cdk.Code).FirstOrCreate(&cdk).Error; err != nil {
		return err
	}

	published := now.Add(-time.Duration(i) * time.Hour)
	noticeTitle, noticeContent := cultivationNotice(i, n)
	notice := model.Notice{
		Code: "catalog_notice_" + n, Title: noticeTitle, Content: noticeContent,
		Type: "公告", Pinned: i <= 3, Published: true, PublishedAt: &published,
	}
	if err := s.DB.Where("code = ?", notice.Code).FirstOrCreate(&notice).Error; err != nil {
		return err
	}

	pool := model.DropPool{Name: cultivationDropPoolName(i, n), SourceType: []string{"探索", "战斗", "副本"}[(i-1)%3], SourceName: dungeon.Name, Enabled: true}
	if err := s.DB.Where("name = ?", pool.Name).FirstOrCreate(&pool).Error; err != nil {
		return err
	}
	entry := model.DropEntry{DropPoolID: pool.ID, ItemID: item.ID, ItemName: item.Name, Weight: 20 + i%81, Minimum: 1, Maximum: int64(1 + i%5)}
	return s.DB.Where("drop_pool_id = ? AND item_id = ?", pool.ID, item.ID).FirstOrCreate(&entry).Error
}

func sensitiveWordSeed(index int) model.SensitiveWord {
	// The catalogue stores common obfuscations as separate editable entries.
	// Runtime matching also normalizes punctuation so operators can add plain
	// phrases without manually entering every separator variant.
	bases := []string{
		"加微信", "加威信", "加微", "私聊联系", "扫码进群", "群号联系", "免费代充", "低价代充", "低价充值", "内部充值",
		"出售账号", "收购账号", "账号交易", "租借账号", "游戏外挂", "辅助脚本", "自动刷币", "免费刷钻", "私服推广", "破解插件",
		"免授权版", "绕过授权", "盗号教程", "钓鱼链接", "点击领奖", "虚假中奖", "冒充客服", "退款诈骗", "兼职刷单", "高薪日结",
		"网络博彩", "在线赌博", "下注平台", "彩票内幕", "包赢技巧", "稳赚不赔", "赌博群", "棋牌提现", "赌场代理", "赌球网站",
		"裸聊", "色情网站", "成人视频", "成人影片", "成人直播", "招嫖信息", "上门服务", "情色交易", "不雅照片", "私密视频",
		"未成年色情", "儿童色情", "强奸视频", "迷奸药", "催情药", "性交易", "裸照交易", "成人视频群", "低俗直播", "色情陪聊",
		"毒品交易", "出售毒品", "购买毒品", "冰毒", "海洛因", "摇头丸", "麻古", "大麻交易", "吸毒教程", "制毒教程",
		"枪支交易", "买卖枪支", "炸弹教程", "爆炸物制作", "管制刀具交易", "杀人教程", "恐怖袭击", "恐怖组织", "极端组织", "非法集会",
		"分裂国家", "颠覆政权", "反动组织", "煽动暴乱", "种族仇恨", "纳粹宣传", "法西斯宣传", "人肉搜索", "身份证号码", "银行卡密码",
		"手机验证码", "家庭住址", "开盒信息", "恶意曝光隐私", "去死吧", "全家去死", "人身威胁", "网络暴力", "恶意辱骂", "仇恨言论",
		"草泥马", "我草泥马", "操你妈", "草你妈", "你妈死了", "妈的", "傻逼", "煞笔", "脑残", "废物东西",
	}
	separators := []string{"", " ", ".", "·", "-", "_", "*", "#", "/", "|"}
	baseIndex := (index - 1) % len(bases)
	variant := ((index - 1) / len(bases)) % len(separators)
	base := strings.ReplaceAll(strings.TrimSpace(bases[baseIndex]), " ", "")
	word := base
	if separator := separators[variant]; separator != "" {
		runes := []rune(base)
		parts := make([]string, 0, len(runes))
		for _, value := range runes {
			parts = append(parts, string(value))
		}
		word = strings.Join(parts, separator)
	}
	replacements := []string{"[广告已屏蔽]", "[诈骗信息已屏蔽]", "[赌博信息已屏蔽]", "[低俗内容已屏蔽]", "[违禁内容已屏蔽]"}
	replacementIndex := baseIndex / 20
	if replacementIndex >= len(replacements) {
		replacementIndex = len(replacements) - 1
	}
	replacement := replacements[replacementIndex]
	return model.SensitiveWord{Word: word, Replacement: replacement, Enabled: true}
}

func (s *Store) migrateGeneratedStaticCatalog() error {
	limit := contentSeedLimit()
	for index := 1; index <= limit; index++ {
		n := cultivationSeedName(index)
		event := cultivationEventProfile(index)
		var oldEvent model.Event
		if s.DB.Where("name = ?", n+"山海异闻").First(&oldEvent).Error == nil {
			if err := s.DB.Model(&oldEvent).Updates(map[string]any{"name": event.Name, "type": event.Type, "description": event.Description, "probability": event.Probability, "reward_json": event.RewardJSON, "condition_json": event.ConditionJSON}).Error; err != nil {
				return err
			}
		}

		skill := cultivationSkillProfile(index, n)
		var oldSkill model.Skill
		if s.DB.Where("name = ?", n+"道诀").First(&oldSkill).Error == nil {
			if err := s.DB.Model(&oldSkill).Updates(map[string]any{"name": skill.Name, "type": skill.Type, "rarity": skill.Rarity, "realm_required": skill.RealmRequired, "description": skill.Description, "effect_json": skill.EffectJSON, "upgrade_json": skill.UpgradeJSON}).Error; err != nil {
				return err
			}
		}

		petName, petTarget := cultivationPetNames(index, n)
		if err := s.updateGeneratedCodeRow(&model.PetTemplate{}, "catalog_pet_"+n, "name", n+"灵兽", map[string]any{"name": petName, "evolution_target": petTarget}); err != nil {
			return err
		}
		dungeonName := cultivationDungeonName(index, n)
		if err := s.updateGeneratedCodeRow(&model.Dungeon{}, "catalog_dungeon_"+n, "name", n+"秘境", map[string]any{"name": dungeonName}); err != nil {
			return err
		}
		artifactName := cultivationArtifactName(index, n)
		if err := s.updateGeneratedCodeRow(&model.ArtifactTemplate{}, "catalog_artifact_"+n, "name", n+"法宝", map[string]any{"name": artifactName}); err != nil {
			return err
		}
		titleName := cultivationTitleName(index, n)
		if err := s.updateGeneratedCodeRow(&model.Title{}, "catalog_title_"+n, "name", n+"行者", map[string]any{"name": titleName, "condition": "完成“" + cultivationTaskName(index) + "”并达成对应修行前置"}); err != nil {
			return err
		}
		activityName, activityEffect := cultivationActivity(index, n)
		if err := s.updateGeneratedCodeRow(&model.Activity{}, "catalog_activity_"+n, "name", n+"仙门庆典", map[string]any{"name": activityName, "effect": activityEffect}); err != nil {
			return err
		}
		mailTitle, mailContent, mailSender := cultivationMail(index, n)
		if err := s.updateGeneratedCodeRow(&model.Mail{}, "catalog_mail_"+n, "title", n+"仙途书信", map[string]any{"title": mailTitle, "content": mailContent, "sender": mailSender}); err != nil {
			return err
		}
		noticeTitle, noticeContent := cultivationNotice(index, n)
		if err := s.updateGeneratedCodeRow(&model.Notice{}, "catalog_notice_"+n, "title", n+"仙途告示", map[string]any{"title": noticeTitle, "content": noticeContent}); err != nil {
			return err
		}
		oldCurrencies := []string{"灵石", "贡献", "竞技币"}
		newCurrencies := []string{"灵石", "银币", "仙金", "贡献", "竞技币"}
		oldCurrency := oldCurrencies[(index-1)%len(oldCurrencies)]
		newCurrency := newCurrencies[(index-1)%len(newCurrencies)]
		newPrice := int64(100 + index*35)
		if newCurrency == "仙金" {
			newPrice = int64(1 + index%98)
		}
		_ = s.DB.Model(&model.ShopEntry{}).Where("code = ? AND currency = ?", "catalog_shop_"+n, oldCurrency).Updates(map[string]any{"currency": newCurrency, "price": newPrice}).Error

		var oldPool model.DropPool
		if s.DB.Where("name = ?", n+"珍藏").First(&oldPool).Error == nil {
			if err := s.DB.Model(&oldPool).Updates(map[string]any{"name": cultivationDropPoolName(index, n), "source_name": dungeonName}).Error; err != nil {
				return err
			}
		}
	}
	// 迁移早期“第X号独立公告”占位内容，不触碰运营人员已改写的真实公告。
	var legacyNotices []model.Notice
	if err := s.DB.Where("content LIKE ?", "%号独立公告内容%").Order("id").Find(&legacyNotices).Error; err != nil {
		return err
	}
	for index := range legacyNotices {
		seedIndex := index + 1
		n := cultivationSeedName(seedIndex)
		title, content := cultivationNotice(seedIndex, n)
		if err := s.DB.Model(&legacyNotices[index]).Updates(map[string]any{"title": title, "content": content}).Error; err != nil {
			return err
		}
	}
	// 早期法宝名使用“诸天法宝·一”占位，改为与正式器谱一致的独立修仙名。
	var legacyArtifacts []model.ArtifactTemplate
	if err := s.DB.Where("name LIKE ?", "诸天法宝·%").Order("id").Find(&legacyArtifacts).Error; err != nil {
		return err
	}
	for index := range legacyArtifacts {
		seedIndex := index + 1
		code := "catalog_artifact_" + cultivationSeedName(seedIndex)
		name := cultivationArtifactName(seedIndex, cultivationSeedName(seedIndex))
		var canonical model.ArtifactTemplate
		err := s.DB.Where("id <> ? AND (code = ? OR name = ?)", legacyArtifacts[index].ID, code, name).Order("id").First(&canonical).Error
		if err == nil {
			if err := s.DB.Model(&model.PlayerArtifact{}).Where("template_id = ?", legacyArtifacts[index].ID).Update("template_id", canonical.ID).Error; err != nil {
				return err
			}
			if err := s.DB.Delete(&legacyArtifacts[index]).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := s.DB.Model(&legacyArtifacts[index]).Updates(map[string]any{"code": code, "name": name}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) normalizeNoticeCategories() error {
	// 普通告示只属于“公告”。版本更新、已确认修复和全区通报
	// 保持独立频道，避免玩家打开公告时混入发布记录。
	if err := s.DB.Model(&model.Notice{}).Where("type IN ?", []string{"系统", "活动"}).Update("type", "公告").Error; err != nil {
		return err
	}
	return s.DB.Model(&model.Notice{}).Where("code LIKE ?", "catalog_notice_%").Update("type", "公告").Error
}

// normalizeGeneratedArtifactCatalog upgrades only system-generated器谱. It
// deliberately leaves operator-created rows and player-customized names alone.
func (s *Store) normalizeGeneratedArtifactCatalog() error {
	var generated []model.ArtifactTemplate
	if err := s.DB.Where("code LIKE ?", "catalog_artifact_%").Order("id").Limit(contentSeedLimit()).Find(&generated).Error; err != nil {
		return err
	}
	for index := range generated {
		seedIndex := index + 1
		profile := cultivationArtifactProfile(seedIndex, cultivationSeedName(seedIndex))
		updates := map[string]any{
			"type": profile.Archetype, "slot": profile.Slot, "archetype": profile.Archetype,
			"positioning": profile.Positioning, "set_name": profile.SetName, "set_bonus_json": profile.SetBonusJSON,
			"materials_json": profile.MaterialsJSON, "attribute_json": profile.AttributeJSON,
			"minimum_realm_sequence": profile.MinimumRealmSequence, "minimum_realm_level": profile.MinimumRealmLevel,
			"minimum_combat_power": profile.MinimumCombatPower, "description": profile.Description,
			"source_json": profile.SourceJSON, "max_level": profile.MaxLevel, "enabled": true,
		}
		if err := s.DB.Model(&generated[index]).Updates(updates).Error; err != nil {
			return err
		}
	}
	core := []struct {
		code, slot, archetype, positioning, description string
	}{
		{"artifact_sword", "本命法器", "仙剑", "攻伐破甲", "青冥剑为入门本命剑器，仅占用本命法器槽位。"},
		{"artifact_bell", "护符", "法钟", "护魂防御", "玄元钟以钟鸣护持元神，占用护符槽位，不会替换本命剑器。"},
	}
	for _, row := range core {
		if err := s.DB.Model(&model.ArtifactTemplate{}).Where("code = ?", row.code).Updates(map[string]any{
			"type": row.archetype, "slot": row.slot, "archetype": row.archetype, "positioning": row.positioning,
			"minimum_realm_sequence": 1, "minimum_realm_level": 1, "description": row.description,
		}).Error; err != nil {
			return err
		}
	}
	return s.normalizeArtifactSlots()
}

func (s *Store) updateGeneratedCodeRow(modelValue any, code, field, oldValue string, updates map[string]any) error {
	var count int64
	if err := s.DB.Model(modelValue).Where("code = ? AND "+field+" = ?", code, oldValue).Count(&count).Error; err != nil || count == 0 {
		return err
	}
	return s.DB.Model(modelValue).Where("code = ? AND "+field+" = ?", code, oldValue).Updates(updates).Error
}

func cultivationEventProfile(index int) model.Event {
	regions := []string{"东洲", "南疆", "西漠", "北原", "中天域", "沧海", "幽冥界", "九霄天", "太虚境", "星河界"}
	sites := []string{"青云古渡", "栖霞剑冢", "听风灵谷", "玄月天池", "赤霄火窟", "万松道场", "落星荒原", "长生药园", "归墟海眼", "问道碑林"}
	omens := []string{"灵泉现世", "古剑夜鸣", "仙鹤衔书", "妖潮叩关", "星火坠野", "残魂问路", "秘藏开门", "雷云悟道", "商旅求援", "因果照心"}
	descriptions := []string{
		"地脉忽然裂开清泉，选择直接饮用可淬炼经脉，先验泉则更稳妥但可能错失灵机。",
		"无主古剑在月下长鸣，拔剑者要承受剑意反噬，参悟剑痕则考验悟性。",
		"受伤仙鹤衔来残缺玉简，救治、追踪或据为己有会走向完全不同的因果。",
		"妖潮冲击附近村寨，可正面守关、布阵拖延或护送凡人撤离，奖惩由选择决定。",
		"天外星火坠入荒野，强取会灼伤神魂，以阵收摄需要材料，静观则可能悟得星法。",
		"古修残魂拦路问道，答其执念可得传承，强行搜魂会积下业力，超度则增长功德。",
		"封闭多年的秘藏短暂开启，进入深层奖励丰厚但有机关，外围搜寻风险较低。",
		"九霄雷云垂落道痕，可引雷淬体、观雷悟法或暂避锋芒，每条分支要求不同属性。",
		"负伤商旅请求援手，护送、疗伤或追剿劫修分别影响声望、灵石与战斗收益。",
		"因果镜映出旧日选择，坦然面对可稳固道心，遮掩会招致心魔，斩镜则承担反噬。",
	}
	i := index - 1
	region := regions[(i/100)%len(regions)]
	site := sites[(i/10)%len(sites)]
	omenIndex := i % len(omens)
	name := region + "·" + site + omens[omenIndex]
	eventTypes := []string{"机缘", "奇遇", "仙缘", "劫难", "天象", "传承", "秘藏", "悟道", "善缘", "心魔"}
	realmSequence := 1 + i/10
	return model.Event{
		Name: name, Type: eventTypes[omenIndex], Description: region + site + "附近，" + descriptions[omenIndex],
		Probability:   float64(2+(index*7)%17) / 100,
		RewardJSON:    fmt.Sprintf(`{"choices":[{"name":"迎难而上","success_rate":%.2f,"reward":{"cultivation":%d,"merit":%d},"failure":{"health_percent":-%d}},{"name":"谨慎查探","success_rate":%.2f,"reward":{"spirit_stones":%d,"reputation":%d}},{"name":"顺应因果","reward":{"immortal_affinity":%d,"dao_heart":%d}}]}`, .45+float64(index%40)/100, 80+index*9, 2+index%17, 5+index%26, .65+float64(index%25)/100, 30+index*4, 1+index%13, 1+index%11, 1+index%7),
		ConditionJSON: fmt.Sprintf(`{"minimum_realm_sequence":%d,"minimum_realm_level":%d,"minimum_luck":%d,"region":"%s"}`, realmSequence, 1+i%10, 10+i%41, region),
		Enabled:       true,
	}
}

func cultivationSkillProfile(index int, n string) model.Skill {
	names := []string{"剑经", "真解", "玄功", "心法", "雷诀", "遁术", "炼神篇", "护体经", "观星术", "御兽典", "阵道录", "丹火篇", "枪典", "刀章", "音律谱", "指法", "掌印", "禁术", "长生书", "虚空法"}
	styles := []string{"剑气贯脉，重在爆发与破甲", "拆解道则，兼顾攻守与悟性", "锻体凝元，越战越稳", "守心养神，提升法力周转", "引雷代罚，克制邪祟", "身随风走，擅长先手与闪避", "月华炼神，专修神识", "法相护身，削减来袭伤害", "观星定命，影响气运与命中", "沟通万兽，强化灵兽协战"}
	i := index - 1
	kinds := []string{"攻击", "均衡", "防御", "辅助"}
	rarities := []string{"凡品", "灵品", "仙品", "神品"}
	return model.Skill{Name: n + names[i%len(names)], Type: kinds[i%len(kinds)], Rarity: rarities[(i/5)%len(rarities)], RealmRequired: realmNameByIndex(index), Description: n + "一脉秘传，" + styles[i%len(styles)] + "；修至高层后会解锁独立招式与克制关系。", EffectJSON: fmt.Sprintf(`{"attack":%d,"defense":%d,"speed":%d,"mana_cost":%d}`, 2+index%37, 1+(index*3)%29, 1+index%19, 5+index%46), UpgradeJSON: fmt.Sprintf(`{"mastery_per_level":%d,"max_level":%d,"breakthrough_item":"%s功法残篇"}`, 80+index*2, 10+index%40, n)}
}

func cultivationPetNames(index int, n string) (string, string) {
	species := []string{"云纹灵狐", "踏霜玄鹿", "赤羽火鸾", "负岳灵龟", "啸月苍狼", "吞雷狻猊", "碧海蛟", "照夜天马", "寻药灵貂", "守山夔牛", "幻梦蝶", "搬山猿", "玄金蜂后", "太阴玉兔", "九幽冥鸦", "星砂螭", "青木麒麟", "流风仙鹤", "镇魂獬豸", "混元鲲鹏"}
	speciesName := species[(index-1)%len(species)]
	return n + speciesName, "太古" + n + speciesName
}

func cultivationDungeonName(index int, n string) string {
	places := []string{"剑冢遗府", "雷狱天牢", "妖皇古墓", "星陨神殿", "归墟海眼", "太阴寒宫", "赤霄火域", "万兽祖庭", "虚空裂谷", "长生药园", "镇魔古关", "浮屠塔林", "龙脉地宫", "问心幻境", "仙舟残骸", "幽都鬼城", "天河战场", "混沌石窟", "上古道场", "因果轮台"}
	return n + places[(index-1)%len(places)]
}

func cultivationArtifactName(index int, n string) string {
	forms := []string{"斩仙剑", "镇岳印", "照魂镜", "渡厄钟", "御风舟", "万法衣", "缚龙索", "吞天葫", "星河幡", "太虚鼎", "量天尺", "离火扇", "玄水珠", "雷罚枪", "山河图", "问心琴", "逐日弓", "护道塔", "因果轮", "混元伞"}
	return n + forms[(index-1)%len(forms)]
}

type artifactSeedProfile struct {
	Name                 string
	Slot                 string
	Archetype            string
	Positioning          string
	SetName              string
	SetBonusJSON         string
	MaterialsJSON        string
	AttributeJSON        string
	MinimumRealmSequence int
	MinimumRealmLevel    int
	MinimumCombatPower   int64
	Description          string
	SourceJSON           string
	MaxLevel             int
}

func cultivationArtifactProfile(index int, n string) artifactSeedProfile {
	forms := []string{"斩仙剑", "镇岳印", "照魂镜", "渡厄钟", "御风舟", "万法衣", "缚龙索", "吞天葫", "星河幡", "太虚鼎", "量天尺", "离火扇", "玄水珠", "雷罚枪", "山河图", "问心琴", "逐日弓", "护道塔", "因果轮", "混元伞"}
	// Each cycle contains exactly two artifacts for every wearable slot. The
	// artifact form remains independent from its slot: bells can hover above a
	// crown, gourds hang at the waist, and flags or cauldrons anchor formations.
	slots := []string{"本命法器", "护符", "项链", "冠冕", "灵靴", "道袍", "护腕", "腰佩", "阵盘", "阵盘", "护腕", "戒指", "戒指", "本命法器", "道袍", "项链", "灵靴", "冠冕", "腰佩", "护符"}
	archetypes := []string{"仙剑", "法印", "宝镜", "法钟", "飞舟", "仙衣", "灵索", "宝葫", "道幡", "丹鼎", "法尺", "灵扇", "道珠", "神枪", "山河图", "道琴", "灵弓", "护道塔", "法轮", "宝伞"}
	positions := []string{"破甲攻伐", "镇压护体", "神识洞察", "护魂减伤", "身法遁行", "气血防御", "控制破势", "纳灵续航", "阵道增幅", "炼化均衡", "近战攻伐", "术法爆发", "法力护魂", "雷道攻伐", "领域镇守", "音律控场", "远程暴击", "元神守护", "因果增幅", "护体卸力"}
	uniqueEffects := []string{"庚金破甲", "镇岳定身", "照魂识破", "渡厄护魂", "御风先机", "万法护体", "缚龙迟滞", "吞天回灵", "星河聚阵", "太虚炼化", "量天精准", "离火灼魂", "玄水回元", "雷罚麻痹", "山河领域", "问心乱神", "逐日追击", "护道减伤", "因果标记", "混元卸力"}
	materials := []string{"玄铁", "星辰砂", "阵基石", "雷灵晶", "妖兽内丹", "赤焰草", "月华花", "龙血芝"}
	i := maxInt(index-1, 0)
	kind := i % len(forms)
	slot := slots[kind]
	base := int64(6 + index*2)
	stats := map[string]int64{}
	switch slot {
	case "本命法器":
		stats["attack"], stats["power"], stats["speed"] = base, base/2+int64(index%7), 1+int64(index%17)
	case "冠冕":
		stats["mana"], stats["defense"], stats["power"] = base*3, base/2, base/3
	case "道袍":
		stats["health"], stats["defense"] = base*5, base
	case "护腕":
		stats["attack"], stats["defense"], stats["speed"] = base*3/4, base/2, 1+int64(index%13)
	case "腰佩":
		stats["mana"], stats["health"], stats["defense"] = base*3, base*2, base/3
	case "灵靴":
		stats["speed"], stats["health"], stats["defense"] = 2+int64(index%31), base*2, base/3
	case "戒指":
		stats["attack"], stats["mana"], stats["power"] = base*2/3, base*2, base/2
	case "项链":
		stats["mana"], stats["defense"], stats["health"] = base*2, base/2, base*2
	case "护符":
		stats["defense"], stats["health"], stats["power"] = base*2/3, base*3, base/2
	case "阵盘":
		stats["power"], stats["defense"], stats["mana"] = base, base/2, base*2
	}
	statsJSON, _ := json.Marshal(stats)
	materialA := materials[(i*3+kind)%len(materials)]
	materialB := materials[(i*7+kind+2)%len(materials)]
	if materialA == materialB {
		materialB = materials[(i*7+kind+3)%len(materials)]
	}
	setName := cultivationSeedName(1+i/10) + "十方道装"
	return artifactSeedProfile{
		Name: n + forms[kind], Slot: slot, Archetype: archetypes[kind], Positioning: positions[kind],
		SetName: setName, SetBonusJSON: fmt.Sprintf(`{"two":{"power":%d},"four":{"health":%d,"mana":%d},"six":{"unique_effect":"%s"}}`, 5+index%41, 30+index*2, 20+index, uniqueEffects[kind]),
		MaterialsJSON: fmt.Sprintf(`{"%s":%d,"%s":%d}`, materialA, 1+index%5, materialB, 1+(index/5)%4),
		AttributeJSON: string(statsJSON), MinimumRealmSequence: 1 + i/10, MinimumRealmLevel: 1 + i%10,
		MinimumCombatPower: int64(75 + index*18), MaxLevel: 20 + index%81,
		Description: fmt.Sprintf("%s一脉的%s，器型为%s，占用%s槽位；定位%s，独有器韵“%s”。", n, forms[kind], archetypes[kind], slot, positions[kind], uniqueEffects[kind]),
		SourceJSON:  fmt.Sprintf(`{"craft":"学器后炼制","dungeon":"%s","boss":"%s"}`, cultivationDungeonName(index, n), n+"镇域妖王"),
	}
}

func cultivationTitleName(index int, n string) string {
	titles := []string{"剑魁", "丹宗", "阵侯", "御兽使", "镇魔君", "渡厄真人", "星河客", "问心者", "护道尊", "长生候", "雷劫主", "灵植师", "秘境行者", "百战天骄", "太虚使", "因果守望", "山海巡游", "九霄客", "红尘仙", "无相尊"}
	return n + titles[(index-1)%len(titles)]
}

func cultivationActivity(index int, n string) (string, string) {
	names := []string{"灵潮开脉", "万剑朝宗", "丹火大会", "御兽巡天", "九霄渡劫周", "秘境寻珍", "宗门论道", "仙侣同心会", "星河观天", "镇魔悬赏"}
	effects := []string{"闭关吐纳收益提升", "剑类功法熟练增长", "炼丹成功率提升", "灵兽经验与忠诚增长", "渡劫准备消耗降低", "秘境掉落权重提升", "宗门贡献获取提升", "双修道缘收益提升", "星力与悟性收益提升", "首领功德奖励提升"}
	i := (index - 1) % len(names)
	return n + names[i], fmt.Sprintf("%s期间%s%d%%，对应玩法前置与消耗仍然生效。", n, effects[i], 5+index%46)
}

func cultivationMail(index int, n string) (string, string, string) {
	titles := []string{"山门来帖", "剑冢飞书", "丹阁药函", "御兽谷密信", "镇魔关急报", "星宫观测札", "故人问候", "宗门嘉奖", "秘境线报", "天机阁谶书"}
	senders := []string{"青云接引使", "守剑老人", "丹霞真人", "百兽山主", "镇魔校尉", "紫微星官", "云游散仙", "执事长老", "寻宝道人", "天机阁主"}
	i := (index - 1) % len(titles)
	return n + titles[i], fmt.Sprintf("%s传来关于%s道脉的消息：此次赠礼对应独立玩法资源，请先查看物品详情与获取前置，再决定使用方向。", senders[i], n), senders[i]
}

func cultivationNotice(index int, n string) (string, string) {
	titles := []string{"灵潮观测", "秘境开放", "镇魔悬赏", "宗门会盟", "丹会预告", "斗法结算", "灵田节气", "渡劫预警", "星河异象", "仙缘见证"}
	contents := []string{"灵潮将改变闭关效率", "新秘境开放并带有境界门槛", "区域首领刷新等待讨伐", "诸宗将按声望重新排位", "丹炉成功率受活动加持", "竞技俸禄已经完成结算", "灵植成熟时间出现变化", "九霄劫云进入活跃期", "星域传送消耗暂时变化", "三生石记录新的仙侣因果"}
	i := (index - 1) % len(titles)
	return n + titles[i], fmt.Sprintf("%s：%s。详细时间、条件、奖励与惩罚以仙盟当期告示为准。", n, contents[i])
}

func cultivationDropPoolName(index int, n string) string {
	names := []string{"山野采撷", "妖兽遗珍", "首领宝库", "秘境木匣", "宗门赏赐", "星河坠物", "雷劫余烬", "古修遗藏", "灵田丰收", "仙缘馈赠"}
	return n + names[(index-1)%len(names)]
}

type medicineProfile struct {
	OutputName        string
	RecipeName        string
	MaterialsJSON     string
	EffectType        string
	EffectFunc        string
	EffectParams      string
	EffectValue       float64
	ItemDescription   string
	RecipeDescription string
}

func cultivationMedicineProfile(index int, seedName string) medicineProfile {
	forms := []string{"归元丹", "养魂散", "清心露", "破障丸", "续命膏", "洗髓液", "神行散", "护脉丹", "凝神露", "渡厄丹"}
	effectTypes := []string{"治疗", "神魂", "道心", "突破", "寿元", "灵根", "身法", "防御", "悟性", "渡劫"}
	effectFuncs := []string{"heal_hp", "add_spirit", "temporary_buff", "breakthrough_bonus", "add_lifespan", "root_refine", "temporary_buff", "temporary_buff", "add_perception", "tribulation_bonus"}
	purpose := []string{
		"补益气血并修复战斗留下的暗伤", "温养神魂，恢复过度消耗的识海", "澄澈杂念并压制心魔躁动", "冲开小境瓶颈并稳固突破后的道基", "补充生机，延缓寿元衰败", "洗炼灵根杂质并提升灵气亲和", "激发经脉风行之力并提升身法", "在经脉表面形成护体药膜", "凝聚神识并辅助参悟功法", "抵御天劫威压并护住元神",
	}
	materials := []string{"凝露草", "赤焰草", "月华花", "龙血芝", "灵果", "灵茶", "仙露", "妖兽内丹", "雷灵晶", "星辰砂", "玄铁", "阵基石"}
	i := maxInt(index-1, 0)
	kind := i % len(forms)
	first := materials[(i*5+kind)%len(materials)]
	second := materials[(i*7+kind+3)%len(materials)]
	if second == first {
		second = materials[(i*7+kind+4)%len(materials)]
	}
	amountA := 1 + index%4
	amountB := 1 + (index/7)%3
	effectValue := float64(25 + index*4 + kind*9)
	params := fmt.Sprintf(`{"catalog":"%s","grade":%d,"duration_minutes":%d}`, seedName, 1+i/100, 20+index%101)
	return medicineProfile{
		OutputName: seedName + forms[kind], RecipeName: seedName + "·" + forms[kind] + "丹经",
		MaterialsJSON: fmt.Sprintf(`{"%s":%d,"%s":%d}`, first, amountA, second, amountB),
		EffectType:    effectTypes[kind], EffectFunc: effectFuncs[kind], EffectParams: params, EffectValue: effectValue,
		ItemDescription:   fmt.Sprintf("以%s与%s合炼而成，服下后可%s；药力强度为%.0f。", first, second, purpose[kind], effectValue),
		RecipeDescription: fmt.Sprintf("记载%s与%s的火候次序，主治方向为%s。丹炉等级越高，越能保全%s药性。", first, second, purpose[kind], seedName),
	}
}

func (s *Store) migrateLegacyMedicineCatalog() error {
	var items []model.Item
	if err := s.DB.Where("code LIKE ?", "catalog_item_%").Order("id").Limit(1000).Find(&items).Error; err != nil {
		return err
	}
	for index := range items {
		profile := cultivationMedicineProfile(index+1, cultivationSeedName(index+1))
		oldName := items[index].Name
		updates := map[string]any{
			"name": profile.OutputName, "category_name": "丹药", "description": profile.ItemDescription,
			"effect_type": profile.EffectType, "effect_func": profile.EffectFunc,
			"effect_params": profile.EffectParams, "effect_value": profile.EffectValue,
		}
		if err := s.DB.Model(&items[index]).Updates(updates).Error; err != nil {
			return err
		}
		if oldName != profile.OutputName {
			if err := s.DB.Model(&model.ShopEntry{}).Where("item_id = ?", items[index].ID).Update("item_name", profile.OutputName).Error; err != nil {
				return err
			}
			if err := s.DB.Model(&model.DropEntry{}).Where("item_id = ?", items[index].ID).Update("item_name", profile.OutputName).Error; err != nil {
				return err
			}
			if err := s.DB.Model(&model.CheckinReward{}).Where("item_name = ?", oldName).Update("item_name", profile.OutputName).Error; err != nil {
				return err
			}
		}
	}
	var recipes []model.AlchemyRecipe
	if err := s.DB.Where("code LIKE ?", "catalog_recipe_%").Order("id").Limit(1000).Find(&recipes).Error; err != nil {
		return err
	}
	for index := range recipes {
		profile := cultivationMedicineProfile(index+1, cultivationSeedName(index+1))
		updates := map[string]any{
			"name": profile.RecipeName, "materials_json": profile.MaterialsJSON,
			"output_name": profile.OutputName, "description": profile.RecipeDescription,
		}
		if index < len(items) {
			updates["output_item_id"] = items[index].ID
		}
		if err := s.DB.Model(&recipes[index]).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func realmNameByIndex(i int) string {
	catalog := realmCatalog()
	index := maxInt(i-1, 0)
	if index >= len(catalog) {
		index = len(catalog) - 1
	}
	return catalog[index].Name
}

func cultivationTaskTemplate(index int) model.TaskTemplate {
	n := cultivationSeedName(index)
	actionTypes := []string{"explore", "hunt", "cultivation", "collect", "boss", "dungeon", "forge", "alchemy", "farm_harvest", "arena_win"}
	descriptions := []string{
		"沿山川灵脉完成一次深度巡游，并记录当地灵气走向", "击败盘踞山野的妖灵，为附近修士清除祸患", "完成一次有效闭关，使周天灵气归于丹田", "采集当地灵植，辨明药性后带回仙府", "讨伐区域首领并在狂暴劫势中存活", "通关一处符合当前道行的秘境副本", "完成一次装备锻造，让器胚承受玄火淬炼", "完成一次有效炼丹并保住丹药药性", "从仙府灵田收获成熟灵植", "在竞技回合中战胜一名同境修士",
	}
	i := maxInt(index-1, 0)
	action := i % len(actionTypes)
	count := 1 + i%5
	prerequisite := map[string]any{
		"minimum_realm_sequence": 1 + i/10,
		"minimum_realm_level":    1 + i%10,
		"minimum_combat_power":   80 + int64(index*15),
	}
	if index > 1 && i%10 != 0 {
		prerequisite["previous_task"] = cultivationTaskName(index - 1)
	}
	if index%17 == 0 {
		prerequisite["sect_required"] = true
	}
	if index%19 == 0 {
		prerequisite["couple_required"] = true
	}
	prerequisiteJSON, _ := json.Marshal(prerequisite)
	return model.TaskTemplate{
		Name: cultivationTaskName(index), Type: []string{"日常", "悬赏", "宗门", "支线", "主线"}[i%5],
		Description: fmt.Sprintf("循%s道意，%s。", n, descriptions[action]), PrerequisiteJSON: string(prerequisiteJSON),
		ObjectiveJSON: fmt.Sprintf(`{"type":"%s","count":%d}`, actionTypes[action], count),
		RewardJSON:    fmt.Sprintf(`{"cultivation":%d,"spirit_stones":%d,"merit":%d,"reputation":%d}`, 200+index*18, 30+index*5, 1+index%15, 2+index%12),
		Weight:        20 + index, Daily: index%2 == 0, Enabled: true,
	}
}

func cultivationTaskName(index int) string {
	actionNames := []string{"巡游问脉", "镇妖除患", "闭关凝真", "采药济世", "斩首伏魔", "秘境勘破", "玄火炼器", "丹炉蕴药", "灵田育生", "论剑争锋"}
	i := maxInt(index-1, 0)
	return cultivationSeedName(index) + "·" + actionNames[i%len(actionNames)] + "令"
}

func (s *Store) migrateLegacyTaskCatalog() error {
	var rows []model.TaskTemplate
	if err := s.DB.Where("name LIKE ?", "百日修行令·%").Order("id").Limit(1000).Find(&rows).Error; err != nil {
		return err
	}
	for index := range rows {
		target := cultivationTaskTemplate(index + 1)
		if err := s.DB.Model(&rows[index]).Updates(map[string]any{
			"name": target.Name, "type": target.Type, "description": target.Description,
			"prerequisite_json": target.PrerequisiteJSON, "objective_json": target.ObjectiveJSON,
			"reward_json": target.RewardJSON, "weight": target.Weight, "daily": target.Daily,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
