package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestSpiritualRootTransferCreatesProtectedPendingSeedAndAbsorbs(t *testing.T) {
	game, store := testGame(t)
	donor := registerPlayer(t, game, "root-lineage-donor", "传灯真人")
	target := registerPlayer(t, game, "root-lineage-target", "承露散人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", donor.ID).Update("spirit_stones", 1000).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(donor.ID, spiritualRootEvolutionKey("evolve"), 18); err != nil {
		t.Fatal(err)
	}
	essence, err := game.itemByName("灵根精粹")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(donor.ID, essence.ID, 2); err != nil {
		t.Fatal(err)
	}

	transferred, handled, err := game.Execute("group", donor.AccountID, mustParse(t, "灵传 @"+target.DaoName))
	if err != nil || !handled || !strings.Contains(transferred.Title, "传承道种已送达") || !strings.Contains(transferred.Content, spiritualRootStageName(5)) {
		t.Fatalf("root transfer: handled=%v err=%v result=%+v", handled, err, transferred)
	}
	pending, err := game.loadPendingSpiritualRoot(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Mode != "transfer" || pending.Result != donor.SpiritualRoot || pending.SourcePlayer != donor.DaoName || pending.SourceStage != 5 {
		t.Fatalf("pending transfer seed=%+v donor=%+v", pending, donor)
	}
	donorAfter, _ := game.players.Get(donor.ID)
	if donorAfter.SpiritualRoot != donor.SpiritualRoot || donorAfter.SpiritStones != 700 || game.itemQuantity(donor.ID, essence.ID) != 1 {
		t.Fatalf("donor settlement=%+v essence=%d", donorAfter, game.itemQuantity(donor.ID, essence.ID))
	}

	blocked, handled, err := game.Execute("group", donor.AccountID, mustParse(t, "灵传 @"+target.DaoName))
	if err != nil || !handled || !strings.Contains(blocked.Title, "已有待定道种") {
		t.Fatalf("pending overwrite guard: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	donorBlocked, _ := game.players.Get(donor.ID)
	if donorBlocked.SpiritStones != 700 || game.itemQuantity(donor.ID, essence.ID) != 1 {
		t.Fatal("blocked transfer consumed resources")
	}

	absorbed, handled, err := game.Execute("group", target.AccountID, mustParse(t, "吸收灵根"))
	if err != nil || !handled || !strings.Contains(absorbed.Title, "传承灵根吸收完成") || strings.Contains(absorbed.Content, "两道本源合炼") {
		t.Fatalf("absorb transferred seed: handled=%v err=%v result=%+v", handled, err, absorbed)
	}
	targetAfter, _ := game.players.Get(target.ID)
	if targetAfter.SpiritualRoot != donor.SpiritualRoot || targetAfter.CombatPower != calculateCombatPower(targetAfter) {
		t.Fatalf("target root/combat power=%+v", targetAfter)
	}
	if game.playerValueInt(target.ID, "spiritual_root.origin_power", 0) != 50 {
		t.Fatalf("inherited origin insight=%d", game.playerValueInt(target.ID, "spiritual_root.origin_power", 0))
	}
	if _, err := game.loadPendingSpiritualRoot(target.ID); err == nil {
		t.Fatal("pending transfer seed remained after absorption")
	}

	poorDonor := registerPlayer(t, game, "root-lineage-poor", "空囊真人")
	poorTarget := registerPlayer(t, game, "root-lineage-poor-target", "候灯散人")
	missing, handled, err := game.Execute("group", poorDonor.AccountID, mustParse(t, "灵传 @"+poorTarget.DaoName))
	if err != nil || !handled || !strings.Contains(missing.Title, "材料不足") {
		t.Fatalf("missing transfer materials: handled=%v err=%v result=%+v", handled, err, missing)
	}
	if _, err := game.loadPendingSpiritualRoot(poorTarget.ID); err == nil {
		t.Fatal("material failure created a pending transfer seed")
	}
}

func TestPetCaptureRequiresOneEncounterAndNeglectCanCauseBetrayal(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "pet-encounter-player", "御风兽师")
	food, err := game.itemByName("灵兽口粮")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, food.ID, 1); err != nil {
		t.Fatal(err)
	}
	var template model.PetTemplate
	if err := store.DB.Where("enabled = ?", true).First(&template).Error; err != nil {
		t.Fatal(err)
	}
	pending := pendingPetEncounter{TemplateID: template.ID, PetName: template.Name, Location: player.Location, StartedAt: time.Now()}
	encoded, _ := json.Marshal(pending)
	expires := time.Now().Add(10 * time.Minute)
	if err := game.setPlayerValue(player.ID, pendingPetEncounterKey, string(encoded), &expires); err != nil {
		t.Fatal(err)
	}
	beforeStamina, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "捕获"))
	if err != nil || !handled || !strings.Contains(first.Content, "捕获率") {
		t.Fatalf("capture encounter: handled=%v err=%v result=%+v", handled, err, first)
	}
	if game.itemQuantity(player.ID, food.ID) != 0 {
		t.Fatal("capture did not consume exactly one food")
	}
	afterStamina, _ := game.currentStamina(player.ID)
	if afterStamina != beforeStamina-5 {
		t.Fatalf("capture stamina=%d want=%d", afterStamina, beforeStamina-5)
	}
	if _, err := game.loadPendingPetEncounter(player.ID); err == nil {
		t.Fatal("capture encounter remained after one attempt")
	}
	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "捕获"))
	if err != nil || !handled || !strings.Contains(second.Title, "没有灵兽遭遇") {
		t.Fatalf("repeat capture without encounter: handled=%v err=%v result=%+v", handled, err, second)
	}
	afterSecond, _ := game.currentStamina(player.ID)
	if afterSecond != afterStamina {
		t.Fatal("capture without encounter consumed stamina")
	}

	neglectPlayer := registerPlayer(t, game, "pet-neglect-player", "忘饲真人")
	pet := model.Pet{PlayerID: neglectPlayer.ID, Name: template.Name, Species: template.Name, Rarity: "凡品", Level: 1, Attack: template.InitialPower, Defense: template.InitialPower / 2, Health: template.InitialPower * 10, Loyalty: 1, Active: true, SkillJSON: `[]`}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", neglectPlayer.ID).Update("active_pet_id", pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(neglectPlayer.ID, petCareKey(pet.ID), time.Now().Add(-72*time.Hour).Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	space, handled, err := game.Execute("group", neglectPlayer.AccountID, mustParse(t, "灵兽"))
	if err != nil || !handled || !strings.Contains(space.Content, "叛变") {
		t.Fatalf("pet betrayal event: handled=%v err=%v result=%+v", handled, err, space)
	}
	var remaining int64
	if err := store.DB.Model(&model.Pet{}).Where("id = ?", pet.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("betrayed pet count=%d err=%v", remaining, err)
	}
	neglectAfter, _ := game.players.Get(neglectPlayer.ID)
	if neglectAfter.ActivePetID != 0 || neglectAfter.CombatPower != calculateCombatPower(neglectAfter) {
		t.Fatalf("betrayal did not clear active pet/combat power: %+v", neglectAfter)
	}
}

func TestXianchenIntroductionIsDiscoverable(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "introduction-player", "初闻道者")
	expectations := map[string]string{"仙尘介绍": "核心规模", "游戏介绍": "每日循环", "世界观": "山河秩序"}
	contents := make(map[string]string, len(expectations))
	for command, anchor := range expectations {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, command))
		if err != nil || !handled || !strings.Contains(result.Content, anchor) {
			t.Fatalf("%s: handled=%v err=%v result=%+v", command, handled, err, result)
		}
		contents[command] = result.Content
	}
	if contents["仙尘介绍"] == contents["游戏介绍"] || contents["仙尘介绍"] == contents["世界观"] || contents["游戏介绍"] == contents["世界观"] {
		t.Fatal("introduction commands must return distinct content")
	}
}
