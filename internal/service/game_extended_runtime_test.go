package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func prepareExtendedTestPlayer(t *testing.T, game *Game, store *storage.Store, player model.Player) model.Player {
	t.Helper()
	err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"combat_power": 1_000_000, "physical_attack": 2_000, "magic_attack": 2_000,
		"physical_defense": 500, "magic_defense": 500, "max_health": 5_000, "health": 5_000,
		"max_mana": 5_000, "mana": 5_000, "spirit": 500, "perception": 500,
		"willpower": 500, "luck": 50, "dao_heart": 100, "immortal_affinity": 500,
		"merit": 10_000, "reputation": 10_000, "spirit_stones": 10_000_000,
	}).Error
	if err != nil {
		t.Fatal(err)
	}
	updated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func grantExtendedTestCosts(t *testing.T, game *Game, store *storage.Store, playerID uint, raw string, multiplier int64) {
	t.Helper()
	var costs map[string]int64
	if err := json.Unmarshal([]byte(raw), &costs); err != nil {
		t.Fatal(err)
	}
	for name, amount := range costs {
		amount *= multiplier
		if amount <= 0 {
			continue
		}
		if name == "灵石" {
			if err := store.DB.Model(&model.Player{}).Where("id = ?", playerID).Update("spirit_stones", gorm.Expr("spirit_stones + ?", amount)).Error; err != nil {
				t.Fatal(err)
			}
			continue
		}
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatalf("missing extended cost item %s: %v", name, err)
		}
		if err := game.players.AdjustItem(playerID, item.ID, amount); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAllExtendedCommandSlotsReachRuntimeHandlers(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-routing-player", "万法巡检使"))
	totalActions := 0
	for category, system := range extendedSystems {
		baseID := extendedCategoryBaseID(category)
		if baseID < 0 {
			t.Fatalf("%s has no command base id", category)
		}
		for index, action := range system.Actions {
			totalActions++
			id := baseID + index
			var commandText string
			for _, spec := range handler.CommandTable {
				if spec.ID == id && spec.Category == category {
					commandText = spec.Command
					break
				}
			}
			if commandText == "" {
				t.Fatalf("%s action %s has no command at id %d", category, action, id)
			}
			message := commandText
			parsed, ok := handler.ParseCommand(message)
			if !ok || parsed.Spec.ID != id || parsed.Spec.Category != category {
				// A few legacy one-word commands share a name. Their documented
				// named form must disambiguate to the extended runtime.
				message = commandText + " 巡检目标"
				parsed, ok = handler.ParseCommand(message)
			}
			if !ok || parsed.Spec.ID != id || parsed.Spec.Category != category {
				t.Fatalf("%s action %s command %q did not map back to id %d: %+v", category, action, message, id, parsed.Spec)
			}
			result, handled, err := game.Execute("group", player.AccountID, parsed)
			if err != nil || !handled || strings.TrimSpace(result.Title) == "" {
				t.Fatalf("%s action %s command %q failed runtime routing: handled=%v err=%v result=%+v", category, action, commandText, handled, err, result)
			}
			if strings.Contains(result.Content, "当前启用配置") {
				t.Fatalf("%s action %s fell back to the old global-template response: %+v", category, action, result)
			}
			_ = store.DB.Where("player_id = ? AND key = ?", player.ID, "pve.battle").Delete(&model.PlayerValue{}).Error
			_ = store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("state", model.PlayerStateIdle).Error
		}
	}
	if totalActions != 100 {
		t.Fatalf("extended runtime action slots=%d, want 100", totalActions)
	}
}

func TestExtendedOwnedListAndBattleSettleOnlyAfterVictory(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-puppet", "玄机关主"))
	var typed model.PuppetConfig
	if err := store.DB.Where("status = ?", "启用").Order("sort_order,id").First(&typed).Error; err != nil {
		t.Fatal(err)
	}
	config := model.GameplayConfigBase(typed)

	owned, handled, err := game.Execute("group", player.AccountID, mustParse(t, "傀儡"))
	if err != nil || !handled || !strings.Contains(owned.Content, "尚未拥有") || strings.Contains(owned.Content, "当前启用配置") || !containsAction(owned.Actions, "炼傀 "+config.Name) {
		t.Fatalf("owned puppet list exposed global configs: handled=%v err=%v result=%+v", handled, err, owned)
	}
	grantExtendedTestCosts(t, game, store, player.ID, config.CostMaterials, 2)
	crafted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼傀 "+config.Name))
	if err != nil || !handled || !strings.Contains(crafted.Title, "完成") {
		t.Fatalf("craft puppet: handled=%v err=%v result=%+v", handled, err, crafted)
	}
	var progress model.PlayerExtendedProgress
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "傀儡", config.Code).First(&progress).Error; err != nil || progress.Uses != 1 {
		t.Fatalf("crafted progress=%+v err=%v", progress, err)
	}

	started, handled, err := game.Execute("group", player.AccountID, mustParse(t, "傀战 "+config.Name))
	if err != nil || !handled || !strings.Contains(started.Title, "战局开启") {
		t.Fatalf("puppet battle did not start: handled=%v err=%v result=%+v", handled, err, started)
	}
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "傀儡", config.Code).First(&progress).Error; err != nil || progress.Uses != 1 {
		t.Fatalf("battle rewarded before victory: progress=%+v err=%v", progress, err)
	}
	raw, err := game.playerValue(player.ID, "pve.battle")
	if err != nil {
		t.Fatal(err)
	}
	var battle mapMonsterBattleState
	if err := json.Unmarshal([]byte(raw), &battle); err != nil {
		t.Fatal(err)
	}
	battle.EnemyHP = 1
	encoded, _ := json.Marshal(battle)
	if err := game.setPlayerValue(player.ID, "pve.battle", string(encoded), nil); err != nil {
		t.Fatal(err)
	}
	finished, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || !strings.Contains(finished.Title, "胜利") {
		t.Fatalf("puppet battle settlement: handled=%v err=%v result=%+v", handled, err, finished)
	}
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "傀儡", config.Code).First(&progress).Error; err != nil || progress.Uses != 2 || progress.Mastery <= 0 {
		t.Fatalf("victory progress not persisted: progress=%+v err=%v", progress, err)
	}
}

func TestWorldLeylineExtendedCommandsUseWorldLeylineRecords(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-leyline", "寻龙地师"))
	var leyline model.WorldLeyline
	if err := store.DB.Where("enabled = ?", true).Order("minimum_realm_sequence,sort_order,id").First(&leyline).Error; err != nil {
		t.Fatal(err)
	}
	root := displayOr(leyline.RequiredRootElement, leyline.Element) + "本源灵根"
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"location": leyline.LocationName, "spiritual_root": root, "mana": 10_000, "max_mana": 10_000}).Error; err != nil {
		t.Fatal(err)
	}
	if leyline.RequiredItemCount > 0 {
		item, err := game.itemByName(leyline.RequiredItem)
		if err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, leyline.RequiredItemCount*2); err != nil {
			t.Fatal(err)
		}
	}
	if err := game.setPlayerValue(player.ID, "leyline.discovered."+uintText(leyline.ID), "true", nil); err != nil {
		t.Fatal(err)
	}
	occupied, handled, err := game.Execute("group", player.AccountID, mustParse(t, "脉占 "+leyline.Name))
	if err != nil || !handled || !strings.Contains(occupied.Title, "成功") {
		t.Fatalf("occupy real world leyline: handled=%v err=%v result=%+v", handled, err, occupied)
	}
	var progress model.PlayerExtendedProgress
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "天地灵脉", worldLeylineProgressCode(leyline.ID)).First(&progress).Error; err != nil || progress.State != "已占据" {
		t.Fatalf("world leyline progress=%+v err=%v", progress, err)
	}
	meditation, handled, err := game.Execute("group", player.AccountID, mustParse(t, "脉修 "+leyline.Name))
	if err != nil || !handled || !strings.Contains(meditation.Title, "入定") {
		t.Fatalf("world leyline practice did not reuse meditation: handled=%v err=%v result=%+v", handled, err, meditation)
	}
}

func TestImmortalHerbHarvestCreatesUsableInventoryItem(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-herb", "百草丹君"))
	mansion := model.Mansion{PlayerID: player.ID, Name: "百草洞天", Level: 5, FarmLevel: 5, AlchemyRoomLevel: 5}
	if err := store.DB.Create(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("mansion_id", mansion.ID).Error; err != nil {
		t.Fatal(err)
	}
	var typed model.ImmortalHerbConfig
	if err := store.DB.Where("status = ?", "启用").Order("sort_order,id").First(&typed).Error; err != nil {
		t.Fatal(err)
	}
	config := model.GameplayConfigBase(typed)
	grantExtendedTestCosts(t, game, store, player.ID, config.CostMaterials, 1)
	atlas, handled, err := game.Execute("group", player.AccountID, mustParse(t, "药鉴"))
	if err != nil || !handled || !containsAction(atlas.Actions, "种药 "+config.Name) {
		t.Fatalf("herb atlas missing named plant action: handled=%v err=%v result=%+v", handled, err, atlas)
	}
	planted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "种药 "+config.Name))
	if err != nil || !handled || !strings.Contains(planted.Title, "入圃") {
		t.Fatalf("plant herb: handled=%v err=%v result=%+v", handled, err, planted)
	}
	past := time.Now().Add(-time.Minute)
	if err := store.DB.Model(&model.PlayerExtendedProgress{}).Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙药培育", config.Code).Update("ready_at", &past).Error; err != nil {
		t.Fatal(err)
	}
	harvested, handled, err := game.Execute("group", player.AccountID, mustParse(t, "采药 "+config.Name))
	if err != nil || !handled || !strings.Contains(harvested.Title, "完成") || !strings.Contains(harvested.Content, "实际药效") {
		t.Fatalf("harvest herb: handled=%v err=%v result=%+v", handled, err, harvested)
	}
	item, err := game.itemByName(config.Name)
	if err != nil || item.EffectFunc != "add_cultivation" || game.itemQuantity(player.ID, item.ID) < 1 {
		t.Fatalf("harvested item not usable: item=%+v qty=%d err=%v", item, game.itemQuantity(player.ID, item.ID), err)
	}
}

func TestArtifactRefinementMutatesOwnedArtifact(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-artifact", "百炼真君"))
	var template model.ArtifactTemplate
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: artifactTemplateSlot(template)}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	iron, err := game.itemByName("玄铁")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, iron.ID, 10); err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼化 "+artifact.Name))
	if err != nil || !handled || !strings.Contains(result.Title, "炼化") || strings.Contains(result.Title, "本命") || !strings.Contains(result.Content, "槽位："+artifactTemplateSlot(template)+" · 器型："+artifactTemplateArchetype(template)) {
		t.Fatalf("refine owned artifact: handled=%v err=%v result=%+v", handled, err, result)
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil || artifact.ForgeLevel != 1 {
		t.Fatalf("artifact forge level=%d err=%v", artifact.ForgeLevel, err)
	}
	var progress model.PlayerExtendedProgress
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "法宝炼化", "owned_artifact:"+uintText(artifact.ID)).First(&progress).Error; err != nil || progress.State != "真火炼化" {
		t.Fatalf("artifact refinement progress=%+v err=%v", progress, err)
	}
}

func TestImmortalEncounterAndBattlefieldPersistPersonalResults(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "extended-events", "问缘战仙"))
	encounter, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙遇"))
	if err != nil || !handled || !strings.Contains(encounter.Title, "已现") || !containsAction(encounter.Actions, "仙选 守心观照") {
		t.Fatalf("trigger encounter: handled=%v err=%v result=%+v", handled, err, encounter)
	}
	chosen, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙选 守心观照"))
	if err != nil || !handled || !strings.Contains(chosen.Content, "个人仙录") {
		t.Fatalf("choose encounter: handled=%v err=%v result=%+v", handled, err, chosen)
	}
	if _, err := game.playerValue(player.ID, "immortal_encounter.pending"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("pending encounter remained: err=%v", err)
	}

	entered, handled, err := game.Execute("group", player.AccountID, mustParse(t, "战场"))
	if err != nil || !handled || !strings.Contains(entered.Title, "进入") {
		t.Fatalf("enter battlefield: handled=%v err=%v result=%+v", handled, err, entered)
	}
	if _, handled, err := game.Execute("group", player.AccountID, mustParse(t, "阵营 仙")); err != nil || !handled {
		t.Fatalf("choose faction: handled=%v err=%v", handled, err)
	}
	var battlefieldConfig model.GameplayConfigBase
	activeCode, err := game.playerValue(player.ID, "battlefield.active")
	if err != nil || store.DB.Table(extendedSystems["仙魔战场"].Table).Where("code = ?", activeCode).First(&battlefieldConfig).Error != nil {
		t.Fatalf("active battlefield config missing: code=%s err=%v", activeCode, err)
	}
	grantExtendedTestCosts(t, game, store, player.ID, battlefieldConfig.CostMaterials, 1)
	started, handled, err := game.Execute("group", player.AccountID, mustParse(t, "战战"))
	if err != nil || !handled || !strings.Contains(started.Title, "战局开启") {
		t.Fatalf("start battlefield battle: handled=%v err=%v result=%+v", handled, err, started)
	}
	raw, _ := game.playerValue(player.ID, "pve.battle")
	var battle mapMonsterBattleState
	_ = json.Unmarshal([]byte(raw), &battle)
	battle.EnemyHP = 1
	encoded, _ := json.Marshal(battle)
	_ = game.setPlayerValue(player.ID, "pve.battle", string(encoded), nil)
	finished, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || !strings.Contains(finished.Title, "凯旋") || game.playerValueInt(player.ID, "battlefield.contribution", 0) <= 0 {
		t.Fatalf("battlefield settlement: handled=%v err=%v points=%d result=%+v", handled, err, game.playerValueInt(player.ID, "battlefield.contribution", 0), finished)
	}
}

func TestImmortalEncounterSettlementConsumesPendingRecordOnce(t *testing.T) {
	game, store := testGame(t)
	player := prepareExtendedTestPlayer(t, game, store, registerPlayer(t, game, "atomic-encounter", "守缘真人"))
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙遇")); err != nil || !handled || !strings.Contains(result.Title, "已现") {
		t.Fatalf("trigger encounter: handled=%v err=%v result=%+v", handled, err, result)
	}
	_, config, pendingValue, err := game.pendingEncounterConfig(player.ID, extendedSystems["仙缘奇遇"])
	if err != nil {
		t.Fatal(err)
	}
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	reward := map[string]any{"spirit_stones": int64(77)}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- game.settleImmortalEncounter(&player, pendingValue, config, "因果圆满", reward)
		}()
	}
	close(start)
	succeeded, rejected := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, errImmortalEncounterSettled):
			rejected++
		default:
			t.Fatalf("unexpected settlement error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("settlement outcomes: succeeded=%d rejected=%d", succeeded, rejected)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SpiritStones-before.SpiritStones != 77 {
		t.Fatalf("reward applied %d times: before=%d after=%d", (after.SpiritStones-before.SpiritStones)/77, before.SpiritStones, after.SpiritStones)
	}
	var progress model.PlayerExtendedProgress
	if err := store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙缘奇遇", config.Code).First(&progress).Error; err != nil {
		t.Fatal(err)
	}
	if progress.Uses != 1 {
		t.Fatalf("encounter uses=%d want=1", progress.Uses)
	}
}
