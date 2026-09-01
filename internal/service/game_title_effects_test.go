package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestWornAlchemyTitleChangesDisplayedAndActualSuccessRate(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "alchemy-title-player", "丹衡真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"title": "丹道圣手", "luck": initialPlayerLuck,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for name, quantity := range map[string]int64{"凝露草": 2, "灵茶": 1} {
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼药 回元散"))
	if err != nil || !handled {
		t.Fatalf("alchemy with title: handled=%v err=%v result=%+v", handled, err, result)
	}
	if !strings.Contains(result.Content, "基础与丹房75.0% · 称号+15.0% · 运气+0.0% · 实际90.0%") {
		t.Fatalf("alchemy title was not included in the real rate: %+v", result)
	}
}

func TestLegacyPillTitleAliasAndUnwornTitleRules(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "legacy-pill-title-player", "旧丹真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("title", "炼丹大师").Error; err != nil {
		t.Fatal(err)
	}
	player, _ = game.players.Get(player.ID)
	if got := game.activeTitleGameplayPercent(&player, "alchemy_percent", "pill_percent"); got != 20 {
		t.Fatalf("legacy pill title bonus=%v want=20", got)
	}
	player.Title = ""
	if got := game.activeTitleGameplayPercent(&player, "alchemy_percent", "pill_percent"); got != 0 {
		t.Fatalf("unworn title still applied bonus=%v", got)
	}
}

func TestCoupleTribulationModeKeepsMedicineAndTitleBonuses(t *testing.T) {
	mode := tribulationModeText(true, activeItemBonus{TribulationRate: .08, DaoHeart: 5}, .15)
	for _, expected := range []string{"仙侣共渡", "丹药护劫+8.0%", "称号护劫+15.0%"} {
		if !strings.Contains(mode, expected) {
			t.Fatalf("tribulation mode %q missing %q", mode, expected)
		}
	}
}

func TestPetEvolutionUsesTemplateRequirementAndOnlyAppliesOnce(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "single-pet-evolution-player", "御衡真人")
	template := model.PetTemplate{
		Code: "single_evolution_pet", Name: "御衡灵鹿", InitialPower: 100, GrowthPerLevel: 10,
		EvolutionCondition: `{"loyalty":82,"level":7}`, EvolutionTarget: "御衡仙鹿", Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	attack, defense, health := model.PetStatsAtLevel(template, 7, false)
	pet := model.Pet{
		PlayerID: player.ID, Name: template.Name, Species: template.Name, Rarity: "凡品", Level: 7,
		Attack: attack, Defense: defense, Health: health, Loyalty: 82, Active: true,
	}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("active_pet_id", pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	player, _ = game.players.Get(player.ID)
	first, handled, err := game.evolvePet(&player)
	if err != nil || !handled || !strings.Contains(first.Content, "进化次数：1/1") {
		t.Fatalf("first evolution: handled=%v err=%v result=%+v", handled, err, first)
	}
	var evolved model.Pet
	if err := store.DB.First(&evolved, pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantAttack, wantDefense, wantHealth := model.PetStatsAtLevel(template, pet.Level, true)
	if evolved.Evolution != 1 || evolved.Attack != wantAttack || evolved.Defense != wantDefense || evolved.Health != wantHealth {
		t.Fatalf("evolved pet=%+v want stats=%d/%d/%d", evolved, wantAttack, wantDefense, wantHealth)
	}
	second, handled, err := game.evolvePet(&player)
	if err != nil || !handled || !strings.Contains(second.Title, "已经进化") {
		t.Fatalf("second evolution was not blocked: handled=%v err=%v result=%+v", handled, err, second)
	}
	var unchanged model.Pet
	if err := store.DB.First(&unchanged, pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Evolution != evolved.Evolution || unchanged.Attack != evolved.Attack || unchanged.Defense != evolved.Defense || unchanged.Health != evolved.Health {
		t.Fatalf("second evolution changed stats: first=%+v second=%+v", evolved, unchanged)
	}
}
