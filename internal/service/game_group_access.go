package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const (
	groupAccessPending  = "待审核"
	groupAccessApproved = "已通过"
	groupAccessRejected = "已拒绝"
)

func (g *Game) knownGroupIDs() []string {
	var raw string
	if err := g.store.DB.Table("system_settings").Select("value").Where("key = ?", "runtime.known_groups").Scan(&raw).Error; err != nil {
		return nil
	}
	var groups []string
	_ = json.Unmarshal([]byte(raw), &groups)
	return groups
}

func (g *Game) addKnownGroupID(groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	groups := g.knownGroupIDs()
	for _, existing := range groups {
		if existing == groupID {
			return nil
		}
	}
	groups = append(groups, groupID)
	encoded, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	row := model.SystemSetting{Key: "runtime.known_groups", Value: string(encoded), ValueType: "json", Description: "全区通报与群接入审核已通过的QQ群"}
	return g.store.DB.Where("key = ?", row.Key).Assign(map[string]any{"value": row.Value, "value_type": row.ValueType, "description": row.Description}).FirstOrCreate(&row).Error
}

func (g *Game) GroupAccessAllowed(groupID string) (bool, string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" || groupID == "私信" {
		return true, groupAccessApproved, nil
	}
	var request model.GroupAccessRequest
	err := g.store.DB.Where("group_id = ?", groupID).First(&request).Error
	if err == nil {
		return request.Status == groupAccessApproved, request.Status, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, "", err
	}
	for _, known := range g.knownGroupIDs() {
		if known != groupID {
			continue
		}
		now := time.Now()
		request = model.GroupAccessRequest{
			GroupID: groupID, GroupName: "历史已接入群", Purpose: "升级前已由仙尘提供服务",
			Status: groupAccessApproved, ReviewReason: "升级时自动保留既有群，不中断玩家数据与服务", ReviewedBy: "系统迁移", ReviewedAt: &now,
		}
		if err := g.store.DB.Where("group_id = ?", groupID).FirstOrCreate(&request).Error; err != nil {
			return false, "", err
		}
		return true, groupAccessApproved, nil
	}
	return false, "未申请", nil
}

func (g *Game) GroupAccessBlockedResult(groupID, status string) GameResult {
	state := "尚未提交接入申请"
	if status == groupAccessPending {
		state = "申请正在等待主人或管理员审核"
	} else if status == groupAccessRejected {
		state = "上次申请未通过，可补充群名和用途后重新提交"
	}
	return GameResult{
		Title: "🔐 本群尚未开放仙尘",
		Content: fmt.Sprintf("群ID：%s\n状态：%s\n━━━━━━━━━━━\n玩家把官机拉入新群后，需先发送“申请入驻 群名/用途”。审核通过前不会创建新角色、消耗物品、推进任务或发送全区通报。", groupID, state),
		Actions: []string{"申请入驻 群名/用途", "群审核状态", "获取ID"},
	}
}

func (g *Game) submitGroupAccess(player *model.Player, groupID, accountID, raw string) (GameResult, bool, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" || groupID == "私信" {
		return GameResult{Title: "🔐 群接入申请", Content: "该指令只能在准备接入仙尘的新QQ群内发送。", Actions: []string{"群审核状态"}}, true, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GameResult{Title: "🔐 群接入申请", Content: "请输入：`申请入驻 群名/用途`\n例如：申请入驻 青云修仙群/供群友共同游玩仙尘", Actions: []string{"申请入驻 群名/用途", "获取ID"}}, true, nil
	}
	groupName, purpose := raw, "玩家申请将仙尘接入本群"
	for _, separator := range []string{"/", "｜", "|"} {
		if before, after, found := strings.Cut(raw, separator); found {
			groupName, purpose = strings.TrimSpace(before), strings.TrimSpace(after)
			break
		}
	}
	if len([]rune(groupName)) > 128 || len([]rune(purpose)) > 500 {
		return GameResult{Title: "🔐 申请内容过长", Content: "群名最多128字，用途最多500字，请精简后重新提交。", Actions: []string{"申请入驻 群名/用途"}}, true, nil
	}
	applicantID, applicantName := uint(0), "未入道申请人"
	if player != nil {
		applicantID, applicantName = player.ID, player.DaoName
	}
	var existing model.GroupAccessRequest
	err := g.store.DB.Where("group_id = ?", groupID).First(&existing).Error
	if err == nil && existing.Status == groupAccessApproved {
		return GameResult{Title: "🔐 本群已经通过审核", Content: fmt.Sprintf("群ID：%s\n群名：%s\n仙尘游戏功能与全区通报均已开放。", groupID, displayOr(existing.GroupName, groupName)), Actions: []string{"菜单", "运行状态", "群审核状态"}}, true, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}
	updates := map[string]any{
		"group_name": groupName, "applicant_account_id": accountID, "applicant_player_id": applicantID,
		"applicant_name": applicantName, "purpose": purpose, "status": groupAccessPending,
		"review_reason": "", "reviewed_by": "", "reviewed_at": nil,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = model.GroupAccessRequest{GroupID: groupID}
		if err := g.store.DB.Where("group_id = ?", groupID).Assign(updates).FirstOrCreate(&existing).Error; err != nil {
			return GameResult{}, true, err
		}
	} else if err := g.store.DB.Model(&existing).Updates(updates).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{
		Title: "🔐 群接入申请已提交",
		Content: fmt.Sprintf("申请编号：#%d\n群ID：%s\n群名：%s\n申请人：%s\n用途：%s\n状态：待审核\n━━━━━━━━━━━\n主人可在仙尘数据后台的“群接入审核”页面处理，也可发送“群审核 %s 通过/拒绝 原因”。", existing.ID, groupID, groupName, applicantName, purpose, groupID),
		Actions: []string{"群审核状态", "获取ID"},
		BroadcastContent: fmt.Sprintf("【新群接入待审】%s申请接入仙尘，群名%s，群ID %s，申请编号#%d。", applicantName, groupName, groupID, existing.ID),
	}, true, nil
}

func (g *Game) groupAccessStatus(groupID string) (GameResult, bool, error) {
	groupID = strings.TrimSpace(groupID)
	var row model.GroupAccessRequest
	err := g.store.DB.Where("group_id = ?", groupID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "🔐 群接入状态", Content: fmt.Sprintf("群ID：%s\n状态：未申请", groupID), Actions: []string{"申请入驻 群名/用途", "获取ID"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	review := "尚未审核"
	if row.ReviewedAt != nil {
		review = fmt.Sprintf("%s于%s处理", displayOr(row.ReviewedBy, "管理员"), row.ReviewedAt.Format("01-02 15:04"))
	}
	return GameResult{Title: "🔐 群接入状态", Content: fmt.Sprintf("申请编号：#%d\n群ID：%s\n群名：%s\n申请人：%s\n用途：%s\n状态：%s\n审核：%s\n说明：%s", row.ID, row.GroupID, row.GroupName, row.ApplicantName, row.Purpose, row.Status, review, displayOr(row.ReviewReason, "无")), Actions: []string{"申请入驻 群名/用途", "菜单"}}, true, nil
}

func (g *Game) reviewGroupAccess(reviewer, raw string) (GameResult, string, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) < 2 {
		return GameResult{Title: "群审核", Content: "格式：`群审核 群ID/申请编号 通过/拒绝 [原因]`"}, "format_error", nil
	}
	var row model.GroupAccessRequest
	target := strings.TrimPrefix(parts[0], "#")
	if id, err := strconv.ParseUint(target, 10, 64); err == nil {
		if err := g.store.DB.First(&row, id).Error; err != nil {
			return GameResult{Title: "群审核", Content: "没有找到该申请编号。"}, target, nil
		}
	} else if err := g.store.DB.Where("group_id = ?", target).First(&row).Error; err != nil {
		return GameResult{Title: "群审核", Content: "没有找到该群的接入申请。"}, target, nil
	}
	approved := map[string]bool{"通过": true, "同意": true, "批准": true}[parts[1]]
	rejected := map[string]bool{"拒绝": true, "驳回": true}[parts[1]]
	if !approved && !rejected {
		return GameResult{Title: "群审核", Content: "审核结果只能填写“通过”或“拒绝”。"}, row.GroupID, nil
	}
	status := groupAccessApproved
	reason := "符合仙尘群接入规则"
	if rejected {
		status, reason = groupAccessRejected, "本次申请未通过"
	}
	if len(parts) > 2 {
		reason = strings.Join(parts[2:], " ")
	}
	now := time.Now()
	if err := g.store.DB.Model(&row).Updates(map[string]any{"status": status, "review_reason": reason, "reviewed_by": reviewer, "reviewed_at": &now}).Error; err != nil {
		return GameResult{}, row.GroupID, err
	}
	if approved {
		if err := g.addKnownGroupID(row.GroupID); err != nil {
			return GameResult{}, row.GroupID, err
		}
	}
	resultText := "已拒绝，本群仍不能使用游戏功能"
	actions := []string{"群审核状态", "申请入驻 " + row.GroupName + "/补充用途"}
	if approved {
		resultText = "已通过，本群立即开放仙尘游戏与全区通报"
		actions = []string{"菜单", "运行状态", "群审核状态"}
	}
	return GameResult{Title: "🔐 群接入审核完成", Content: fmt.Sprintf("申请编号：#%d\n群ID：%s\n群名：%s\n结果：%s\n审核人：%s\n说明：%s", row.ID, row.GroupID, row.GroupName, resultText, reviewer, reason), Actions: actions, BroadcastContent: fmt.Sprintf("【群接入审核】%s的仙尘接入申请%s。说明：%s", row.GroupName, status, reason)}, row.GroupID, nil
}

