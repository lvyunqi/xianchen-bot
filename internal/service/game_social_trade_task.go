package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeSocial(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 81:
		return g.broadcast(player, command.RawArguments)
	case 82:
		return g.whisper(player, command.Arguments)
	case 83:
		return g.friendList(player)
	case 84:
		return g.addFriend(player, command.RawArguments)
	case 85:
		return g.apprentice(player, command.RawArguments)
	case 86:
		return g.acceptDisciple(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) broadcast(player *model.Player, content string) (GameResult, bool, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return GameResult{Title: "全服传音", Content: "请输入：`传音 内容`"}, true, nil
	}
	if rejected, blocked, err := g.rejectSensitiveContent("传音", player, content); err != nil || blocked {
		return rejected, true, err
	}
	duration := time.Duration(g.settingInt("broadcast.cooldown_seconds", 60)) * time.Second
	remaining, ok, err := g.cooldown(player.ID, "broadcast", duration)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "传音冷却", Content: "还需" + formatDuration(remaining) + "。"}, true, nil
	}
	row := model.Broadcast{Content: content, Level: "玩家", CreatedBy: player.DaoName}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.queueContentReview("传音", player, content)
	return GameResult{Title: "全服传音 · " + player.DaoName, Content: content}, true, nil
}

func (g *Game) whisper(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) < 2 {
		return GameResult{Title: "私密传音", Content: "请输入：`密语 @对方 内容`"}, true, nil
	}
	target, err := g.findPlayer(args[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "传音失败", Content: "目标不存在。"}, true, nil
	}
	content := strings.Join(args[1:], " ")
	if rejected, blocked, err := g.rejectSensitiveContent("密语", player, content); err != nil || blocked {
		return rejected, true, err
	}
	message := model.SocialMessage{SenderID: player.ID, ReceiverID: target.ID, Type: "whisper", Content: content}
	if err := g.social.Create(&message); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "密语已送达", Content: fmt.Sprintf("收信人：%s\n内容：%s", target.DaoName, content)}, true, nil
}

func (g *Game) friendList(player *model.Player) (GameResult, bool, error) {
	type friendRow struct {
		AccountID string
		DaoName   string
		RealmName string
		Online    bool
		Intimacy  int64
	}
	var rows []friendRow
	err := g.store.DB.Table("friendships").Select("players.account_id, players.dao_name, players.realm_name, players.online, friendships.intimacy").Joins("JOIN players ON players.id = friendships.friend_id").Where("friendships.player_id = ? AND friendships.status = ?", player.ID, "正常").Order("friendships.intimacy DESC").Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "好友", Content: "好友列表为空。", Actions: []string{"寻缘"}}, true, nil
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		state := "离线"
		if row.Online {
			state = "在线"
		}
		lines = append(lines, fmt.Sprintf("- %s · %s · %s · 亲密%d", row.DaoName, row.RealmName, state, row.Intimacy))
	}
	return GameResult{Title: "好友列表", Content: strings.Join(lines, "\n")}, true, nil
}

func (g *Game) addFriend(player *model.Player, argument string) (GameResult, bool, error) {
	target, err := g.findPlayer(argument)
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "添加好友", Content: "请输入：`加友 @对方`"}, true, nil
	}
	var reverse model.Friendship
	reverseErr := g.store.DB.Where("player_id = ? AND friend_id = ? AND status = ?", target.ID, player.ID, "待确认").First(&reverse).Error
	if reverseErr == nil {
		err = g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&reverse).Update("status", "正常").Error; err != nil {
				return err
			}
			mine := model.Friendship{PlayerID: player.ID, FriendID: target.ID, Status: "正常"}
			return tx.Where("player_id = ? AND friend_id = ?", player.ID, target.ID).FirstOrCreate(&mine).Error
		})
		if err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "好友已添加", Content: "你与" + target.DaoName + "互为好友。"}, true, nil
	}
	request := model.Friendship{PlayerID: player.ID, FriendID: target.ID, Status: "待确认"}
	if err := g.store.DB.Where("player_id = ? AND friend_id = ?", player.ID, target.ID).FirstOrCreate(&request).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "好友请求", Content: fmt.Sprintf("已向%s发送好友请求。\n对方使用 `加友 %s` 即可互相确认。", target.DaoName, player.AccountID)}, true, nil
}

func (g *Game) apprentice(player *model.Player, argument string) (GameResult, bool, error) {
	master, err := g.findPlayer(argument)
	if err != nil || master.ID == player.ID {
		return GameResult{Title: "拜师", Content: "请输入：`拜师 @对方`"}, true, nil
	}
	if master.RealmID <= player.RealmID {
		return GameResult{Title: "拜师失败", Content: "师父境界必须高于徒弟。"}, true, nil
	}
	var existing int64
	g.store.DB.Model(&model.Mentorship{}).Where("disciple_id = ? AND status = ?", player.ID, "正常").Count(&existing)
	if existing > 0 {
		return GameResult{Title: "已有师承", Content: "你已经拜入他人门下。"}, true, nil
	}
	row := model.Mentorship{MasterID: master.ID, DiscipleID: player.ID, Status: "待确认"}
	if err := g.store.DB.Where("master_id = ? AND disciple_id = ?", master.ID, player.ID).FirstOrCreate(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "拜师请求", Content: fmt.Sprintf("你向%s行拜师礼。\n对方发送 `收徒 %s` 后正式建立师承。", master.DaoName, player.AccountID)}, true, nil
}

func (g *Game) acceptDisciple(player *model.Player, argument string) (GameResult, bool, error) {
	disciple, err := g.findPlayer(argument)
	if err != nil || disciple.ID == player.ID {
		return GameResult{Title: "收徒", Content: "请输入：`收徒 @对方`"}, true, nil
	}
	var row model.Mentorship
	if err := g.store.DB.Where("master_id = ? AND disciple_id = ? AND status = ?", player.ID, disciple.ID, "待确认").First(&row).Error; err != nil {
		return GameResult{Title: "收徒失败", Content: "没有该玩家的拜师请求。"}, true, nil
	}
	_ = g.store.DB.Model(&row).Update("status", "正常").Error
	return GameResult{Title: "师徒礼成", Content: fmt.Sprintf("%s正式拜入%s门下。", disciple.DaoName, player.DaoName)}, true, nil
}

func (g *Game) executeTrade(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 87:
		return g.createListing(player, command.Arguments)
	case 88:
		return g.buyListing(player, command.Arguments)
	case 89:
		return g.marketList(player)
	case 90:
		return g.barter(player, command.Arguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) createListing(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) < 2 {
		return GameResult{Title: "🏪 摆摊", Content: "请输入：`摆摊 物品名*数量 单价`，例如 `摆摊 灵果*99 5`。\n未写数量时默认上架一件；数量不设玩法上限，但不能超过实际可交易库存。", Actions: []string{"背包", "集市", "交易菜单"}}, true, nil
	}
	itemName, quantity, quantityErr := parseStackQuantity(strings.Join(args[:len(args)-1], " "))
	item, err := g.itemByName(itemName)
	price := parsePositiveInt(args[len(args)-1], 0)
	if quantityErr != nil || err != nil || price <= 0 || quantity <= 0 {
		return GameResult{Title: "🏪 上架失败", Content: "物品、数量或单价不正确。格式：`摆摊 物品名*数量 单价`。", Actions: []string{"背包", "集市"}}, true, nil
	}
	if quantity > math.MaxInt64/price {
		return GameResult{Title: "🏪 上架数额过大", Content: "数量与单价的总额超过安全计算范围，请拆分上架。"}, true, nil
	}
	var owned model.PlayerItem
	if err := g.store.DB.Where("player_id = ? AND item_id = ?", player.ID, item.ID).First(&owned).Error; err != nil || owned.Quantity < quantity {
		return GameResult{Title: "🏪 上架库存不足", Content: fmt.Sprintf("上架%s×%d需要足额库存，当前持有%d。", item.Name, quantity, max64(owned.Quantity, 0)), Actions: []string{"背包", "物品 " + item.Name}}, true, nil
	}
	if !barterItemAllowed(item, owned) {
		return GameResult{Title: "🏪 物品不可摆摊", Content: "绑定、礼包、任务、充值、定制或图鉴标记为不可交易的物品不能上架。", Actions: []string{"物品 " + item.Name, "背包"}}, true, nil
	}
	listing := model.TradeListing{SellerID: player.ID, SellerName: player.DaoName, ItemID: item.ID, ItemName: item.Name, Quantity: quantity, UnitPrice: price, Status: "在售", ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -quantity); err != nil {
			return err
		}
		return tx.Create(&listing).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏪 摆摊成功", Content: fmt.Sprintf("摊位号：%d\n上架：%s×%d\n单价：%d灵石\n整批总值：%d灵石\n背包剩余：%d\n有效时间：24小时\n━━━━━━━━━━━\n买家可按需要分批购买，未售数量会继续留在同一摊位。", listing.ID, item.Name, quantity, price, price*quantity, owned.Quantity-quantity), Actions: []string{"集市", "背包", "摆摊 " + item.Name + "*1 " + fmt.Sprint(price)}}, true, nil
}

func (g *Game) buyListing(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) == 1 {
		return g.buyMarketByName(player, args[0])
	}
	if len(args) < 2 {
		return GameResult{Title: "购买指引", Content: "玩家集市无需输入账号或ID：发送 `集市` 后点击蓝字，或发送 `买下 物品名`。\n系统商品请发送 `货铺`，灵田种子请发送 `种子商店`。", Actions: []string{"集市", "货铺", "种子商店"}}, true, nil
	}
	seller, err := g.findPlayer(args[0])
	if err != nil || seller.ID == player.ID {
		return GameResult{Title: "购买失败", Content: "摊主不存在。"}, true, nil
	}
	itemName, quantity, parseErr := parseStackQuantity(strings.Join(args[1:], " "))
	if parseErr != nil {
		return GameResult{Title: "购买数量错误", Content: parseErr.Error(), Actions: []string{"集市"}}, true, nil
	}
	var listing model.TradeListing
	if err := g.store.DB.Where("seller_id = ? AND item_name = ? AND quantity >= ? AND status = ? AND expires_at > ?", seller.ID, itemName, quantity, "在售", time.Now()).Order("unit_price,id").First(&listing).Error; err != nil {
		return GameResult{Title: "商品不存在", Content: fmt.Sprintf("%s没有足量在售的%s×%d。", seller.DaoName, itemName, quantity), Actions: []string{"集市"}}, true, nil
	}
	return g.purchaseListing(player, seller, listing, quantity)
}

func (g *Game) marketList(player *model.Player) (GameResult, bool, error) {
	var rows []model.TradeListing
	if err := g.store.DB.Where("status = ? AND expires_at > ?", "在售", time.Now()).Order("unit_price,id").Limit(8).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "修仙集市", Content: "当前没有在售商品。", Actions: []string{"摆摊"}}, true, nil
	}
	lines := make([]string, 0, len(rows))
	actions := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("- %s × %d · 单价%d灵石 · 摊主%s · 剩余%s", row.ItemName, row.Quantity, row.UnitPrice, row.SellerName, formatDuration(time.Until(row.ExpiresAt))))
		var seller model.Player
		if g.store.DB.First(&seller, row.SellerID).Error == nil && seller.ID != player.ID {
			actions = append(actions, "买下 "+row.ItemName)
		}
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("你的灵石：%d\n点击物品蓝字即可购买当前最低价摊品，不显示也不需要任何玩家ID。", player.SpiritStones))
	actions = append(actions, "摆摊", "货铺")
	return GameResult{Title: "修仙集市", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) buyMarketByName(player *model.Player, argument string) (GameResult, bool, error) {
	name, quantity, parseErr := parseStackQuantity(argument)
	if parseErr != nil {
		return GameResult{Title: "买下数量错误", Content: parseErr.Error(), Actions: []string{"集市"}}, true, nil
	}
	if name == "" {
		return GameResult{Title: "买下摊品", Content: "发送 `集市` 查看在售物品，或发送 `买下 物品名*数量`；无需摊主账号或商品ID。", Actions: []string{"集市"}}, true, nil
	}
	var listing model.TradeListing
	if err := g.store.DB.Where("item_name = ? AND quantity >= ? AND status = ? AND expires_at > ? AND seller_id <> ?", name, quantity, "在售", time.Now(), player.ID).Order("unit_price,id").First(&listing).Error; err != nil {
		return GameResult{Title: "摊品不足", Content: fmt.Sprintf("当前没有其他玩家以单个摊位出售%s×%d，可能数量不足、已售或已下架。", name, quantity), Actions: []string{"集市", "货铺"}}, true, nil
	}
	var seller model.Player
	if err := g.store.DB.First(&seller, listing.SellerID).Error; err != nil {
		return GameResult{}, true, err
	}
	return g.purchaseListing(player, seller, listing, quantity)
}

func (g *Game) purchaseListing(player *model.Player, seller model.Player, listing model.TradeListing, quantity int64) (GameResult, bool, error) {
	if quantity <= 0 || quantity > listing.Quantity || listing.UnitPrice > 0 && quantity > math.MaxInt64/listing.UnitPrice {
		return GameResult{Title: "购买数量错误", Content: "购买数量超过摊位库存或安全计算范围。", Actions: []string{"集市"}}, true, nil
	}
	total := listing.UnitPrice * quantity
	if player.SpiritStones < total {
		return GameResult{Title: "灵石不足", Content: fmt.Sprintf("商品：%s×%d\n需要：%d灵石\n当前：%d灵石\n还差：%d灵石", listing.ItemName, quantity, total, player.SpiritStones, total-player.SpiritStones), Actions: []string{"集市", "任务菜单"}}, true, nil
	}
	if seller.SpiritStones > math.MaxInt64-total {
		return GameResult{Title: "摊主灵石已满", Content: "摊主灵石接近安全上限，本次没有扣款或取货。", Actions: []string{"集市"}}, true, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.TradeListing{}).Where("id = ? AND quantity >= ? AND status = ? AND expires_at > ?", listing.ID, quantity, "在售", time.Now()).Updates(map[string]any{
			"quantity": gorm.Expr("quantity - ?", quantity),
			"status":   gorm.Expr("CASE WHEN quantity = ? THEN ? ELSE ? END", quantity, "已售", "在售"),
			"buyer_id": player.ID,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(player).Update("spirit_stones", gorm.Expr("spirit_stones - ?", total)).Error; err != nil {
			return err
		}
		if err := tx.Model(&seller).Update("spirit_stones", gorm.Expr("spirit_stones + ?", total)).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, listing.ItemID, quantity); err != nil {
			return err
		}
		record := model.TradeRecord{ListingID: listing.ID, SellerID: seller.ID, BuyerID: player.ID, ItemID: listing.ItemID, Quantity: quantity, TotalPrice: total}
		return tx.Create(&record).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GameResult{Title: "慢了一步", Content: "该摊品刚刚被其他道友买走，请刷新集市。", Actions: []string{"集市"}}, true, nil
		}
		return GameResult{}, true, err
	}
	remaining := listing.Quantity - quantity
	return GameResult{Title: "集市成交", Content: fmt.Sprintf("买下：%s × %d\n支付：%d灵石\n摊主：%s\n你的剩余灵石：%d\n摊位剩余：%d\n物品已放入乾坤袋。", listing.ItemName, quantity, total, seller.DaoName, player.SpiritStones-total, remaining), Actions: []string{"背包", "物品 " + listing.ItemName, "集市", "买下 " + listing.ItemName + "*1"}}, true, nil
}

var (
	errBarterUnavailable      = errors.New("barter request is no longer available")
	errBarterInventoryChanged = errors.New("barter inventory changed")
)

type barterNotificationPayload struct {
	RequestID         uint   `json:"request_id"`
	OfferedItemName   string `json:"offered_item_name"`
	OfferedQuantity   int64  `json:"offered_quantity"`
	RequestedItemName string `json:"requested_item_name"`
	RequestedQuantity int64  `json:"requested_quantity"`
}

func (g *Game) barter(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) < 3 {
		return GameResult{Title: "🏪 以物易物", Content: "请输入：`易物 @对方 我的物品*数量 对方物品*数量`\n━━━━━━━━━━━\n发起后不会立刻扣除或交换物品；必须由对方发送“确认易物 申请编号”才会成交。", Actions: []string{"易物请求", "交易菜单", "背包"}}, true, nil
	}
	target, err := g.findPlayer(args[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🏪 易物申请失败", Content: "没有找到交换对象，且不能向自己发起易物。请使用对方的全服唯一道号。", Actions: []string{"易物请求", "交易菜单"}}, true, nil
	}
	offerName, offerQuantity, offerErr := parseStackQuantity(args[1])
	requestName, requestQuantity, requestErr := parseStackQuantity(args[2])
	if offerErr != nil || requestErr != nil {
		return GameResult{Title: "🏪 易物数量有误", Content: "物品数量请使用“物品名*正整数”，例如：`易物 @对方 灵果*2 龙血芝*1`。", Actions: []string{"背包", "易物请求"}}, true, nil
	}
	offered, offerErr := g.itemByName(offerName)
	requested, requestErr := g.itemByName(requestName)
	if offerErr != nil || requestErr != nil {
		return GameResult{Title: "🏪 易物物品未收录", Content: "请核对双方物品的完整名称；可先通过物品图鉴查询。", Actions: []string{"物品图鉴", "背包", "易物请求"}}, true, nil
	}
	if offered.ID == requested.ID {
		return GameResult{Title: "🏪 易物申请无效", Content: "不能用同一种物品与同一种物品交换。", Actions: []string{"易物请求", "背包"}}, true, nil
	}
	var offeredOwned, requestedOwned model.PlayerItem
	if err := g.store.DB.Where("player_id = ? AND item_id = ?", player.ID, offered.ID).First(&offeredOwned).Error; err != nil || offeredOwned.Quantity < offerQuantity {
		return GameResult{Title: "🏪 你的物品不足", Content: fmt.Sprintf("发起申请需要持有%s×%d，当前可用%d。", offered.Name, offerQuantity, max64(offeredOwned.Quantity, 0)), Actions: []string{"背包", "物品 " + offered.Name}}, true, nil
	}
	if err := g.store.DB.Where("player_id = ? AND item_id = ?", target.ID, requested.ID).First(&requestedOwned).Error; err != nil || requestedOwned.Quantity < requestQuantity {
		return GameResult{Title: "🏪 对方物品不足", Content: fmt.Sprintf("%s目前没有足够的%s×%d，本次未建立申请。", target.DaoName, requested.Name, requestQuantity), Actions: []string{"易物请求", "物品 " + requested.Name}}, true, nil
	}
	if !barterItemAllowed(offered, offeredOwned) || !barterItemAllowed(requested, requestedOwned) {
		return GameResult{Title: "🏪 物品不可易物", Content: "绑定、礼包、任务、充值、定制或图鉴标记为不可交易的物品不能参与易物。", Actions: []string{"物品 " + offered.Name, "物品 " + requested.Name, "背包"}}, true, nil
	}

	minutes := max64(g.settingInt("trade.barter_expiry_minutes", 10), 1)
	request := model.BarterRequest{
		InitiatorID: player.ID, InitiatorName: player.DaoName, RecipientID: target.ID, RecipientName: target.DaoName,
		OfferedItemID: offered.ID, OfferedItemName: offered.Name, OfferedQuantity: offerQuantity,
		RequestedItemID: requested.ID, RequestedItemName: requested.Name, RequestedQuantity: requestQuantity,
		Status: "待确认", ExpiresAt: time.Now().Add(time.Duration(minutes) * time.Minute),
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(barterNotificationPayload{RequestID: request.ID, OfferedItemName: offered.Name, OfferedQuantity: offerQuantity, RequestedItemName: requested.Name, RequestedQuantity: requestQuantity})
		if err != nil {
			return err
		}
		message := model.SocialMessage{SenderID: player.ID, ReceiverID: target.ID, Type: "barter_request", Content: string(payload), Read: false}
		return g.social.CreateInTx(tx, &message)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	content := fmt.Sprintf("申请编号：%d\n交换道友：%s\n你愿交出：%s×%d\n希望获得：%s×%d\n有效时间：%d分钟\n━━━━━━━━━━━\n物品尚未扣除，也尚未发生交换。对方确认时会再次校验双方背包，并在同一笔事务中一次性成交。", request.ID, target.DaoName, offered.Name, offerQuantity, requested.Name, requestQuantity, minutes)
	return GameResult{Title: "🏪 易物申请已送达", Content: content, Actions: []string{"易物请求", "通知", "背包", "交易菜单"}}, true, nil
}

func barterItemAllowed(item model.Item, owned model.PlayerItem) bool {
	return !owned.Bound && item.Tradable && item.CategoryName != "礼包" && item.CategoryName != "任务物品" && item.EffectFunc != "open_gift_pack" && !strings.HasPrefix(item.EffectFunc, "customize_") && !strings.Contains(item.EffectType, "付费")
}

func (g *Game) acceptBarter(player *model.Player, raw string) (GameResult, bool, error) {
	requestID := uint(parsePositiveInt(strings.TrimSpace(raw), 0))
	if requestID == 0 {
		return GameResult{Title: "🏪 确认易物", Content: "请输入：`确认易物 申请编号`。\n只有收到申请的一方可以确认；可先查看易物请求或通知信箱。", Actions: []string{"易物请求", "通知"}}, true, nil
	}
	var request model.BarterRequest
	if err := g.store.DB.Where("id = ? AND recipient_id = ?", requestID, player.ID).First(&request).Error; err != nil {
		return GameResult{Title: "🏪 易物申请不存在", Content: "没有找到发给你的该笔申请，请核对编号。", Actions: []string{"易物请求", "通知"}}, true, nil
	}
	if request.Status != "待确认" || !request.ExpiresAt.After(time.Now()) {
		if request.Status == "待确认" {
			_ = g.store.DB.Model(&request).Update("status", "已过期").Error
		}
		return GameResult{Title: "🏪 易物申请已失效", Content: fmt.Sprintf("申请%d当前状态：%s。没有重复交换任何物品。", request.ID, displayOr(request.Status, "已过期")), Actions: []string{"易物请求"}}, true, nil
	}

	completedAt := time.Now()
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		claim := tx.Model(&model.BarterRequest{}).Where("id = ? AND recipient_id = ? AND status = ? AND expires_at > ?", request.ID, player.ID, "待确认", completedAt).Update("status", "结算中")
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			return errBarterUnavailable
		}
		var offeredOwned, requestedOwned model.PlayerItem
		if err := tx.Where("player_id = ? AND item_id = ?", request.InitiatorID, request.OfferedItemID).First(&offeredOwned).Error; err != nil || offeredOwned.Quantity < request.OfferedQuantity {
			return errBarterInventoryChanged
		}
		if err := tx.Where("player_id = ? AND item_id = ?", request.RecipientID, request.RequestedItemID).First(&requestedOwned).Error; err != nil || requestedOwned.Quantity < request.RequestedQuantity {
			return errBarterInventoryChanged
		}
		var offered, requested model.Item
		if tx.First(&offered, request.OfferedItemID).Error != nil || tx.First(&requested, request.RequestedItemID).Error != nil || !barterItemAllowed(offered, offeredOwned) || !barterItemAllowed(requested, requestedOwned) {
			return errBarterInventoryChanged
		}
		repo := storage.NewPlayerRepository(tx)
		if err := repo.AdjustItem(request.InitiatorID, request.OfferedItemID, -request.OfferedQuantity); err != nil {
			return errBarterInventoryChanged
		}
		if err := repo.AdjustItem(request.RecipientID, request.RequestedItemID, -request.RequestedQuantity); err != nil {
			return errBarterInventoryChanged
		}
		if err := repo.AdjustItem(request.InitiatorID, request.RequestedItemID, request.RequestedQuantity); err != nil {
			return err
		}
		if err := repo.AdjustItem(request.RecipientID, request.OfferedItemID, request.OfferedQuantity); err != nil {
			return err
		}
		if err := tx.Model(&model.BarterRequest{}).Where("id = ?", request.ID).Updates(map[string]any{"status": "已成交", "responded_at": &completedAt}).Error; err != nil {
			return err
		}
		return g.markBarterNotificationRead(tx, request)
	})
	if errors.Is(err, errBarterUnavailable) {
		return GameResult{Title: "🏪 易物申请已被处理", Content: "该申请已确认、拒绝、撤回或过期，没有重复交换。", Actions: []string{"易物请求"}}, true, nil
	}
	if errors.Is(err, errBarterInventoryChanged) {
		_ = g.store.DB.Model(&model.BarterRequest{}).Where("id = ? AND status = ?", request.ID, "待确认").Updates(map[string]any{"status": "库存变化", "responded_at": &completedAt}).Error
		return GameResult{Title: "🏪 易物自动失效", Content: "确认时发现一方物品数量、绑定状态或交易资格已经变化。本次事务已全部回滚，双方都没有损失物品。", Actions: []string{"易物请求", "背包"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(request.InitiatorID, "易物成交", fmt.Sprintf("%s已确认申请%d。你交出%s×%d，获得%s×%d。", request.RecipientName, request.ID, request.OfferedItemName, request.OfferedQuantity, request.RequestedItemName, request.RequestedQuantity))
	content := fmt.Sprintf("申请编号：%d\n交易道友：%s\n你交出：%s×%d\n你获得：%s×%d\n━━━━━━━━━━━\n双方物品已在同一笔事务中完成交换。", request.ID, request.InitiatorName, request.RequestedItemName, request.RequestedQuantity, request.OfferedItemName, request.OfferedQuantity)
	return GameResult{Title: "🏪 易物成交", Content: content, Actions: []string{"背包", "易物请求", "通知"}}, true, nil
}

func (g *Game) rejectBarter(player *model.Player, raw string) (GameResult, bool, error) {
	requestID := uint(parsePositiveInt(strings.TrimSpace(raw), 0))
	if requestID == 0 {
		return GameResult{Title: "🏪 拒绝或撤回易物", Content: "请输入：`拒绝易物 申请编号`。收件人会拒绝申请，发起人会撤回申请。", Actions: []string{"易物请求", "通知"}}, true, nil
	}
	var request model.BarterRequest
	if err := g.store.DB.Where("id = ? AND (initiator_id = ? OR recipient_id = ?)", requestID, player.ID, player.ID).First(&request).Error; err != nil {
		return GameResult{Title: "🏪 易物申请不存在", Content: "没有找到与你有关的该笔申请。", Actions: []string{"易物请求"}}, true, nil
	}
	status := "已拒绝"
	otherID := request.InitiatorID
	verb := "拒绝"
	if player.ID == request.InitiatorID {
		status, otherID, verb = "已撤回", request.RecipientID, "撤回"
	}
	otherName := request.InitiatorName
	if player.ID == request.InitiatorID {
		otherName = request.RecipientName
	}
	now := time.Now()
	result := g.store.DB.Model(&model.BarterRequest{}).Where("id = ? AND status = ? AND expires_at > ?", request.ID, "待确认", now).Updates(map[string]any{"status": status, "responded_at": &now})
	if result.Error != nil {
		return GameResult{}, true, result.Error
	}
	if result.RowsAffected != 1 {
		return GameResult{Title: "🏪 易物申请已失效", Content: "该申请已经被处理或超过有效期，没有扣除物品。", Actions: []string{"易物请求"}}, true, nil
	}
	_ = g.markBarterNotificationRead(g.store.DB, request)
	_ = g.createPlayerNotification(otherID, "易物"+verb, fmt.Sprintf("申请%d已被%s；双方均未交换物品。", request.ID, verb))
	return GameResult{Title: "🏪 易物申请" + status, Content: fmt.Sprintf("申请编号：%d\n已%s与%s的易物申请。\n双方物品均未扣除。", request.ID, verb, displayOr(otherName, "对方")), Actions: []string{"易物请求", "背包"}}, true, nil
}

func (g *Game) barterRequests(player *model.Player, raw string) (GameResult, bool, error) {
	now := time.Now()
	_ = g.store.DB.Model(&model.BarterRequest{}).Where("status = ? AND expires_at <= ?", "待确认", now).Update("status", "已过期").Error
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.BarterRequest{}).Where("initiator_id = ? OR recipient_id = ?", player.ID, player.ID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	var rows []model.BarterRequest
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d笔", page, pages, total), "━━━━━━━━━━━"}
	actions := []string{"易物 @对方 我的物品*1 对方物品*1", "通知", "背包", "交易菜单"}
	for _, row := range rows {
		direction := "你向" + row.RecipientName + "发起"
		if row.RecipientID == player.ID {
			direction = row.InitiatorName + "向你发起"
		}
		lines = append(lines, fmt.Sprintf("#%d · %s · %s\n%s×%d ⇄ %s×%d\n有效至：%s", row.ID, row.Status, direction, row.OfferedItemName, row.OfferedQuantity, row.RequestedItemName, row.RequestedQuantity, row.ExpiresAt.Format("01-02 15:04")), "━━━━━━━")
		if row.Status == "待确认" && row.RecipientID == player.ID {
			actions = append(actions, fmt.Sprintf("确认易物 %d", row.ID), fmt.Sprintf("拒绝易物 %d", row.ID))
		} else if row.Status == "待确认" && row.InitiatorID == player.ID {
			actions = append(actions, fmt.Sprintf("取消易物 %d", row.ID))
		}
	}
	if len(rows) == 0 {
		lines = append(lines, "当前没有易物记录。发起申请后，必须等待对方明确确认才会交换。")
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("易物请求 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("易物请求 %d", page+1))
	}
	return GameResult{Title: "🏪 易物申请", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) markBarterNotificationRead(db *gorm.DB, request model.BarterRequest) error {
	pattern := fmt.Sprintf("%%\"request_id\":%d%%", request.ID)
	return db.Model(&model.SocialMessage{}).Where("sender_id = ? AND receiver_id = ? AND type = ? AND content LIKE ?", request.InitiatorID, request.RecipientID, "barter_request", pattern).Update("read", true).Error
}

func (g *Game) executeTask(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 91:
		return g.dailyTasks(player, command.RawArguments)
	case 92:
		return g.acceptTask(player, command.RawArguments)
	case 93:
		return g.submitTask(player, command.RawArguments)
	case 94:
		return g.bountyList(player, command.RawArguments)
	case 95:
		return g.achievements(player, command.RawArguments)
	case 96:
		return g.achievementStats(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) dailyTasks(player *model.Player, argument string) (GameResult, bool, error) {
	const pageSize = 6
	query := g.store.DB.Model(&model.TaskTemplate{}).Where("enabled = ? AND daily = ?", true, true)
	var tasks []model.TaskTemplate
	if err := query.Find(&tasks).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(tasks) == 0 {
		return GameResult{Title: "每日任务", Content: "今日任务榜暂未颁布，请稍后再来查看。"}, true, nil
	}
	sortTasksByRealmTier(tasks)
	page := g.taskCatalogPageForPlayer(player, tasks, argument, pageSize)
	pages := maxInt((len(tasks)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start := minInt((page-1)*pageSize, len(tasks))
	end := minInt(start+pageSize, len(tasks))
	lines := []string{fmt.Sprintf("按前置境界从最低到最高排列 · 第%d/%d页", page, pages), fmt.Sprintf("当前：%s·%d层 · 默认自动定位当前段位附近任务", player.RealmName, player.RealmLevel), "━━━━━━━━━━━"}
	actions := make([]string, 0, pageSize+3)
	for _, task := range tasks[start:end] {
		requirement, unmet, _ := g.prerequisiteStatus(player, task.PrerequisiteJSON)
		state := "可接取"
		if len(unmet) > 0 {
			state = "未解锁"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  %s\n  前置：%s\n  目标：%s\n  奖励：%s", task.Name, state, task.Description, requirement, taskObjectiveText(task.ObjectiveJSON), taskRewardText(task)))
		actions = append(actions, "接任务 "+task.Name)
	}
	lines = append(lines, fmt.Sprintf("━━━━━━━━━━━\n共%d项 · 前页为低阶任务，后页为更高境界路线", len(tasks)))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("日常 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("日常 %d", page+1))
	}
	return GameResult{Title: "每日任务", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) acceptTask(player *model.Player, argument string) (GameResult, bool, error) {
	var task model.TaskTemplate
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "接取任务", Content: "请输入：`接任务 任务名`", Actions: []string{"日常", "悬赏"}}, true, nil
	}
	query := g.store.DB.Where("enabled = ? AND name = ?", true, name)
	if legacyID := parsePositiveInt(name, 0); legacyID > 0 {
		query = g.store.DB.Where("enabled = ? AND id = ?", true, legacyID)
	}
	if query.First(&task).Error != nil {
		return GameResult{Title: "接取失败", Content: "没有找到名为“" + name + "”的任务，请从日常、悬赏或当前位置的蓝字任务中选择。", Actions: []string{"日常", "悬赏", "位置"}}, true, nil
	}
	requirement, unmet, requirementErr := g.prerequisiteStatus(player, task.PrerequisiteJSON)
	if requirementErr != nil {
		return GameResult{Title: "任务道纹紊乱", Content: "任务前置条件无法解析，本次不会接取或扣除资源，请主人检查任务数据。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: "任务尚未解锁", Content: fmt.Sprintf("任务：%s\n前置：%s\n━━━━━━━━━━━\n未满足：\n- %s", task.Name, requirement, strings.Join(unmet, "\n- ")), Actions: append(g.prerequisiteActions(unmet), "任务菜单")}, true, nil
	}
	date := time.Now().Format("2006-01-02")
	var row model.PlayerTask
	existingErr := g.store.DB.Where("player_id = ? AND task_template_id = ? AND assigned_date = ?", player.ID, task.ID, date).Order("id DESC").First(&row).Error
	if existingErr == nil {
		progress := g.taskProgressForRow(player, task, row)
		if row.Status == "进行中" {
			return GameResult{Title: "任务已在进行", Content: fmt.Sprintf("任务：%s\n目标：%s\n当前进度：%d/%d\n━━━━━━━━━━━\n无需重复接取，继续完成目标后直接交付。", task.Name, taskObjectiveText(task.ObjectiveJSON), progress, taskObjectiveCount(task.ObjectiveJSON)), Actions: taskObjectiveActions(task.ObjectiveJSON, task.Name)}, true, nil
		}
		if row.Status == "已完成" {
			return GameResult{Title: "今日任务已完成", Content: fmt.Sprintf("任务：%s\n今日状态：已完成并领取奖励\n━━━━━━━━━━━\n同一任务不会重复接取或重复发奖，明日刷新后可再次查看。", task.Name), Actions: []string{"日常", "悬赏", "位置"}}, true, nil
		}
	} else if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return GameResult{}, true, existingErr
	}
	baseline := g.taskObjectiveProgress(player, task.ObjectiveJSON)
	progressState := taskProgressState{Count: 0}
	if taskUsesPostAcceptanceProgress(decodeTaskObjective(task.ObjectiveJSON).Type) {
		progressState.Baseline = &baseline
	}
	progressJSON, marshalErr := json.Marshal(progressState)
	if marshalErr != nil {
		return GameResult{}, true, marshalErr
	}
	row = model.PlayerTask{PlayerID: player.ID, TaskTemplateID: task.ID, ProgressJSON: string(progressJSON), Status: "进行中", AssignedDate: date}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "任务已接取", Content: fmt.Sprintf("任务：%s\n类型：%s\n剧情：%s\n前置：%s\n目标：%s\n当前进度：%d/%d\n奖励：%s", task.Name, task.Type, task.Description, requirement, taskObjectiveText(task.ObjectiveJSON), g.taskProgressForRow(player, task, row), taskObjectiveCount(task.ObjectiveJSON), taskRewardText(task)), Actions: taskObjectiveActions(task.ObjectiveJSON, task.Name)}, true, nil
}

func (g *Game) submitTask(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	var task model.TaskTemplate
	query := g.store.DB.Where("name = ?", name)
	if legacyID := parsePositiveInt(name, 0); legacyID > 0 {
		query = g.store.DB.Where("id = ?", legacyID)
	}
	if name == "" || query.First(&task).Error != nil {
		return GameResult{Title: "提交失败", Content: "请输入进行中的任务名称，例如：`交任务 山野巡查`。", Actions: []string{"日常", "悬赏"}}, true, nil
	}
	var row model.PlayerTask
	if g.store.DB.Where("player_id = ? AND task_template_id = ? AND status = ?", player.ID, task.ID, "进行中").Order("id DESC").First(&row).Error != nil {
		return GameResult{Title: "提交失败", Content: "你没有正在进行的“" + task.Name + "”。", Actions: []string{"接任务 " + task.Name, "日常"}}, true, nil
	}
	progress := g.taskProgressForRow(player, task, row)
	required := taskObjectiveCount(task.ObjectiveJSON)
	if progress < required {
		return GameResult{Title: "任务未完成", Content: fmt.Sprintf("任务：%s\n目标：%s\n当前进度：%d/%d\n还需完成：%d", task.Name, taskObjectiveText(task.ObjectiveJSON), progress, required, required-progress), Actions: taskObjectiveActions(task.ObjectiveJSON, task.Name)}, true, nil
	}
	var rewards map[string]any
	if json.Unmarshal([]byte(task.RewardJSON), &rewards) != nil {
		return GameResult{Title: "任务奖励配置错误", Content: "请管理员检查该任务的奖励JSON。"}, true, nil
	}
	objective := decodeTaskObjective(task.ObjectiveJSON)
	reward := rewardNumber(rewards, "cultivation")
	if reward <= 0 {
		reward = 100
	}
	merit := rewardNumber(rewards, "merit")
	reputation := rewardNumber(rewards, "reputation")
	stones := rewardNumber(rewards, "spirit_stones")
	silver := rewardNumber(rewards, "silver_coins")
	if silver <= 0 {
		silver = rewardNumber(rewards, "silver")
	}
	if silver <= 0 {
		silver = model.TaskSilverReward(task)
	}
	affinity := rewardNumber(rewards, "affinity")
	contribution := rewardNumber(rewards, "contribution")
	sectFunds := rewardNumber(rewards, "sect_funds")
	rewardTitle, _ := rewards["title"].(string)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if objective.Type == "submit_item" && objective.Item != "" {
			var item model.Item
			if err := tx.Where("name = ? OR code = ?", objective.Item, objective.Item).First(&item).Error; err != nil {
				return err
			}
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -required); err != nil {
				return fmt.Errorf("上交%s失败: %w", objective.Item, err)
			}
		}
		if err := tx.Model(player).Updates(map[string]any{"cultivation": gorm.Expr("cultivation + ?", reward), "spirit_stones": gorm.Expr("spirit_stones + ?", stones), "silver_coins": gorm.Expr("silver_coins + ?", silver), "merit": gorm.Expr("merit + ?", merit), "reputation": gorm.Expr("reputation + ?", reputation)}).Error; err != nil {
			return err
		}
		if strings.TrimSpace(rewardTitle) != "" {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("title", rewardTitle).Error; err != nil {
				return err
			}
		}
		if affinity > 0 && player.CoupleID != 0 {
			if err := tx.Model(&model.Couple{}).Where("id = ? AND status = ?", player.CoupleID, model.CoupleStatusActive).Update("affinity", gorm.Expr("affinity + ?", affinity)).Error; err != nil {
				return err
			}
		}
		if contribution > 0 {
			if err := tx.Model(&model.SectMember{}).Where("player_id = ?", player.ID).Update("contribution", gorm.Expr("contribution + ?", contribution)).Error; err != nil {
				return err
			}
		}
		if sectFunds > 0 {
			sectIDs := tx.Model(&model.SectMember{}).Select("sect_id").Where("player_id = ?", player.ID)
			if err := tx.Model(&model.Sect{}).Where("id IN (?)", sectIDs).Update("funds", gorm.Expr("funds + ?", sectFunds)).Error; err != nil {
				return err
			}
		}
		if items, ok := rewards["items"].(map[string]any); ok {
			repo := storage.NewPlayerRepository(tx)
			for itemName, rawQuantity := range items {
				quantity := int64FromAny(rawQuantity)
				if quantity <= 0 {
					continue
				}
				var item model.Item
				if err := tx.Where("name = ?", itemName).First(&item).Error; err != nil {
					return fmt.Errorf("任务奖励物品%s未配置: %w", itemName, err)
				}
				if err := repo.AdjustItem(player.ID, item.ID, quantity); err != nil {
					return err
				}
			}
		}
		return tx.Model(&row).Update("status", "已完成").Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "任务完成", Content: fmt.Sprintf("任务：%s\n目标进度：%d/%d\n━━━━━━━━━━━\n完整奖励：%s\n银币等数值、称号与物品已按同一配置实际入账。\n前序完成状态已记录，可解锁后续任务。", task.Name, progress, required, taskRewardText(task)), Actions: []string{"背包", "银币来源", "日常", "悬赏", "状态"}}, true, nil
}

type taskObjective struct {
	Type   string `json:"type"`
	Count  int64  `json:"count"`
	Target string `json:"target"`
	Item   string `json:"item"`
}

type taskProgressState struct {
	Count    int64  `json:"count"`
	Baseline *int64 `json:"baseline,omitempty"`
}

func decodeTaskObjective(raw string) taskObjective {
	objective := taskObjective{Count: 1}
	_ = json.Unmarshal([]byte(raw), &objective)
	if objective.Count < 1 {
		objective.Count = 1
	}
	return objective
}

func taskObjectiveCount(raw string) int64 { return decodeTaskObjective(raw).Count }

func taskUsesPostAcceptanceProgress(objectiveType string) bool {
	switch objectiveType {
	case "explore", "collect", "hunt", "battle_wins", "cultivation", "boss", "dungeon", "forge", "alchemy", "farm_harvest", "arena_win", "dual_cultivation", "sect_patrol", "pet_capture", "action", "catalog_action":
		return true
	default:
		return false
	}
}

func (g *Game) taskProgressForRow(player *model.Player, task model.TaskTemplate, row model.PlayerTask) int64 {
	objective := decodeTaskObjective(task.ObjectiveJSON)
	progress := g.taskObjectiveProgress(player, task.ObjectiveJSON)
	if taskUsesPostAcceptanceProgress(objective.Type) {
		var state taskProgressState
		if json.Unmarshal([]byte(row.ProgressJSON), &state) == nil && state.Baseline != nil {
			progress -= *state.Baseline
		}
	}
	if progress < 0 {
		progress = 0
	}
	if progress > objective.Count {
		progress = objective.Count
	}
	return progress
}

func taskObjectiveText(raw string) string {
	objective := decodeTaskObjective(raw)
	verbs := map[string]string{
		"action": "完成探索或战斗", "catalog_action": "完成指定修行行动", "explore": "完成世界探索",
		"hunt": "击败普通妖灵", "battle_wins": "赢得战斗", "cultivation": "完成有效闭关分钟",
		"collect": "采集地图灵植", "boss": "击败区域首领", "dungeon": "通关副本",
		"forge": "完成装备锻造", "alchemy": "成功炼制丹药", "farm_harvest": "收获仙府灵植",
		"arena_win": "赢得竞技对战", "dual_cultivation": "完成仙侣双修", "sect_patrol": "完成宗门巡查",
		"submit_item": "上交指定物品", "affinity": "提升仙侣道缘", "pet_capture": "收服灵兽", "realm": "抵达指定境界",
	}
	text := verbs[objective.Type]
	if text == "" {
		text = "完成任务指定目标"
	}
	if objective.Target != "" {
		text += "“" + objective.Target + "”"
	}
	if objective.Item != "" {
		text += "“" + objective.Item + "”"
	}
	return fmt.Sprintf("%s × %d", text, objective.Count)
}

func taskRewardText(task model.TaskTemplate) string {
	var rewards map[string]any
	if json.Unmarshal([]byte(task.RewardJSON), &rewards) != nil {
		return displayConfigText(task.RewardJSON)
	}
	if rewards == nil {
		rewards = map[string]any{}
	}
	if rewardNumber(rewards, "silver_coins") <= 0 && rewardNumber(rewards, "silver") <= 0 {
		if silver := model.TaskSilverReward(task); silver > 0 {
			rewards["silver_coins"] = silver
		}
	}
	encoded, err := json.Marshal(rewards)
	if err != nil {
		return displayConfigText(task.RewardJSON)
	}
	return displayConfigText(string(encoded))
}

func (g *Game) taskObjectiveProgress(player *model.Player, raw string) int64 {
	objective := decodeTaskObjective(raw)
	switch objective.Type {
	case "explore", "collect":
		key := "stats.explores"
		if objective.Type == "collect" {
			key = "stats.collects"
		}
		return g.playerValueInt(player.ID, key, 0)
	case "hunt", "battle_wins":
		return g.playerValueInt(player.ID, "stats.wins", 0)
	case "cultivation":
		return g.playerValueInt(player.ID, "stats.cultivation_minutes", 0)
	case "boss":
		return g.playerValueInt(player.ID, "stats.boss_wins", 0)
	case "dungeon":
		return g.playerValueInt(player.ID, "stats.dungeons", 0)
	case "forge":
		return g.playerValueInt(player.ID, "stats.forges", 0)
	case "alchemy":
		return g.playerValueInt(player.ID, "stats.alchemy", 0)
	case "farm_harvest":
		return g.playerValueInt(player.ID, "farm.harvested", 0)
	case "arena_win":
		return g.playerValueInt(player.ID, "stats.arena_wins", 0)
	case "dual_cultivation":
		return g.playerValueInt(player.ID, "stats.dual_cultivation", 0)
	case "sect_patrol":
		return g.playerValueInt(player.ID, "stats.sect_patrol", 0)
	case "submit_item":
		if objective.Item == "" {
			return 0
		}
		item, err := g.itemByName(objective.Item)
		if err != nil {
			return 0
		}
		return g.itemQuantity(player.ID, item.ID)
	case "affinity":
		if player.CoupleID == 0 {
			return 0
		}
		var couple model.Couple
		if g.store.DB.First(&couple, player.CoupleID).Error != nil || couple.Status != model.CoupleStatusActive {
			return 0
		}
		return couple.Affinity
	case "pet_capture":
		var count int64
		_ = g.store.DB.Model(&model.Pet{}).Where("player_id = ?", player.ID).Count(&count).Error
		return count
	case "realm":
		sequence, err := g.playerRealmSequence(player)
		if err != nil {
			return 0
		}
		if objective.Target == "飞升" {
			if sequence >= 1000 && player.RealmLevel >= 10 {
				return 1
			}
			return 0
		}
		var target model.Realm
		if g.store.DB.Where("name = ?", objective.Target).First(&target).Error == nil && sequence >= target.Sequence {
			return 1
		}
		return 0
	case "action", "catalog_action":
		return g.playerValueInt(player.ID, "stats.explores", 0) + g.playerValueInt(player.ID, "stats.wins", 0)
	default:
		return 0
	}
}

func taskObjectiveActions(raw, taskName string) []string {
	objective := decodeTaskObjective(raw)
	actions := map[string][]string{
		"explore": {"探索", "地图"}, "collect": {"位置", "采集"}, "hunt": {"猎妖", "位置"},
		"battle_wins": {"战斗", "竞技"}, "cultivation": {"修炼", "出关"}, "boss": {"首领", "讨伐"},
		"dungeon": {"副本"}, "forge": {"装备背包", "锻造"}, "alchemy": {"丹方", "炼药"},
		"farm_harvest": {"灵田", "收菜"}, "arena_win": {"竞技"}, "dual_cultivation": {"双修"},
		"sect_patrol": {"宗务"}, "action": {"探索", "猎妖"}, "catalog_action": {"探索", "猎妖"},
		"submit_item": {"背包", "查询 " + objective.Item},
		"affinity":    {"心意", "双修"}, "pet_capture": {"捕获", "灵兽菜单"}, "realm": {"修炼", "突破", "渡劫"},
	}[objective.Type]
	return append(actions, "交任务 "+taskName)
}

func (g *Game) bountyList(player *model.Player, argument string) (GameResult, bool, error) {
	const pageSize = 8
	query := g.store.DB.Model(&model.TaskTemplate{}).Where("enabled = ? AND type = ?", true, "悬赏")
	var tasks []model.TaskTemplate
	if err := query.Find(&tasks).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(tasks) == 0 {
		return GameResult{Title: "悬赏榜", Content: "今日暂无悬赏。"}, true, nil
	}
	sortTasksByRealmTier(tasks)
	page := g.taskCatalogPageForPlayer(player, tasks, argument, pageSize)
	pages := maxInt((len(tasks)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start := minInt((page-1)*pageSize, len(tasks))
	end := minInt(start+pageSize, len(tasks))
	lines := []string{fmt.Sprintf("按前置境界从最低到最高排列 · 第%d/%d页", page, pages), fmt.Sprintf("当前：%s·%d层 · 不再把最高阶悬赏放到新人首页", player.RealmName, player.RealmLevel), "━━━━━━━━━━━"}
	actions := make([]string, 0, pageSize+3)
	for _, task := range tasks[start:end] {
		requirement, unmet, _ := g.prerequisiteStatus(player, task.PrerequisiteJSON)
		state := "可接取"
		if len(unmet) > 0 {
			state = "未解锁"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  %s\n  前置：%s\n  目标：%s\n  奖励：%s", task.Name, state, task.Description, requirement, taskObjectiveText(task.ObjectiveJSON), taskRewardText(task)))
		actions = append(actions, "接任务 "+task.Name)
	}
	lines = append(lines, fmt.Sprintf("━━━━━━━━━━━\n共%d项 · 可向后翻页预览更高境界悬赏", len(tasks)))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("悬赏 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("悬赏 %d", page+1))
	}
	return GameResult{Title: "悬赏榜", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func taskRealmTier(task model.TaskTemplate) (int, int, int64) {
	requirement, err := decodeGameplayPrerequisite(task.PrerequisiteJSON)
	if err != nil {
		return 1, 1, 0
	}
	return maxInt(requirement.MinimumRealmSequence, 1), maxInt(requirement.MinimumRealmLevel, 1), requirement.MinimumCombatPower
}

func sortTasksByRealmTier(tasks []model.TaskTemplate) {
	sort.SliceStable(tasks, func(left, right int) bool {
		leftSequence, leftLevel, leftPower := taskRealmTier(tasks[left])
		rightSequence, rightLevel, rightPower := taskRealmTier(tasks[right])
		if leftSequence != rightSequence {
			return leftSequence < rightSequence
		}
		if leftLevel != rightLevel {
			return leftLevel < rightLevel
		}
		if leftPower != rightPower {
			return leftPower < rightPower
		}
		if tasks[left].Weight != tasks[right].Weight {
			return tasks[left].Weight > tasks[right].Weight
		}
		return tasks[left].ID < tasks[right].ID
	})
}

func (g *Game) taskCatalogPageForPlayer(player *model.Player, tasks []model.TaskTemplate, raw string, pageSize int) int {
	if strings.TrimSpace(raw) != "" {
		return maxInt(int(parsePositiveInt(raw, 1)), 1)
	}
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		sequence = 1
	}
	for index, task := range tasks {
		taskSequence, taskLevel, _ := taskRealmTier(task)
		if taskSequence > sequence || taskSequence == sequence && taskLevel >= player.RealmLevel {
			return index/pageSize + 1
		}
	}
	return maxInt((len(tasks)+pageSize-1)/pageSize, 1)
}

func (g *Game) achievements(player *model.Player, argument string) (GameResult, bool, error) {
	var titles []model.Title
	if err := g.store.DB.Where("enabled = ?", true).Order("id").Find(&titles).Error; err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 12
	pages := maxInt((len(titles)+pageSize-1)/pageSize, 1)
	page := maxInt(int(parsePositiveInt(argument, 1)), 1)
	if page > pages {
		page = pages
	}
	start := minInt((page-1)*pageSize, len(titles))
	end := minInt(start+pageSize, len(titles))
	lines := []string{fmt.Sprintf("称号总数：%d · 已解锁：%d · 第%d/%d页", len(titles), achievementUnlockedCount(titles, player), page, pages)}
	for _, title := range titles[start:end] {
		unlocked := achievementTitleUnlocked(title, player)
		mark := "未解锁"
		if unlocked {
			mark = "已解锁"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】· %s", title.Name, mark, title.Condition))
	}
	actions := []string{"成统计", "状态", "特殊菜单"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("成就 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("成就 %d", page+1))
	}
	return GameResult{Title: "成就称号", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func achievementUnlockedCount(titles []model.Title, player *model.Player) int {
	count := 0
	for _, title := range titles {
		if achievementTitleUnlocked(title, player) {
			count++
		}
	}
	return count
}

func achievementTitleUnlocked(title model.Title, player *model.Player) bool {
	return title.Name == "初入仙途" ||
		(title.Name == "道侣同心" && player.CoupleID != 0) ||
		(title.Name == "天眷之人" && normalizedPlayerLuck(player.Luck) >= maximumPlayerLuck) ||
		(title.Name == "飞升仙人" && player.RealmName == "飞升")
}

func (g *Game) achievementStats(player *model.Player) (GameResult, bool, error) {
	var total int64
	g.store.DB.Model(&model.Title{}).Where("enabled = ?", true).Count(&total)
	unlocked := int64(1)
	if player.CoupleID != 0 {
		unlocked++
	}
	if player.RealmName == "飞升" {
		unlocked++
	}
	if normalizedPlayerLuck(player.Luck) >= maximumPlayerLuck {
		unlocked++
	}
	percent := int64(0)
	if total > 0 {
		percent = unlocked * 100 / total
	}
	return GameResult{Title: "成就统计", Content: fmt.Sprintf("已解锁：%d/%d\n完成度：%d%%\n当前称号：%s", unlocked, total, percent, displayOr(player.Title, "初入仙途"))}, true, nil
}

func (g *Game) executeSpecial(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 97:
		var rows []model.SocialMessage
		_ = g.store.DB.Where("(sender_id = ? OR receiver_id = ?) AND type IN ?", player.ID, player.ID, []string{"couple_request", "dissolve_request", "diary"}).Order("id DESC").Limit(20).Find(&rows).Error
		if len(rows) == 0 {
			return GameResult{Title: "三生石", Content: "石面清净，尚无重要回忆。"}, true, nil
		}
		lines := make([]string, 0, len(rows))
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("- %s · %s", row.CreatedAt.Format("2006-01-02"), row.Content))
		}
		return GameResult{Title: "三生石", Content: strings.Join(lines, "\n")}, true, nil
	case 98:
		content := strings.TrimSpace(command.RawArguments)
		if content == "" || isPositivePageArgument(content) {
			return g.diaryEntries(player, content)
		}
		if rejected, blocked, err := g.rejectSensitiveContent("日记", player, content); err != nil || blocked {
			return rejected, true, err
		}
		row := model.SocialMessage{SenderID: player.ID, ReceiverID: player.ID, Type: "diary", Content: content, Read: true}
		if err := g.social.Create(&row); err != nil {
			return GameResult{}, true, err
		}
		_ = g.queueContentReview("日记", player, content)
		return GameResult{Title: "感悟已记", Content: fmt.Sprintf("写于：%s\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n这篇感悟已收入你的修仙日记与修仙年表。", row.CreatedAt.Format("2006-01-02 15:04"), content), Actions: []string{"日记", "三生石", "年表"}}, true, nil
	case 99:
		if len(command.Arguments) == 0 || len(command.Arguments) == 1 && isPositivePageArgument(command.Arguments[0]) {
			return g.receivedMessages(player, command.RawArguments)
		}
		if len(command.Arguments) < 2 {
			return GameResult{Title: "道侣留言", Content: "请输入：`留言 @对方 内容`；单独发送“留言”可查看收到的留言。", Actions: []string{"留言", "通知"}}, true, nil
		}
		target, err := g.findPlayer(command.Arguments[0])
		if err != nil {
			return GameResult{Title: "留言失败", Content: "目标不存在。"}, true, nil
		}
		content := strings.Join(command.Arguments[1:], " ")
		if rejected, blocked, err := g.rejectSensitiveContent("留言", player, content); err != nil || blocked {
			return rejected, true, err
		}
		row := model.SocialMessage{SenderID: player.ID, ReceiverID: target.ID, Type: "message", Content: content}
		if err := g.social.Create(&row); err != nil {
			return GameResult{}, true, err
		}
		_ = g.queueContentReview("留言", player, content)
		return GameResult{Title: "留言已送达", Content: "收信人：" + target.DaoName + "\n送达时间：" + row.CreatedAt.Format("2006-01-02 15:04") + "\n━━━━━━━━━━━\n" + content + "\n━━━━━━━━━━━\n对方会在“通知”与“留言”信箱中看到这条内容。", Actions: []string{"留言", "通知", "心意"}}, true, nil
	case 100:
		var players []model.Player
		if err := g.store.DB.Order("merit DESC,id").Limit(10).Find(&players).Error; err != nil {
			return GameResult{}, true, err
		}
		lines := make([]string, 0, len(players))
		for i, row := range players {
			lines = append(lines, fmt.Sprintf("%d. %s · %d功德", i+1, row.DaoName, row.Merit))
		}
		return GameResult{Title: "功德榜", Content: strings.Join(lines, "\n")}, true, nil
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) diaryEntries(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 5
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.SocialMessage{}).Where("sender_id = ? AND receiver_id = ? AND type = ?", player.ID, player.ID, "diary")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.SocialMessage
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("道友：%s · 共%d篇 · 第%d/%d页", player.DaoName, total, page, pages), "━━━━━━━━━━━"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("【%s】\n%s", row.CreatedAt.Format("2006-01-02 15:04"), row.Content), "━━━━━━━")
	}
	if len(rows) == 0 {
		lines = append(lines, "尚未写下修行感悟。发送“日记 内容”记录第一篇。")
	}
	actions := []string{"日记 今日感悟：", "三生石", "年表"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("日记 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("日记 %d", page+1))
	}
	return GameResult{Title: "修仙日记", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) receivedMessages(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.SocialMessage{}).Where("receiver_id = ? AND type = ?", player.ID, "message")
	var total, unread int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	if err := g.store.DB.Model(&model.SocialMessage{}).Where("receiver_id = ? AND type = ? AND read = ?", player.ID, "message", false).Count(&unread).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	type messageRow struct {
		ID         uint
		SenderName string
		Content    string
		Read       bool
		CreatedAt  time.Time
	}
	var rows []messageRow
	if err := g.store.DB.Table("social_messages AS messages").Select("messages.id, players.dao_name AS sender_name, messages.content, messages.read, messages.created_at").Joins("LEFT JOIN players ON players.id = messages.sender_id").Where("messages.receiver_id = ? AND messages.type = ?", player.ID, "message").Order("messages.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("共%d条 · %d条未读 · 第%d/%d页", total, unread, page, pages), "━━━━━━━━━━━"}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		mark := ""
		if !row.Read {
			mark = "【未读】"
		}
		lines = append(lines, fmt.Sprintf("%s%s · %s\n%s", mark, displayOr(row.SenderName, "无名道友"), row.CreatedAt.Format("01-02 15:04"), row.Content), "━━━━━━━")
		ids = append(ids, row.ID)
	}
	if len(rows) == 0 {
		lines = append(lines, "信笺空空，尚未收到道友留言。")
	}
	if len(ids) > 0 {
		_ = g.store.DB.Model(&model.SocialMessage{}).Where("id IN ?", ids).Update("read", true).Error
	}
	actions := []string{"留言 @对方 内容", "通知"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("留言 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("留言 %d", page+1))
	}
	return GameResult{Title: "道侣留言簿", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func isPositivePageArgument(raw string) bool {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	return err == nil && value > 0
}
