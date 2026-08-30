package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func TestCraftArtifactUsesTemplateSlotForGourd(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "craft-gourd-slot", "悬葫真人")
	template := model.ArtifactTemplate{
		Code: "craft_gourd_slot_test", Name: "青冥青莲化劫吞天葫", Type: "宝葫", Archetype: "宝葫", Slot: "腰佩",
		MaterialsJSON: `{"仙府材料":3}`, AttributeJSON: `{"mana":20}`, MaxLevel: 20, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "artifact_recipe."+template.Code, "learned", nil); err != nil {
		t.Fatal(err)
	}
	material, err := game.itemByName("仙府材料")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, material.ID, 3); err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.craftArtifact(&player, template.Name)
	if err != nil || !handled || !strings.Contains(result.Title, "出炉") || !strings.Contains(result.Content, "槽位：腰佩 · 器型：宝葫") {
		t.Fatalf("craft gourd: handled=%v err=%v result=%+v", handled, err, result)
	}
	var owned model.PlayerArtifact
	if err := store.DB.Where("player_id = ? AND template_id = ?", player.ID, template.ID).First(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if owned.Slot != "腰佩" {
		t.Fatalf("crafted gourd slot=%q want 腰佩", owned.Slot)
	}
	list, handled, err := game.viewArtifacts(&player)
	if err != nil || !handled || !strings.Contains(list.Content, template.Name+"【腰佩 · 宝葫】") || strings.Contains(list.Content, template.Name+"【本命法器") {
		t.Fatalf("artifact list does not expose real gourd slot: handled=%v err=%v result=%+v", handled, err, list)
	}
	bag, handled, err := game.equipmentBag(&player, "1")
	if err != nil || !handled || !strings.Contains(bag.Content, "槽位：腰佩 · 器型：宝葫") {
		t.Fatalf("equipment bag does not expose real gourd slot: handled=%v err=%v result=%+v", handled, err, bag)
	}
	item, handled, err := game.itemDetails(template.Name)
	if err != nil || !handled || !strings.Contains(item.Content, "槽位：腰佩 · 器型：宝葫") || strings.Contains(item.Title, "不存在") {
		t.Fatalf("item lookup does not route artifact to its real slot: handled=%v err=%v result=%+v", handled, err, item)
	}
	query, handled, err := game.queryEverything(&player, template.Name)
	if err != nil || !handled || !strings.Contains(query.Content, "【装备】"+template.Name) || !strings.Contains(query.Content, "槽位：腰佩 · 器型：宝葫") {
		t.Fatalf("global query does not expose real gourd slot: handled=%v err=%v result=%+v", handled, err, query)
	}
}

func TestCraftArtifactUsesTemplateSlotForFlyingBoat(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "craft-flying-boat-slot", "御舟真人")
	template := model.ArtifactTemplate{
		Code: "craft_flying_boat_slot_test", Name: "青冥青莲逐星御风舟", Type: "飞舟", Archetype: "飞舟", Slot: "灵靴",
		MaterialsJSON: `{"仙府材料":3}`, AttributeJSON: `{"speed":20}`, MaxLevel: 20, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "artifact_recipe."+template.Code, "learned", nil); err != nil {
		t.Fatal(err)
	}
	material, err := game.itemByName("仙府材料")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, material.ID, 3); err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.craftArtifact(&player, template.Name)
	if err != nil || !handled || !strings.Contains(result.Content, "槽位：灵靴 · 器型：飞舟") {
		t.Fatalf("craft flying boat output: handled=%v err=%v result=%+v", handled, err, result)
	}
	var owned model.PlayerArtifact
	if err := store.DB.Where("player_id = ? AND template_id = ?", player.ID, template.ID).First(&owned).Error; err != nil || owned.Slot != "灵靴" {
		t.Fatalf("crafted flying boat slot=%q want 灵靴 err=%v", owned.Slot, err)
	}
}

func TestForgeEquipmentReportsRealTransitionAndDoesNotChargeAtLimit(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "forge-transition", "百炼验收使")
	template := model.ArtifactTemplate{Code: "forge_transition_test", Name: "百炼验收剑", Type: "仙剑", Slot: "本命法器", Archetype: "仙剑", AttributeJSON: `{"attack":10}`, MaxLevel: 40, Enabled: true}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: "本命法器", ForgeLevel: 8}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	iron, err := game.itemByName("玄铁")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, iron.ID, 38); err != nil {
		t.Fatal(err)
	}

	result, handled, err := game.forgeEquipment(&player, artifact.Name)
	if err != nil || !handled || !strings.Contains(result.Content, "槽位：本命法器 · 器型：仙剑") || !strings.Contains(result.Content, "锻造：8 → 9") || strings.Contains(result.Content, "锻造：9 → 9") {
		t.Fatalf("forge transition output: handled=%v err=%v result=%+v", handled, err, result)
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil || artifact.ForgeLevel != 9 || game.itemQuantity(player.ID, iron.ID) != 20 {
		t.Fatalf("forge settlement artifact=%+v iron=%d err=%v", artifact, game.itemQuantity(player.ID, iron.ID), err)
	}
	if err := store.DB.Model(&model.PlayerArtifact{}).Where("id = ?", artifact.ID).Update("forge_level", 30).Error; err != nil {
		t.Fatal(err)
	}
	before := game.itemQuantity(player.ID, iron.ID)
	limit, handled, err := game.forgeEquipment(&player, artifact.Name)
	if err != nil || !handled || !strings.Contains(limit.Title, "上限") || !strings.Contains(limit.Content, "未扣除玄铁") {
		t.Fatalf("forge cap response: handled=%v err=%v result=%+v", handled, err, limit)
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil || artifact.ForgeLevel != 30 || game.itemQuantity(player.ID, iron.ID) != before {
		t.Fatalf("forge cap charged or mutated: level=%d iron=%d before=%d err=%v", artifact.ForgeLevel, game.itemQuantity(player.ID, iron.ID), before, err)
	}
}

func TestArtifactSlotMigrationCollisionKeepsStrongestAndRecalculatesStats(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "artifact-slot-collision", "归槽真人")
	gourdTemplate := model.ArtifactTemplate{Code: "slot_collision_gourd", Name: "归槽吞天葫", Type: "宝葫", Slot: "腰佩", Archetype: "宝葫", AttributeJSON: `{"attack":10}`, Enabled: true}
	beltTemplate := model.ArtifactTemplate{Code: "slot_collision_belt", Name: "归槽旧玉佩", Type: "灵佩", Slot: "腰佩", Archetype: "灵佩", AttributeJSON: `{"attack":4}`, Enabled: true}
	if err := store.DB.Create(&gourdTemplate).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&beltTemplate).Error; err != nil {
		t.Fatal(err)
	}
	gourd := model.PlayerArtifact{PlayerID: player.ID, TemplateID: gourdTemplate.ID, Name: gourdTemplate.Name, Level: 3, Quality: "凡品", Slot: "腰佩", ForgeLevel: 2, Inscription: "玄水护魂纹", Equipped: true}
	belt := model.PlayerArtifact{PlayerID: player.ID, TemplateID: beltTemplate.ID, Name: beltTemplate.Name, Level: 1, Quality: "凡品", Slot: "腰佩", Equipped: true}
	if err := store.DB.Create(&gourd).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&belt).Error; err != nil {
		t.Fatal(err)
	}
	gourdStats, beltStats := game.equipmentStats(gourd), game.equipmentStats(belt)
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"physical_attack": player.PhysicalAttack + gourdStats.Attack + gourdStats.Power + beltStats.Attack + beltStats.Power,
		"magic_attack":    player.MagicAttack + gourdStats.Attack + gourdStats.Power + beltStats.Attack + beltStats.Power,
	}).Error; err != nil {
		t.Fatal(err)
	}
	marker := model.PlayerValue{PlayerID: player.ID, Key: storage.ArtifactSlotSyncMigrationKey, Value: "true"}
	if err := store.DB.Create(&marker).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.repairMigratedArtifactSlots(); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.First(&gourd, gourd.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.First(&belt, belt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !gourd.Equipped || belt.Equipped || gourd.ForgeLevel != 2 || gourd.Inscription != "玄水护魂纹" {
		t.Fatalf("collision repair damaged artifacts: gourd=%+v belt=%+v", gourd, belt)
	}
	latest, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.PhysicalAttack != player.PhysicalAttack+gourdStats.Attack+gourdStats.Power || latest.MagicAttack != player.MagicAttack+gourdStats.Attack+gourdStats.Power {
		t.Fatalf("collision stats not recalculated: latest=%+v base physical=%d magic=%d gourd=%+v", latest, player.PhysicalAttack, player.MagicAttack, gourdStats)
	}
	overview, handled, err := game.equipmentOverview(&latest)
	if err != nil || !handled || !strings.Contains(overview.Content, "腰佩："+gourd.Name) || strings.Contains(overview.Content, "本命法器："+gourd.Name) {
		t.Fatalf("migrated equipment overview has wrong slot: handled=%v err=%v result=%+v", handled, err, overview)
	}
	var markerCount int64
	if err := store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, storage.ArtifactSlotSyncMigrationKey).Count(&markerCount).Error; err != nil || markerCount != 0 {
		t.Fatalf("migration marker count=%d err=%v", markerCount, err)
	}
}
