package service

import (
	"fmt"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestNPCShopBareCommandSelectionPurchaseGiftAndRelationship(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "npc-shop-player", "访市真人")
	location := model.WorldLocation{
		Code: "test_npc_market", Name: "青石仙市", Region: "东洲", Description: "测试人物商铺。",
		NPCJSON: `["云游药师","守山剑客"]`, TasksJSON: `[]`, NeighborsJSON: `[]`,
		MinimumRealmSequence: 1, MinimumRealmLevel: 1, MinimumLevel: 1, Enabled: true,
	}
	if err := store.DB.Create(&location).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"location": location.Name, "spirit_stones": 1_000_000}).Error; err != nil {
		t.Fatal(err)
	}
	player.Location = location.Name
	player.SpiritStones = 1_000_000

	selection, handled, err := game.Execute("group", player.AccountID, mustParse(t, "NPC商店"))
	if err != nil || !handled || !strings.Contains(selection.Title, "人物仙商") || !containsAction(selection.Actions, "NPC商店 云游药师") || !containsAction(selection.Actions, "NPC商店 守山剑客") {
		t.Fatalf("bare npc shop: handled=%v err=%v result=%+v", handled, err, selection)
	}
	shop, handled, err := game.Execute("group", player.AccountID, mustParse(t, "NPC商店 云游药师"))
	if err != nil || !handled || !strings.Contains(shop.Title, "云游药师的店铺") || !strings.Contains(shop.Content, "好感度") || !strings.Contains(shop.Content, "灵石") {
		t.Fatalf("npc shop: handled=%v err=%v result=%+v", handled, err, shop)
	}
	npc := localNPC{Location: location, Name: "云游药师", Index: 0}
	entries, err := game.npcInventory(npc)
	if err != nil || len(entries) == 0 || entries[0].RequiredAffinity != 0 {
		t.Fatalf("npc inventory: entries=%+v err=%v", entries, err)
	}
	entry := entries[0]
	beforeQuantity := game.itemQuantity(player.ID, entry.Item.ID)
	purchased, handled, err := game.Execute("group", player.AccountID, mustParse(t, "NPC购买 "+entry.Item.Name+"*2"))
	if err != nil || !handled || !strings.Contains(purchased.Title, "成交") || game.itemQuantity(player.ID, entry.Item.ID) != beforeQuantity+2 {
		t.Fatalf("npc purchase: handled=%v err=%v result=%+v quantity=%d", handled, err, purchased, game.itemQuantity(player.ID, entry.Item.ID))
	}
	beforeAffinity := game.playerValueInt(player.ID, npcAffinityKey(npc), 0)
	gifted, handled, err := game.Execute("group", player.AccountID, mustParse(t, fmt.Sprintf("NPC赠送 云游药师 %s*1", entry.Item.Name)))
	if err != nil || !handled || !strings.Contains(gifted.Title, "人物赠礼") || game.playerValueInt(player.ID, npcAffinityKey(npc), 0) <= beforeAffinity || game.itemQuantity(player.ID, entry.Item.ID) != beforeQuantity+1 {
		t.Fatalf("npc gift: handled=%v err=%v result=%+v", handled, err, gifted)
	}
	relationship, handled, err := game.Execute("group", player.AccountID, mustParse(t, "NPC关系 云游药师"))
	if err != nil || !handled || !strings.Contains(relationship.Content, "云游药师") || !strings.Contains(relationship.Content, "好感度") {
		t.Fatalf("npc relationship: handled=%v err=%v result=%+v", handled, err, relationship)
	}
	dialogue, handled, err := game.Execute("group", player.AccountID, mustParse(t, "对话 云游药师"))
	if err != nil || !handled || !containsAction(dialogue.Actions, "NPC商店 云游药师") || !containsAction(dialogue.Actions, "NPC赠送 云游药师") {
		t.Fatalf("npc dialogue commerce actions: handled=%v err=%v result=%+v", handled, err, dialogue)
	}
}

func TestFarmWeatherCommandsHaveBusinessRoutes(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "farm-weather-route-player", "司天真人")
	for _, command := range []string{"灵田天象", "护持灵田", "灵田灾异录"} {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, command))
		if err != nil || !handled || strings.Contains(result.Title, "天机紊乱") || strings.TrimSpace(result.Content) == "" {
			t.Fatalf("command %s: handled=%v err=%v result=%+v", command, handled, err, result)
		}
	}
}
