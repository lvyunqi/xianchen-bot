package service

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type birthdayPrize struct {
	Name      string
	Quantity  int64
	Weight    int64
	Equipment bool
}

type birthdayExchangeEntry struct {
	Name        string
	Price       int64
	Description string
	Equipment   bool
}

var birthdayPrizePool = []birthdayPrize{
	{Name: "灵果", Quantity: 3, Weight: 280},
	{Name: "阵基石", Quantity: 2, Weight: 190},
	{Name: "功法残卷", Quantity: 1, Weight: 150},
	{Name: "雷灵晶", Quantity: 1, Weight: 105},
	{Name: "龙血芝", Quantity: 1, Weight: 75},
	{Name: "长生蟠桃", Quantity: 1, Weight: 65},
	{Name: "生辰许愿灯", Quantity: 1, Weight: 40},
	{Name: "万福同心礼匣", Quantity: 1, Weight: 20},
	{Name: birthdayArtifactName, Quantity: 1, Weight: 5, Equipment: true},
}

var birthdayExchangeCatalog = []birthdayExchangeEntry{
	{Name: "长生蟠桃", Price: 8, Description: "服用后获得888修为并同步角色经验"},
	{Name: "生辰许愿灯", Price: 12, Description: "两小时闭关修炼收益提高25%"},
	{Name: "万福同心礼匣", Price: 24, Description: "生辰道具、材料、银币、灵石与功德礼匣"},
	{Name: "造化仙壤", Price: 16, Description: "高阶灵田催生、增产与抗灾灵肥"},
	{Name: "灵根精粹", Price: 20, Description: "灵根合成与本源传承材料"},
	{Name: birthdayArtifactName, Price: 60, Description: "生辰限定腰佩，每名玩家终身只能获得一件", Equipment: true},
}

func parseBirthdayDrawCount(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(raw), "次"), "抽"))
	raw = strings.TrimLeft(raw, "*×")
	if raw == "" {
		return 1, nil
	}
	count, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || count <= 0 {
		return 0, fmt.Errorf("抽奖次数需为正整数，例如 `生辰抽奖 10`")
	}
	return count, nil
}

func drawBirthdayPrize() (birthdayPrize, error) {
	total := int64(0)
	for _, prize := range birthdayPrizePool {
		total += prize.Weight
	}
	if total <= 0 {
		return birthdayPrize{}, fmt.Errorf("birthday prize pool is empty")
	}
	roll, err := cryptorand.Int(cryptorand.Reader, big.NewInt(total))
	if err != nil {
		return birthdayPrize{}, err
	}
	value := roll.Int64()
	for _, prize := range birthdayPrizePool {
		if value < prize.Weight {
			return prize, nil
		}
		value -= prize.Weight
	}
	return birthdayPrizePool[len(birthdayPrizePool)-1], nil
}

func grantBirthdayArtifactTx(tx *gorm.DB, playerID uint) (bool, error) {
	var template model.ArtifactTemplate
	if err := tx.Where("code = ? AND enabled = ?", birthdayArtifactCode, true).First(&template).Error; err != nil {
		return false, err
	}
	var existing int64
	if err := tx.Model(&model.PlayerArtifact{}).Where("player_id = ? AND template_id = ?", playerID, template.ID).Count(&existing).Error; err != nil {
		return false, err
	}
	if existing > 0 {
		return false, nil
	}
	row := model.PlayerArtifact{
		PlayerID: playerID, TemplateID: template.ID, Name: template.Name,
		Level: 1, Quality: "灵品", Slot: artifactTemplateSlot(template), StarLevel: 1,
	}
	return true, tx.Create(&row).Error
}

func (g *Game) birthdayLottery(player *model.Player, raw string) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	count, parseErr := parseBirthdayDrawCount(raw)
	if parseErr != nil {
		return GameResult{Title: "🎊 生辰抽奖次数错误", Content: parseErr.Error(), Actions: []string{"生辰抽奖", "生日菜单"}}, true, nil
	}
	const ticketCost int64 = 3
	if count > math.MaxInt64/ticketCost {
		return GameResult{Title: "🎊 抽奖次数过大", Content: "福签消耗超过安全计算范围，请拆分抽取。", Actions: []string{"生辰抽奖"}}, true, nil
	}
	totalCost := count * ticketCost
	if g.birthdayTicketQuantity(player.ID) < totalCost {
		return GameResult{Title: "🎊 岁序福签不足", Content: fmt.Sprintf("抽取%d次需要岁序福签×%d，当前持有%d。可完成生日任务、接受祝福或领取仙尘生辰礼。", count, totalCost, g.birthdayTicketQuantity(player.ID)), Actions: []string{"生日任务", "领取生日礼物", "生辰签到", "生日菜单"}}, true, nil
	}
	rewards := make(map[string]int64)
	equipmentWins := int64(0)
	for index := int64(0); index < count; index++ {
		prize, err := drawBirthdayPrize()
		if err != nil {
			return GameResult{}, true, err
		}
		if prize.Equipment {
			equipmentWins += prize.Quantity
		} else {
			rewards[prize.Name] += prize.Quantity
		}
	}
	convertedTickets := int64(0)
	artifactGranted := false
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var ticket model.Item
		if err := tx.Where("name = ?", birthdayTicketName).First(&ticket).Error; err != nil {
			return err
		}
		repo := storage.NewPlayerRepository(tx)
		if err := repo.AdjustItem(player.ID, ticket.ID, -totalCost); err != nil {
			return errInsufficientCurrency
		}
		for name, quantity := range rewards {
			if err := grantNamedItemTx(tx, player.ID, name, quantity); err != nil {
				return err
			}
		}
		for index := int64(0); index < equipmentWins; index++ {
			granted, err := grantBirthdayArtifactTx(tx, player.ID)
			if err != nil {
				return err
			}
			if granted {
				artifactGranted = true
			} else {
				convertedTickets += 20
			}
		}
		if convertedTickets > 0 {
			if err := repo.AdjustItem(player.ID, ticket.ID, convertedTickets); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errInsufficientCurrency) {
		return GameResult{Title: "🎊 福签数量刚刚变化", Content: "抽奖结算前福签数量发生变化，本次没有发放任何奖励，请重新查看。", Actions: []string{"生日菜单", "背包"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	names := make([]string, 0, len(rewards)+2)
	for name := range rewards {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := []string{fmt.Sprintf("抽取：%d次 · 消耗岁序福签×%d", count, totalCost), "━━━━━━━━━━━"}
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("获得：%s×%d", name, rewards[name]))
	}
	if artifactGranted {
		lines = append(lines, "获得限定法器：岁序长生佩【灵品 · 1星】")
	}
	if convertedTickets > 0 {
		lines = append(lines, fmt.Sprintf("重复限定法器自动化为岁序福签×%d", convertedTickets))
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("剩余岁序福签：%d", g.birthdayTicketQuantity(player.ID)))
	return GameResult{Title: "🎊 生辰限定抽奖", Content: strings.Join(lines, "\n"), Actions: []string{"生辰抽奖", "生辰兑换", "装备背包", "背包", "生日菜单"}}, true, nil
}

func (g *Game) birthdayExchange(player *model.Player, raw string) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		lines := []string{fmt.Sprintf("岁序福签：%d", g.birthdayTicketQuantity(player.ID)), "兑换表只在本人生日当天开放，不影响任何原商城。", "━━━━━━━━━━━"}
		actions := []string{"生日菜单", "生辰抽奖"}
		for _, entry := range birthdayExchangeCatalog {
			state := ""
			if entry.Equipment {
				var template model.ArtifactTemplate
				var count int64
				if g.store.DB.Where("code = ?", birthdayArtifactCode).First(&template).Error == nil {
					_ = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND template_id = ?", player.ID, template.ID).Count(&count).Error
				}
				if count > 0 {
					state = " · 已拥有"
				}
			}
			lines = append(lines, fmt.Sprintf("- %s · 福签%d%s\n  %s", entry.Name, entry.Price, state, entry.Description))
			if state == "" {
				actions = append(actions, "生辰兑换 "+entry.Name)
			}
		}
		return GameResult{Title: "🎁 生辰福签兑换", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(strings.Fields(raw))
	if parseErr != nil {
		return GameResult{Title: "🎁 兑换数量错误", Content: parseErr.Error(), Actions: []string{"生辰兑换"}}, true, nil
	}
	var selected birthdayExchangeEntry
	found := false
	for _, entry := range birthdayExchangeCatalog {
		if entry.Name == name {
			selected, found = entry, true
			break
		}
	}
	if !found {
		return GameResult{Title: "🎁 生辰兑换不存在", Content: "请从今日生辰兑换表蓝字中选择。", Actions: []string{"生辰兑换"}}, true, nil
	}
	if selected.Equipment && quantity != 1 {
		return GameResult{Title: "🎁 限定法器唯一", Content: "岁序长生佩每名玩家终身只能获得一件，兑换数量只能为1。", Actions: []string{"生辰兑换", "装备背包"}}, true, nil
	}
	if selected.Price <= 0 || quantity > math.MaxInt64/selected.Price {
		return GameResult{Title: "🎁 兑换数量过大", Content: "福签消耗超过安全计算范围，请拆分兑换。", Actions: []string{"生辰兑换"}}, true, nil
	}
	total := selected.Price * quantity
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var ticket model.Item
		if err := tx.Where("name = ?", birthdayTicketName).First(&ticket).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, ticket.ID, -total); err != nil {
			return errInsufficientCurrency
		}
		if selected.Equipment {
			granted, err := grantBirthdayArtifactTx(tx, player.ID)
			if err != nil {
				return err
			}
			if !granted {
				return errBirthdayUniqueArtifact
			}
			return nil
		}
		return grantNamedItemTx(tx, player.ID, selected.Name, quantity)
	})
	if errors.Is(err, errInsufficientCurrency) {
		return GameResult{Title: "🎁 岁序福签不足", Content: fmt.Sprintf("兑换%s×%d需要福签%d，当前持有%d。本次没有发放物品。", selected.Name, quantity, total, g.birthdayTicketQuantity(player.ID)), Actions: []string{"生日任务", "生辰抽奖", "生日菜单"}}, true, nil
	}
	if errors.Is(err, errBirthdayUniqueArtifact) {
		return GameResult{Title: "🎁 限定法器已拥有", Content: "你已经拥有岁序长生佩，本次兑换已完整回滚，没有扣除福签。", Actions: []string{"装备背包", "生辰兑换"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	quality := ""
	actions := []string{"物品 " + selected.Name, "背包", "生辰兑换", "生日菜单"}
	if selected.Equipment {
		quality = "【灵品 · 1星】"
		actions = []string{"装备详情 " + selected.Name, "装备背包", "穿戴 " + selected.Name, "生辰兑换", "生日菜单"}
	}
	return GameResult{Title: "🎁 生辰兑换完成", Content: fmt.Sprintf("获得：%s%s×%d\n消耗：岁序福签×%d\n剩余福签：%d", selected.Name, quality, quantity, total, g.birthdayTicketQuantity(player.ID)), Actions: actions}, true, nil
}

func (g *Game) birthdayRanking(player *model.Player, raw string) (GameResult, bool, error) {
	rows, err := g.todayBirthdayPlayers(time.Now())
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "🎂 寿星榜今日未开", Content: "今天没有已登记生日的寿星，榜单与对应尊号暂不显示。", Actions: []string{"状态"}}, true, nil
	}
	argument := "生辰"
	if page := strings.TrimSpace(raw); page != "" {
		argument += " " + page
	}
	return g.rankingCenter(player, argument)
}
