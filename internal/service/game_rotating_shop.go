package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type rotatingShopGood struct {
	Code        string
	ItemName    string
	Price       int64
	Stock       int64
	Description string
}

type rotatingShopConfig struct {
	Code        string
	Title       string
	ListCommand string
	BuyCommand  string
	Intro       string
	Slots       int
	Goods       []rotatingShopGood
}

var mysteryShopConfig = rotatingShopConfig{
	Code:        "mystery",
	Title:       "🕯️ 神秘商城",
	ListCommand: "神秘商城",
	BuyCommand:  "神秘购买",
	Intro:       "太虚行商每日带来一批难刷、难合成的修仙物资。商品每天零点轮换，价格与个人库存当日固定。",
	Slots:       6,
	Goods: []rotatingShopGood{
		{Code: "formation_stone", ItemName: "阵基石", Price: 58, Stock: 8, Description: "布阵、护府、篆刻与渡劫合成的核心材料"},
		{Code: "root_essence", ItemName: "灵根精粹", Price: 360, Stock: 2, Description: "随机灵根合成与本源传承材料"},
		{Code: "thunder_crystal", ItemName: "雷灵晶", Price: 220, Stock: 3, Description: "引劫玉符、避劫符与高阶炼器材料"},
		{Code: "dragon_blood", ItemName: "龙血芝", Price: 480, Stock: 2, Description: "高阶丹药、轮回丹与造化仙壤材料"},
		{Code: "star_sand", ItemName: "星辰砂", Price: 95, Stock: 5, Description: "空间、雷炼、炼器与高阶合成材料"},
		{Code: "beast_core", ItemName: "妖兽内丹", Price: 70, Stock: 8, Description: "破境丹药、灵根精粹与炼器常用材料"},
		{Code: "moon_flower", ItemName: "月华花", Price: 130, Stock: 5, Description: "破境、高阶疗伤与星砂精炼材料"},
		{Code: "skill_scroll", ItemName: "功法残卷", Price: 180, Stock: 3, Description: "参悟、学习及分享功法所需残卷"},
		{Code: "teleport_charm", ItemName: "传送符", Price: 150, Stock: 4, Description: "已刻录地点传送与扫荡券合成材料"},
		{Code: "sweep_ticket", ItemName: "扫荡券", Price: 220, Stock: 3, Description: "用于扫荡已经手动通关的副本"},
		{Code: "leyline_fertilizer", ItemName: "地脉灵肥", Price: 110, Stock: 5, Description: "灵田催生、增产并提高抗灾能力"},
		{Code: "creation_soil", ItemName: "造化仙壤", Price: 390, Stock: 2, Description: "高阶灵田强力催生、增产与抗灾灵肥"},
	},
}

var limitedShopConfig = rotatingShopConfig{
	Code:        "limited",
	Title:       "⏳ 限时商城",
	ListCommand: "限时商城",
	BuyCommand:  "限时购买",
	Intro:       "巡界宝舟每六小时停靠一次，只出售小库存珍品与高阶成品。每轮货架、价格和个人库存固定。",
	Slots:       5,
	Goods: []rotatingShopGood{
		{Code: "tribulation_talisman", ItemName: "引劫玉符", Price: 650, Stock: 1, Description: "大境十层圆满后开启三重天劫的必要玉符"},
		{Code: "tribulation_charm", ItemName: "避劫符", Price: 380, Stock: 2, Description: "渡劫时抵消部分劫雷威能"},
		{Code: "revive_pill", ItemName: "九转还魂丹", Price: 1200, Stock: 1, Description: "濒死时恢复全部气血的高阶救命丹"},
		{Code: "rebirth_pill", ItemName: "轮回丹", Price: 2000, Stock: 1, Description: "高境界兵解转世时护持真灵"},
		{Code: "double_cultivation", ItemName: "双倍修为卡", Price: 600, Stock: 1, Description: "一小时内提高闭关修炼收益"},
		{Code: "root_essence", ItemName: "灵根精粹", Price: 320, Stock: 2, Description: "本轮折价供应的灵根合成材料"},
		{Code: "creation_soil", ItemName: "造化仙壤", Price: 350, Stock: 2, Description: "高阶灵田催生、增产与抗灾灵肥"},
		{Code: "thunder_gem", ItemName: "九霄雷罡石", Price: 520, Stock: 1, Description: "同时增强攻伐与身法的神品嵌灵宝石"},
		{Code: "star_gem", ItemName: "星河道力核", Price: 580, Stock: 1, Description: "凝聚星河道力的神品嵌灵宝石"},
		{Code: "hunyuan_gem", ItemName: "混元五炁珠", Price: 720, Stock: 1, Description: "均衡增强多项基础战斗属性的神品宝珠"},
		{Code: "sweep_ticket", ItemName: "扫荡券", Price: 190, Stock: 3, Description: "用于扫荡已经手动通关的副本"},
		{Code: "skill_scroll", ItemName: "功法残卷", Price: 150, Stock: 3, Description: "本轮折价供应的功法参悟材料"},
	},
}

var (
	errRotatingShopStockChanged = errors.New("rotating shop stock changed")
	errRotatingShopStockEmpty   = errors.New("rotating shop stock insufficient")
)

func rotatingShopFor(kind string) (rotatingShopConfig, bool) {
	switch kind {
	case mysteryShopConfig.Code:
		return mysteryShopConfig, true
	case limitedShopConfig.Code:
		return limitedShopConfig, true
	default:
		return rotatingShopConfig{}, false
	}
}

func rotatingShopWindow(kind string, now time.Time) (string, time.Time, time.Time) {
	location := now.Location()
	if kind == limitedShopConfig.Code {
		hour := now.Hour() / 6 * 6
		start := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, location)
		return start.Format("20060102-15"), start, start.Add(6 * time.Hour)
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return start.Format("20060102"), start, start.AddDate(0, 0, 1)
}

func rotatingShopSelection(config rotatingShopConfig, windowID string) []rotatingShopGood {
	goods := append([]rotatingShopGood(nil), config.Goods...)
	sort.SliceStable(goods, func(left, right int) bool {
		leftScore := rotatingShopScore(config.Code + "|" + windowID + "|" + goods[left].Code)
		rightScore := rotatingShopScore(config.Code + "|" + windowID + "|" + goods[right].Code)
		if leftScore == rightScore {
			return goods[left].Code < goods[right].Code
		}
		return leftScore < rightScore
	})
	if config.Slots > 0 && len(goods) > config.Slots {
		goods = goods[:config.Slots]
	}
	return goods
}

func rotatingShopScore(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum64()
}

func rotatingShopStockKey(kind, goodCode string) string {
	return "shop.rotation." + kind + "." + goodCode
}

func rotatingShopCounter(value, windowID string) int64 {
	parts := strings.SplitN(strings.TrimSpace(value), "|", 2)
	if len(parts) != 2 || parts[0] != windowID {
		return 0
	}
	count, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func (g *Game) rotatingShopPurchased(playerID uint, config rotatingShopConfig, good rotatingShopGood, windowID string) int64 {
	value, err := g.playerValue(playerID, rotatingShopStockKey(config.Code, good.Code))
	if err != nil {
		return 0
	}
	return rotatingShopCounter(value, windowID)
}

func (g *Game) rotatingShopList(player *model.Player, kind string) (GameResult, bool, error) {
	config, exists := rotatingShopFor(kind)
	if !exists {
		return GameResult{}, false, nil
	}
	now := time.Now()
	windowID, _, refreshAt := rotatingShopWindow(kind, now)
	goods := rotatingShopSelection(config, windowID)
	lines := []string{
		config.Intro,
		fmt.Sprintf("本轮编号：%s · 距离刷新%s", windowID, formatDuration(time.Until(refreshAt))),
		fmt.Sprintf("当前银币：%d · 库存为每位玩家本轮独立限购", player.SilverCoins),
		"━━━━━━━━━━━",
	}
	actions := make([]string, 0, len(goods)+4)
	available := 0
	for _, good := range goods {
		var item model.Item
		if err := g.store.DB.Where("name = ?", good.ItemName).First(&item).Error; err != nil {
			continue
		}
		purchased := g.rotatingShopPurchased(player.ID, config, good, windowID)
		remaining := max64(good.Stock-purchased, 0)
		state := fmt.Sprintf("个人剩余%d/%d", remaining, good.Stock)
		if remaining == 0 {
			state = "本轮已购完"
		} else {
			actions = append(actions, config.BuyCommand+" "+good.ItemName)
		}
		lines = append(lines, fmt.Sprintf("- %s · %d银币 · %s\n  %s", good.ItemName, good.Price, state, good.Description))
		available++
	}
	if available == 0 {
		lines = append(lines, "本轮货品道纹尚未载入，请主人检查物品数据。")
	}
	lines = append(lines,
		"━━━━━━━━━━━",
		fmt.Sprintf("刷新时间：%s", refreshAt.Format("2006-01-02 15:04:05")),
		fmt.Sprintf("购买格式：%s 物品名*数量", config.BuyCommand),
	)
	actions = append(actions, "神秘商城", "限时商城", "银币来源", "货币")
	return GameResult{Title: config.Title, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) buyRotatingShop(player *model.Player, arguments []string, kind string) (GameResult, bool, error) {
	config, exists := rotatingShopFor(kind)
	if !exists {
		return GameResult{}, false, nil
	}
	if len(arguments) == 0 {
		return GameResult{Title: config.Title, Content: fmt.Sprintf("请输入：`%s 物品名*数量`，也可直接点击商城中的商品蓝字。", config.BuyCommand), Actions: []string{config.ListCommand}}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(arguments)
	if parseErr != nil {
		return GameResult{Title: "购买数量错误", Content: parseErr.Error(), Actions: []string{config.ListCommand}}, true, nil
	}
	now := time.Now()
	windowID, _, refreshAt := rotatingShopWindow(kind, now)
	var good rotatingShopGood
	found := false
	for _, candidate := range rotatingShopSelection(config, windowID) {
		if candidate.ItemName == name {
			good, found = candidate, true
			break
		}
	}
	if !found {
		return GameResult{Title: "本轮未上架", Content: fmt.Sprintf("%s本轮没有“%s”，请从当前货架蓝字中选择。", config.Title, name), Actions: []string{config.ListCommand}}, true, nil
	}
	if good.Price <= 0 || quantity > int64(^uint64(0)>>1)/good.Price {
		return GameResult{Title: "数量过大", Content: "购买总价超过系统可安全计算范围，请拆分购买。", Actions: []string{config.ListCommand}}, true, nil
	}
	total := good.Price * quantity
	var item model.Item
	if err := g.store.DB.Where("name = ?", good.ItemName).First(&item).Error; err != nil {
		return GameResult{Title: "货品配置错误", Content: "商品没有关联有效物品，本次没有扣款。", Actions: []string{config.ListCommand}}, true, nil
	}
	remainingBefore := int64(0)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		key := rotatingShopStockKey(config.Code, good.Code)
		var marker model.PlayerValue
		markerErr := tx.Where("player_id = ? AND key = ?", player.ID, key).First(&marker).Error
		if markerErr != nil && !errors.Is(markerErr, gorm.ErrRecordNotFound) {
			return markerErr
		}
		purchased := int64(0)
		if markerErr == nil && (marker.ExpiresAt == nil || marker.ExpiresAt.After(now)) {
			purchased = rotatingShopCounter(marker.Value, windowID)
		}
		remainingBefore = max64(good.Stock-purchased, 0)
		if quantity > remainingBefore {
			return errRotatingShopStockEmpty
		}
		newValue := fmt.Sprintf("%s|%d", windowID, purchased+quantity)
		if errors.Is(markerErr, gorm.ErrRecordNotFound) {
			marker = model.PlayerValue{PlayerID: player.ID, Key: key, Value: newValue, ExpiresAt: &refreshAt}
			if err := tx.Create(&marker).Error; err != nil {
				return errRotatingShopStockChanged
			}
		} else {
			result := tx.Model(&model.PlayerValue{}).
				Where("id = ? AND value = ?", marker.ID, marker.Value).
				Updates(map[string]any{"value": newValue, "expires_at": &refreshAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errRotatingShopStockChanged
			}
		}
		paid := tx.Model(&model.Player{}).
			Where("id = ? AND silver_coins >= ?", player.ID, total).
			Update("silver_coins", gorm.Expr("silver_coins - ?", total))
		if paid.Error != nil {
			return paid.Error
		}
		if paid.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		return storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity)
	})
	if errors.Is(err, errRotatingShopStockEmpty) {
		return GameResult{Title: "本轮库存不足", Content: fmt.Sprintf("%s本轮个人库存仅剩%d件，无法购买%s×%d。本次没有扣款。", good.ItemName, remainingBefore, good.ItemName, quantity), Actions: []string{config.ListCommand}}, true, nil
	}
	if errors.Is(err, errInsufficientCurrency) {
		return GameResult{Title: "银币不足", Content: fmt.Sprintf("购买%s×%d需要%d银币，当前余额不足。本次没有占用库存，也没有发放物品。", good.ItemName, quantity, total), Actions: []string{config.ListCommand, "银币来源", "签到", "货币"}}, true, nil
	}
	if errors.Is(err, errRotatingShopStockChanged) {
		return GameResult{Title: "库存刚刚变化", Content: "同一笔库存刚被其他请求更新，请重新打开商城后再试。本次没有扣款。", Actions: []string{config.ListCommand}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{
		Title:   config.Title + "购买成功",
		Content: fmt.Sprintf("获得：%s×%d\n支付：%d银币\n本轮个人剩余：%d/%d\n下次刷新：%s\n━━━━━━━━━━━\n物品已经收入乾坤袋。", good.ItemName, quantity, total, remainingBefore-quantity, good.Stock, refreshAt.Format("2006-01-02 15:04:05")),
		Actions: []string{"物品 " + good.ItemName, "背包", config.ListCommand, "货币"},
	}, true, nil
}
