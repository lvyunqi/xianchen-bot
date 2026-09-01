package storage

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func TestReferralMigrationRepairsAccountChangedForSamePlayer(t *testing.T) {
	db, err := gorm.Open(sqliteDialector(filepath.Join(t.TempDir(), "referral-migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(
		&model.Player{}, &model.PlayerValue{}, &model.ReferralCode{}, &model.ReferralBinding{},
		&model.ReferralClaim{}, &model.AccountMigrationCode{},
	); err != nil {
		t.Fatal(err)
	}
	store := &Store{DB: db}
	player := model.Player{AccountID: "old-open-id", DaoName: "迁邀真人"}
	invitee := model.Player{AccountID: "invitee-open-id", DaoName: "受邀真人"}
	if err := db.Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&invitee).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.PlayerValue{PlayerID: player.ID, Key: "activity.invite.code", Value: "XCAAAABC"}
	code := model.ReferralCode{AccountID: player.AccountID, CurrentPlayerID: player.ID, Code: legacy.Value}
	binding := model.ReferralBinding{
		InviteeAccountID: invitee.AccountID, InviteePlayerID: invitee.ID,
		InviterAccountID: player.AccountID, InviterPlayerID: player.ID,
		InvitationCode: code.Code, Rewarded: true,
	}
	claim := model.ReferralClaim{AccountID: player.AccountID, ClaimKey: "activity.companion.1"}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&claim).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	migration := model.AccountMigrationCode{
		PlayerID: player.ID, OldAccountID: player.AccountID, NewAccountID: "new-open-id",
		TokenHash: "referral-migration-token", Status: "used", ExpiresAt: now.Add(time.Hour), UsedAt: &now,
	}
	if err := db.Create(&migration).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&player).Update("account_id", "new-open-id").Error; err != nil {
		t.Fatal(err)
	}

	if err := store.migrateAccountScopedReferrals(); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateAccountScopedReferrals(); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	if err := db.First(&code, code.ID).Error; err != nil || code.AccountID != "new-open-id" || code.Code != "XCAAAABC" || code.CurrentPlayerID != player.ID {
		t.Fatalf("referral code was not preserved: row=%+v err=%v", code, err)
	}
	if err := db.First(&binding, binding.ID).Error; err != nil || binding.InviterAccountID != "new-open-id" || binding.InviteeAccountID != invitee.AccountID || binding.InvitationCode != code.Code || !binding.Rewarded {
		t.Fatalf("referral binding was not preserved: row=%+v err=%v", binding, err)
	}
	if err := db.First(&claim, claim.ID).Error; err != nil || claim.AccountID != "new-open-id" || claim.ClaimKey != "activity.companion.1" {
		t.Fatalf("referral claim was not preserved: row=%+v err=%v", claim, err)
	}
	var count int64
	if err := db.Model(&model.ReferralCode{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("unexpected referral code count=%d err=%v", count, err)
	}
}

func TestReferralMigrationMergesSamePlayerDuplicatesAndPreservesHistory(t *testing.T) {
	db, err := gorm.Open(sqliteDialector(filepath.Join(t.TempDir(), "referral-duplicates.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Player{}, &model.PlayerValue{}, &model.ReferralCode{}, &model.ReferralBinding{},
		&model.ReferralClaim{}, &model.AccountMigrationCode{},
	); err != nil {
		t.Fatal(err)
	}
	store := &Store{DB: db}
	inviter := model.Player{AccountID: "new-open-id", DaoName: "迁邀真人"}
	firstInvitee := model.Player{AccountID: "first-invitee", DaoName: "首位受邀"}
	secondInvitee := model.Player{AccountID: "second-invitee", DaoName: "次位受邀"}
	if err := db.Create(&inviter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&firstInvitee).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&secondInvitee).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.PlayerValue{PlayerID: inviter.ID, Key: "activity.invite.code", Value: "XCAAAOLD"}
	oldCode := model.ReferralCode{AccountID: "old-open-id", CurrentPlayerID: inviter.ID, Code: legacy.Value}
	duplicateCode := model.ReferralCode{AccountID: inviter.AccountID, CurrentPlayerID: inviter.ID, Code: "XCAAANEW"}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&oldCode).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&duplicateCode).Error; err != nil {
		t.Fatal(err)
	}
	bindings := []model.ReferralBinding{
		{InviteeAccountID: firstInvitee.AccountID, InviteePlayerID: firstInvitee.ID, InviterAccountID: "old-open-id", InviterPlayerID: inviter.ID, InvitationCode: oldCode.Code, Rewarded: true},
		{InviteeAccountID: secondInvitee.AccountID, InviteePlayerID: secondInvitee.ID, InviterAccountID: inviter.AccountID, InviterPlayerID: inviter.ID, InvitationCode: duplicateCode.Code, Rewarded: true},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	migration := model.AccountMigrationCode{
		PlayerID: inviter.ID, OldAccountID: "old-open-id", NewAccountID: inviter.AccountID,
		TokenHash: "duplicate-referral-token", Status: "used", ExpiresAt: now.Add(time.Hour), UsedAt: &now,
	}
	if err := db.Create(&migration).Error; err != nil {
		t.Fatal(err)
	}
	claims := []model.ReferralClaim{
		{AccountID: "old-open-id", ClaimKey: "activity.companion.first", CreatedAt: now.Add(-time.Hour)},
		{AccountID: "old-open-id", ClaimKey: "activity.companion.three", CreatedAt: now.Add(-time.Hour)},
		{AccountID: inviter.AccountID, ClaimKey: "activity.companion.first", CreatedAt: now.Add(-30 * time.Minute)},
	}
	if err := db.Create(&claims).Error; err != nil {
		t.Fatal(err)
	}

	if err := store.migrateAccountScopedReferrals(); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateAccountScopedReferrals(); err != nil {
		t.Fatalf("duplicate repair is not idempotent: %v", err)
	}

	var codes []model.ReferralCode
	if err := db.Find(&codes).Error; err != nil || len(codes) != 1 || codes[0].AccountID != inviter.AccountID || codes[0].CurrentPlayerID != inviter.ID || codes[0].Code != legacy.Value {
		t.Fatalf("canonical referral code was not preserved: rows=%+v err=%v", codes, err)
	}
	var repairedBindings []model.ReferralBinding
	if err := db.Order("id ASC").Find(&repairedBindings).Error; err != nil || len(repairedBindings) != 2 {
		t.Fatalf("referral bindings were lost: rows=%+v err=%v", repairedBindings, err)
	}
	for _, row := range repairedBindings {
		if row.InviterAccountID != inviter.AccountID || row.InviterPlayerID != inviter.ID || row.InvitationCode != legacy.Value || !row.Rewarded {
			t.Fatalf("referral binding was not normalized: %+v", row)
		}
	}
	var repairedClaims []model.ReferralClaim
	if err := db.Where("account_id = ?", inviter.AccountID).Order("claim_key ASC").Find(&repairedClaims).Error; err != nil || len(repairedClaims) != 2 {
		t.Fatalf("referral claims were not merged: rows=%+v err=%v", repairedClaims, err)
	}
}

func TestReferralMigrationDoesNotMoveClaimsCreatedAfterOldAccountWasReleased(t *testing.T) {
	db, err := gorm.Open(sqliteDialector(filepath.Join(t.TempDir(), "referral-reused-account.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Player{}, &model.PlayerValue{}, &model.ReferralCode{}, &model.ReferralBinding{},
		&model.ReferralClaim{}, &model.AccountMigrationCode{},
	); err != nil {
		t.Fatal(err)
	}
	store := &Store{DB: db}
	player := model.Player{AccountID: "new-open-id", DaoName: "迁领奖真人"}
	if err := db.Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	usedAt := time.Now().Add(-time.Hour)
	migration := model.AccountMigrationCode{
		PlayerID: player.ID, OldAccountID: "released-open-id", NewAccountID: player.AccountID,
		TokenHash: "claim-cutoff-token", Status: "used", ExpiresAt: usedAt.Add(time.Hour), UsedAt: &usedAt,
	}
	if err := db.Create(&migration).Error; err != nil {
		t.Fatal(err)
	}
	oldClaim := model.ReferralClaim{AccountID: migration.OldAccountID, ClaimKey: "activity.companion.first", CreatedAt: usedAt.Add(-time.Minute)}
	reusedAccountClaim := model.ReferralClaim{AccountID: migration.OldAccountID, ClaimKey: "activity.companion.three", CreatedAt: usedAt.Add(time.Minute)}
	if err := db.Create(&oldClaim).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&reusedAccountClaim).Error; err != nil {
		t.Fatal(err)
	}

	if err := store.migrateAccountScopedReferrals(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&oldClaim, oldClaim.ID).Error; err != nil || oldClaim.AccountID != player.AccountID {
		t.Fatalf("pre-migration claim was not moved: row=%+v err=%v", oldClaim, err)
	}
	if err := db.First(&reusedAccountClaim, reusedAccountClaim.ID).Error; err != nil || reusedAccountClaim.AccountID != migration.OldAccountID {
		t.Fatalf("post-migration claim was stolen from reused account: row=%+v err=%v", reusedAccountClaim, err)
	}
}
