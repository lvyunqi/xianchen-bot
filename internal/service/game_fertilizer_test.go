package service

import (
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestFertilizerAppliesOnceAndPersistsCropEffects(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "fertilizer-single-player", "青禾散人")

	seed, err := game.itemByName("凝露草籽")
	if err != nil {
		t.Fatal(err)
	}
	fertilizer, err := game.itemByName("灵壤肥")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, seed.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fertilizer.ID, 2); err != nil {
		t.Fatal(err)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "种植 凝露草籽 1")); err != nil || !handled || !strings.Contains(result.Title, "播种") {
		t.Fatalf("plant crop: handled=%v err=%v result=%+v", handled, err, result)
	}

	var mansion model.Mansion
	if err := store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	var before model.MansionCrop
	if err := store.DB.Where("mansion_id = ? AND plot = ? AND harvested = ?", mansion.ID, 1, false).First(&before).Error; err != nil {
		t.Fatal(err)
	}

	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "施肥 1 灵壤肥"))
	if err != nil || !handled || !strings.Contains(result.Title, "灵肥归壤") || !strings.Contains(result.Content, "预计增产：+1株") || !strings.Contains(result.Content, "抗灾道韵：10") {
		t.Fatalf("fertilize crop: handled=%v err=%v result=%+v", handled, err, result)
	}
	var after model.MansionCrop
	if err := store.DB.First(&after, before.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !after.Fertilized || after.FertilizerName != fertilizer.Name || after.DisasterResistance != 10 || after.Quantity != before.Quantity+1 {
		t.Fatalf("fertilizer state not persisted: before=%+v after=%+v", before, after)
	}
	advance := before.ReadyAt.Sub(after.ReadyAt)
	if advance < 10*time.Minute-time.Second || advance > 10*time.Minute+time.Second {
		t.Fatalf("unexpected fertilizer advance: %s", advance)
	}
	if quantity := game.itemQuantity(player.ID, fertilizer.ID); quantity != 1 {
		t.Fatalf("fertilizer quantity after first use=%d, want 1", quantity)
	}

	repeated, handled, err := game.Execute("group", player.AccountID, mustParse(t, "施肥 1 造化仙壤"))
	if err != nil || !handled || !strings.Contains(repeated.Title, "已经施肥") || !strings.Contains(repeated.Content, "没有扣除") {
		t.Fatalf("repeat fertilizer rejection: handled=%v err=%v result=%+v", handled, err, repeated)
	}
	if quantity := game.itemQuantity(player.ID, fertilizer.ID); quantity != 1 {
		t.Fatalf("repeat fertilizer consumed item: %d", quantity)
	}
}

func TestLegacyNullFertilizerStateMatchesOverviewAndBothActions(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "fertilizer-legacy-null-player", "归壤真人")
	seed, err := game.itemByName("凝露草籽")
	if err != nil {
		t.Fatal(err)
	}
	fertilizer, err := game.itemByName("灵壤肥")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, seed.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fertilizer.ID, 2); err != nil {
		t.Fatal(err)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键种植 凝露草籽")); err != nil || !handled || !strings.Contains(result.Title, "完成") {
		t.Fatalf("plant all: handled=%v err=%v result=%+v", handled, err, result)
	}
	var mansion model.Mansion
	if err := store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Exec("UPDATE mansion_crops SET fertilized = NULL WHERE mansion_id = ? AND harvested = ?", mansion.ID, false).Error; err != nil {
		t.Fatal(err)
	}

	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "我的灵田"))
	if err != nil || !handled || strings.Count(overview.Content, "待施灵肥") != 2 {
		t.Fatalf("legacy overview: handled=%v err=%v result=%+v", handled, err, overview)
	}
	single, handled, err := game.Execute("group", player.AccountID, mustParse(t, "施肥 1 灵壤肥"))
	if err != nil || !handled || !strings.Contains(single.Title, "灵肥归壤") {
		t.Fatalf("single legacy fertilizer: handled=%v err=%v result=%+v", handled, err, single)
	}
	batch, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键施肥 灵壤肥"))
	if err != nil || !handled || !strings.Contains(batch.Title, "一键施肥完成") || !strings.Contains(batch.Content, "施用：灵壤肥 × 1") {
		t.Fatalf("batch legacy fertilizer: handled=%v err=%v result=%+v", handled, err, batch)
	}
	var fertilized int64
	if err := store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND fertilized = ?", mansion.ID, false, true).Count(&fertilized).Error; err != nil {
		t.Fatal(err)
	}
	if fertilized != 2 || game.itemQuantity(player.ID, fertilizer.ID) != 0 {
		t.Fatalf("legacy fertilizer settlement: fertilized=%d held=%d", fertilized, game.itemQuantity(player.ID, fertilizer.ID))
	}
}

func TestOneClickFertilizerOnlyConsumesSuccessfullyAppliedPlots(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "fertilizer-batch-player", "沃土真人")

	seed, err := game.itemByName("凝露草籽")
	if err != nil {
		t.Fatal(err)
	}
	fertilizer, err := game.itemByName("灵壤肥")
	if err != nil {
		t.Fatal(err)
	}
	highTier, err := game.itemByName("地脉灵肥")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, seed.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fertilizer.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, highTier.ID, 1); err != nil {
		t.Fatal(err)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键种植 凝露草籽")); err != nil || !handled || !strings.Contains(result.Title, "完成") {
		t.Fatalf("plant all: handled=%v err=%v result=%+v", handled, err, result)
	}

	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键施肥 地脉灵肥"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "田阶无法承载") || game.itemQuantity(player.ID, highTier.ID) != 1 {
		t.Fatalf("high-tier fertilizer prerequisite: handled=%v err=%v result=%+v", handled, err, blocked)
	}

	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键施肥 灵壤肥"))
	if err != nil || !handled || !strings.Contains(first.Title, "完成") || !strings.Contains(first.Content, "施用：灵壤肥 × 1") || !strings.Contains(first.Content, "尚有1块") {
		t.Fatalf("partial one-click fertilizer: handled=%v err=%v result=%+v", handled, err, first)
	}
	var mansion model.Mansion
	if err := store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	var fertilized int64
	if err := store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND fertilized = ?", mansion.ID, false, true).Count(&fertilized).Error; err != nil {
		t.Fatal(err)
	}
	if fertilized != 1 || game.itemQuantity(player.ID, fertilizer.ID) != 0 {
		t.Fatalf("partial one-click state: fertilized=%d item=%d", fertilized, game.itemQuantity(player.ID, fertilizer.ID))
	}

	if err := game.players.AdjustItem(player.ID, fertilizer.ID, 2); err != nil {
		t.Fatal(err)
	}
	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键施肥 灵壤肥"))
	if err != nil || !handled || !strings.Contains(second.Content, "施用：灵壤肥 × 1") {
		t.Fatalf("remaining one-click fertilizer: handled=%v err=%v result=%+v", handled, err, second)
	}
	if err := store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND fertilized = ?", mansion.ID, false, true).Count(&fertilized).Error; err != nil {
		t.Fatal(err)
	}
	if fertilized != 2 || game.itemQuantity(player.ID, fertilizer.ID) != 1 {
		t.Fatalf("second one-click state: fertilized=%d item=%d", fertilized, game.itemQuantity(player.ID, fertilizer.ID))
	}

	none, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键施肥 灵壤肥"))
	if err != nil || !handled || !strings.Contains(none.Title, "无需") || !strings.Contains(none.Content, "没有扣除") || game.itemQuantity(player.ID, fertilizer.ID) != 1 {
		t.Fatalf("fully fertilized no-op: handled=%v err=%v result=%+v", handled, err, none)
	}
}

func TestFertilizerCatalogIncludesShopRecipesAndCommands(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "fertilizer-catalog-player", "百草居士")

	for _, name := range []string{"灵壤肥", "地脉灵肥", "造化仙壤"} {
		item, err := game.itemByName(name)
		if err != nil || item.EffectFunc != "fertilize_crop" || item.CategoryName != "灵肥" {
			t.Fatalf("fertilizer item %s: item=%+v err=%v", name, item, err)
		}
		var shopCount, recipeCount int64
		if err := store.DB.Model(&model.ShopEntry{}).Where("item_name = ? AND enabled = ? AND purchase_limit = ?", name, true, 0).Count(&shopCount).Error; err != nil {
			t.Fatal(err)
		}
		if err := store.DB.Model(&model.SynthesisRecipe{}).Where("output_name = ? AND enabled = ?", name, true).Count(&recipeCount).Error; err != nil {
			t.Fatal(err)
		}
		if shopCount == 0 || recipeCount == 0 {
			t.Fatalf("fertilizer %s source missing: shop=%d recipe=%d", name, shopCount, recipeCount)
		}
	}

	catalog, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵肥图鉴"))
	if err != nil || !handled || !strings.Contains(catalog.Content, "灵壤肥") || !strings.Contains(catalog.Content, "地脉灵肥") || strings.Contains(catalog.Content, "长内容保留") || !containsAction(catalog.Actions, "灵肥图鉴 2") || !containsAction(catalog.Actions, "一键施肥 灵壤肥") {
		t.Fatalf("fertilizer catalog: handled=%v err=%v result=%+v", handled, err, catalog)
	}
	catalogPageTwo, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵肥图鉴 2"))
	if err != nil || !handled || !strings.Contains(catalogPageTwo.Content, "造化仙壤") || strings.Contains(catalogPageTwo.Content, "长内容保留") {
		t.Fatalf("fertilizer catalog page two: handled=%v err=%v result=%+v", handled, err, catalogPageTwo)
	}
	menu, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙府菜单"))
	if err != nil || !handled || !strings.Contains(menu.Content, "灵肥图鉴") || !strings.Contains(menu.Content, "一键施肥") {
		t.Fatalf("farm menu fertilizer entry: handled=%v err=%v result=%+v", handled, err, menu)
	}
}
