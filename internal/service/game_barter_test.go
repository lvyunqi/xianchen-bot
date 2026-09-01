package service

import (
	"fmt"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestBarterRequiresRecipientConfirmationAndSettlesAtomically(t *testing.T) {
	game, store := testGame(t)
	initiator := registerPlayer(t, game, "barter-initiator", "青衡散人")
	recipient := registerPlayer(t, game, "barter-recipient", "照夜真人")
	offered := model.Item{Code: "test_barter_moon_sand", Name: "月魄灵砂", CategoryName: "材料", RarityName: "凡品", Stackable: true, Tradable: true}
	requested := model.Item{Code: "test_barter_thunder_jade", Name: "雷纹玄玉", CategoryName: "材料", RarityName: "凡品", Stackable: true, Tradable: true}
	if err := store.DB.Create(&offered).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&requested).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(initiator.ID, offered.ID, 4); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(recipient.ID, requested.ID, 5); err != nil {
		t.Fatal(err)
	}

	created, handled, err := game.Execute("group", initiator.AccountID, mustParse(t, "易物 @"+recipient.DaoName+" "+offered.Name+"*2 "+requested.Name+"*3"))
	if err != nil || !handled || !strings.Contains(created.Title, "易物申请已送达") || !strings.Contains(created.Content, "物品尚未扣除") {
		t.Fatalf("create barter request: handled=%v err=%v result=%+v", handled, err, created)
	}
	var request model.BarterRequest
	if err := store.DB.Where("initiator_id = ? AND recipient_id = ?", initiator.ID, recipient.ID).Order("id DESC").First(&request).Error; err != nil {
		t.Fatal(err)
	}
	if request.Status != "待确认" || game.itemQuantity(initiator.ID, offered.ID) != 4 || game.itemQuantity(recipient.ID, requested.ID) != 5 || game.itemQuantity(initiator.ID, requested.ID) != 0 || game.itemQuantity(recipient.ID, offered.ID) != 0 {
		t.Fatalf("barter moved items before consent: request=%+v initiator_offer=%d recipient_request=%d", request, game.itemQuantity(initiator.ID, offered.ID), game.itemQuantity(recipient.ID, requested.ID))
	}

	inbox, handled, err := game.Execute("group", recipient.AccountID, mustParse(t, "通知"))
	if err != nil || !handled || !strings.Contains(inbox.Content, "易物申请") || !containsAction(inbox.Actions, fmt.Sprintf("确认易物 %d", request.ID)) || !containsAction(inbox.Actions, fmt.Sprintf("拒绝易物 %d", request.ID)) {
		t.Fatalf("barter notification: handled=%v err=%v result=%+v", handled, err, inbox)
	}

	accepted, handled, err := game.Execute("group", recipient.AccountID, mustParse(t, fmt.Sprintf("确认易物 %d", request.ID)))
	if err != nil || !handled || !strings.Contains(accepted.Title, "易物成交") {
		t.Fatalf("accept barter: handled=%v err=%v result=%+v", handled, err, accepted)
	}
	if game.itemQuantity(initiator.ID, offered.ID) != 2 || game.itemQuantity(initiator.ID, requested.ID) != 3 || game.itemQuantity(recipient.ID, offered.ID) != 2 || game.itemQuantity(recipient.ID, requested.ID) != 2 {
		t.Fatalf("barter settlement mismatch: initiator=(%d,%d) recipient=(%d,%d)", game.itemQuantity(initiator.ID, offered.ID), game.itemQuantity(initiator.ID, requested.ID), game.itemQuantity(recipient.ID, offered.ID), game.itemQuantity(recipient.ID, requested.ID))
	}
	if err := store.DB.First(&request, request.ID).Error; err != nil || request.Status != "已成交" {
		t.Fatalf("barter request status=%s err=%v", request.Status, err)
	}

	duplicate, handled, err := game.Execute("group", recipient.AccountID, mustParse(t, fmt.Sprintf("确认易物 %d", request.ID)))
	if err != nil || !handled || !strings.Contains(duplicate.Content, "没有重复交换") || game.itemQuantity(initiator.ID, offered.ID) != 2 {
		t.Fatalf("duplicate acceptance was not idempotent: handled=%v err=%v result=%+v", handled, err, duplicate)
	}
}

func TestRejectedBarterDoesNotMoveItems(t *testing.T) {
	game, store := testGame(t)
	initiator := registerPlayer(t, game, "barter-reject-initiator", "临川道人")
	recipient := registerPlayer(t, game, "barter-reject-recipient", "怀玉真君")
	offered := model.Item{Code: "test_barter_cloud_wood", Name: "云纹灵木", CategoryName: "材料", Stackable: true, Tradable: true}
	requested := model.Item{Code: "test_barter_flame_crystal", Name: "离火晶髓", CategoryName: "材料", Stackable: true, Tradable: true}
	if err := store.DB.Create(&offered).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&requested).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(initiator.ID, offered.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(recipient.ID, requested.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := game.Execute("group", initiator.AccountID, mustParse(t, "易物 @"+recipient.DaoName+" "+offered.Name+" "+requested.Name)); err != nil {
		t.Fatal(err)
	}
	var request model.BarterRequest
	if err := store.DB.Order("id DESC").First(&request).Error; err != nil {
		t.Fatal(err)
	}
	rejected, handled, err := game.Execute("group", recipient.AccountID, mustParse(t, fmt.Sprintf("拒绝易物 %d", request.ID)))
	if err != nil || !handled || !strings.Contains(rejected.Content, "均未扣除") {
		t.Fatalf("reject barter: handled=%v err=%v result=%+v", handled, err, rejected)
	}
	if game.itemQuantity(initiator.ID, offered.ID) != 1 || game.itemQuantity(recipient.ID, requested.ID) != 1 || game.itemQuantity(initiator.ID, requested.ID) != 0 || game.itemQuantity(recipient.ID, offered.ID) != 0 {
		t.Fatal("rejected barter changed inventory")
	}
}
