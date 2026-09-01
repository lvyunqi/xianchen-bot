package storage

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

func TestLegacyNumericCatalogUpgradeMergesUniqueNames(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "legacy-upgrade.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	seedName := cultivationSeedName(1)
	itemCode := "catalog_item_" + seedName
	artifactCode := "catalog_artifact_" + seedName
	artifactName := cultivationArtifactName(1, seedName)
	var item model.Item
	if err := store.DB.Where("code = ?", itemCode).First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&item).Updates(map[string]any{"code": "catalog_item_001", "name": "百宝图鉴·001"}).Error; err != nil {
		t.Fatal(err)
	}
	var artifact model.ArtifactTemplate
	if err := store.DB.Where("code = ?", artifactCode).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&artifact).Updates(map[string]any{"code": "catalog_artifact_001", "name": "诸天法宝·001"}).Error; err != nil {
		t.Fatal(err)
	}
	owned := model.PlayerArtifact{PlayerID: 987654, TemplateID: artifact.ID, Name: "旧库法宝", Level: 1, Quality: "凡品"}
	if err := store.DB.Create(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.21.legacy").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Delete(&model.SensitiveWord{}, "1 = 1").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatalf("legacy database failed to open: %v", err)
	}
	defer upgraded.Close()
	var count int64
	if err := upgraded.DB.Model(&model.Item{}).Where("code = ?", itemCode).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canonical item count=%d err=%v", count, err)
	}
	if err := upgraded.DB.Model(&model.ArtifactTemplate{}).Where("code = ? AND name = ?", artifactCode, artifactName).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canonical artifact count=%d err=%v", count, err)
	}
	if err := upgraded.DB.Model(&model.ArtifactTemplate{}).Where("name = ?", "诸天法宝·001").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("legacy artifact count=%d err=%v", count, err)
	}
	var migratedOwned model.PlayerArtifact
	if err := upgraded.DB.First(&migratedOwned, owned.ID).Error; err != nil {
		t.Fatal(err)
	}
	var canonical model.ArtifactTemplate
	if err := upgraded.DB.Where("code = ?", artifactCode).First(&canonical).Error; err != nil {
		t.Fatal(err)
	}
	if migratedOwned.TemplateID != canonical.ID {
		t.Fatalf("owned artifact template=%d, canonical=%d", migratedOwned.TemplateID, canonical.ID)
	}
}

func TestLegacyCreatedSkillMigrationKeepsSkillPrivateAndRestoresAuthor(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "legacy-created-skill.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	creator := model.Player{AccountID: "legacy-skill-author", DaoName: "旧卷真人", RealmName: "炼气", RealmLevel: 1}
	if err := store.DB.Create(&creator).Error; err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{Name: "旧卷观心诀", Type: "神魂", Rarity: "自创", RealmRequired: "炼气", EffectJSON: `{"mana":20}`}
	if err := store.DB.Create(&skill).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&model.PlayerSkill{PlayerID: creator.ID, SkillID: skill.ID, Level: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Where("skill_id = ?", skill.ID).Delete(&model.SkillPublication{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.23.257.46").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var publication model.SkillPublication
	if err := upgraded.DB.Where("skill_id = ?", skill.ID).First(&publication).Error; err != nil {
		t.Fatal(err)
	}
	if publication.CreatorPlayerID != creator.ID || publication.CreatorName != creator.DaoName || publication.Published || publication.PublishedAt != nil {
		t.Fatalf("migrated publication=%+v", publication)
	}
}

func TestLegacyNullFertilizerStateIsNormalized(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "legacy-fertilizer-state.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mansion := model.Mansion{PlayerID: 7654321, Name: "旧田洞天", Level: 1, FarmLevel: 1}
	if err := store.DB.Create(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	crop := model.MansionCrop{MansionID: mansion.ID, Plot: 1, Quantity: 1, PlantedAt: time.Now(), ReadyAt: time.Now().Add(time.Hour)}
	if err := store.DB.Create(&crop).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Exec("UPDATE mansion_crops SET fertilized = NULL WHERE id = ?", crop.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.23.257.47").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var nullCount, unfertilized int64
	if err := upgraded.DB.Model(&model.MansionCrop{}).Where("id = ? AND fertilized IS NULL", crop.ID).Count(&nullCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := upgraded.DB.Model(&model.MansionCrop{}).Where("id = ? AND fertilized = ?", crop.ID, false).Count(&unfertilized).Error; err != nil {
		t.Fatal(err)
	}
	if nullCount != 0 || unfertilized != 1 {
		t.Fatalf("fertilizer migration left null=%d unfertilized=%d", nullCount, unfertilized)
	}
}

func TestLegacyPlayerCultivationBackfillsLevelWithoutLosingData(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "legacy-player-level.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	legacy := model.Player{
		AccountID: "legacy-level-player", DaoName: "旧阶真人", RealmName: "筑基", RealmLevel: 4,
		Level: 1, Experience: 0, Cultivation: 500, SilverCoins: 7788, SpiritStones: 321, Location: "旧州古道",
		Health: 80, MaxHealth: 100, Mana: 40, MaxMana: 50,
		PhysicalAttack: 10, MagicAttack: 11, PhysicalDefense: 5, MagicDefense: 6,
		Agility: 10, Strength: 10, Constitution: 10, Spirit: 10, Perception: 10, Willpower: 10,
	}
	if err := store.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	expected := legacy
	model.ApplyPlayerExperience(&expected, expected.Cultivation)
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.23.257.48").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var actual model.Player
	if err := upgraded.DB.First(&actual, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if actual.Level != expected.Level || actual.Experience != expected.Experience || actual.MaxHealth != expected.MaxHealth || actual.MaxMana != expected.MaxMana || actual.PhysicalAttack != expected.PhysicalAttack || actual.Agility != expected.Agility {
		t.Fatalf("legacy level backfill actual=%+v expected=%+v", actual, expected)
	}
	if actual.Cultivation != legacy.Cultivation || actual.SilverCoins != legacy.SilverCoins || actual.SpiritStones != legacy.SpiritStones || actual.Location != legacy.Location || actual.RealmName != legacy.RealmName || actual.RealmLevel != legacy.RealmLevel {
		t.Fatalf("legacy data was changed: actual=%+v legacy=%+v", actual, legacy)
	}
}

func TestBalanceMigrationRepairsRepeatedPetEvolutionAndClampsCreatedSkill(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "balance-migration.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	player := model.Player{
		AccountID: "balance-migration-player", DaoName: "守衡真人", RealmName: "炼气", RealmLevel: 1,
		Level: 1, Health: 100, MaxHealth: 100, Mana: 50, MaxMana: 50, CombatPower: 9_000_000_000_000,
	}
	if err := store.DB.Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	template := model.PetTemplate{
		Code: "balance_migration_pet", Name: "守衡云兽", InitialPower: 2842, GrowthPerLevel: 2,
		EvolutionCondition: `{"loyalty":86,"level":23}`, EvolutionTarget: "守衡天兽", Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	pet := model.Pet{
		PlayerID: player.ID, Name: template.Name, Species: template.Name, Rarity: "灵品", Level: 5,
		Attack: 9_198_698_713_111, Defense: 4_596_979_232_431, Health: 91_995_857_202_253,
		Loyalty: 100, Evolution: 54, Active: true,
	}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{
		Name: "守衡妙法", Type: "剑道", Rarity: "自创", RealmRequired: "炼气",
		EffectJSON: `{"physical_attack":5445,"speed":907,"crit_rate":0.016}`,
	}
	if err := store.DB.Create(&skill).Error; err != nil {
		t.Fatal(err)
	}
	learned := model.PlayerSkill{PlayerID: player.ID, SkillID: skill.ID, Level: 7, Mastery: 321, Equipped: true}
	if err := store.DB.Create(&learned).Error; err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	publication := model.SkillPublication{
		SkillID: skill.ID, CreatorPlayerID: player.ID, CreatorName: player.DaoName,
		Published: true, PublishedAt: &publishedAt,
	}
	if err := store.DB.Create(&publication).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&player).Updates(map[string]any{"active_pet_id": pet.ID, "current_skill_id": skill.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.23.257.49").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var repairedPet model.Pet
	if err := upgraded.DB.First(&repairedPet, pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantAttack, wantDefense, wantHealth := model.PetStatsAtLevel(template, pet.Level, false)
	if repairedPet.Evolution != 0 || repairedPet.Rarity != "凡品" || repairedPet.Attack != wantAttack || repairedPet.Defense != wantDefense || repairedPet.Health != wantHealth {
		t.Fatalf("repaired pet=%+v want evolution=0 rarity=凡品 stats=%d/%d/%d", repairedPet, wantAttack, wantDefense, wantHealth)
	}
	var repairedSkill model.Skill
	if err := upgraded.DB.First(&repairedSkill, skill.ID).Error; err != nil {
		t.Fatal(err)
	}
	var effect map[string]float64
	if err := json.Unmarshal([]byte(repairedSkill.EffectJSON), &effect); err != nil {
		t.Fatal(err)
	}
	if effect["physical_attack"] != 480 || effect["speed"] != 80 || effect["crit_rate"] != .016 {
		t.Fatalf("repaired skill effect=%v", effect)
	}
	var retainedSkill model.PlayerSkill
	if err := upgraded.DB.First(&retainedSkill, learned.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retainedSkill.Level != learned.Level || retainedSkill.Mastery != learned.Mastery || !retainedSkill.Equipped {
		t.Fatalf("learned skill data changed: %+v", retainedSkill)
	}
	var retainedPublication model.SkillPublication
	if err := upgraded.DB.First(&retainedPublication, publication.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retainedPublication.CreatorPlayerID != player.ID || retainedPublication.CreatorName != player.DaoName || !retainedPublication.Published || retainedPublication.PublishedAt == nil {
		t.Fatalf("publication data changed: %+v", retainedPublication)
	}
	var markerCount int64
	if err := upgraded.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, migrationCombatPowerSyncKey).Count(&markerCount).Error; err != nil || markerCount != 1 {
		t.Fatalf("combat-power sync marker count=%d err=%v", markerCount, err)
	}
}

func TestLuckMigrationCapsPlayersAndUnreachablePrerequisites(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "luck-migration.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player := model.Player{AccountID: "legacy-luck", DaoName: "旧运真人", Gender: "", Luck: 88}
	if err := store.DB.Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	var event model.Event
	if err := store.DB.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&event).Update("condition_json", `{"minimum_luck":80}`).Error; err != nil {
		t.Fatal(err)
	}
	var encounter model.ImmortalEncounterConfig
	if err := store.DB.First(&encounter).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&encounter).Update("prerequisite", `{"minimum_luck":75}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.normalizeLuckSystem(); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	if player.Luck != 50 || player.Gender != "未定" {
		t.Fatalf("migrated player luck=%d gender=%q", player.Luck, player.Gender)
	}
	for _, test := range []struct {
		table, column string
		id            uint
	}{{"events", "condition_json", event.ID}, {"immortal_encounter_configs", "prerequisite", encounter.ID}} {
		var raw string
		if err := store.DB.Table(test.table).Select(test.column).Where("id = ?", test.id).Scan(&raw).Error; err != nil {
			t.Fatal(err)
		}
		values := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			t.Fatal(err)
		}
		if values["minimum_luck"].(float64) != 50 {
			t.Fatalf("%s prerequisite not capped: %s", test.table, raw)
		}
	}
	var title model.Title
	if err := store.DB.Where("code = ?", "title_lucky").First(&title).Error; err != nil || title.Condition != "运气达到50" {
		t.Fatalf("lucky title=%+v err=%v", title, err)
	}
}

func TestUpgradeReopensOneExtendedDaoistConfigWhenAllAreClosed(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "closed-root-configs.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SpiritualRootEvolutionConfig{}).Where("1 = 1").Update("status", "停用").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "legacy.closed.configs").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var enabled int64
	if err := upgraded.DB.Model(&model.SpiritualRootEvolutionConfig{}).Where("status = ?", "启用").Count(&enabled).Error; err != nil || enabled < 1 {
		t.Fatalf("enabled spiritual root configs=%d err=%v", enabled, err)
	}
	var leaked int64
	if err := upgraded.DB.Model(&model.SpiritualRootEvolutionConfig{}).Where("description LIKE ?", "%后台%").Count(&leaked).Error; err != nil || leaked != 0 {
		t.Fatalf("player-facing management terminology remained in root configs: count=%d err=%v", leaked, err)
	}
}

func TestUpgradeRepairsBasicAlchemyOutputsAndMaterialGuidance(t *testing.T) {
	directory := t.TempDir()
	cfg := config.Runtime(directory)
	cfg.Database.DSN = filepath.Join(directory, "alchemy-upgrade.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.AlchemyRecipe{}).Where("code = ?", "recipe_spirit").Updates(map[string]any{
		"materials_json": `{"灵果":2,"灵茶":1}`, "output_name": "灵果", "output_item_id": 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.AlchemyRecipe{}).Where("code = ?", "recipe_recovery").Updates(map[string]any{
		"materials_json": `{"灵茶":1}`, "output_name": "仙露", "output_item_id": 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Item{}).Where("code = ?", "item_formation_stone").Updates(map[string]any{
		"description": "布置护府阵法所需。", "effect_type": "仙府",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.22.257.33").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	for code, expectedOutput := range map[string]string{"recipe_spirit": "聚灵丹", "recipe_recovery": "回元散"} {
		var recipe model.AlchemyRecipe
		if err := upgraded.DB.Where("code = ?", code).First(&recipe).Error; err != nil {
			t.Fatal(err)
		}
		if recipe.OutputName != expectedOutput || recipe.OutputItemID == 0 {
			t.Fatalf("recipe %s not repaired: %+v", code, recipe)
		}
	}
	var spiritRecipe model.AlchemyRecipe
	if err := upgraded.DB.Where("code = ?", "recipe_spirit").First(&spiritRecipe).Error; err != nil || !strings.Contains(spiritRecipe.MaterialsJSON, "赤焰草") {
		t.Fatalf("spirit recipe lacks fire herb: recipe=%+v err=%v", spiritRecipe, err)
	}
	var stone model.Item
	if err := upgraded.DB.Where("code = ?", "item_formation_stone").First(&stone).Error; err != nil || !strings.Contains(stone.Description, "引劫玉符") || stone.EffectType != "阵法材料" {
		t.Fatalf("formation stone guidance not repaired: item=%+v err=%v", stone, err)
	}
}

func TestWorldLeylineUpgradeAddsReincarnationAndOpeningRealmAccess(t *testing.T) {
	directory := t.TempDir()
	cfg := config.Runtime(directory)
	cfg.Database.DSN = filepath.Join(directory, "leyline-origin-upgrade.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	legacy := legacyWorldLeylineProfile(10)
	if err := store.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "system.schema_version").Update("value", "2026.07.23.legacy-leylines").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(cfg)
	if err != nil {
		t.Fatalf("upgrade legacy leylines: %v", err)
	}
	defer upgraded.Close()
	var migrated model.WorldLeyline
	if err := upgraded.DB.Where("code = ?", indexWorldLeylineCode(10)).First(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.Element != "轮回" || migrated.RequiredRootElement != "轮回" || migrated.MinimumRealmSequence != 1 || migrated.MinimumRealmLevel != 1 || migrated.LocationName != "青云山脚" {
		t.Fatalf("reincarnation leyline was not migrated to opening hub: %+v", migrated)
	}
	seenElements := map[string]bool{}
	seenNames := map[string]bool{}
	for index := 1; index <= 10; index++ {
		row := worldLeylineProfile(index)
		seenElements[row.Element] = true
		if seenNames[row.Name] {
			t.Fatalf("duplicate opening leyline name: %s", row.Name)
		}
		seenNames[row.Name] = true
		if row.MinimumRealmSequence != 1 || row.MinimumRealmLevel != 1 || row.LocationName != "青云山脚" {
			t.Fatalf("opening leyline %d is inaccessible: %+v", index, row)
		}
	}
	for _, element := range []string{"庚金", "乙木", "玄水", "离火", "厚土", "风灵", "雷灵", "冰魄", "时空", "轮回"} {
		if !seenElements[element] {
			t.Fatalf("opening hub lacks %s leyline", element)
		}
	}
}
