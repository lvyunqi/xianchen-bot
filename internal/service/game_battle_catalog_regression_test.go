package service

import (
	"encoding/json"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestNormalHuntSettlementIsSingleUseAndImmediateRechallengeCostsNothing(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "hunt-settlement-player", "止戈真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"physical_attack": 100_000, "magic_attack": 100_000,
	}).Error; err != nil {
		t.Fatal(err)
	}
	started, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挑战 东洲·青云山脚妖灵"))
	if err != nil || !handled || !strings.Contains(started.Title, "妖兽挑战开始") {
		t.Fatalf("start hunt: handled=%v err=%v result=%+v", handled, err, started)
	}
	raw, err := game.playerValue(player.ID, "pve.battle")
	if err != nil {
		t.Fatal(err)
	}
	var state mapMonsterBattleState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatal(err)
	}
	won, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || !strings.Contains(won.Title, "猎妖胜利") || !strings.Contains(won.Content, "收势：8秒") || containsAction(won.Actions, "挑战 "+state.EnemyName) {
		t.Fatalf("finish hunt: handled=%v err=%v result=%+v", handled, err, won)
	}
	settledAgain, err := game.claimNormalMonsterBattleSettlement(player.ID, state)
	if err != nil || settledAgain {
		t.Fatalf("settled battle could be claimed twice: settled=%v err=%v", settledAgain, err)
	}
	staminaBefore, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挑战 东洲·青云山脚妖灵"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "妖息尚未平复") || !strings.Contains(blocked.Content, "没有扣除体力") {
		t.Fatalf("immediate rechallenge: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	staminaAfter, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if staminaAfter != staminaBefore {
		t.Fatalf("blocked rechallenge changed stamina: before=%d after=%d", staminaBefore, staminaAfter)
	}
}

func TestAttackWhileCultivatingRequestsExitInsteadOfArenaMatch(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "cultivating-attack-player", "静修真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("state", model.PlayerStateCultivating).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || !strings.Contains(result.Title, "当前无法出手") || !strings.Contains(result.Content, "修炼中") || !containsAction(result.Actions, "出关") || strings.Contains(result.Title, "尚未匹配") {
		t.Fatalf("cultivating attack guidance: handled=%v err=%v result=%+v", handled, err, result)
	}
}

func TestGeneratedCategoryCatalogAliasesRouteToRequestedPage(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "catalog-alias-player", "阅藏真人")
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙缘奇遇图鉴 200"))
	if err != nil || !handled || !strings.Contains(result.Title, "仙缘奇遇图鉴") || !strings.Contains(result.Content, "古洞逢仙缘") {
		t.Fatalf("generated catalog alias: handled=%v err=%v result=%+v", handled, err, result)
	}
}
