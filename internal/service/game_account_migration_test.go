package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestAccountMigrationPreservesPlayerAndExplainsDatabaseCopyOrder(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "old-open-id", "迁云真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"silver_coins": 321, "spirit_stones": 654, "cultivation": 987,
	}).Error; err != nil {
		t.Fatal(err)
	}
	referralCode := model.ReferralCode{AccountID: player.AccountID, CurrentPlayerID: player.ID, Code: "XCAAAAMG"}
	referralClaim := model.ReferralClaim{AccountID: player.AccountID, ClaimKey: "activity.companion.1"}
	invited := registerPlayer(t, game, "invited-open-id", "同行真人")
	sponsor := registerPlayer(t, game, "sponsor-open-id", "引路真人")
	inviterBinding := model.ReferralBinding{
		InviteeAccountID: invited.AccountID, InviteePlayerID: invited.ID,
		InviterAccountID: player.AccountID, InviterPlayerID: player.ID,
		InvitationCode: referralCode.Code, Rewarded: true,
	}
	inviteeBinding := model.ReferralBinding{
		InviteeAccountID: player.AccountID, InviteePlayerID: player.ID,
		InviterAccountID: sponsor.AccountID, InviterPlayerID: sponsor.ID,
		InvitationCode: "XCAASPON", Rewarded: true,
	}
	if err := store.DB.Create(&referralCode).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&referralClaim).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&inviterBinding).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&inviteeBinding).Error; err != nil {
		t.Fatal(err)
	}
	generated, handled, err := game.Execute("group", player.AccountID, mustParse(t, "生成迁移码"))
	if err != nil || !handled || !strings.Contains(generated.Content, "生成完毕后") || !strings.Contains(generated.Content, "停止旧官机") || !strings.Contains(generated.Content, "不能在复制数据库之后才生成") {
		t.Fatalf("migration generation guide: handled=%v err=%v result=%+v", handled, err, generated)
	}
	token := ""
	for _, line := range strings.Split(generated.Content, "\n") {
		if strings.HasPrefix(line, "迁移凭证：") {
			token = strings.TrimSpace(strings.TrimPrefix(line, "迁移凭证："))
			break
		}
	}
	if token == "" {
		t.Fatalf("migration token missing: %s", generated.Content)
	}
	imported, handled, err := game.importAccountMigration("new-group", "new-open-id", token)
	if err != nil || !handled || !strings.Contains(imported.Title, "迁移成功") {
		t.Fatalf("migration import: handled=%v err=%v result=%+v", handled, err, imported)
	}
	migrated, err := game.players.GetByAccount("new-open-id")
	if err != nil {
		t.Fatal(err)
	}
	if migrated.ID != player.ID || migrated.DaoName != player.DaoName || migrated.SilverCoins != 321 || migrated.SpiritStones != 654 || migrated.Cultivation != 987 || migrated.ServerName != "new-group" {
		t.Fatalf("migrated player changed unexpectedly: %+v", migrated)
	}
	if _, err := game.players.GetByAccount("old-open-id"); err == nil {
		t.Fatal("old OpenID still owns the migrated player")
	}
	if err := store.DB.First(&referralCode, referralCode.ID).Error; err != nil || referralCode.AccountID != "new-open-id" || referralCode.CurrentPlayerID != player.ID || referralCode.Code != "XCAAAAMG" {
		t.Fatalf("account migration lost referral code: row=%+v err=%v", referralCode, err)
	}
	if err := store.DB.First(&referralClaim, referralClaim.ID).Error; err != nil || referralClaim.AccountID != "new-open-id" || referralClaim.ClaimKey != "activity.companion.1" {
		t.Fatalf("account migration lost referral claim: row=%+v err=%v", referralClaim, err)
	}
	if err := store.DB.First(&inviterBinding, inviterBinding.ID).Error; err != nil || inviterBinding.InviterAccountID != "new-open-id" || inviterBinding.InviteeAccountID != invited.AccountID || inviterBinding.InvitationCode != referralCode.Code || !inviterBinding.Rewarded {
		t.Fatalf("account migration lost inviter binding: row=%+v err=%v", inviterBinding, err)
	}
	if err := store.DB.First(&inviteeBinding, inviteeBinding.ID).Error; err != nil || inviteeBinding.InviteeAccountID != "new-open-id" || inviteeBinding.InviterAccountID != sponsor.AccountID || inviteeBinding.InvitationCode != "XCAASPON" || !inviteeBinding.Rewarded {
		t.Fatalf("account migration lost invitee binding: row=%+v err=%v", inviteeBinding, err)
	}
	reused, handled, err := game.importAccountMigration("new-group", "another-open-id", token)
	if err != nil || !handled || !strings.Contains(reused.Title, "已失效") {
		t.Fatalf("used token was not rejected: handled=%v err=%v result=%+v", handled, err, reused)
	}
	missing, handled, err := game.importAccountMigration("new-group", "missing-open-id", "XC-AAAAAAAAAAAAAAAA")
	if err != nil || !handled || !strings.Contains(missing.Title, "未找到") || !strings.Contains(missing.Content, "复制于凭证生成之前") || !strings.Contains(missing.Content, "没有创建、覆盖或改动") {
		t.Fatalf("missing token diagnosis: handled=%v err=%v result=%+v", handled, err, missing)
	}
}
