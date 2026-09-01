package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type giftPackRewards struct {
	Items        map[string]int64 `json:"items"`
	Artifacts    []string         `json:"artifacts"`
	SpiritStones int64            `json:"spirit_stones"`
	SilverCoins  int64            `json:"silver_coins"`
	ImmortalJade int64            `json:"immortal_jade"`
	ArenaCoins   int64            `json:"arena_coins"`
	Cultivation  int64            `json:"cultivation"`
	Merit        int64            `json:"merit"`
	Reputation   int64            `json:"reputation"`
	DaoHeart     int64            `json:"dao_heart"`
}

func (g *Game) giftPackList(player *model.Player, argument string) (GameResult, bool, error) {
	const pageSize = 4
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(argument), 1)), 1)
	var total int64
	if err := g.store.DB.Model(&model.Item{}).Where("effect_func = ?", "open_gift_pack").Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	type giftRow struct {
		model.Item
		OwnedQuantity int64
	}
	var rows []giftRow
	query := g.store.DB.Table("items").Select("items.*, COALESCE(player_items.quantity, 0) AS owned_quantity").Joins("LEFT JOIN player_items ON player_items.item_id = items.id AND player_items.player_id = ?", player.ID).Where("items.effect_func = ?", "open_gift_pack")
	if err := query.Order("owned_quantity DESC, items.id").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "仙途礼包", Content: "天机阁尚未收录礼包。可通过签到、任务、活动、兑换码与仙门商会获取后再来查看。", Actions: []string{"签到", "任务菜单", "商城"}}, true, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	type shopSource struct {
		ItemID   uint
		Currency string
		Price    int64
	}
	var shopRows []shopSource
	_ = g.store.DB.Model(&model.ShopEntry{}).Select("item_id, currency, price").Where("enabled = ? AND item_id IN ?", true, ids).Order("price").Scan(&shopRows).Error
	sources := make(map[uint]shopSource, len(shopRows))
	for _, source := range shopRows {
		if _, exists := sources[source.ItemID]; !exists {
			sources[source.ItemID] = source
		}
	}
	var ownedTypes int64
	_ = g.store.DB.Table("player_items").Joins("JOIN items ON items.id = player_items.item_id").Where("player_items.player_id = ? AND player_items.quantity > 0 AND items.effect_func = ?", player.ID, "open_gift_pack").Count(&ownedTypes).Error
	header := []string{
		fmt.Sprintf("礼包图鉴：共%d类 · 当前持有%d类", total, ownedTypes),
		"每一类修行礼包均有独立名称、专属丹药、器胚和奖励数值；礼包进入乾坤袋后由你确认开启。",
		"━━━━━━━━━━━",
	}
	lines := append([]string(nil), header...)
	markdownLines := append([]string(nil), header...)
	actions := make([]string, 0, len(rows)+4)
	for _, row := range rows {
		var reward giftPackRewards
		_ = json.Unmarshal([]byte(row.EffectParams), &reward)
		holding := "未持有"
		if row.OwnedQuantity > 0 {
			holding = fmt.Sprintf("持有%d", row.OwnedQuantity)
		}
		sourceText := "活动、任务、兑换与诸界探索"
		sourceAction := "帮助 特殊"
		if row.Code == "gift_starter_qingyun" {
			sourceText = "首次建立道籍时赠送"
			sourceAction = "帮助"
		} else if source, exists := sources[row.ID]; exists {
			sourceText = fmt.Sprintf("%s商城 · %s×%d", source.Currency, source.Currency, source.Price)
			sourceAction = source.Currency + "商城"
		}
		plain := fmt.Sprintf("🎁 %s【%s · %s】\n道藏：%s\n内含：%s\n获取：%s", row.Name, displayOr(row.RarityName, "灵品"), holding, row.Description, describeGiftRewards(reward), sourceText)
		markdown := fmt.Sprintf("🎁 %s【%s · %s】\n道藏：%s\n内含：%s\n获取：%s", markdownInlineCommand(row.Name, "物品 "+row.Name), displayOr(row.RarityName, "灵品"), holding, row.Description, describeGiftRewards(reward), markdownInlineCommand(sourceText, sourceAction))
		if row.OwnedQuantity > 0 {
			markdown += "\n" + markdownInlineCommand("开启此礼包", "开启礼包 "+row.Name)
			actions = append(actions, "开启礼包 "+row.Name)
		}
		lines = append(lines, plain, "━━━━━━━")
		markdownLines = append(markdownLines, markdown, "━━━━━━━")
	}
	pageLine := fmt.Sprintf("第%d/%d页 · 共%d类礼包", page, pages, total)
	lines = append(lines, pageLine)
	markdownLines = append(markdownLines, pageLine)
	actions = append(actions, "背包", "签到", "银币商城", "仙金商城")
	if page > 1 {
		actions = append(actions, fmt.Sprintf("礼包 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("礼包 %d", page+1))
	}
	return GameResult{Title: "仙途礼包", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: actions}, true, nil
}

func (g *Game) openGiftPack(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "开启礼包", Content: "请发送 `礼包` 查看持有礼包，再点击蓝字开启。", Actions: []string{"礼包"}}, true, nil
	}
	var pack model.Item
	if err := g.store.DB.Where("name = ? AND effect_func = ?", name, "open_gift_pack").First(&pack).Error; err != nil || g.itemQuantity(player.ID, pack.ID) < 1 {
		return GameResult{Title: "礼包不存在", Content: "乾坤袋中没有“" + name + "”。", Actions: []string{"礼包", "背包"}}, true, nil
	}
	var reward giftPackRewards
	if err := json.Unmarshal([]byte(pack.EffectParams), &reward); err != nil {
		return GameResult{Title: "礼包配置错误", Content: "礼包奖励道纹无法解析，请主人检查该礼包的奖励配置。"}, true, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		repo := storage.NewPlayerRepository(tx)
		if err := repo.AdjustItem(player.ID, pack.ID, -1); err != nil {
			return err
		}
		for itemName, quantity := range reward.Items {
			if quantity <= 0 {
				continue
			}
			var item model.Item
			if err := tx.Where("name = ?", itemName).First(&item).Error; err != nil {
				return fmt.Errorf("礼包物品%s不存在: %w", itemName, err)
			}
			if err := repo.AdjustItem(player.ID, item.ID, quantity); err != nil {
				return err
			}
		}
		for _, artifactName := range reward.Artifacts {
			var template model.ArtifactTemplate
			if err := tx.Where("name = ? AND enabled = ?", artifactName, true).First(&template).Error; err != nil {
				return fmt.Errorf("礼包装备%s不存在: %w", artifactName, err)
			}
			artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: artifactTemplateSlot(template)}
			if err := tx.Create(&artifact).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"spirit_stones": gorm.Expr("spirit_stones + ?", reward.SpiritStones),
			"silver_coins":  gorm.Expr("silver_coins + ?", reward.SilverCoins),
			"immortal_jade": gorm.Expr("immortal_jade + ?", reward.ImmortalJade),
			"arena_coins":   gorm.Expr("arena_coins + ?", reward.ArenaCoins),
			"cultivation":   gorm.Expr("cultivation + ?", reward.Cultivation),
			"merit":         gorm.Expr("merit + ?", reward.Merit),
			"reputation":    gorm.Expr("reputation + ?", reward.Reputation),
			"dao_heart":     gorm.Expr("MIN(dao_heart + ?, 100)", reward.DaoHeart),
		}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	result := GameResult{Title: "礼包开启", Content: fmt.Sprintf("开启：%s\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n奖励已分别收入乾坤袋和角色道籍。", pack.Name, describeGiftRewards(reward)), Actions: []string{"背包", "状态", "礼包", "帮助"}}
	if strings.Contains(pack.Name, "月卡") || pack.RarityName == "仙品" || pack.RarityName == "神品" {
		kind := "天赐"
		verb := "开启"
		if strings.Contains(pack.Name, "月卡") {
			kind = "月卡"
			verb = "激活"
		}
		broadcast := fmt.Sprintf("【%s贺讯】恭喜道友%s%s了%s，获赐%s，仙途气运由此更盛！", kind, player.DaoName, verb, pack.Name, describeGiftRewards(reward))
		_ = g.publishWorldBroadcast(kind, player.DaoName+verb+pack.Name, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func describeGiftRewards(reward giftPackRewards) string {
	var lines []string
	itemNames := make([]string, 0, len(reward.Items))
	for name := range reward.Items {
		itemNames = append(itemNames, name)
	}
	sort.Strings(itemNames)
	for _, name := range itemNames {
		quantity := reward.Items[name]
		lines = append(lines, fmt.Sprintf("%s×%d", name, quantity))
	}
	for _, artifact := range reward.Artifacts {
		lines = append(lines, "装备·"+artifact)
	}
	if reward.SpiritStones > 0 {
		lines = append(lines, fmt.Sprintf("灵石×%d", reward.SpiritStones))
	}
	if reward.SilverCoins > 0 {
		lines = append(lines, fmt.Sprintf("银币×%d", reward.SilverCoins))
	}
	if reward.ImmortalJade > 0 {
		lines = append(lines, fmt.Sprintf("仙金×%d", reward.ImmortalJade))
	}
	if reward.ArenaCoins > 0 {
		lines = append(lines, fmt.Sprintf("竞技币×%d", reward.ArenaCoins))
	}
	if reward.Cultivation > 0 {
		lines = append(lines, fmt.Sprintf("修为×%d", reward.Cultivation))
	}
	if reward.Merit > 0 {
		lines = append(lines, fmt.Sprintf("功德×%d", reward.Merit))
	}
	if reward.Reputation > 0 {
		lines = append(lines, fmt.Sprintf("声望×%d", reward.Reputation))
	}
	if reward.DaoHeart > 0 {
		lines = append(lines, fmt.Sprintf("道心×%d", reward.DaoHeart))
	}
	if len(lines) == 0 {
		return "暂无奖励"
	}
	return strings.Join(lines, "、")
}
