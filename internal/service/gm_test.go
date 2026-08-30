package service

import (
	"strconv"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func mustParseGM(t *testing.T, message string) GMCommand {
	t.Helper()
	command, ok := ParseGMCommand(message)
	if !ok {
		t.Fatalf("GM command %q did not parse", message)
	}
	return command
}

func setTestOwner(t *testing.T, game *Game, ownerID string) {
	t.Helper()
	if err := game.store.DB.Model(&model.SystemSetting{}).Where("key = ?", "owner.user_id").Update("value", ownerID).Error; err != nil {
		t.Fatal(err)
	}
}

func TestGMUnauthorizedUserStaysSilent(t *testing.T) {
	game, store := testGame(t)
	_, handled, err := game.ExecuteGM("stranger", mustParseGM(t, "天降灵石 100"))
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("unauthorized GM command must stay silent")
	}
	var count int64
	if err := store.DB.Model(&model.OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unauthorized command wrote %d audit records", count)
	}
}

func TestGMOwnerCanGrantResourcesAndItems(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "grant-target", "受赐真人")
	setTestOwner(t, game, "owner-openid")

	result, handled, err := game.ExecuteGM("owner-openid", mustParseGM(t, "天赐灵石 grant-target 250"))
	if err != nil || !handled {
		t.Fatalf("grant spirit stones: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(result.Content, "250") {
		t.Fatalf("unexpected result: %+v", result)
	}
	current, err := game.players.GetByAccount(player.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if current.SpiritStones != player.SpiritStones+250 {
		t.Fatalf("spirit stones=%d want=%d", current.SpiritStones, player.SpiritStones+250)
	}
	levelHealth := current.MaxHealth
	result, handled, err = game.ExecuteGM("owner-openid", mustParseGM(t, "天赐修为 grant-target 100"))
	if err != nil || !handled || !strings.Contains(result.Content, "角色经验同步：+100") || !strings.Contains(result.Content, "LV1→LV2") {
		t.Fatalf("grant cultivation: handled=%v err=%v result=%+v", handled, err, result)
	}
	current, err = game.players.GetByAccount(player.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Cultivation != 100 || current.Level != 2 || current.Experience != 0 || current.MaxHealth <= levelHealth {
		t.Fatalf("granted cultivation did not level player: %+v", current)
	}

	_, handled, err = game.ExecuteGM("owner-openid", mustParseGM(t, "天赐仙物 grant-target 灵果 3"))
	if err != nil || !handled {
		t.Fatalf("grant item: handled=%v err=%v", handled, err)
	}
	item, err := game.itemByName("灵果")
	if err != nil {
		t.Fatal(err)
	}
	var inventory model.PlayerItem
	if err := store.DB.Where("player_id = ? AND item_id = ?", player.ID, item.ID).First(&inventory).Error; err != nil {
		t.Fatal(err)
	}
	if inventory.Quantity != 3 {
		t.Fatalf("item quantity=%d want=3", inventory.Quantity)
	}

	var auditCount int64
	if err := store.DB.Model(&model.OperationLog{}).Where("gm_name = ?", "主人·道祖").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("audit count=%d want=3", auditCount)
	}
}

func TestGMManagerRoleEnforcementAndBanLifecycle(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "ban-target", "守序真人")
	manager := model.ManagerAccount{UserID: "manager-openid", Name: "执法者", Role: "护法", Enabled: true}
	if err := store.DB.Create(&manager).Error; err != nil {
		t.Fatal(err)
	}

	result, handled, err := game.ExecuteGM(manager.UserID, mustParseGM(t, "天罚禁 ban-target"))
	if err != nil || !handled {
		t.Fatalf("low-role command: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(result.Title, "权限不足") {
		t.Fatalf("unexpected permission result: %+v", result)
	}
	current, _ := game.players.GetByAccount(player.AccountID)
	if current.Banned {
		t.Fatal("low-role manager banned the player")
	}

	if err := store.DB.Model(&manager).Update("role", "道祖").Error; err != nil {
		t.Fatal(err)
	}
	_, handled, err = game.ExecuteGM(manager.UserID, mustParseGM(t, "天罚禁 ban-target"))
	if err != nil || !handled {
		t.Fatalf("ban: handled=%v err=%v", handled, err)
	}
	current, _ = game.players.GetByAccount(player.AccountID)
	if !current.Banned {
		t.Fatal("authorized manager did not ban the player")
	}

	_, handled, err = game.ExecuteGM(manager.UserID, mustParseGM(t, "天罚解 ban-target"))
	if err != nil || !handled {
		t.Fatalf("unban: handled=%v err=%v", handled, err)
	}
	current, _ = game.players.GetByAccount(player.AccountID)
	if current.Banned || current.BanReason != "" {
		t.Fatalf("player remains banned: %+v", current)
	}

	var auditCount int64
	if err := store.DB.Model(&model.OperationLog{}).Where("gm_name LIKE ?", "执法者%").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("manager audit count=%d want=3", auditCount)
	}
}

func TestGMRechargeCumulativeTotalItemGrantAndFourPagePriceTable(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "recharge-target", "商会真人")
	setTestOwner(t, game, "owner-recharge")

	commands := []struct {
		text      string
		yuan      string
		broadcast bool
	}{
		{"充值 商会真人 灵石 4000000", "本次累充：+2元", false},
		{"充值 商会真人 仙金 6000", "本次累充：+3元", true},
		{"充值 商会真人 仙金 1999", "本次不计累充", true},
		{"充值 商会真人 银币 500", "本次不计累充", false},
	}
	for _, test := range commands {
		result, handled, err := game.ExecuteGM("owner-recharge", mustParseGM(t, test.text))
		if err != nil || !handled || !strings.Contains(result.Title, "充值成功") || !strings.Contains(result.Content, test.yuan) {
			t.Fatalf("%s: handled=%v err=%v result=%+v", test.text, handled, err, result)
		}
		if (result.BroadcastContent != "") != test.broadcast {
			t.Fatalf("%s broadcast=%q want=%v", test.text, result.BroadcastContent, test.broadcast)
		}
	}

	grant, handled, err := game.ExecuteGM("owner-recharge", mustParseGM(t, "发放道具 商会真人 灵果 12"))
	if err != nil || !handled || !strings.Contains(grant.Content, "灵果 × 12") {
		t.Fatalf("item grant: handled=%v err=%v result=%+v", handled, err, grant)
	}
	fruit, _ := game.itemByName("灵果")
	current, _ := game.players.Get(player.ID)
	if current.SpiritStones != player.SpiritStones+4_000_000 || current.ImmortalJade != player.ImmortalJade+7_999 || current.SilverCoins != player.SilverCoins+500 || game.itemQuantity(player.ID, fruit.ID) != 12 {
		t.Fatalf("recharge or grant persistence mismatch: player=%+v fruit=%d", current, game.itemQuantity(player.ID, fruit.ID))
	}

	total, handled, err := game.Execute("group", player.AccountID, mustParse(t, "累充"))
	if err != nil || !handled || !strings.Contains(total.Content, "累计充值：5元") {
		t.Fatalf("cumulative recharge: handled=%v err=%v result=%+v", handled, err, total)
	}
	for page := 1; page <= 4; page++ {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "氪金菜单 "+strconv.Itoa(page)))
		if err != nil || !handled || !strings.Contains(result.Content, "第"+strconv.Itoa(page)+"/4页") {
			t.Fatalf("price page %d: handled=%v err=%v result=%+v", page, handled, err, result)
		}
		if page > 1 && !containsAction(result.Actions, "氪金菜单 "+strconv.Itoa(page-1)) {
			t.Fatalf("price page %d missing previous action: %+v", page, result.Actions)
		}
		if page < 4 && !containsAction(result.Actions, "氪金菜单 "+strconv.Itoa(page+1)) {
			t.Fatalf("price page %d missing next action: %+v", page, result.Actions)
		}
		if strings.Contains(result.Content, "定制神位") || strings.Contains(result.Content, "定制战舰") {
			t.Fatalf("price page %d exposed removed custom services: %s", page, result.Content)
		}
	}

	var auditCount int64
	if err := store.DB.Model(&model.OperationLog{}).Count(&auditCount).Error; err != nil || auditCount != int64(len(commands)+1) {
		t.Fatalf("recharge audit count=%d err=%v", auditCount, err)
	}
}
