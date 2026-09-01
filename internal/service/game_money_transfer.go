package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) transferCurrency(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) < 2 || len(parts) > 3 {
		return GameResult{Title: "🏦 道友转账", Content: "格式：`转账 对方道号 数量`（默认银币）或 `转账 对方道号 银币/灵石 数量`。\n仙金不可由玩家互转。", Actions: []string{"钱庄", "好友", "钱庄账簿"}}, true, nil
	}
	target, err := g.findPlayer(parts[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🏦 收款道友无效", Content: "请填写另一名现存道友的全服唯一道号。", Actions: []string{"好友", "钱庄"}}, true, nil
	}
	currency := "银币"
	amountText := parts[1]
	if len(parts) == 3 {
		currency, amountText = parts[1], parts[2]
	}
	if currency != "银币" && currency != "灵石" {
		return GameResult{Title: "🏦 币种不可转账", Content: "玩家转账只支持银币和灵石。仙金属于充值货币，不能转让。", Actions: []string{"货币", "钱庄规则"}}, true, nil
	}
	amount, err := strconv.ParseInt(amountText, 10, 64)
	if err != nil || amount <= 0 {
		return GameResult{Title: "🏦 转账数量错误", Content: "转账数量必须是正整数。"}, true, nil
	}
	column := "silver_coins"
	balance, targetBalance := player.SilverCoins, target.SilverCoins
	if currency == "灵石" {
		column, balance, targetBalance = "spirit_stones", player.SpiritStones, target.SpiritStones
	}
	if balance < amount {
		return GameResult{Title: "🏦 转账余额不足", Content: fmt.Sprintf("当前随身%s%d，无法转出%d。钱庄存款需先取出。", currency, balance, amount), Actions: []string{"取款 " + currency + " " + amountText, "钱庄", "货币"}}, true, nil
	}
	if targetBalance > math.MaxInt64-amount {
		return GameResult{Title: "🏦 对方余额已满", Content: "收款方余额接近安全上限，本次没有扣款。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND "+column+" >= ?", player.ID, amount).Update(column, gorm.Expr(column+" - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", target.ID).Update(column, gorm.Expr(column+" + ?", amount)).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.BankTransaction{PlayerID: player.ID, Type: "转出", Currency: currency, Amount: amount, BalanceAfter: balance - amount, Description: "转给" + target.DaoName}).Error; err != nil {
			return err
		}
		return tx.Create(&model.BankTransaction{PlayerID: target.ID, Type: "转入", Currency: currency, Amount: amount, BalanceAfter: targetBalance + amount, Description: "来自" + player.DaoName}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.ID, "钱庄转账", fmt.Sprintf("%s向你转入%s×%d，当前随身余额%d。", player.DaoName, currency, amount, targetBalance+amount))
	return GameResult{Title: "🏦 转账完成", Content: fmt.Sprintf("收款道友：%s\n转出：%s×%d\n你的随身余额：%d\n双方钱庄账簿均已记录，对方通知信箱已收到凭据。", target.DaoName, currency, amount, balance-amount), Actions: []string{"钱庄账簿", "转账 " + target.DaoName + " " + currency + " ", "通知", "钱庄"}}, true, nil
}

func (g *Game) sellItemToAlliance(player *model.Player, raw string) (GameResult, bool, error) {
	name, quantity, err := parseStackQuantity(raw)
	if err != nil || name == "" {
		return GameResult{Title: "🏪 仙盟回收", Content: "请输入：`出售 物品名` 或 `出售 物品名*数量`。\n装备请使用“装备分解”；充值、礼包、绑定与任务物品不可回收。", Actions: []string{"背包", "装备分解", "集市"}}, true, nil
	}
	var item model.Item
	if g.store.DB.Where("name = ?", name).First(&item).Error != nil {
		return GameResult{Title: "🏪 物品未收录", Content: "物品图鉴中没有“" + name + "”。", Actions: []string{"物品图鉴", "背包"}}, true, nil
	}
	var owned model.PlayerItem
	if g.store.DB.Where("player_id = ? AND item_id = ?", player.ID, item.ID).First(&owned).Error != nil || owned.Quantity < quantity {
		return GameResult{Title: "🏪 持有数量不足", Content: fmt.Sprintf("需要%s×%d，当前持有%d。", name, quantity, max64(owned.Quantity, 0)), Actions: []string{"背包", "物品 " + name}}, true, nil
	}
	protected := owned.Bound || !item.Tradable || item.CategoryName == "礼包" || item.CategoryName == "任务物品" || item.EffectFunc == "open_gift_pack" || strings.HasPrefix(item.EffectFunc, "customize_") || strings.Contains(item.EffectType, "付费")
	if protected {
		return GameResult{Title: "🏪 仙盟拒绝回收", Content: name + "属于绑定、礼包、任务或充值类道具，为避免误售不能交给系统。可查看物品详情确认用途。", Actions: []string{"物品 " + name, "背包"}}, true, nil
	}
	unitPrice := max64(item.BaseValue*60/100, 1)
	if quantity > math.MaxInt64/unitPrice {
		return GameResult{Title: "🏪 数量过大", Content: "回收总价超过安全范围，请拆分出售。"}, true, nil
	}
	total := unitPrice * quantity
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -quantity); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("spirit_stones", gorm.Expr("spirit_stones + ?", total)).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏪 仙盟回收完成", Content: fmt.Sprintf("交付：%s×%d\n图鉴基础价值：%d/件\n回收价：%d灵石/件（六成）\n获得：灵石×%d\n背包剩余：%d", name, quantity, item.BaseValue, unitPrice, total, owned.Quantity-quantity), Actions: []string{"背包", "物品 " + name, "仙盟回收 ", "集市", "货币"}}, true, nil
}
