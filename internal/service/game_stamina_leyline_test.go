package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestStaminaMaximumGrowsByOneHundredPerMajorRealm(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "stamina-growth-player", "长风行者")
	var realms []model.Realm
	if err := store.DB.Order("sequence").Limit(10).Find(&realms).Error; err != nil || len(realms) < 10 {
		t.Fatalf("load realms: count=%d err=%v", len(realms), err)
	}
	assertMaximum := func(realm model.Realm, want int64) {
		t.Helper()
		if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"realm_id": realm.ID, "realm_name": realm.Name}).Error; err != nil {
			t.Fatal(err)
		}
		got, err := game.staminaMaximum(player.ID)
		if err != nil || got != want {
			t.Fatalf("realm %d stamina maximum=%d want=%d err=%v", realm.Sequence, got, want, err)
		}
	}
	assertMaximum(realms[0], 100)
	assertMaximum(realms[1], 200)
	assertMaximum(realms[9], 1000)
	var finalRealm model.Realm
	if err := store.DB.Where("sequence = ?", 1000).First(&finalRealm).Error; err != nil {
		finalRealm = model.Realm{Name: "无量测试境", Sequence: 1000, Description: "验证体力恢复速度不封顶。"}
		if err := store.DB.Create(&finalRealm).Error; err != nil {
			t.Fatal(err)
		}
	}
	assertMaximum(finalRealm, 100000)
	if recovery, err := game.staminaRecoveryPerMinute(player.ID); err != nil || recovery != 10000 {
		t.Fatalf("realm 1000 uncapped stamina recovery=%d want=10000 err=%v", recovery, err)
	}
	assertMaximum(realms[9], 1000)
	if recovery, err := game.staminaRecoveryPerMinute(player.ID); err != nil || recovery != 100 {
		t.Fatalf("realm 10 stamina recovery=%d want=100 err=%v", recovery, err)
	}
	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 100); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, "stamina.value").UpdateColumn("updated_at", time.Now().Add(-5*time.Minute-5*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if current, err := game.currentStamina(player.ID); err != nil || current != 600 {
		t.Fatalf("scaled natural recovery stamina=%d want=600 err=%v", current, err)
	}

	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().AddDate(0, 0, -1).Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 7); err != nil {
		t.Fatal(err)
	}
	if current, err := game.currentStamina(player.ID); err != nil || current != 1000 {
		t.Fatalf("daily reset stamina=%d want=1000 err=%v", current, err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "体力"))
	if err != nil || !handled || !strings.Contains(result.Content, "当前体力：1000/1000") || !strings.Contains(result.Content, "每提升一个大境界，体力上限永久+100") || !strings.Contains(result.Content, "当前恢复：每分钟自动+100") || !strings.Contains(result.Content, "约10分钟可从零回满") || !strings.Contains(result.Content, "恢复速度不设上限") || !strings.Contains(result.Content, "无需打坐") || hasGlobalPagination(result) {
		t.Fatalf("stamina overview: handled=%v err=%v result=%+v", handled, err, result)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "display.status_image_mode").Update("value", "false").Error; err != nil {
		t.Fatal(err)
	}
	status, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !strings.Contains(status.Content, "体力：1000/1000") {
		t.Fatalf("text status stamina: handled=%v err=%v result=%+v", handled, err, status)
	}
}

func TestLeylineRealmRestrictionAndElementGuidance(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "leyline-guidance-player", "观脉真人")
	var realms []model.Realm
	if err := store.DB.Order("sequence").Limit(2).Find(&realms).Error; err != nil || len(realms) < 2 {
		t.Fatalf("load realms: count=%d err=%v", len(realms), err)
	}
	customRoot := model.SpiritualRootTemplate{Code: "test_time_root", Name: "太虚时空道莲灵根", Element: "时空", Grade: "仙灵", BaseQuality: 90, CultivationBonus: 1.5, RarityWeight: 1, Enabled: true}
	if err := store.DB.Create(&customRoot).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"spiritual_root": customRoot.Name, "realm_id": realms[0].ID, "realm_name": realms[0].Name, "realm_level": 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
	player, _ = game.players.Get(player.ID)

	rootLookup, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵根详情 庚金本源"))
	if err != nil || !handled || strings.Contains(rootLookup.Title, "未收录") || !strings.Contains(rootLookup.Title, "庚金本源") || !strings.Contains(rootLookup.Content, "筛选本源：庚金") {
		t.Fatalf("root element lookup: handled=%v err=%v result=%+v", handled, err, rootLookup)
	}

	var leylines []model.WorldLeyline
	if err := store.DB.Order("id").Limit(2).Find(&leylines).Error; err != nil || len(leylines) < 2 {
		t.Fatalf("load leylines: count=%d err=%v", len(leylines), err)
	}
	gold := leylines[0]
	if err := store.DB.Model(&gold).Updates(map[string]any{
		"location_name": player.Location, "required_root_element": "庚金", "minimum_realm_sequence": 2, "minimum_realm_level": 3,
		"minimum_combat_power": 0, "minimum_spirit": 0, "discovery_mana_cost": 0, "required_item": "龙血芝", "required_item_count": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&leylines[1]).Updates(map[string]any{"element": "时空", "required_root_element": "时空"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "leyline.discovered."+uintText(gold.ID), "true", nil); err != nil {
		t.Fatal(err)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵脉打坐 "+gold.Name))
	if err != nil || !handled || !strings.Contains(blocked.Content, "境界不足") || !strings.Contains(blocked.Content, realms[1].Name+"·3层") {
		t.Fatalf("realm restriction: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"realm_id": realms[1].ID, "realm_name": realms[1].Name, "realm_level": 3}).Error; err != nil {
		t.Fatal(err)
	}
	blocked, handled, err = game.Execute("group", player.AccountID, mustParse(t, "灵脉打坐 "+gold.Name))
	if err != nil || !handled || !strings.Contains(blocked.Content, "灵根不契合") || !containsAction(blocked.Actions, "灵根图鉴 庚金") || !containsAction(blocked.Actions, "灵脉地图 时空") || !containsAction(blocked.Actions, "物品 龙血芝") {
		t.Fatalf("leyline guidance: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	filtered, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵脉地图 时空"))
	if err != nil || !handled || !strings.Contains(filtered.Content, "本源筛选：时空") || !strings.Contains(filtered.Content, leylines[1].Name) || strings.Contains(filtered.Content, gold.Name) {
		t.Fatalf("leyline element filter: handled=%v err=%v result=%+v", handled, err, filtered)
	}
}

func TestReincarnationLeylineSearchShowsReachableRoute(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "reincarnation-route-player", "六道寻脉人")
	root := model.SpiritualRootTemplate{Code: "test_reincarnation_root", Name: "无极轮回道莲灵根", Element: "轮回", Grade: "超脱", BaseQuality: 90, CultivationBonus: 1.7, RarityWeight: 1, Enabled: true}
	if err := store.DB.Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	var locations []model.WorldLocation
	if err := store.DB.Order("sort_order").Limit(2).Find(&locations).Error; err != nil || len(locations) != 2 {
		t.Fatalf("load route locations: count=%d err=%v", len(locations), err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"spiritual_root": root.Name, "location": locations[1].Name}).Error; err != nil {
		t.Fatal(err)
	}
	leyline := model.WorldLeyline{
		Code: "test_reincarnation_leyline", Name: "六道归真轮回灵脉", Region: locations[0].Region, LocationName: locations[0].Name,
		Element: "轮回", Grade: "微型灵脉", AuraPerMinute: 200, CultivationMultiplier: 2.1, MeditationSlots: 4,
		MinimumRealmSequence: 1, MinimumRealmLevel: 1, RequiredRootElement: "轮回", Enabled: true, SortOrder: 1,
	}
	if err := store.DB.Create(&leyline).Error; err != nil {
		t.Fatal(err)
	}
	before, _ := game.players.Get(player.ID)
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵脉地图 轮回"))
	if err != nil || !handled || !strings.Contains(result.Content, leyline.Name) || !strings.Contains(result.Content, "以下为全界结果") || !strings.Contains(result.Content, "距此1步") || !containsAction(result.Actions, "前往 "+locations[0].Name) {
		t.Fatalf("reincarnation leyline map: handled=%v err=%v result=%+v", handled, err, result)
	}
	guided, handled, err := game.Execute("group", player.AccountID, mustParse(t, "寻脉"))
	if err != nil || !handled || !strings.Contains(guided.Title, "地脉指路") || !strings.Contains(guided.Content, leyline.Name) || !strings.Contains(guided.Content, "本次没有扣除法力") || !containsAction(guided.Actions, "前往 "+locations[0].Name) {
		t.Fatalf("reincarnation discovery guidance: handled=%v err=%v result=%+v", handled, err, guided)
	}
	after, _ := game.players.Get(player.ID)
	if after.Mana != before.Mana {
		t.Fatalf("remote guidance deducted mana: before=%d after=%d", before.Mana, after.Mana)
	}
}

func uintText(value uint) string {
	return fmt.Sprint(value)
}
