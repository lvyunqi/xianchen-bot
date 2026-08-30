package service

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) rechargePriceTable(player *model.Player, raw string) GameResult {
	page := minInt(maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1), 4)
	stoneRate := max64(g.settingInt("recharge.spirit_stones_per_yuan", 2_000_000), 1)
	jadeRate := max64(g.settingInt("recharge.jade_per_yuan", 2_000), 1)
	pages := [][]string{
		{
			fmt.Sprintf("道号：%s · 当前灵石%d · 仙金%d", player.DaoName, player.SpiritStones, player.ImmortalJade),
			"【货币比例】",
			fmt.Sprintf("灵石：1元 = %d（200万）", stoneRate),
			fmt.Sprintf("仙金：1元 = %d", jadeRate),
			"银币：只由签到、任务、活动、交易与运营奖励产出，不开放人民币直充。",
			"━━━━━━━",
			"到账由主人使用神令按全服唯一道号发放；灵石、仙金与累充记录在同一事务写入，并保留操作审计。发送“累充”或“发累充”查询本人累计额度。",
			g.settingText("recharge.instructions", "请联系主人办理。"),
		},
		{
			"【自动定制服务】",
			"定制灵根 · 25元：从已开放灵根图鉴指定一条真实灵根，按新本源重算属性，不叠加旧灵根。",
			"定制法宝 · 30元/件：更名并首次觉醒1星；同一件法宝以后改名不重复加星。新系列器模50元/件。",
			"定制称号 · 40元起：名称审核通过后获得固定均衡属性；更名不重复叠加，同一时间只佩戴一个称号。",
			"定制灵兽 · 50元/只：更名并首次获得攻击、防御、体魄各10%的血契成长；进化时保留，不可反复叠加。",
			"定制仙府 · 50元：更名并首次获得洞天繁荣与基础设施增益；重复更名只改展示。",
			"━━━━━━━",
			"以上自动项目都需要对应定制凭证；凭证只在事务成功后消耗，失败不会扣除。",
		},
		{
			"【人工审核定制】",
			"伤害功法 · 25元：单目标招式，威力按同境功法预算。",
			"增幅功法 · 50元：单项增幅；100元为双项，200元至尊版最多三项，百分比有统一上限。",
			"完整自创功法 · 50元；至尊道统版200元，仍受境界、法力消耗与冷却约束。",
			"定制装备 · 30元/件；新系列50元/件。套装效果按件数逐级触发，不给无条件满额属性。",
			"定制傀儡 · 50元起：攻、防、辅三种定位择一；传送阵纹额外20元，仅解锁已有阵点。",
			"定制地图全套 · 50元：独立地点、NPC、妖兽、采集、任务与路线均需审核，不建立直达高阶世界的捷径。",
			"灵根进化定制 · 50元/次：在现有进化规则与属性预算内调整路线，不跳过境界天关。",
			"━━━━━━━",
			"仙尘不提供神位、战舰定制，也不出售后台权限、无限属性、必胜、无消耗或全图直达。",
		},
		{
			"【属性预算规则】",
			"基础档：一项核心定位，固定属性总预算不超过同境普通装备两件。",
			"进阶档：最多两项定位；单项战斗百分比不超过5%，修炼/产出类不超过10%。",
			"至尊档：最多三项定位；总战斗百分比不超过12%，且必须配置消耗、冷却或境界前置。",
			"称号固定预算：攻击+20、防御+12、气血+120、法力+60；低于排行榜前三尊号，不覆盖活动与成就称号价值。",
			"灵兽血契：当前攻击、防御、体魄各+10%，每只仅首次生效一次。法宝器魂：每件仅首次+1星。仙府地契：首次繁荣+200、阵法/兽室/仓库各+1级。",
			"━━━━━━━",
			"所有定制文本先过敏感词与重名审核；属性由系统固定预算生成，玩家不能填写任意大数。付款、凭证、审核、发放与属性变化均留审计记录。",
			"安全提示：官机不会索要密码、验证码或远程控制。",
		},
	}
	lines := append([]string{fmt.Sprintf("仙尘独立氪金价格表 · 第%d/4页", page), "━━━━━━━━━━━"}, pages[page-1]...)
	actions := []string{"累充"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("氪金菜单 %d", page-1))
	}
	if page < 4 {
		actions = append(actions, fmt.Sprintf("氪金菜单 %d", page+1))
	}
	return GameResult{Title: "💎 仙尘氪金菜单", Content: strings.Join(lines, "\n"), Actions: actions}
}

func (g *Game) cumulativeRecharge(player *model.Player) GameResult {
	total := g.playerValueInt(player.ID, "recharge.total_yuan", 0)
	return GameResult{Title: "💎 累计充值", Content: fmt.Sprintf("道号：%s\n累计充值：%d元\n当前灵石：%d\n当前仙金：%d\n━━━━━━━━━━━\n累计额度只统计主人通过统一充值神令完成的真实入账；活动赠送、反馈奖励、交易所得与管理补偿不计入累充。", player.DaoName, total, player.SpiritStones, player.ImmortalJade), Actions: []string{"氪金菜单"}}
}

func (g *Game) shopList(player *model.Player, argument string, seedsOnly bool) (GameResult, bool, error) {
	const pageSize = 8
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(argument), 1)), 1)
	query := g.store.DB.Model(&model.ShopEntry{}).Where("enabled = ?", true)
	if seedsOnly {
		query = query.Where("code LIKE ?", "seed_shop_%")
	} else {
		query = query.Where("code NOT LIKE ? AND currency = ?", "seed_shop_%", "灵石")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	var rows []model.ShopEntry
	if err := query.Order("sort,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	title := "仙门货铺"
	command := "货铺"
	buyCommand := "购入 "
	intro := "仙盟经营的常设货铺，所有商品常设不限购；购买数量只受实际余额与系统安全数值范围约束。"
	if seedsOnly {
		title, command, buyCommand = "灵植种子商店", "种子商店", "购买种子 "
		intro = "每枚种子都有对应灵植、生长周期与基础产量。购买后发送 `种田 种子名` 播种。"
	}
	if len(rows) == 0 {
		return GameResult{Title: title, Content: "掌柜今日尚未上架商品，请主人检查商店货品配置。", Actions: []string{"交易菜单", "仙府"}}, true, nil
	}
	lines := []string{intro, "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows)+3)
	for _, row := range rows {
		var item model.Item
		_ = g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).Order("id").First(&item).Error
		lines = append(lines, fmt.Sprintf("- %s · %d%s · 常设不限购\n  %s", row.ItemName, row.Price, row.Currency, displayOr(item.Description, "可在对应修仙玩法中使用。")))
		actions = append(actions, buyCommand+row.ItemName)
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d件", page, pages, total), fmt.Sprintf("灵石：%d · 银币：%d · 仙金：%d · 竞技币：%d", player.SpiritStones, player.SilverCoins, player.ImmortalJade, player.ArenaCoins))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command, page+1))
	}
	if seedsOnly {
		actions = append(actions, "仙府", "种田")
	} else {
		actions = append(actions, "集市", "背包")
	}
	return GameResult{Title: title, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) shopHub(player *model.Player) GameResult {
	var contribution int64
	_ = g.store.DB.Model(&model.SectMember{}).Select("contribution").Where("player_id = ?", player.ID).Scan(&contribution).Error
	content := fmt.Sprintf("道友：%s\n━━━━━━━━━━━\n灵石：%d · 大世界流通与基础物资\n银币：%d · 签到、任务免费获得\n仙金：%d · 充值货币与外观便利\n宗门贡献：%d · 宗务、宗战所得\n竞技币：%d · 问剑竞技所得\n━━━━━━━━━━━\n【神秘商城】每日轮换难刷、难合成材料，个人当日限购\n【限时商城】每六小时轮换高阶成品与珍品，个人本轮限购\n【仙门货铺】基础丹药、材料与修行消耗品\n【银币商城】免费货币兑换的日常补给\n【仙金商城】礼包、护劫与定制凭证，不出售境界直升\n【宗门商店】以贡献兑换宗门物资\n【竞技商店】以竞技币兑换战斗与炼器材料\n【种子商店】灵田全部已启用灵种\n【玩家集市】玩家上架和购买真实库存", player.DaoName, player.SpiritStones, player.SilverCoins, player.ImmortalJade, contribution, player.ArenaCoins)
	return GameResult{Title: "🛒 仙尘万宝商会", Content: content, Actions: []string{"神秘商城", "限时商城", "货铺", "银币商城", "仙金商城", "宗商", "竞技商店", "种子商店", "集市", "礼包", "货币"}}
}

func (g *Game) buyShopCommand(player *model.Player, arguments []string, seedsOnly bool) (GameResult, bool, error) {
	guide, listCommand := "购入 物品名 [数量]", "货铺"
	if seedsOnly {
		guide, listCommand = "购买种子 种子名 [数量]", "种子商店"
	}
	if len(arguments) == 0 {
		return GameResult{Title: "购买指引", Content: "请输入：`" + guide + "`。所有商品均可从列表蓝字直接购买，不需要编号或ID。", Actions: []string{listCommand}}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(arguments)
	if parseErr != nil {
		return GameResult{Title: "购买数量错误", Content: parseErr.Error(), Actions: []string{listCommand}}, true, nil
	}
	query := g.store.DB.Where("item_name = ? AND enabled = ?", name, true)
	if seedsOnly {
		query = query.Where("code LIKE ?", "seed_shop_%")
	} else {
		query = query.Where("code NOT LIKE ?", "seed_shop_%")
	}
	var row model.ShopEntry
	if err := query.Order("sort,id").First(&row).Error; err != nil {
		return GameResult{Title: "商品不存在", Content: "货架上没有“" + name + "”，请从当前货铺蓝字中选择。", Actions: []string{listCommand}}, true, nil
	}
	if row.Price < 0 || row.Price > 0 && quantity > int64(^uint64(0)>>1)/row.Price {
		return GameResult{Title: "数量过大", Content: "购买总价超过系统可安全计算范围，请拆分购买。", Actions: []string{listCommand}}, true, nil
	}
	total := row.Price * quantity
	var item model.Item
	if err := g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).Order("id").First(&item).Error; err != nil {
		return GameResult{Title: "货品道纹缺失", Content: "该商品没有关联有效物品，请主人检查商店数据。", Actions: []string{listCommand}}, true, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		switch row.Currency {
		case "灵石":
			result := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, total).Update("spirit_stones", gorm.Expr("spirit_stones - ?", total))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientCurrency
			}
		case "贡献":
			result := tx.Model(&model.SectMember{}).Where("player_id = ? AND contribution >= ?", player.ID, total).Update("contribution", gorm.Expr("contribution - ?", total))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientCurrency
			}
		case "竞技币":
			result := tx.Model(&model.Player{}).Where("id = ? AND arena_coins >= ?", player.ID, total).Update("arena_coins", gorm.Expr("arena_coins - ?", total))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientCurrency
			}
		case "银币":
			result := tx.Model(&model.Player{}).Where("id = ? AND silver_coins >= ?", player.ID, total).Update("silver_coins", gorm.Expr("silver_coins - ?", total))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientCurrency
			}
		case "仙金":
			result := tx.Model(&model.Player{}).Where("id = ? AND immortal_jade >= ?", player.ID, total).Update("immortal_jade", gorm.Expr("immortal_jade - ?", total))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientCurrency
			}
		default:
			return fmt.Errorf("不支持的货币类型：%s", row.Currency)
		}
		return storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity)
	})
	if err != nil {
		if err == errInsufficientCurrency {
			guide, sourceActions := currencyAcquisitionGuide(row.Currency)
			actions := append([]string{listCommand}, sourceActions...)
			return GameResult{Title: row.Currency + "不足", Content: fmt.Sprintf("购买%s×%d需要%d%s，当前余额不足。\n━━━━━━━━━━━\n%s", row.ItemName, quantity, total, row.Currency, guide), Actions: actions}, true, nil
		}
		return GameResult{}, true, err
	}
	return GameResult{Title: "购入成功", Content: fmt.Sprintf("获得：%s × %d\n支付：%d%s\n购买规则：常设不限购\n物品已收入乾坤袋。", row.ItemName, quantity, total, row.Currency), Actions: []string{"物品 " + row.ItemName, "背包", listCommand}}, true, nil
}

func currencyAcquisitionGuide(currency string) (string, []string) {
	switch currency {
	case "灵石":
		return "获取灵石：新道友可先开启青云入道礼匣；之后完成日常与悬赏、探索妖兽、讨伐首领、通关副本、领取挂机收获、出售灵植或参与玩家交易。", []string{"礼包", "日常", "悬赏", "探索", "首领", "副本", "挂机", "灵田仓库", "集市", "货币"}
	case "银币":
		return "获取银币：每日签到、活跃任务、竞技俸禄、排行俸禄与仙盟活动均可免费获得。", []string{"签到", "日常", "竞技奖励", "排行榜", "货币"}
	case "仙金":
		return "获取仙金：查看统一充值价格表并联系主人完成入账；玩法商城不会从其他货币中代扣。", []string{"充值菜单", "价格表", "货币"}
	case "贡献":
		return "获取宗门贡献：加入宗门后完成宗务、宗门任务、宗门首领与宗门战争。", []string{"宗门菜单", "宗务", "宗门战争菜单"}
	case "竞技币":
		return "获取竞技币：参与逐回合问剑竞技并领取每日段位俸禄。", []string{"竞技", "竞技奖励", "竞技档案"}
	default:
		return "可在货币钱庄查看该货币的实际获取途径。", []string{"货币", "帮助"}
	}
}

func (g *Game) currencyShopList(player *model.Player, raw, currency string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 8
	query := g.store.DB.Model(&model.ShopEntry{}).Where("enabled = ? AND currency = ? AND code NOT LIKE ?", true, currency, "seed_shop_%")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.ShopEntry
	if err := query.Order("sort,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	balance := player.SilverCoins
	buyCommand := "银币购买 "
	if currency == "仙金" {
		balance = player.ImmortalJade
		buyCommand = "仙金购买 "
	}
	lines := []string{fmt.Sprintf("当前%s：%d", currency, balance), "商品名称就是购买凭证，不需要编号或玩家ID。", "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows)+4)
	for _, row := range rows {
		var item model.Item
		_ = g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).Order("id").First(&item).Error
		lines = append(lines, fmt.Sprintf("- %s · %d%s\n  %s · 常设不限购", row.ItemName, row.Price, currency, displayOr(item.Description, "暂无物品说明")))
		actions = append(actions, buyCommand+row.ItemName)
	}
	if len(rows) == 0 {
		lines = append(lines, "当前货架尚未上架商品，请主人检查货币为“"+currency+"”的商店数据。")
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d件", page, pages, total))
	command := currency + "商城"
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command, page+1))
	}
	actions = append(actions, "货币", "银币商城", "仙金商城")
	return GameResult{Title: currency + "商城", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) buyCurrencyShop(player *model.Player, arguments []string, currency string) (GameResult, bool, error) {
	if len(arguments) == 0 {
		return GameResult{Title: currency + "购买", Content: fmt.Sprintf("请输入：`%s购买 物品名 [数量]`，也可直接点击商城蓝字。", currency), Actions: []string{currency + "商城"}}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(arguments)
	if parseErr != nil {
		return GameResult{Title: "购买数量错误", Content: parseErr.Error(), Actions: []string{currency + "商城"}}, true, nil
	}
	var row model.ShopEntry
	if err := g.store.DB.Where("item_name = ? AND currency = ? AND enabled = ? AND code NOT LIKE ?", name, currency, true, "seed_shop_%").Order("sort,id").First(&row).Error; err != nil {
		return GameResult{Title: "商品不存在", Content: currency + "商城没有上架“" + name + "”。", Actions: []string{currency + "商城"}}, true, nil
	}
	if row.Price < 0 || row.Price > 0 && quantity > int64(^uint64(0)>>1)/row.Price {
		return GameResult{Title: "数量过大", Content: "购买总价超过系统可安全计算范围，请拆分购买。"}, true, nil
	}
	total := row.Price * quantity
	var item model.Item
	if err := g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).Order("id").First(&item).Error; err != nil {
		return GameResult{Title: "货品配置错误", Content: "商品没有关联有效物品。"}, true, nil
	}
	column := "silver_coins"
	if currency == "仙金" {
		column = "immortal_jade"
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND "+column+" >= ?", player.ID, total).Update(column, gorm.Expr(column+" - ?", total))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		return storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity)
	})
	if err == errInsufficientCurrency {
		return GameResult{Title: currency + "不足", Content: fmt.Sprintf("购买%s×%d需要%d%s。", row.ItemName, quantity, total, currency), Actions: []string{"货币", currency + "商城", "签到"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "购买成功", Content: fmt.Sprintf("获得：%s×%d\n支付：%d%s\n购买规则：常设不限购\n物品已经收入乾坤袋。", row.ItemName, quantity, total, currency), Actions: []string{"物品 " + row.ItemName, "背包", currency + "商城", "货币"}}, true, nil
}

var errInsufficientCurrency = fmt.Errorf("insufficient currency")

func parseShopPurchase(arguments []string) (string, int64, error) {
	raw := strings.TrimSpace(strings.Join(arguments, " "))
	if strings.ContainsAny(raw, "*×") {
		return parseStackQuantity(raw)
	}
	quantity := int64(1)
	nameParts := arguments
	if len(arguments) > 1 {
		if parsed, err := strconv.ParseInt(arguments[len(arguments)-1], 10, 64); err == nil {
			if parsed <= 0 {
				return "", 0, fmt.Errorf("数量格式不正确，请使用正整数")
			}
			quantity = parsed
			nameParts = arguments[:len(arguments)-1]
		}
	}
	name := strings.TrimSpace(strings.Join(nameParts, " "))
	if name == "" {
		return "", 0, fmt.Errorf("商品名称不能为空")
	}
	return name, quantity, nil
}
