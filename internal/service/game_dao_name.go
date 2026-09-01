package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

type daoNameTransferRequest struct {
	TransferredName string `json:"transferred_name"`
	DonorNewName    string `json:"donor_new_name"`
}

func (g *Game) requestDaoNameTransfer(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) < 2 {
		return GameResult{Title: "转让道号", Content: "格式：`转让道号 对方道号 自己的新道号`\n示例：`转让道号 青玄 云海散人`\n对方接受后获得你当前道号，你改用新道号；手续费500银币。", Actions: []string{"货币", "好友"}}, true, nil
	}
	target, err := g.findPlayer(args[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "转让失败", Content: "没有找到目标道友，不能向自己转让。请填写对方当前道号。"}, true, nil
	}
	newName := strings.TrimSpace(strings.Join(args[1:], " "))
	if invalid := validateDaoName(newName); invalid != "" {
		return GameResult{Title: "新道号格式审核未通过", Content: invalid + "\n转让请求没有发出。"}, true, nil
	}
	if _, _, matched, err := g.matchSensitiveWord(newName); err != nil {
		return GameResult{}, true, err
	} else if matched {
		return GameResult{Title: "新道号审核未通过", Content: "你选择的新道号触发仙盟禁用词，请更换后重试。"}, true, nil
	}
	var existing int64
	if err := g.store.DB.Model(&model.Player{}).Where("dao_name = ?", newName).Count(&existing).Error; err != nil {
		return GameResult{}, true, err
	} else if existing > 0 {
		return GameResult{Title: "新道号已占用", Content: "“" + newName + "”已被其他道友使用，转让请求没有发出。"}, true, nil
	}
	if player.SilverCoins < 500 {
		return GameResult{Title: "转让银币不足", Content: fmt.Sprintf("转让道号需要500银币，当前持有%d。银币可通过每日签到和活动获得。", player.SilverCoins), Actions: []string{"签到", "货币"}}, true, nil
	}
	request := daoNameTransferRequest{TransferredName: player.DaoName, DonorNewName: newName}
	encoded, _ := json.Marshal(request)
	row := model.SocialMessage{SenderID: player.ID, ReceiverID: target.ID, Type: "dao_transfer", Content: string(encoded)}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "道号转让请求已送达", Content: fmt.Sprintf("受让人：%s\n转让道号：%s\n你的新道号：%s\n手续费：500银币（接受时扣除）\n请求二十四小时内有效，对方发送 `接受道号` 完成。", target.DaoName, player.DaoName, newName), Actions: []string{"状态", "货币"}}, true, nil
}

func (g *Game) acceptDaoNameTransfer(player *model.Player) (GameResult, bool, error) {
	var message model.SocialMessage
	err := g.store.DB.Where("receiver_id = ? AND type = ? AND read = ? AND created_at >= ?", player.ID, "dao_transfer", false, time.Now().Add(-24*time.Hour)).Order("id DESC").First(&message).Error
	if err != nil {
		return GameResult{Title: "没有待接道号", Content: "当前没有二十四小时内有效的道号转让请求。", Actions: []string{"好友", "状态"}}, true, nil
	}
	var request daoNameTransferRequest
	if json.Unmarshal([]byte(message.Content), &request) != nil || request.TransferredName == "" || request.DonorNewName == "" {
		return GameResult{Title: "转让请求损坏", Content: "请求内容无法读取，请转让方重新发起。"}, true, nil
	}
	if invalid := validateDaoName(request.DonorNewName); invalid != "" {
		return GameResult{Title: "转让请求未通过复核", Content: "转让方的新道号不符合当前审核规则：" + invalid}, true, nil
	}
	var donor model.Player
	if err := g.store.DB.First(&donor, message.SenderID).Error; err != nil || donor.DaoName != request.TransferredName {
		return GameResult{Title: "转让请求失效", Content: "转让方已经改名或离开，原请求不再有效。"}, true, nil
	}
	oldRecipientName := player.DaoName
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND dao_name = ? AND silver_coins >= ?", donor.ID, request.TransferredName, 500).Updates(map[string]any{"dao_name": request.DonorNewName, "silver_coins": gorm.Expr("silver_coins - 500")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("转让方银币不足或道号已经变化")
		}
		if err := tx.Model(&model.Player{}).Where("id = ? AND dao_name = ?", player.ID, oldRecipientName).Update("dao_name", request.TransferredName).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Couple{}).Where("player_a_id = ?", donor.ID).Update("player_a_name", request.DonorNewName).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Couple{}).Where("player_b_id = ?", donor.ID).Update("player_b_name", request.DonorNewName).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Couple{}).Where("player_a_id = ?", player.ID).Update("player_a_name", request.TransferredName).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Couple{}).Where("player_b_id = ?", player.ID).Update("player_b_name", request.TransferredName).Error; err != nil {
			return err
		}
		return tx.Model(&message).Update("read", true).Error
	})
	if err != nil {
		return GameResult{Title: "道号转让失败", Content: err.Error()}, true, nil
	}
	_ = g.queueContentReview("道号", &donor, request.DonorNewName)
	_ = g.queueContentReview("道号", player, request.TransferredName)
	broadcast := fmt.Sprintf("【道号传承】%s将道号“%s”传予%s，自此改号“%s”；原受让道号“%s”已经释放。", donor.DaoName, request.TransferredName, oldRecipientName, request.DonorNewName, oldRecipientName)
	_ = g.publishWorldBroadcast("道号", request.TransferredName+"完成传承", broadcast)
	return GameResult{Title: "道号传承完成", Content: fmt.Sprintf("你已承接道号：%s\n原道号%s已经释放，可被其他新道友注册。\n转让方新道号：%s\n所有道籍与仙侣显示已经同步更新。", request.TransferredName, oldRecipientName, request.DonorNewName), Actions: []string{"状态", "公告"}, BroadcastContent: broadcast}, true, nil
}
