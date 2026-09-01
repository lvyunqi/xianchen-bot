package service

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func grantCustomizationVoucher(t *testing.T, game *Game, playerID uint, name string, quantity int64) model.Item {
	t.Helper()
	item, err := game.itemByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(playerID, item.ID, quantity); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestCustomTitleHasFixedStatsAndRenameDoesNotStack(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "custom-title-player", "玉册真人")
	voucher := grantCustomizationVoucher(t, game, player.ID, titleCustomizationVoucher, 2)
	before, _ := game.players.Get(player.ID)

	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制称号 云台道君"))
	if err != nil || !handled || !strings.Contains(first.Content, "攻击+20") {
		t.Fatalf("first title customization: handled=%v err=%v result=%+v", handled, err, first)
	}
	afterFirst, _ := game.players.Get(player.ID)
	if afterFirst.PhysicalAttack != before.PhysicalAttack+20 || afterFirst.MagicAttack != before.MagicAttack+20 ||
		afterFirst.PhysicalDefense != before.PhysicalDefense+12 || afterFirst.MagicDefense != before.MagicDefense+12 ||
		afterFirst.MaxHealth != before.MaxHealth+120 || afterFirst.MaxMana != before.MaxMana+60 {
		t.Fatalf("fixed title stats not applied: before=%+v after=%+v", before, afterFirst)
	}
	if game.itemQuantity(player.ID, voucher.ID) != 1 {
		t.Fatal("first title customization did not consume exactly one voucher")
	}

	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制称号 星河道君"))
	if err != nil || !handled || !strings.Contains(second.Content, "不会重复叠加") {
		t.Fatalf("title rename: handled=%v err=%v result=%+v", handled, err, second)
	}
	afterSecond, _ := game.players.Get(player.ID)
	if afterSecond.PhysicalAttack != afterFirst.PhysicalAttack || afterSecond.MagicAttack != afterFirst.MagicAttack ||
		afterSecond.PhysicalDefense != afterFirst.PhysicalDefense || afterSecond.MagicDefense != afterFirst.MagicDefense ||
		afterSecond.MaxHealth != afterFirst.MaxHealth || afterSecond.MaxMana != afterFirst.MaxMana {
		t.Fatalf("title rename stacked stats: first=%+v second=%+v", afterFirst, afterSecond)
	}
	if game.itemQuantity(player.ID, voucher.ID) != 0 {
		t.Fatal("title rename did not consume exactly one voucher")
	}
	var title model.Title
	if err := store.DB.Where("code = ?", "custom_title_player_"+customTestUintText(player.ID)).First(&title).Error; err != nil {
		t.Fatal(err)
	}
	var stats equipmentStats
	if json.Unmarshal([]byte(title.AttributeBonus), &stats) != nil || stats != customTitleStats || !game.playerValueExists(player.ID, titleUnlockKey(title)) {
		t.Fatalf("custom title catalog entry mismatch: title=%+v stats=%+v", title, stats)
	}
}

func TestLegacyUnwornCustomTitleReceivesFreeAttributeUpgrade(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "legacy-title-player", "旧契真人")
	legacy := model.Title{Code: "custom_title_player_" + customTestUintText(player.ID), Name: "旧梦尊号", Condition: "旧版付费定制", AttributeBonus: `{}`, Type: "定制", Enabled: true}
	if err := store.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&model.ContentReview{Type: "称号定制", PlayerID: player.ID, PlayerName: player.DaoName, Content: "无称号 → 旧梦尊号", Status: "已通过"}).Error; err != nil {
		t.Fatal(err)
	}
	before, _ := game.players.Get(player.ID)
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制称号 旧梦尊号"))
	if err != nil || !handled || !strings.Contains(result.Content, "旧版已付费称号补发") {
		t.Fatalf("legacy title upgrade: handled=%v err=%v result=%+v", handled, err, result)
	}
	after, _ := game.players.Get(player.ID)
	if after.Title != legacy.Name || after.PhysicalAttack != before.PhysicalAttack+20 || after.MaxHealth != before.MaxHealth+120 {
		t.Fatalf("legacy title upgrade did not apply once: before=%+v after=%+v", before, after)
	}
	if game.hasCustomizationVoucher(player.ID, titleCustomizationVoucher) {
		t.Fatal("legacy upgrade unexpectedly created or required a voucher")
	}
}

func TestCustomPetBonusSurvivesEvolutionAndReleaseCleansMarker(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "custom-pet-player", "山海真人")
	var template model.PetTemplate
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&template).Error; err != nil {
		t.Fatal(err)
	}
	attack, defense, health := model.PetStatsAtLevel(template, 5, false)
	pet := model.Pet{PlayerID: player.ID, Name: "青岚", Species: template.Name, Rarity: "凡品", Level: 5, Attack: attack, Defense: defense, Health: health, Loyalty: 100, Active: true}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("active_pet_id", pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	voucher := grantCustomizationVoucher(t, game, player.ID, petCustomizationVoucher, 2)

	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制灵兽 青岚=照月"))
	if err != nil || !handled || !strings.Contains(first.Content, "首次血契增益") {
		t.Fatalf("first pet customization: handled=%v err=%v result=%+v", handled, err, first)
	}
	var firstPet model.Pet
	if err := store.DB.First(&firstPet, pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantAttackBonus := int64(math.Ceil(float64(attack) * .10))
	wantDefenseBonus := int64(math.Ceil(float64(defense) * .10))
	wantHealthBonus := int64(math.Ceil(float64(health) * .10))
	if firstPet.Attack != attack+wantAttackBonus || firstPet.Defense != defense+wantDefenseBonus || firstPet.Health != health+wantHealthBonus {
		t.Fatalf("pet blood pact mismatch: got=%+v bonuses=%d/%d/%d", firstPet, wantAttackBonus, wantDefenseBonus, wantHealthBonus)
	}

	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制灵兽 照月=曦光"))
	if err != nil || !handled || !strings.Contains(second.Content, "不重复叠加") {
		t.Fatalf("pet rename: handled=%v err=%v result=%+v", handled, err, second)
	}
	var renamed model.Pet
	_ = store.DB.First(&renamed, pet.ID).Error
	if renamed.Attack != firstPet.Attack || renamed.Defense != firstPet.Defense || renamed.Health != firstPet.Health || game.itemQuantity(player.ID, voucher.ID) != 0 {
		t.Fatalf("pet rename stacked or voucher mismatch: first=%+v renamed=%+v vouchers=%d", firstPet, renamed, game.itemQuantity(player.ID, voucher.ID))
	}

	evolvedResult, handled, err := game.Execute("group", player.AccountID, mustParse(t, "进化"))
	if err != nil || !handled || !strings.Contains(evolvedResult.Title, "灵兽进化") {
		t.Fatalf("pet evolution: handled=%v err=%v result=%+v", handled, err, evolvedResult)
	}
	var evolved model.Pet
	_ = store.DB.First(&evolved, pet.ID).Error
	baseAttack, baseDefense, baseHealth := model.PetStatsAtLevel(template, 5, true)
	if evolved.Attack != baseAttack+wantAttackBonus || evolved.Defense != baseDefense+wantDefenseBonus || evolved.Health != baseHealth+wantHealthBonus {
		t.Fatalf("pet evolution lost or duplicated custom bonus: got=%+v base=%d/%d/%d", evolved, baseAttack, baseDefense, baseHealth)
	}

	released, handled, err := game.Execute("group", player.AccountID, mustParse(t, "放生 曦光"))
	if err != nil || !handled || !strings.Contains(released.Title, "放归") {
		t.Fatalf("release custom pet: handled=%v err=%v result=%+v", handled, err, released)
	}
	var markerCount int64
	key := "custom.pet." + customTestUintText(pet.ID) + ".bonus"
	if err := store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, key).Count(&markerCount).Error; err != nil || markerCount != 0 {
		t.Fatalf("released pet marker count=%d err=%v", markerCount, err)
	}
}

func TestCustomArtifactAndMansionBonusesOnlyApplyOnce(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "custom-object-player", "洞天器主")
	var template model.ArtifactTemplate
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: "青冥法剑", Level: 1, Quality: "凡品", Slot: template.Slot}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	mansion := model.Mansion{PlayerID: player.ID, Name: "清修洞府", Level: 2, FarmLevel: 3, FormationLevel: 1, BeastRoomLevel: 2, WarehouseLevel: 3, Prosperity: 50}
	if err := store.DB.Create(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	artifactVoucher := grantCustomizationVoucher(t, game, player.ID, artifactCustomizationVoucher, 2)
	mansionVoucher := grantCustomizationVoucher(t, game, player.ID, mansionCustomizationVoucher, 2)

	for _, command := range []string{"定制法宝 青冥法剑=照夜法剑", "定制法宝 照夜法剑=流光法剑"} {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, command))
		if err != nil || !handled || !strings.Contains(result.Title, "定制完成") {
			t.Fatalf("%s: handled=%v err=%v result=%+v", command, handled, err, result)
		}
	}
	var updatedArtifact model.PlayerArtifact
	_ = store.DB.First(&updatedArtifact, artifact.ID).Error
	if updatedArtifact.StarLevel != 1 || game.itemQuantity(player.ID, artifactVoucher.ID) != 0 {
		t.Fatalf("artifact customization stacked: artifact=%+v vouchers=%d", updatedArtifact, game.itemQuantity(player.ID, artifactVoucher.ID))
	}

	for _, command := range []string{"定制仙府 星落洞天", "定制仙府 云隐洞天"} {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, command))
		if err != nil || !handled || !strings.Contains(result.Title, "定制完成") {
			t.Fatalf("%s: handled=%v err=%v result=%+v", command, handled, err, result)
		}
	}
	var updatedMansion model.Mansion
	_ = store.DB.First(&updatedMansion, mansion.ID).Error
	if updatedMansion.Prosperity != 250 || updatedMansion.FormationLevel != 2 || updatedMansion.BeastRoomLevel != 3 || updatedMansion.WarehouseLevel != 4 ||
		game.itemQuantity(player.ID, mansionVoucher.ID) != 0 {
		t.Fatalf("mansion customization stacked: mansion=%+v vouchers=%d", updatedMansion, game.itemQuantity(player.ID, mansionVoucher.ID))
	}
}

func customTestUintText(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
