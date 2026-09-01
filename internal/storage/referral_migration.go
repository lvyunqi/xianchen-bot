package storage

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func (s *Store) migrateAccountScopedReferrals() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := normalizeReferralAccountOwnershipTx(tx); err != nil {
			return err
		}
		return migrateLegacyReferralValuesTx(tx)
	})
}

func migrateLegacyReferralValuesTx(tx *gorm.DB) error {
	var codeRows []model.PlayerValue
	if err := tx.Where("key = ?", "activity.invite.code").Find(&codeRows).Error; err != nil {
		return err
	}
	for _, value := range codeRows {
		var player model.Player
		if tx.Unscoped().First(&player, value.PlayerID).Error != nil || player.AccountID == "" || value.Value == "" {
			continue
		}
		legacyCode := strings.ToUpper(strings.TrimSpace(value.Value))
		var row model.ReferralCode
		err := tx.Where("current_player_id = ?", player.ID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = tx.Where("account_id = ?", player.AccountID).First(&row).Error
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = tx.Where("code = ?", legacyCode).First(&row).Error
			if err == nil && row.CurrentPlayerID != player.ID {
				return fmt.Errorf("referral code %s belongs to player %d, cannot assign to player %d", legacyCode, row.CurrentPlayerID, player.ID)
			}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = model.ReferralCode{AccountID: player.AccountID, CurrentPlayerID: player.ID, Code: legacyCode}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"account_id": player.AccountID, "current_player_id": player.ID}).Error; err != nil {
			return err
		}
		if row.Code != legacyCode {
			if err := tx.Model(&value).Update("value", row.Code).Error; err != nil {
				return err
			}
		}
	}
	var bindings []model.PlayerValue
	if err := tx.Where("key = ?", "activity.invite.inviter").Find(&bindings).Error; err != nil {
		return err
	}
	for _, value := range bindings {
		inviterID, err := strconv.ParseUint(value.Value, 10, 64)
		if err != nil || inviterID == 0 {
			continue
		}
		var invitee, inviter model.Player
		if tx.Unscoped().First(&invitee, value.PlayerID).Error != nil || tx.Unscoped().First(&inviter, uint(inviterID)).Error != nil {
			continue
		}
		var code model.ReferralCode
		_ = tx.Where("account_id = ?", inviter.AccountID).First(&code).Error
		row := model.ReferralBinding{
			InviteeAccountID: invitee.AccountID, InviteePlayerID: invitee.ID,
			InviterAccountID: inviter.AccountID, InviterPlayerID: inviter.ID,
			InvitationCode: code.Code, Rewarded: true,
		}
		if err := tx.Where("invitee_account_id = ?", invitee.AccountID).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	var claims []model.PlayerValue
	if err := tx.Where("key LIKE ?", "activity.companion.%").Find(&claims).Error; err != nil {
		return err
	}
	for _, value := range claims {
		var player model.Player
		if tx.Unscoped().First(&player, value.PlayerID).Error != nil || player.AccountID == "" {
			continue
		}
		row := model.ReferralClaim{AccountID: player.AccountID, ClaimKey: value.Key}
		if err := tx.Where("account_id = ? AND claim_key = ?", row.AccountID, row.ClaimKey).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// normalizeReferralAccountOwnershipTx repairs rows left behind by an OpenID
// migration performed before referral data was moved in the same transaction.
func normalizeReferralAccountOwnershipTx(tx *gorm.DB) error {
	var codes []model.ReferralCode
	if err := tx.Order("id ASC").Find(&codes).Error; err != nil {
		return err
	}
	codesByPlayer := make(map[uint][]model.ReferralCode)
	playerIDs := make([]uint, 0)
	for _, row := range codes {
		if row.CurrentPlayerID == 0 {
			continue
		}
		if _, exists := codesByPlayer[row.CurrentPlayerID]; !exists {
			playerIDs = append(playerIDs, row.CurrentPlayerID)
		}
		codesByPlayer[row.CurrentPlayerID] = append(codesByPlayer[row.CurrentPlayerID], row)
	}
	sort.Slice(playerIDs, func(i, j int) bool { return playerIDs[i] < playerIDs[j] })
	for _, playerID := range playerIDs {
		var player model.Player
		if tx.Unscoped().First(&player, playerID).Error != nil || strings.TrimSpace(player.AccountID) == "" {
			continue
		}
		rows := codesByPlayer[playerID]
		canonical := rows[0]
		var legacy model.PlayerValue
		if err := tx.Where("player_id = ? AND key = ?", playerID, "activity.invite.code").First(&legacy).Error; err == nil {
			legacyCode := strings.ToUpper(strings.TrimSpace(legacy.Value))
			for _, row := range rows {
				if strings.EqualFold(strings.TrimSpace(row.Code), legacyCode) {
					canonical = row
					break
				}
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var target model.ReferralCode
		if err := tx.Where("account_id = ?", player.AccountID).First(&target).Error; err == nil {
			if target.CurrentPlayerID != playerID {
				return fmt.Errorf("referral account %s belongs to player %d, cannot assign to player %d", player.AccountID, target.CurrentPlayerID, playerID)
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		for _, row := range rows {
			if row.ID == canonical.ID {
				continue
			}
			if row.Code != canonical.Code {
				if err := tx.Model(&model.ReferralBinding{}).Where("invitation_code = ?", row.Code).Update("invitation_code", canonical.Code).Error; err != nil {
					return err
				}
			}
			if err := tx.Delete(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&canonical).Updates(map[string]any{"account_id": player.AccountID, "current_player_id": playerID}).Error; err != nil {
			return err
		}
	}

	var bindings []model.ReferralBinding
	if err := tx.Order("id ASC").Find(&bindings).Error; err != nil {
		return err
	}
	bindingsByInvitee := make(map[uint][]model.ReferralBinding)
	inviteeIDs := make([]uint, 0)
	for _, row := range bindings {
		if row.InviteePlayerID == 0 {
			continue
		}
		if _, exists := bindingsByInvitee[row.InviteePlayerID]; !exists {
			inviteeIDs = append(inviteeIDs, row.InviteePlayerID)
		}
		bindingsByInvitee[row.InviteePlayerID] = append(bindingsByInvitee[row.InviteePlayerID], row)
	}
	sort.Slice(inviteeIDs, func(i, j int) bool { return inviteeIDs[i] < inviteeIDs[j] })
	for _, inviteeID := range inviteeIDs {
		var invitee model.Player
		if tx.Unscoped().First(&invitee, inviteeID).Error != nil || strings.TrimSpace(invitee.AccountID) == "" {
			continue
		}
		rows := bindingsByInvitee[inviteeID]
		canonical := rows[0]
		var legacy model.PlayerValue
		if err := tx.Where("player_id = ? AND key = ?", inviteeID, "activity.invite.inviter").First(&legacy).Error; err == nil {
			legacyInviterID, _ := strconv.ParseUint(strings.TrimSpace(legacy.Value), 10, 64)
			for _, row := range rows {
				if row.InviterPlayerID == uint(legacyInviterID) {
					canonical = row
					break
				}
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var target model.ReferralBinding
		if err := tx.Where("invitee_account_id = ?", invitee.AccountID).First(&target).Error; err == nil {
			if target.InviteePlayerID != inviteeID {
				return fmt.Errorf("referral invitee account %s belongs to player %d, cannot assign to player %d", invitee.AccountID, target.InviteePlayerID, inviteeID)
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		rewarded := canonical.Rewarded
		for _, row := range rows {
			if row.Rewarded {
				rewarded = true
			}
			if row.ID != canonical.ID {
				if err := tx.Delete(&row).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&canonical).Updates(map[string]any{
			"invitee_account_id": invitee.AccountID,
			"invitee_player_id":  inviteeID,
			"rewarded":           rewarded,
		}).Error; err != nil {
			return err
		}
	}

	bindings = nil
	if err := tx.Find(&bindings).Error; err != nil {
		return err
	}
	for _, row := range bindings {
		if row.InviterPlayerID == 0 {
			continue
		}
		var inviter model.Player
		if tx.Unscoped().First(&inviter, row.InviterPlayerID).Error == nil && strings.TrimSpace(inviter.AccountID) != "" && inviter.AccountID != row.InviterAccountID {
			if err := tx.Model(&row).Update("inviter_account_id", inviter.AccountID).Error; err != nil {
				return err
			}
		}
	}

	var migrations []model.AccountMigrationCode
	if err := tx.Where("status = ? AND new_account_id <> '' AND used_at IS NOT NULL", "used").Order("used_at ASC, id ASC").Find(&migrations).Error; err != nil {
		return err
	}
	for _, migration := range migrations {
		var player model.Player
		if migration.UsedAt == nil || tx.Unscoped().First(&player, migration.PlayerID).Error != nil || player.AccountID == "" || migration.OldAccountID == "" || migration.OldAccountID == player.AccountID {
			continue
		}
		var oldClaims []model.ReferralClaim
		if err := tx.Where("account_id = ? AND created_at <= ?", migration.OldAccountID, *migration.UsedAt).Find(&oldClaims).Error; err != nil {
			return err
		}
		for _, claim := range oldClaims {
			var duplicate int64
			if err := tx.Model(&model.ReferralClaim{}).Where("account_id = ? AND claim_key = ?", player.AccountID, claim.ClaimKey).Count(&duplicate).Error; err != nil {
				return err
			}
			if duplicate > 0 {
				if err := tx.Delete(&claim).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Model(&claim).Update("account_id", player.AccountID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
