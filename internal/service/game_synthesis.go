package service

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) synthesisMenu(player *model.Player) GameResult {
	var total int64
	_ = g.store.DB.Model(&model.SynthesisRecipe{}).Where("enabled = ?", true).Count(&total).Error
	lines := []string{
		"将采集、妖兽、副本和灵田材料炼成突破丹药、渡劫法器与高级素材。",
		"━━━━━━━━━━━",
		fmt.Sprintf("当前启用配方：%d种", total),
		"查看配方：合成图鉴",
		"单次合成：合成 淬脉丹",
		"批量合成：合成 破境丹*10",
		"查看记录：合成记录",
		"━━━━━━━━━━━",
		"批量数量不设游戏上限，但必须拥有完整材料；每份配方独立判定成功率，失败也会消耗该份材料。",
		"突破链：淬脉丹 → 凝元丹 → 破境丹 → 引劫玉符。",
	}
	return GameResult{Title: "🔧 万法合成", Content: strings.Join(lines, "\n"), Actions: []string{"合成列表", "配方 淬脉丹", "合成 淬脉丹", "合成 凝元丹", "合成 破境丹", "合成 引劫玉符", "合成记录", "背包"}}
}

func (g *Game) synthesisCatalog(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.SynthesisRecipe{}).Where("enabled = ?", true)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.SynthesisRecipe
	if err := query.Order("sort_order,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"🔧 〓 仙 家 合 成 谱 〓 🔧", fmt.Sprintf("第%d/%d页 · 共%d种配方", page, pages, total), "━━━━━━━━━━━"}
	actions := []string{"合成菜单"}
	for _, row := range rows {
		requirement, unmet, _ := g.prerequisiteStatus(player, row.PrerequisiteJSON)
		state := "已解锁"
		if len(unmet) > 0 {
			state = "未解锁"
		}
		actualRate, luckBonus := probabilityWithLuck(row.SuccessRate, player.Luck, luckSynthesisBonusCap)
		lines = append(lines, fmt.Sprintf("🔸【%s】· %s · %s\n📥 需要：%s\n🎁 获得：%s×%d\n📜 前置：%s\n✨ 成功率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%", row.Name, row.Category, state, displayConfigText(row.MaterialsJSON), row.OutputName, max64(row.OutputQuantity, 1), requirement, row.SuccessRate*100, luckBonus*100, actualRate*100), "━━━━━━━━━━━")
		actions = append(actions, "合成 "+row.Name, "配方 "+row.Name, "物品 "+row.OutputName)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("合成图鉴 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("合成图鉴 %d", page+1))
	}
	return GameResult{Title: "🔧 合成配方", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) synthesisRecipeDetails(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return g.synthesisCatalog(player, "")
	}
	var recipe model.SynthesisRecipe
	if err := g.store.DB.Where("enabled = ? AND (name = ? OR output_name = ? OR code = ?)", true, name, name, name).Order("sort_order,id").First(&recipe).Error; err != nil {
		return GameResult{Title: "🔧 配方不存在", Content: "仙家合成谱中没有收录“" + name + "”。", Actions: []string{"合成列表", "合成菜单"}}, true, nil
	}
	requirement, unmet, _ := g.prerequisiteStatus(player, recipe.PrerequisiteJSON)
	state := "当前已解锁"
	if len(unmet) > 0 {
		state = "尚未解锁：\n- " + strings.Join(unmet, "\n- ")
	}
	sources, sourceActions := g.craftingMaterialGuide(recipe.MaterialsJSON)
	actualRate, luckBonus := probabilityWithLuck(recipe.SuccessRate, player.Luck, luckSynthesisBonusCap)
	content := fmt.Sprintf("配方：%s\n类别：%s\n━━━━━━━━━━━\n📥 材料：%s\n🎁 产物：%s×%d\n✨ 成功率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n📜 前置：%s\n当前判定：%s\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n【材料来源】\n%s", recipe.Name, recipe.Category, displayConfigText(recipe.MaterialsJSON), recipe.OutputName, max64(recipe.OutputQuantity, 1), recipe.SuccessRate*100, luckBonus*100, actualRate*100, requirement, state, recipe.Description, sources)
	actions := []string{"合成 " + recipe.Name, "物品 " + recipe.OutputName, "合成列表", "背包"}
	actions = append(actions, sourceActions...)
	return GameResult{Title: "🔧 配方详情", Content: content, Actions: actions}, true, nil
}

func (g *Game) synthesizeItem(player *model.Player, raw string) (GameResult, bool, error) {
	name, quantity, err := parseStackQuantity(raw)
	if err != nil || strings.TrimSpace(name) == "" {
		return GameResult{Title: "材料合成", Content: "请输入：`合成 配方名` 或 `合成 配方名*数量`。例如：`合成 破境丹*10`。", Actions: []string{"合成图鉴", "合成菜单"}}, true, nil
	}
	var recipe model.SynthesisRecipe
	if err := g.store.DB.Where("enabled = ? AND (name = ? OR output_name = ? OR code = ?)", true, name, name, name).First(&recipe).Error; err != nil {
		return GameResult{Title: "配方不存在", Content: "没有找到“" + name + "”的合成配方，请从合成图鉴蓝字选择。", Actions: []string{"合成图鉴", "合成菜单"}}, true, nil
	}
	requirement, unmet, requirementErr := g.prerequisiteStatus(player, recipe.PrerequisiteJSON)
	if requirementErr != nil {
		return GameResult{Title: "配方道纹紊乱", Content: "前置条件无法解析，本次没有扣除材料，请主人检查配方配置。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: "合成尚未解锁", Content: fmt.Sprintf("配方：%s\n前置：%s\n━━━━━━━━━━━\n未满足：\n- %s", recipe.Name, requirement, strings.Join(unmet, "\n- ")), Actions: append(g.prerequisiteActions(unmet), "合成图鉴")}, true, nil
	}
	materials := make(map[string]int64)
	if json.Unmarshal([]byte(recipe.MaterialsJSON), &materials) != nil || len(materials) == 0 {
		return GameResult{Title: "配方配置错误", Content: "材料配置为空或格式不正确。"}, true, nil
	}
	scaled := make(map[string]int64, len(materials))
	for material, amount := range materials {
		if amount <= 0 || quantity > math.MaxInt64/amount {
			return GameResult{Title: "合成数量过大", Content: "材料总数超过安全计算范围，请减少本次数量。"}, true, nil
		}
		scaled[material] = amount * quantity
	}
	scaledJSON, _ := json.Marshal(scaled)
	costText, missing, costErr := g.extendedCostStatus(player, string(scaledJSON))
	if costErr != nil {
		return GameResult{}, true, costErr
	}
	if len(missing) > 0 {
		sources, sourceActions := g.craftingMaterialGuide(recipe.MaterialsJSON)
		actions := []string{"配方 " + recipe.Name, "背包", "地图", "副本", "合成列表"}
		actions = append(actions, sourceActions...)
		return GameResult{Title: "合成材料不足", Content: fmt.Sprintf("配方：%s×%d\n需要：%s\n━━━━━━━━━━━\n缺少：\n- %s\n━━━━━━━━━━━\n【获取指引】\n%s", recipe.Name, quantity, costText, strings.Join(missing, "\n- "), sources), Actions: actions}, true, nil
	}
	actualRate, luckBonus := probabilityWithLuck(recipe.SuccessRate, player.Luck, luckSynthesisBonusCap)
	successes := synthesisSuccessCount(quantity, actualRate)
	outputPerSuccess := max64(recipe.OutputQuantity, 1)
	if successes > 0 && outputPerSuccess > math.MaxInt64/successes {
		return GameResult{Title: "产物数量过大", Content: "合成产物超过安全计算范围，请联系主人检查配方。"}, true, nil
	}
	outputQuantity := successes * outputPerSuccess
	var output model.Item
	if outputQuantity > 0 {
		if err := g.store.DB.Where("id = ? OR name = ?", recipe.OutputItemID, recipe.OutputName).Order("id").First(&output).Error; err != nil {
			return GameResult{Title: "产物道纹缺失", Content: "配方没有关联有效产物，本次没有扣除任何材料，请主人检查产物关联。"}, true, nil
		}
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeExtendedCostTx(tx, player.ID, scaled); err != nil {
			return err
		}
		if outputQuantity > 0 {
			return storage.NewPlayerRepository(tx).AdjustItem(player.ID, output.ID, outputQuantity)
		}
		return nil
	}); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.synthesis_attempts", quantity)
	_, _ = g.addPlayerValueInt(player.ID, "stats.synthesis_successes", successes)
	failed := quantity - successes
	return GameResult{Title: "合成结算", Content: fmt.Sprintf("配方：%s\n尝试：%d份\n成功率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n成功：%d份 · 失败：%d份\n消耗：%s\n获得：%s×%d\n━━━━━━━━━━━\n每份配方独立判定，失败份数的材料同样已经消耗。", recipe.Name, quantity, recipe.SuccessRate*100, luckBonus*100, actualRate*100, successes, failed, costText, recipe.OutputName, outputQuantity), Actions: []string{"物品 " + recipe.OutputName, "背包", "合成 " + recipe.Name, "合成图鉴", "合成记录", "仙缘"}}, true, nil
}

func (g *Game) synthesisRecord(player *model.Player) GameResult {
	attempts := g.playerValueInt(player.ID, "stats.synthesis_attempts", 0)
	successes := g.playerValueInt(player.ID, "stats.synthesis_successes", 0)
	rate := float64(0)
	if attempts > 0 {
		rate = float64(successes) * 100 / float64(attempts)
	}
	var owned []string
	for _, name := range []string{"淬脉丹", "凝元丹", "破境丹", "引劫玉符"} {
		item, err := g.itemByName(name)
		if err == nil {
			owned = append(owned, fmt.Sprintf("%s×%d", name, g.itemQuantity(player.ID, item.ID)))
		}
	}
	sort.Strings(owned)
	return GameResult{Title: "合成记录", Content: fmt.Sprintf("累计尝试：%d份\n成功产出：%d份\n历史成功率：%.2f%%\n━━━━━━━━━━━\n突破前置库存：%s", attempts, successes, rate, strings.Join(owned, "、")), Actions: []string{"合成菜单", "合成图鉴", "背包"}}
}

func synthesisSuccessCount(quantity int64, rate float64) int64 {
	if rate <= 0 || quantity <= 0 {
		return 0
	}
	if rate >= 1 {
		return quantity
	}
	if quantity <= 10000 {
		var successes int64
		for index := int64(0); index < quantity; index++ {
			if rand.Float64() <= rate {
				successes++
			}
		}
		return successes
	}
	mean := float64(quantity) * rate
	deviation := math.Sqrt(float64(quantity) * rate * (1 - rate))
	value := int64(math.Round(mean + rand.NormFloat64()*deviation))
	return min64(max64(value, 0), quantity)
}
