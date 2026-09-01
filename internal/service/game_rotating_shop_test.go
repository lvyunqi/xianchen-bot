package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestRotatingShopWindowsAndSelectionsAreStable(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 23, 17, 42, 9, 0, location)

	dailyID, dailyStart, dailyEnd := rotatingShopWindow(mysteryShopConfig.Code, now)
	if dailyID != "20260723" || dailyStart.Hour() != 0 || dailyEnd.Sub(dailyStart) != 24*time.Hour {
		t.Fatalf("daily window: id=%s start=%s end=%s", dailyID, dailyStart, dailyEnd)
	}
	limitedID, limitedStart, limitedEnd := rotatingShopWindow(limitedShopConfig.Code, now)
	if limitedID != "20260723-12" || limitedStart.Hour() != 12 || limitedEnd.Hour() != 18 || limitedEnd.Sub(limitedStart) != 6*time.Hour {
		t.Fatalf("limited window: id=%s start=%s end=%s", limitedID, limitedStart, limitedEnd)
	}

	first := rotatingShopSelection(limitedShopConfig, limitedID)
	second := rotatingShopSelection(limitedShopConfig, limitedID)
	if len(first) != limitedShopConfig.Slots || fmt.Sprint(first) != fmt.Sprint(second) {
		t.Fatalf("selection is not stable: first=%+v second=%+v", first, second)
	}
	seen := map[string]bool{}
	for _, good := range first {
		if seen[good.Code] {
			t.Fatalf("duplicate rotating good: %+v", good)
		}
		seen[good.Code] = true
	}
}

func TestMysteryShopBatchPurchaseEnforcesPersonalRotationStock(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "mystery-shop-player", "太虚行客")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", 100_000).Error; err != nil {
		t.Fatal(err)
	}
	player, _ = game.players.Get(player.ID)

	windowID, _, _ := rotatingShopWindow(mysteryShopConfig.Code, time.Now())
	goods := rotatingShopSelection(mysteryShopConfig, windowID)
	if len(goods) == 0 {
		t.Fatal("mystery rotation has no goods")
	}
	selected := goods[0]
	var item model.Item
	if err := store.DB.Where("name = ?", selected.ItemName).First(&item).Error; err != nil {
		t.Fatalf("selected item is not real: %s: %v", selected.ItemName, err)
	}

	page, handled, err := game.Execute("group", player.AccountID, mustParse(t, "神秘商城"))
	if err != nil || !handled || !strings.Contains(page.Title, "神秘商城") || !strings.Contains(page.Content, "距离刷新") || !strings.Contains(page.Content, "个人剩余") || !containsAction(page.Actions, "神秘购买 "+selected.ItemName) {
		t.Fatalf("mystery shop page: handled=%v err=%v result=%+v", handled, err, page)
	}

	beforeQuantity := game.itemQuantity(player.ID, item.ID)
	beforeSilver := player.SilverCoins
	bought, handled, err := game.Execute("group", player.AccountID, mustParse(t, "神秘购买 "+selected.ItemName+"*2"))
	if err != nil || !handled || !strings.Contains(bought.Title, "购买成功") || !strings.Contains(bought.Content, selected.ItemName+"×2") || !strings.Contains(bought.Content, fmt.Sprintf("支付：%d银币", selected.Price*2)) {
		t.Fatalf("mystery batch purchase: handled=%v err=%v result=%+v", handled, err, bought)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SilverCoins != beforeSilver-selected.Price*2 || game.itemQuantity(player.ID, item.ID) != beforeQuantity+2 {
		t.Fatalf("purchase settlement: silver=%d quantity=%d", after.SilverCoins, game.itemQuantity(player.ID, item.ID))
	}
	value, err := game.playerValue(player.ID, rotatingShopStockKey(mysteryShopConfig.Code, selected.Code))
	if err != nil || rotatingShopCounter(value, windowID) != 2 {
		t.Fatalf("purchase marker: value=%q err=%v", value, err)
	}

	remaining := selected.Stock - 2
	beforeRejected, _ := game.players.Get(player.ID)
	beforeRejectedQuantity := game.itemQuantity(player.ID, item.ID)
	rejected, handled, err := game.Execute("group", player.AccountID, mustParse(t, fmt.Sprintf("神秘购买 %s*%d", selected.ItemName, remaining+1)))
	if err != nil || !handled || !strings.Contains(rejected.Title, "库存不足") || !strings.Contains(rejected.Content, "没有扣款") {
		t.Fatalf("over-stock purchase: handled=%v err=%v result=%+v", handled, err, rejected)
	}
	afterRejected, _ := game.players.Get(player.ID)
	if afterRejected.SilverCoins != beforeRejected.SilverCoins || game.itemQuantity(player.ID, item.ID) != beforeRejectedQuantity {
		t.Fatal("rejected stock purchase changed currency or inventory")
	}
}

func TestLimitedShopInsufficientSilverRollsBackStockAndInventory(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "limited-shop-player", "候潮散人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", 0).Error; err != nil {
		t.Fatal(err)
	}

	windowID, _, _ := rotatingShopWindow(limitedShopConfig.Code, time.Now())
	goods := rotatingShopSelection(limitedShopConfig, windowID)
	if len(goods) == 0 {
		t.Fatal("limited rotation has no goods")
	}
	selected := goods[0]
	var item model.Item
	if err := store.DB.Where("name = ?", selected.ItemName).First(&item).Error; err != nil {
		t.Fatalf("selected item is not real: %s: %v", selected.ItemName, err)
	}

	page, handled, err := game.Execute("group", player.AccountID, mustParse(t, "限时商城"))
	if err != nil || !handled || !strings.Contains(page.Title, "限时商城") || !strings.Contains(page.Content, "每六小时") || !containsAction(page.Actions, "限时购买 "+selected.ItemName) {
		t.Fatalf("limited shop page: handled=%v err=%v result=%+v", handled, err, page)
	}
	beforeQuantity := game.itemQuantity(player.ID, item.ID)
	rejected, handled, err := game.Execute("group", player.AccountID, mustParse(t, "限时购买 "+selected.ItemName))
	if err != nil || !handled || !strings.Contains(rejected.Title, "银币不足") || !strings.Contains(rejected.Content, "没有占用库存") {
		t.Fatalf("insufficient silver purchase: handled=%v err=%v result=%+v", handled, err, rejected)
	}
	if game.itemQuantity(player.ID, item.ID) != beforeQuantity {
		t.Fatal("insufficient silver purchase granted inventory")
	}
	var markerCount int64
	if err := store.DB.Model(&model.PlayerValue{}).
		Where("player_id = ? AND key = ?", player.ID, rotatingShopStockKey(limitedShopConfig.Code, selected.Code)).
		Count(&markerCount).Error; err != nil {
		t.Fatal(err)
	}
	if markerCount != 0 {
		t.Fatalf("insufficient silver purchase consumed stock marker: %d", markerCount)
	}
}

func TestRotatingShopCatalogOnlyUsesRealItems(t *testing.T) {
	_, store := testGame(t)
	for _, config := range []rotatingShopConfig{mysteryShopConfig, limitedShopConfig} {
		for _, good := range config.Goods {
			var count int64
			if err := store.DB.Model(&model.Item{}).Where("name = ?", good.ItemName).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 || good.Price <= 0 || good.Stock <= 0 {
				t.Fatalf("invalid rotating good in %s: %+v count=%d", config.Code, good, count)
			}
		}
	}
}
