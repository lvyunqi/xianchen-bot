package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

type localNPC struct {
	Location model.WorldLocation
	Name     string
	Index    int
}

type npcShopEntry struct {
	Item             model.Item
	Price            int64
	RequiredAffinity int64
}

func npcAffinityKey(npc localNPC) string {
	return fmt.Sprintf("npc.affinity.%d.%d", npc.Location.ID, npc.Index)
}

func npcRelationshipName(affinity int64) string {
	switch {
	case affinity >= 500:
		return "生死故交"
	case affinity >= 240:
		return "莫逆之交"
	case affinity >= 120:
		return "推心置腹"
	case affinity >= 60:
		return "相交甚笃"
	case affinity >= 30:
		return "颇为熟悉"
	case affinity >= 10:
		return "略有印象"
	default:
		return "初次相识"
	}
}

func (g *Game) resolveLocalNPC(player *model.Player, raw, lastKey string) (localNPC, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return localNPC{}, false, err
	}
	npcs := decodeTextList(location.NPCJSON)
	name := strings.TrimSpace(raw)
	if name == "" && lastKey != "" {
		if value, valueErr := g.playerValue(player.ID, lastKey); valueErr == nil {
			name = strings.TrimSpace(value)
		}
	}
	if name == "" {
		if value, valueErr := g.playerValue(player.ID, "npc.last_met"); valueErr == nil {
			name = strings.TrimSpace(value)
		}
	}
	if name == "" && len(npcs) == 1 {
		name = npcs[0]
	}
	for index, candidate := range npcs {
		if candidate == name {
			return localNPC{Location: location, Name: candidate, Index: index}, true, nil
		}
	}
	return localNPC{Location: location}, false, nil
}

func npcSelectionResult(location model.WorldLocation, title, command string, player *model.Player, game *Game) GameResult {
	npcs := decodeTextList(location.NPCJSON)
	if len(npcs) == 0 {
		return GameResult{Title: title, Content: fmt.Sprintf("%s当前没有可交互人物，请前往城镇、宗门驻地或带有NPC的地图。", location.Name), Actions: []string{"位置", "地图", "NPC图鉴"}}
	}
	lines := []string{fmt.Sprintf("当前位置：%s·%s", location.Region, location.Name), "请选择人物：", "━━━━━━━━━━━"}
	actions := make([]string, 0, len(npcs)+2)
	for index, name := range npcs {
		npc := localNPC{Location: location, Name: name, Index: index}
		affinity := game.playerValueInt(player.ID, npcAffinityKey(npc), 0)
		lines = append(lines, fmt.Sprintf("- %s · 好感%d（%s）", name, affinity, npcRelationshipName(affinity)))
		actions = append(actions, command+" "+name)
	}
	actions = append(actions, "位置", "NPC图鉴")
	return GameResult{Title: title, Content: strings.Join(lines, "\n"), Actions: actions}
}

func (g *Game) npcInventory(npc localNPC) ([]npcShopEntry, error) {
	var items []model.Item
	if err := g.store.DB.Where("tradable = ? AND base_value > ? AND category_name <> ?", true, 0, "礼包").Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(npc.Location.Code + "|" + npc.Name))
	start := int(hasher.Sum64() % uint64(len(items)))
	thresholds := []int64{0, 10, 30, 60, 120, 240, 360, 500}
	count := minInt(len(items), len(thresholds))
	entries := make([]npcShopEntry, 0, count)
	for index := 0; index < count; index++ {
		item := items[(start+index)%len(items)]
		price := max64(item.StorePrice, item.BaseValue)
		if price < 1 {
			price = 1
		}
		price = price * int64(100+index*5) / 100
		entries = append(entries, npcShopEntry{Item: item, Price: price, RequiredAffinity: thresholds[index]})
	}
	return entries, nil
}

func (g *Game) npcShop(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	page := 1
	if len(parts) > 0 {
		if parsed, err := strconv.Atoi(parts[len(parts)-1]); err == nil && parsed > 0 {
			page = parsed
			parts = parts[:len(parts)-1]
		}
	}
	npc, found, err := g.resolveLocalNPC(player, strings.Join(parts, " "), "npc.last_shop")
	if err != nil {
		return GameResult{}, true, err
	}
	if !found {
		return npcSelectionResult(npc.Location, "人物仙商", "NPC商店", player, g), true, nil
	}
	entries, err := g.npcInventory(npc)
	if err != nil {
		return GameResult{}, true, err
	}
	if len(entries) == 0 {
		return GameResult{Title: npc.Name + "的店铺", Content: "这位人物当前没有可售货物。", Actions: []string{"对话 " + npc.Name, "位置"}}, true, nil
	}
	_ = g.setPlayerValue(player.ID, "npc.last_shop", npc.Name, nil)
	affinity := g.playerValueInt(player.ID, npcAffinityKey(npc), 0)
	const pageSize = 4
	pages := maxInt((len(entries)+pageSize-1)/pageSize, 1)
	page = minInt(maxInt(page, 1), pages)
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(entries))
	lines := []string{fmt.Sprintf("人物：%s", npc.Name), fmt.Sprintf("好感度：%d（%s）", affinity, npcRelationshipName(affinity)), fmt.Sprintf("第%d/%d页 · 以灵石结算", page, pages), "━━━━━━━━━━━"}
	actions := make([]string, 0, pageSize+4)
	for _, entry := range entries[start:end] {
		state := "可购买"
		if affinity < entry.RequiredAffinity {
			state = fmt.Sprintf("需好感%d", entry.RequiredAffinity)
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  价格：%d灵石 · %s\n  %s", entry.Item.Name, displayOr(entry.Item.RarityName, "凡品"), entry.Price, state, entry.Item.Description))
		actions = append(actions, "NPC购买 "+entry.Item.Name+"*1")
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("NPC商店 %s %d", npc.Name, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("NPC商店 %s %d", npc.Name, page+1))
	}
	actions = append(actions, "NPC赠送 "+npc.Name, "NPC关系 "+npc.Name, "对话 "+npc.Name)
	return GameResult{Title: npc.Name + "的店铺", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) buyFromLocalNPC(player *model.Player, raw string) (GameResult, bool, error) {
	itemName, quantity, parseErr := parseFlexibleStackQuantity(raw)
	if parseErr != nil {
		return GameResult{Title: "人物仙商购买", Content: "请输入：`NPC购买 商品名*数量`。请先打开人物的NPC商店。", Actions: []string{"NPC商店", "位置"}}, true, nil
	}
	npc, found, err := g.resolveLocalNPC(player, "", "npc.last_shop")
	if err != nil {
		return GameResult{}, true, err
	}
	if !found {
		return GameResult{Title: "尚未选择人物商店", Content: "请先发送“NPC商店”并选择当前位置的人物。", Actions: []string{"NPC商店", "位置"}}, true, nil
	}
	entries, err := g.npcInventory(npc)
	if err != nil {
		return GameResult{}, true, err
	}
	var selected npcShopEntry
	for _, entry := range entries {
		if entry.Item.Name == itemName {
			selected = entry
			break
		}
	}
	if selected.Item.ID == 0 {
		return GameResult{Title: "商品未上架", Content: fmt.Sprintf("%s的店铺没有出售“%s”。", npc.Name, itemName), Actions: []string{"NPC商店 " + npc.Name}}, true, nil
	}
	affinity := g.playerValueInt(player.ID, npcAffinityKey(npc), 0)
	if affinity < selected.RequiredAffinity {
		return GameResult{Title: "好感度不足", Content: fmt.Sprintf("购买%s需要与%s达到%d好感，当前%d。本次没有扣除灵石。", selected.Item.Name, npc.Name, selected.RequiredAffinity, affinity), Actions: []string{"NPC赠送 " + npc.Name, "NPC关系 " + npc.Name, "NPC商店 " + npc.Name}}, true, nil
	}
	if selected.Price > 0 && quantity > math.MaxInt64/selected.Price {
		return GameResult{Title: "购买数量过大", Content: "本次数额超过安全范围，没有扣除灵石。", Actions: []string{"NPC商店 " + npc.Name}}, true, nil
	}
	total := selected.Price * quantity
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		debit := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, total).Update("spirit_stones", gorm.Expr("spirit_stones - ?", total))
		if debit.Error != nil {
			return debit.Error
		}
		if debit.RowsAffected != 1 {
			return errors.New("insufficient spirit stones")
		}
		var owned model.PlayerItem
		if err := tx.Where("player_id = ? AND item_id = ?", player.ID, selected.Item.ID).First(&owned).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&model.PlayerItem{PlayerID: player.ID, ItemID: selected.Item.ID, Quantity: quantity}).Error
		} else if err != nil {
			return err
		}
		if quantity > math.MaxInt64-owned.Quantity {
			return errors.New("inventory quantity overflow")
		}
		return tx.Model(&owned).Update("quantity", gorm.Expr("quantity + ?", quantity)).Error
	})
	if err != nil {
		if strings.Contains(err.Error(), "insufficient spirit stones") {
			return GameResult{Title: "灵石不足", Content: fmt.Sprintf("购买%s×%d需要%d灵石，当前余额不足。本次没有发放物品。", selected.Item.Name, quantity, total), Actions: []string{"货币", "NPC商店 " + npc.Name}}, true, nil
		}
		if strings.Contains(err.Error(), "inventory quantity overflow") {
			return GameResult{Title: "购买数量过大", Content: "背包数量将超过安全范围，本次事务已回滚。", Actions: []string{"背包", "NPC商店 " + npc.Name}}, true, nil
		}
		return GameResult{}, true, err
	}
	return GameResult{Title: "人物仙商成交", Content: fmt.Sprintf("人物：%s\n购得：%s×%d\n支付：灵石-%d\n好感要求：%d · 当前%d", npc.Name, selected.Item.Name, quantity, total, selected.RequiredAffinity, affinity), Actions: []string{"物品 " + selected.Item.Name, "背包", "NPC商店 " + npc.Name, "货币"}}, true, nil
}

func (g *Game) giftLocalNPC(player *model.Player, raw string) (GameResult, bool, error) {
	npc, itemRaw, found, err := g.parseNPCGiftTarget(player, raw)
	if err != nil {
		return GameResult{}, true, err
	}
	if !found {
		return npcSelectionResult(npc.Location, "人物赠礼", "NPC赠送", player, g), true, nil
	}
	itemName, quantity, parseErr := parseFlexibleStackQuantity(itemRaw)
	if parseErr != nil {
		return GameResult{Title: "人物赠礼", Content: fmt.Sprintf("请输入：`NPC赠送 %s 物品名*数量`。", npc.Name), Actions: []string{"背包", "NPC关系 " + npc.Name}}, true, nil
	}
	item, err := g.itemByName(itemName)
	if err != nil {
		return GameResult{Title: "赠礼失败", Content: "没有找到物品：“ + itemName + ”。", Actions: []string{"背包"}}, true, nil
	}
	var owned model.PlayerItem
	if err := g.store.DB.Where("player_id = ? AND item_id = ?", player.ID, item.ID).First(&owned).Error; err != nil || owned.Quantity < quantity || owned.Bound || !item.Tradable {
		return GameResult{Title: "赠礼失败", Content: "该物品数量不足、已经绑定或属于不可交易物品，不能赠予人物。本次没有扣除物品。", Actions: []string{"背包", "NPC关系 " + npc.Name}}, true, nil
	}
	unitGain := min64(max64(item.BaseValue/100, 1), 10)
	gain := int64(1000)
	if quantity <= 1000/unitGain {
		gain = quantity * unitGain
	}
	before := g.playerValueInt(player.ID, npcAffinityKey(npc), 0)
	after := before + gain
	if before > math.MaxInt64-gain {
		after = math.MaxInt64
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeNamedItemTx(tx, player.ID, item.Name, quantity); err != nil {
			return err
		}
		value := model.PlayerValue{PlayerID: player.ID, Key: npcAffinityKey(npc), Value: strconv.FormatInt(after, 10)}
		return tx.Where("player_id = ? AND key = ?", player.ID, value.Key).Assign(map[string]any{"value": value.Value, "expires_at": nil}).FirstOrCreate(&value).Error
	})
	if err != nil {
		return GameResult{Title: "赠礼未完成", Content: "物品库存在结算时发生变化，事务已回滚。", Actions: []string{"背包", "NPC关系 " + npc.Name}}, true, nil
	}
	_ = g.setPlayerValue(player.ID, "npc.last_met", npc.Name, nil)
	return GameResult{Title: "人物赠礼", Content: fmt.Sprintf("赠予%s：%s×%d\n好感：+%d（%d → %d）\n关系：%s", npc.Name, item.Name, quantity, gain, before, after, npcRelationshipName(after)), Actions: []string{"NPC关系 " + npc.Name, "NPC商店 " + npc.Name, "对话 " + npc.Name}}, true, nil
}

func (g *Game) parseNPCGiftTarget(player *model.Player, raw string) (localNPC, string, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return localNPC{}, "", false, err
	}
	raw = strings.TrimSpace(raw)
	for index, name := range decodeTextList(location.NPCJSON) {
		if raw == name || strings.HasPrefix(raw, name+" ") {
			return localNPC{Location: location, Name: name, Index: index}, strings.TrimSpace(strings.TrimPrefix(raw, name)), true, nil
		}
	}
	npc, found, err := g.resolveLocalNPC(player, "", "npc.last_met")
	return npc, raw, found, err
}

func (g *Game) localNPCRelationship(player *model.Player, raw string) (GameResult, bool, error) {
	npc, found, err := g.resolveLocalNPC(player, raw, "npc.last_met")
	if err != nil {
		return GameResult{}, true, err
	}
	if !found {
		return npcSelectionResult(npc.Location, "人物关系", "NPC关系", player, g), true, nil
	}
	affinity := g.playerValueInt(player.ID, npcAffinityKey(npc), 0)
	next := int64(10)
	for _, threshold := range []int64{10, 30, 60, 120, 240, 500} {
		if affinity < threshold {
			next = threshold
			break
		}
		next = 500
	}
	progress := "关系已经达到最高记载"
	if affinity < 500 {
		progress = fmt.Sprintf("距离下一关系还需%d好感", next-affinity)
	}
	return GameResult{Title: "人物关系", Content: fmt.Sprintf("人物：%s\n所在：%s·%s\n好感度：%d\n关系：%s\n%s\n━━━━━━━━━━━\n对话用于了解当地委托；赠送本人持有且未绑定的物品可增加好感，高好感会解锁人物商店中的后续货物。", npc.Name, npc.Location.Region, npc.Location.Name, affinity, npcRelationshipName(affinity), progress), Actions: []string{"NPC赠送 " + npc.Name, "NPC商店 " + npc.Name, "对话 " + npc.Name, "位置"}}, true, nil
}
