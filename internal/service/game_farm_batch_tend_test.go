package service

import (
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestOneClickWeedingAndPestRemovalAreAtomic(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "batch-tend-player", "锄月散人")
	seed, err := game.itemByName("凝露草籽")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, seed.ID, 2); err != nil {
		t.Fatal(err)
	}
	if planted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键种植 凝露草籽")); err != nil || !handled || !strings.Contains(planted.Title, "完成") {
		t.Fatalf("plant all: handled=%v err=%v result=%+v", handled, err, planted)
	}
	var mansion model.Mansion
	if err := store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		t.Fatal(err)
	}
	var before []model.MansionCrop
	if err := store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Order("plot").Find(&before).Error; err != nil || len(before) != 2 {
		t.Fatalf("crops=%d err=%v", len(before), err)
	}
	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 4); err != nil {
		t.Fatal(err)
	}
	weeded, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键除草"))
	if err != nil || !handled || !strings.Contains(weeded.Title, "完成") || !strings.Contains(weeded.Content, "完成除草：2块") || !strings.Contains(weeded.Content, "共+2株") {
		t.Fatalf("one-click weeding: handled=%v err=%v result=%+v", handled, err, weeded)
	}
	var afterWeeding []model.MansionCrop
	if err := store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Order("plot").Find(&afterWeeding).Error; err != nil {
		t.Fatal(err)
	}
	for index, crop := range afterWeeding {
		if !crop.Weeded || crop.Quantity != before[index].Quantity+1 || before[index].ReadyAt.Sub(crop.ReadyAt) < 5*time.Minute-time.Second {
			t.Fatalf("weeded crop %d before=%+v after=%+v", index, before[index], crop)
		}
	}
	if stamina, _ := game.currentStamina(player.ID); stamina != 2 {
		t.Fatalf("stamina after weeding=%d want=2", stamina)
	}
	again, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键除草"))
	if err != nil || !handled || !strings.Contains(again.Title, "无需") || !strings.Contains(again.Content, "没有扣除体力") {
		t.Fatalf("repeat weeding: handled=%v err=%v result=%+v", handled, err, again)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 1); err != nil {
		t.Fatal(err)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键除虫"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "体力不足") || !strings.Contains(blocked.Content, "没有处理任何") {
		t.Fatalf("insufficient pest removal: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	var unchanged int64
	if err := store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND pest_free = ?", mansion.ID, false, true).Count(&unchanged).Error; err != nil || unchanged != 0 {
		t.Fatalf("pest state changed during blocked batch: count=%d err=%v", unchanged, err)
	}
	if stamina, _ := game.currentStamina(player.ID); stamina != 1 {
		t.Fatalf("blocked batch consumed stamina: %d", stamina)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 2); err != nil {
		t.Fatal(err)
	}
	cleared, handled, err := game.Execute("group", player.AccountID, mustParse(t, "一键除虫"))
	if err != nil || !handled || !strings.Contains(cleared.Content, "完成除虫：2块") {
		t.Fatalf("one-click pest removal: handled=%v err=%v result=%+v", handled, err, cleared)
	}
	if err := store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND pest_free = ?", mansion.ID, false, true).Count(&unchanged).Error; err != nil || unchanged != 2 {
		t.Fatalf("pest removal count=%d err=%v", unchanged, err)
	}
}
