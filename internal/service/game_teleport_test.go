package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestTeleportRequiresRecordedDestinationAndConsumesOneCharm(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "teleport-local-player", "踏云真人")
	var locations []model.WorldLocation
	if err := store.DB.Where("region = ?", "东洲").Order("minimum_realm_level,sort_order,id").Limit(2).Find(&locations).Error; err != nil || len(locations) != 2 {
		t.Fatalf("load east maps: count=%d err=%v", len(locations), err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("realm_level", 3).Error; err != nil {
		t.Fatal(err)
	}
	charm, err := game.itemByName("传送符")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, charm.ID, 5); err != nil {
		t.Fatal(err)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "地图")); err != nil || !handled || !strings.Contains(result.Content, "传送阵：已激活") || !containsAction(result.Actions, "传送列表") {
		t.Fatalf("activate current array: handled=%v err=%v result=%+v", handled, err, result)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "传送 "+locations[1].Name))
	if err != nil || !handled || !strings.Contains(blocked.Title, "尚未刻录") || game.itemQuantity(player.ID, charm.ID) != 5 {
		t.Fatalf("unrecorded destination: handled=%v err=%v result=%+v charms=%d", handled, err, blocked, game.itemQuantity(player.ID, charm.ID))
	}
	if walked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "前往 "+locations[1].Name)); err != nil || !handled || !strings.Contains(walked.Content, "已永久刻录") {
		t.Fatalf("walk and record: handled=%v err=%v result=%+v", handled, err, walked)
	}
	if _, handled, err := game.Execute("group", player.AccountID, mustParse(t, "前往 "+locations[0].Name)); err != nil || !handled {
		t.Fatalf("walk back: handled=%v err=%v", handled, err)
	}
	teleported, handled, err := game.Execute("group", player.AccountID, mustParse(t, "传送 "+locations[1].Name))
	if err != nil || !handled || !strings.Contains(teleported.Title, "界内挪移成功") || !strings.Contains(teleported.Content, "传送符：-1") {
		t.Fatalf("local teleport: handled=%v err=%v result=%+v", handled, err, teleported)
	}
	updated, _ := game.players.Get(player.ID)
	if updated.Location != locations[1].Name || game.itemQuantity(player.ID, charm.ID) != 4 {
		t.Fatalf("local teleport state: location=%s charms=%d", updated.Location, game.itemQuantity(player.ID, charm.ID))
	}
}

func TestCrossWorldTeleportShowsLockedWorldAndUnlocksEntryGate(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "teleport-cross-player", "观界真人")
	var realms []model.Realm
	if err := store.DB.Order("sequence").Limit(2).Find(&realms).Error; err != nil || len(realms) < 2 {
		t.Fatalf("load realms: count=%d err=%v", len(realms), err)
	}
	southGate := model.WorldLocation{
		Code: "test_south_world_gate", Name: "南疆·朱雀接引台", Region: "南疆", Description: "南疆入口界门。",
		NPCJSON: "[]", TasksJSON: "[]", NeighborsJSON: "[]", TeleportEnabled: true, CrossRegionEnabled: true,
		MinimumRealmSequence: realms[1].Sequence, MinimumRealmLevel: 1, MinimumLevel: 1, Enabled: true, SortOrder: 1,
	}
	if err := store.DB.Create(&southGate).Error; err != nil {
		t.Fatal(err)
	}
	charm, err := game.itemByName("传送符")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, charm.ID, 6); err != nil {
		t.Fatal(err)
	}
	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "诸界列表"))
	if err != nil || !handled || !strings.Contains(overview.Content, "【南疆】") || !strings.Contains(overview.Content, "未解锁") || !strings.Contains(overview.Content, "地图") {
		t.Fatalf("locked world overview: handled=%v err=%v result=%+v", handled, err, overview)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "跨界传送 南疆"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "尚未解锁") || game.itemQuantity(player.ID, charm.ID) != 6 {
		t.Fatalf("locked cross-world teleport: handled=%v err=%v result=%+v charms=%d", handled, err, blocked, game.itemQuantity(player.ID, charm.ID))
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"realm_id": realms[1].ID, "realm_name": realms[1].Name, "realm_level": 1}).Error; err != nil {
		t.Fatal(err)
	}
	teleported, handled, err := game.Execute("group", player.AccountID, mustParse(t, "跨界传送 南疆"))
	if err != nil || !handled || !strings.Contains(teleported.Title, "跨界接引成功") || !strings.Contains(teleported.Content, "传送符：-3") {
		t.Fatalf("cross-world teleport: handled=%v err=%v result=%+v", handled, err, teleported)
	}
	updated, _ := game.players.Get(player.ID)
	if updated.Location != southGate.Name || game.itemQuantity(player.ID, charm.ID) != 3 {
		t.Fatalf("cross-world state: location=%s charms=%d", updated.Location, game.itemQuantity(player.ID, charm.ID))
	}
	if _, err := game.playerValue(player.ID, teleportActivationKey(southGate.ID)); err != nil {
		t.Fatalf("destination gate was not recorded: %v", err)
	}
}

func TestBareTeleportListUsesCurrentRegionWhileWorldListUsesOverview(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "teleport-list-mode-player", "列阵真人")
	local, handled, err := game.Execute("group", player.AccountID, mustParse(t, "传送列表"))
	if err != nil || !handled || !strings.Contains(local.Title, "东洲传送阵图") || strings.Contains(local.Content, "天地已勘定十座正式界域") {
		t.Fatalf("bare local teleport list: handled=%v err=%v result=%+v", handled, err, local)
	}
	worlds, handled, err := game.Execute("group", player.AccountID, mustParse(t, "诸界列表"))
	if err != nil || !handled || !strings.Contains(worlds.Title, "诸界传送列表") || !strings.Contains(worlds.Content, "天地已勘定十座正式界域") || strings.Contains(worlds.Title, "东洲传送阵图") {
		t.Fatalf("world overview: handled=%v err=%v result=%+v", handled, err, worlds)
	}
}
