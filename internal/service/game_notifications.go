package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xianlv/internal/model"
)

var personalNotificationTypes = []string{
	"notification", "message", "whisper", "encounter",
	"couple_request", "dissolve_request", "guard_request", "dao_transfer",
	"barter_request",
}

var passiveNotificationTypes = []string{"notification", "message", "whisper", "encounter"}

func (g *Game) notificationInbox(player *model.Player, raw string, unreadOnly bool) (GameResult, bool, error) {
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.SocialMessage{}).Where("receiver_id = ? AND type IN ?", player.ID, personalNotificationTypes)
	if unreadOnly {
		query = query.Where("read = ?", false)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	var unread int64
	if err := g.store.DB.Model(&model.SocialMessage{}).Where("receiver_id = ? AND type IN ? AND read = ?", player.ID, personalNotificationTypes, false).Count(&unread).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	type notificationRow struct {
		ID         uint
		SenderName string
		Type       string
		Content    string
		Read       bool
		CreatedAt  time.Time
	}
	var rows []notificationRow
	rowsQuery := g.store.DB.Table("social_messages AS messages").
		Select("messages.id, players.dao_name AS sender_name, messages.type, messages.content, messages.read, messages.created_at").
		Joins("LEFT JOIN players ON players.id = messages.sender_id").
		Where("messages.receiver_id = ? AND messages.type IN ?", player.ID, personalNotificationTypes)
	if unreadOnly {
		rowsQuery = rowsQuery.Where("messages.read = ?", false)
	}
	if err := rowsQuery.Order("messages.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("共%d条 · %d条未读 · 第%d/%d页", total, unread, page, pages), "━━━━━━━━━━━"}
	passiveIDs := make([]uint, 0, len(rows))
	actions := []string{"通知", "通知未读", "留言", "清理已读通知"}
	seenActions := make(map[string]struct{})
	for _, row := range rows {
		mark := ""
		if !row.Read {
			mark = "【未读】"
		}
		label, content, rowActions := formatPersonalNotification(row.Type, row.SenderName, row.Content)
		lines = append(lines, fmt.Sprintf("%s%s · %s\n%s", mark, label, row.CreatedAt.Format("01-02 15:04"), content), "━━━━━━━")
		if containsText(passiveNotificationTypes, row.Type) {
			passiveIDs = append(passiveIDs, row.ID)
		}
		for _, action := range rowActions {
			if _, exists := seenActions[action]; !exists {
				seenActions[action] = struct{}{}
				actions = append(actions, action)
			}
		}
	}
	if len(rows) == 0 {
		message := "通知信箱空空如也。个人留言、仙缘请求、道号转让和系统事件会汇集在这里。"
		if unreadOnly {
			message = "当前没有未读通知。"
		}
		lines = append(lines, message)
	}
	if len(passiveIDs) > 0 {
		if err := g.store.DB.Model(&model.SocialMessage{}).Where("id IN ?", passiveIDs).Update("read", true).Error; err != nil {
			return GameResult{}, true, err
		}
	}
	baseCommand := "通知"
	if unreadOnly {
		baseCommand = "通知未读"
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", baseCommand, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", baseCommand, page+1))
	}
	return GameResult{Title: "📨 通知信箱", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func formatPersonalNotification(kind, senderName, content string) (string, string, []string) {
	senderName = displayOr(senderName, "仙盟")
	switch kind {
	case "message":
		return "【留言】" + senderName, content, []string{"留言 @" + senderName + " "}
	case "whisper":
		return "【密语】" + senderName, content, []string{"密语 @" + senderName + " "}
	case "encounter":
		return "【奇遇】", content, []string{"探索"}
	case "couple_request":
		return "【仙缘】" + senderName, senderName + "向你递来同心玉，请决定是否应缘。", []string{"应缘", "寻缘"}
	case "dissolve_request":
		return "【仙缘变故】" + senderName, senderName + "提出解除仙侣道契，请前往道侣菜单处理。", []string{"道侣菜单", "心意"}
	case "guard_request":
		return "【护法请求】" + senderName, content, []string{"护法 @" + senderName}
	case "dao_transfer":
		var request daoNameTransferRequest
		if json.Unmarshal([]byte(content), &request) == nil && request.TransferredName != "" {
			content = fmt.Sprintf("%s希望将道号“%s”传承给你；其承接的新道号为“%s”。请求二十四小时内有效。", senderName, request.TransferredName, request.DonorNewName)
		}
		return "【道号传承】" + senderName, content, []string{"接受道号"}
	case "barter_request":
		var request barterNotificationPayload
		if json.Unmarshal([]byte(content), &request) != nil || request.RequestID == 0 {
			return "【易物申请】" + senderName, "收到一笔无法解析的旧版易物申请，请让对方重新发起。", []string{"易物请求"}
		}
		content = fmt.Sprintf("%s愿交出%s×%d，换取你的%s×%d。确认前不会扣除物品；申请成交时会再次校验双方背包。", senderName, request.OfferedItemName, request.OfferedQuantity, request.RequestedItemName, request.RequestedQuantity)
		return "【易物申请】" + senderName, content, []string{fmt.Sprintf("确认易物 %d", request.RequestID), fmt.Sprintf("拒绝易物 %d", request.RequestID), "易物请求"}
	default:
		return "【系统通知】", content, nil
	}
}

func (g *Game) clearReadNotifications(player *model.Player) (GameResult, bool, error) {
	result := g.store.DB.Where("receiver_id = ? AND type IN ? AND read = ?", player.ID, passiveNotificationTypes, true).Delete(&model.SocialMessage{})
	if result.Error != nil {
		return GameResult{}, true, result.Error
	}
	return GameResult{Title: "已读通知已清理", Content: fmt.Sprintf("已清理%d条已读普通通知。\n待处理的结缘、护法、解缘与道号转让请求不会被误删。", result.RowsAffected), Actions: []string{"通知", "通知未读"}}, true, nil
}

func (g *Game) createPlayerNotification(playerID uint, category, content string) error {
	if playerID == 0 || strings.TrimSpace(content) == "" {
		return nil
	}
	content = strings.TrimSpace(content)
	if strings.TrimSpace(category) != "" {
		content = "【" + strings.TrimSpace(category) + "】" + content
	}
	return g.social.Create(&model.SocialMessage{ReceiverID: playerID, Type: "notification", Content: content, Read: false})
}
