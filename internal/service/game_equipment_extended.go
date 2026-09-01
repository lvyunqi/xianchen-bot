package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeEquipmentExtended(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 1150:
		return g.activateEquipmentCollection(player, command.RawArguments)
	case 1151:
		return g.equipmentPavilion(player, command.RawArguments)
	case 1152:
		return g.buyEquipmentFromPavilion(player, command.RawArguments)
	case 1153:
		return g.transferArtifact(player, command.RawArguments)
	case 1154:
		return g.equipmentSetCatalog(player, command.RawArguments)
	case 1155:
		return g.equipmentSetDetails(player, command.RawArguments)
	case 1156:
		return g.currentEquipmentSets(player)
	case 1157:
		return g.decomposeArtifact(player, command.RawArguments)
	case 1158:
		return g.decomposeArtifactsByQuality(player, command.RawArguments)
	case 1159:
		return g.openArtifactSocket(player, command.RawArguments)
	case 1160:
		return g.socketArtifactGem(player, command.RawArguments)
	case 1161:
		return g.removeArtifactGem(player, command.RawArguments, false)
	case 1162:
		return g.removeArtifactGem(player, command.RawArguments, true)
	case 1163:
		return g.gemCatalog(command.RawArguments)
	case 1164:
		return g.starRefineArtifact(player, command.RawArguments)
	case 1165:
		return g.moveArtifactSmelter(player, command.RawArguments, true)
	case 1166:
		return g.moveArtifactSmelter(player, command.RawArguments, false)
	case 1167:
		return g.fuseArtifacts(player, command.RawArguments)
	case 1168:
		return g.equipmentGuide(player), true, nil
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) activateEquipmentCollection(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	query := g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND activated = ?", player.ID, false)
	if name != "" {
		query = query.Where("name = ?", name)
	}
	var rows []model.PlayerArtifact
	if err := query.Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "🛡️ 器物道藏无需激活", Content: "没有待激活的持有装备。图鉴激活只记录收集成就，不会重复叠加装备属性。", Actions: []string{"装备背包", "装备图鉴", "套装大全"}}, true, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if err := g.store.DB.Model(&model.PlayerArtifact{}).Where("id IN ?", ids).Update("activated", true).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🛡️ 器物道藏激活", Content: fmt.Sprintf("本次激活%d件器物图鉴。\n激活用于套装收藏、成就和图鉴完成度；实际战斗属性只由当前穿戴装备、灵孔、锻造、星阶和套装共鸣提供。", len(rows)), Actions: []string{"装备图鉴", "当前装备", "套装大全", "成就"}}, true, nil
}

func equipmentPavilionPrice(row model.ArtifactTemplate) int64 {
	return max64(180+int64(row.MinimumRealmSequence)*120+row.MinimumCombatPower/4+int64(row.MaxLevel)*8, 100)
}

func (g *Game) equipmentPavilion(player *model.Player, raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterPage(raw)
	const pageSize = 6
	query := g.store.DB.Where("enabled = ?", true)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("slot = ? OR archetype = ? OR name LIKE ?", filter, filter, like)
	}
	var total int64
	_ = query.Model(&model.ArtifactTemplate{}).Count(&total).Error
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	var rows []model.ArtifactTemplate
	if err := query.Order("minimum_realm_sequence,slot,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("银币：%d · 第%d/%d页 · 共%d件常备器胚", player.SilverCoins, page, pages, total), "器阁出售凡品器胚；高品质仍需亲自炼器、锻造、首领掉落或副本获取。", "━━━━━━━━━━━"}
	actions := []string{"装备系统", "银币商城"}
	for _, row := range rows {
		price := equipmentPavilionPrice(row)
		lines = append(lines, fmt.Sprintf("%s【%s · %s】\n%d银币 · 前置%s\n%s", row.Name, artifactTemplateSlot(row), artifactTemplateArchetype(row), price, g.artifactRequirementText(row), artifactTemplateStatsText(row.AttributeJSON)), "━━━━━━━")
		actions = append(actions, "兑购 "+row.Name, "装备详情 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, catalogPageAction("仙盟器阁", filter, page-1))
	}
	if page < pages {
		actions = append(actions, catalogPageAction("仙盟器阁", filter, page+1))
	}
	return GameResult{Title: "🏯 仙盟百炼器阁", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) buyEquipmentFromPavilion(player *model.Player, raw string) (GameResult, bool, error) {
	name, quantity, err := parseStackQuantity(raw)
	if err != nil || name == "" {
		return GameResult{Title: "🏯 器阁兑购", Content: "请输入：`兑购 装备名` 或 `兑购 装备名*数量`。", Actions: []string{"仙盟器阁"}}, true, nil
	}
	var template model.ArtifactTemplate
	if g.store.DB.Where("enabled = ? AND name = ?", true, name).First(&template).Error != nil {
		return GameResult{Title: "🏯 器胚未上架", Content: "仙盟器阁没有“" + name + "”。", Actions: []string{"仙盟器阁", "装备图鉴"}}, true, nil
	}
	if unmet := g.artifactRequirementStatus(player, template); len(unmet) > 0 {
		return GameResult{Title: "🏯 兑购前置未满", Content: "- " + strings.Join(unmet, "\n- "), Actions: []string{"装备详情 " + name, "修炼", "突破"}}, true, nil
	}
	price := equipmentPavilionPrice(template)
	if quantity > 0 && price > 0 && quantity > int64(^uint64(0)>>1)/price {
		return GameResult{Title: "🏯 数量过大", Content: "总价超过安全记账范围，请拆分兑购。"}, true, nil
	}
	total := price * quantity
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND silver_coins >= ?", player.ID, total).Update("silver_coins", gorm.Expr("silver_coins - ?", total))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		for i := int64(0); i < quantity; i++ {
			row := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: artifactTemplateSlot(template)}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == errInsufficientCurrency {
		return GameResult{Title: "🏯 银币不足", Content: fmt.Sprintf("兑购%s×%d需要%d银币，当前%d。", name, quantity, total, player.SilverCoins), Actions: []string{"钱庄", "签到", "仙盟器阁"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏯 器胚兑购完成", Content: fmt.Sprintf("获得：%s×%d【凡品】\n槽位：%s · 器型：%s\n支付：%d银币", name, quantity, artifactTemplateSlot(template), artifactTemplateArchetype(template), total), Actions: []string{"装备背包", "穿戴 " + name, "装备激活 " + name, "仙盟器阁"}}, true, nil
}

func (g *Game) transferArtifact(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) < 2 {
		return GameResult{Title: "🛡️ 法器传承", Content: "请输入：`法器传承 对方道号 装备名`。已穿戴或玄火炉中的装备不能传承。", Actions: []string{"装备背包", "装备帮助"}}, true, nil
	}
	target, err := g.findPlayer(parts[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🛡️ 传承对象无效", Content: "请填写另一名现存道友的全服唯一道号。", Actions: []string{"好友", "装备背包"}}, true, nil
	}
	name := strings.Join(parts[1:], " ")
	var artifact model.PlayerArtifact
	if g.store.DB.Where("player_id = ? AND name = ?", player.ID, name).Order("level DESC,id DESC").First(&artifact).Error != nil {
		return GameResult{Title: "🛡️ 法器不存在", Content: "装备背包中没有“" + name + "”。", Actions: []string{"装备背包"}}, true, nil
	}
	if artifact.Equipped || artifact.InSmelter {
		return GameResult{Title: "🛡️ 法器不可传承", Content: "请先卸下并从玄火炉取出该装备。", Actions: []string{"卸下 " + name, "熔炉取出 " + name, "装备背包"}}, true, nil
	}
	if err := g.store.DB.Model(&artifact).Updates(map[string]any{"player_id": target.ID, "activated": false}).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.ID, "法器传承", fmt.Sprintf("%s向你传承了%s【%s +%d】；装备已进入你的装备背包。", player.DaoName, artifact.Name, artifact.Quality, artifact.Level))
	return GameResult{Title: "🛡️ 法器传承完成", Content: fmt.Sprintf("受赠道友：%s\n法器：%s【%s +%d】\n品质、锻造、星阶、灵纹与嵌灵宝石均完整保留。", target.DaoName, artifact.Name, artifact.Quality, artifact.Level), Actions: []string{"装备背包", "通知"}}, true, nil
}

func (g *Game) equipmentSetCatalog(player *model.Player, raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 6
	type setRow struct {
		Name  string
		Count int64
	}
	var rows []setRow
	if err := g.store.DB.Model(&model.ArtifactTemplate{}).Select("set_name AS name, COUNT(*) AS count").Where("enabled = ? AND set_name <> ''", true).Group("set_name").Order("MIN(id)").Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start, end := minInt((page-1)*pageSize, len(rows)), minInt(page*pageSize, len(rows))
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d套", page, pages, len(rows)), "套装共鸣按当前穿戴件数计算，图鉴激活不等于穿戴。", "━━━━━━━━━━━"}
	actions := []string{"当前套装", "装备图鉴"}
	for _, row := range rows[start:end] {
		lines = append(lines, fmt.Sprintf("%s · 收录%d件\n二件、四件、六件逐层唤醒套装道韵。", row.Name, row.Count), "━━━━━━━")
		actions = append(actions, "套装查询 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("套装大全 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("套装大全 %d", page+1))
	}
	return GameResult{Title: "🛡️ 万象套装道藏", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) equipmentSetDetails(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "🛡️ 套装查询", Content: "请输入：`套装查询 套装名`。", Actions: []string{"套装大全"}}, true, nil
	}
	var rows []model.ArtifactTemplate
	if g.store.DB.Where("enabled = ? AND set_name = ?", true, name).Order("slot,id").Find(&rows).Error != nil || len(rows) == 0 {
		return GameResult{Title: "🛡️ 套装未收录", Content: "没有找到“" + name + "”。", Actions: []string{"套装大全"}}, true, nil
	}
	equipped := int64(0)
	_ = g.store.DB.Table("player_artifacts").Joins("JOIN artifact_templates ON artifact_templates.id = player_artifacts.template_id").Where("player_artifacts.player_id = ? AND player_artifacts.equipped = ? AND artifact_templates.set_name = ?", player.ID, true, name).Count(&equipped).Error
	lines := []string{fmt.Sprintf("套装：%s · 当前穿戴%d/%d", name, equipped, len(rows)), "共鸣：" + displayConfigText(rows[0].SetBonusJSON), "━━━━━━━━━━━"}
	actions := []string{"当前套装", "装备背包"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s【%s · %s】", row.Name, artifactTemplateSlot(row), artifactTemplateArchetype(row)))
		actions = append(actions, "装备详情 "+row.Name)
	}
	return GameResult{Title: "🛡️ 套装详情", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) currentEquipmentSets(player *model.Player) (GameResult, bool, error) {
	type row struct {
		SetName string
		Count   int64
	}
	var rows []row
	err := g.store.DB.Table("player_artifacts").Select("artifact_templates.set_name, COUNT(*) AS count").Joins("JOIN artifact_templates ON artifact_templates.id = player_artifacts.template_id").Where("player_artifacts.player_id = ? AND player_artifacts.equipped = ? AND artifact_templates.set_name <> ''", player.ID, true).Group("artifact_templates.set_name").Order("count DESC").Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "🛡️ 当前套装", Content: "当前没有穿戴任何套装器物。散件属性仍正常生效。", Actions: []string{"装备背包", "套装大全", "仙盟器阁"}}, true, nil
	}
	lines := []string{"当前套装共鸣", "━━━━━━━━━━━"}
	actions := []string{"当前装备", "套装大全"}
	for _, row := range rows {
		stage := "未成共鸣"
		if row.Count >= 6 {
			stage = "六件道韵"
		} else if row.Count >= 4 {
			stage = "四件共鸣"
		} else if row.Count >= 2 {
			stage = "二件共鸣"
		}
		lines = append(lines, fmt.Sprintf("%s · %d件 · %s", row.SetName, row.Count, stage))
		actions = append(actions, "套装查询 "+row.SetName)
	}
	return GameResult{Title: "🛡️ 当前套装", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func artifactRecycleYield(row model.PlayerArtifact) int64 {
	quality := map[string]int64{"凡品": 1, "灵品": 3, "仙品": 8, "神品": 20}[row.Quality]
	if quality == 0 {
		quality = 1
	}
	return quality + int64(maxInt(row.Level-1, 0))/3 + int64(row.ForgeLevel)*2 + int64(row.StarLevel)*3
}

func (g *Game) decomposeArtifact(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	var row model.PlayerArtifact
	if name == "" || g.store.DB.Where("player_id = ? AND name = ?", player.ID, name).Order("level,id").First(&row).Error != nil {
		return GameResult{Title: "🔥 法器分解", Content: "请输入：`装备分解 装备名`。", Actions: []string{"装备背包", "装备帮助"}}, true, nil
	}
	if row.Equipped || row.InSmelter {
		return GameResult{Title: "🔥 法器受保护", Content: "已穿戴或玄火炉中的装备不能分解。", Actions: []string{"卸下 " + name, "熔炉取出 " + name}}, true, nil
	}
	yield := artifactRecycleYield(row)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var iron model.Item
		if err := tx.Where("name = ?", "玄铁").First(&iron).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, iron.ID, yield); err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🔥 法器归炉", Content: fmt.Sprintf("分解：%s【%s +%d】\n返还：玄铁×%d\n该器物的强化、锻造、星阶与灵纹已随器胚消散。", row.Name, row.Quality, row.Level, yield), Actions: []string{"装备背包", "物品 玄铁", "仙盟器阁"}}, true, nil
}

func (g *Game) decomposeArtifactsByQuality(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) != 2 || parts[1] != "确认" || (parts[0] != "凡品" && parts[0] != "灵品") {
		return GameResult{Title: "🔥 批量分解保护", Content: "格式：`装备一键分解 凡品 确认` 或 `装备一键分解 灵品 确认`。\n只处理未穿戴、未入炉的指定品质装备；仙品与神品禁止批量分解。", Actions: []string{"装备背包", "装备帮助"}}, true, nil
	}
	var rows []model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ? AND quality = ? AND equipped = ? AND in_smelter = ?", player.ID, parts[0], false, false).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "🔥 无可分解器物", Content: "没有符合保护条件的“" + parts[0] + "”装备。", Actions: []string{"装备背包"}}, true, nil
	}
	total := int64(0)
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		total += artifactRecycleYield(row)
		ids = append(ids, row.ID)
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var iron model.Item
		if err := tx.Where("name = ?", "玄铁").First(&iron).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, iron.ID, total); err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.PlayerArtifact{}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🔥 批量归炉完成", Content: fmt.Sprintf("分解：%s装备%d件\n返还：玄铁×%d\n仙品、神品、已穿戴与入炉装备均未处理。", parts[0], len(rows), total), Actions: []string{"装备背包", "物品 玄铁"}}, true, nil
}

func artifactSocketLimit(quality string) int {
	return map[string]int{"凡品": 1, "灵品": 2, "仙品": 3, "神品": 5}[quality]
}

func (g *Game) openArtifactSocket(player *model.Player, raw string) (GameResult, bool, error) {
	row, err := g.ownedArtifact(player.ID, strings.TrimSpace(raw))
	if err != nil {
		return GameResult{Title: "🧿 开辟灵孔", Content: "请输入：`装备开孔 装备名`。", Actions: []string{"装备背包"}}, true, nil
	}
	limit := artifactSocketLimit(row.Quality)
	if row.SocketCount >= limit {
		return GameResult{Title: "🧿 灵孔已满", Content: fmt.Sprintf("%s为%s，最多开辟%d孔。提升品质后可继续开孔。", row.Name, row.Quality, limit), Actions: []string{"锻造 " + row.Name, "装备详情 " + row.Name}}, true, nil
	}
	cost := int64(row.SocketCount + 1)
	if err := g.adjustNamedItem(player.ID, "阵基石", -cost); err != nil {
		return GameResult{Title: "🧿 开孔材料不足", Content: fmt.Sprintf("第%d孔需要阵基石×%d。", row.SocketCount+1, cost), Actions: []string{"物品 阵基石", "合成图鉴", "地图"}}, true, nil
	}
	if err := g.store.DB.Model(&row).Update("socket_count", row.SocketCount+1).Error; err != nil {
		_ = g.adjustNamedItem(player.ID, "阵基石", cost)
		return GameResult{}, true, err
	}
	return GameResult{Title: "🧿 灵孔开辟", Content: fmt.Sprintf("装备：%s\n灵孔：%d → %d/%d\n消耗：阵基石×%d", row.Name, row.SocketCount, row.SocketCount+1, limit, cost), Actions: []string{"宝石查询", "装备镶嵌 " + row.Name, "装备详情 " + row.Name}}, true, nil
}

func (g *Game) ownedArtifact(playerID uint, name string) (model.PlayerArtifact, error) {
	var row model.PlayerArtifact
	err := g.store.DB.Where("player_id = ? AND name = ?", playerID, name).Order("level DESC,id DESC").First(&row).Error
	return row, err
}

func (g *Game) parseArtifactAndGem(playerID uint, raw string) (model.PlayerArtifact, string, error) {
	var rows []model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ?", playerID).Order("LENGTH(name) DESC,id DESC").Find(&rows).Error; err != nil {
		return model.PlayerArtifact{}, "", err
	}
	raw = strings.TrimSpace(raw)
	for _, row := range rows {
		prefix := row.Name + " "
		if strings.HasPrefix(raw, prefix) {
			return row, strings.TrimSpace(strings.TrimPrefix(raw, prefix)), nil
		}
	}
	return model.PlayerArtifact{}, "", gorm.ErrRecordNotFound
}

func (g *Game) socketArtifactGem(player *model.Player, raw string) (GameResult, bool, error) {
	row, gemName, err := g.parseArtifactAndGem(player.ID, raw)
	if err != nil || gemName == "" {
		return GameResult{Title: "🧿 嵌灵", Content: "请输入：`装备镶嵌 装备名 宝石名`。", Actions: []string{"装备背包", "宝石查询"}}, true, nil
	}
	var gems []string
	_ = json.Unmarshal([]byte(row.SocketJSON), &gems)
	if len(gems) >= row.SocketCount {
		return GameResult{Title: "🧿 没有空灵孔", Content: "请先开辟新灵孔或取下已有宝石。", Actions: []string{"装备开孔 " + row.Name, "装备摘孔 " + row.Name}}, true, nil
	}
	var gem model.Item
	if g.store.DB.Where("name = ? AND effect_func = ?", gemName, "equipment_gem").First(&gem).Error != nil {
		return GameResult{Title: "🧿 宝石未收录", Content: "嵌灵图鉴中没有“" + gemName + "”。", Actions: []string{"宝石查询"}}, true, nil
	}
	before := g.equipmentStats(row)
	if err := g.adjustNamedItem(player.ID, gemName, -1); err != nil {
		return GameResult{Title: "🧿 未持有宝石", Content: "乾坤袋中没有“" + gemName + "”。", Actions: []string{"物品 " + gemName, "宝石查询", "商城"}}, true, nil
	}
	gems = append(gems, gemName)
	encoded, _ := json.Marshal(gems)
	updated := row
	updated.SocketJSON = string(encoded)
	after := g.equipmentStats(updated)
	if err := g.store.DB.Model(&row).Update("socket_json", string(encoded)).Error; err != nil {
		_ = g.adjustNamedItem(player.ID, gemName, 1)
		return GameResult{}, true, err
	}
	if row.Equipped {
		_ = g.applyEquipmentStatDifference(player.ID, before, after)
	}
	return GameResult{Title: "🧿 嵌灵完成", Content: fmt.Sprintf("装备：%s\n嵌入：%s\n灵孔：%d/%d\n宝石属性：%s", row.Name, gemName, len(gems), row.SocketCount, artifactTemplateStatsText(gem.EffectParams)), Actions: []string{"装备详情 " + row.Name, "装备摘孔 " + row.Name + " " + gemName, "当前装备"}}, true, nil
}

func (g *Game) removeArtifactGem(player *model.Player, raw string, all bool) (GameResult, bool, error) {
	var row model.PlayerArtifact
	gemName := ""
	var err error
	if all {
		row, err = g.ownedArtifact(player.ID, strings.TrimSpace(raw))
	} else {
		row, gemName, err = g.parseArtifactAndGem(player.ID, raw)
		if err != nil {
			row, err = g.ownedArtifact(player.ID, strings.TrimSpace(raw))
		}
	}
	if err != nil {
		return GameResult{Title: "🧿 取灵", Content: "请输入：`装备摘孔 装备名 [宝石名]` 或 `一键摘孔 装备名`。", Actions: []string{"装备背包"}}, true, nil
	}
	var gems []string
	_ = json.Unmarshal([]byte(row.SocketJSON), &gems)
	if len(gems) == 0 {
		return GameResult{Title: "🧿 灵孔为空", Content: row.Name + "没有已嵌入宝石。", Actions: []string{"宝石查询", "装备镶嵌 " + row.Name}}, true, nil
	}
	index := len(gems) - 1
	if gemName != "" {
		index = -1
		for i, name := range gems {
			if name == gemName {
				index = i
				break
			}
		}
		if index < 0 {
			return GameResult{Title: "🧿 未嵌此石", Content: row.Name + "没有嵌入“" + gemName + "”。", Actions: []string{"装备详情 " + row.Name}}, true, nil
		}
	}
	removed := []string{gems[index]}
	remaining := append(append([]string{}, gems[:index]...), gems[index+1:]...)
	if all {
		removed, remaining = gems, nil
	}
	before := g.equipmentStats(row)
	encoded, _ := json.Marshal(remaining)
	updated := row
	updated.SocketJSON = string(encoded)
	after := g.equipmentStats(updated)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&row).Update("socket_json", string(encoded)).Error; err != nil {
			return err
		}
		repo := storage.NewPlayerRepository(tx)
		for _, name := range removed {
			var item model.Item
			if err := tx.Where("name = ?", name).First(&item).Error; err != nil {
				return err
			}
			if err := repo.AdjustItem(player.ID, item.ID, 1); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if row.Equipped {
		_ = g.applyEquipmentStatDifference(player.ID, before, after)
	}
	return GameResult{Title: "🧿 取灵完成", Content: fmt.Sprintf("装备：%s\n取回：%s\n剩余嵌灵：%d/%d", row.Name, strings.Join(removed, "、"), len(remaining), row.SocketCount), Actions: []string{"装备详情 " + row.Name, "宝石查询", "背包"}}, true, nil
}

func (g *Game) gemCatalog(raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 6
	var total int64
	_ = g.store.DB.Model(&model.Item{}).Where("effect_func = ?", "equipment_gem").Count(&total).Error
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	var rows []model.Item
	if err := g.store.DB.Where("effect_func = ?", "equipment_gem").Order("rarity_id,base_value,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d种嵌灵宝石", page, pages, total), "━━━━━━━━━━━"}
	actions := []string{"装备背包", "装备开孔"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s【%s】\n属性：%s\n%s", row.Name, row.RarityName, artifactTemplateStatsText(row.EffectParams), row.Description), "━━━━━━━")
		actions = append(actions, "物品 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("宝石查询 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("宝石查询 %d", page+1))
	}
	return GameResult{Title: "🧿 嵌灵宝石图鉴", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) applyEquipmentStatDifference(playerID uint, before, after equipmentStats) error {
	var player model.Player
	if err := g.store.DB.First(&player, playerID).Error; err != nil {
		return err
	}
	skillBonus := g.activeSkillStatBonus(&player)
	return applyEquipmentStatDifferenceTx(g.store.DB, playerID, before, after, skillBonus)
}

func applyEquipmentStatDifferenceTx(tx *gorm.DB, playerID uint, before, after equipmentStats, skillBonus skillStatBonus) error {
	var player model.Player
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, playerID).Error; err != nil {
		return err
	}
	var realm model.Realm
	if err := tx.First(&realm, player.RealmID).Error; err != nil {
		return err
	}
	updated := playerAfterEquipmentStatDifference(player, realm, before, after, skillBonus)
	return tx.Model(&model.Player{}).Where("id = ?", playerID).Updates(equipmentPlayerStatUpdates(updated)).Error
}

func playerAfterEquipmentStatDifference(player model.Player, realm model.Realm, before, after equipmentStats, skillBonus skillStatBonus) model.Player {
	deltaAttack := after.Attack + after.Power - before.Attack - before.Power
	deltaDefense := after.Defense - before.Defense
	deltaHealth := after.Health - before.Health
	deltaMana := after.Mana - before.Mana
	deltaSpeed := after.Speed - before.Speed

	levelFloor := model.PlayerLevelStats(player.Level)
	attackFloor := max64(levelFloor.PhysicalAttack, max64(realm.BaseAttack, 1))
	defenseFloor := max64(levelFloor.PhysicalDefense, max64(realm.BaseDefense, 1))
	healthFloor := max64(levelFloor.MaxHealth, max64(realm.BaseHealth, 1))
	manaFloor := max64(levelFloor.MaxMana, max64(realm.BaseMana, 1))
	speedFloor := max64(levelFloor.Agility, max64(realm.BaseSpeed, 1))
	player.PhysicalAttack = max64(player.PhysicalAttack+deltaAttack, attackFloor)
	player.MagicAttack = max64(player.MagicAttack+deltaAttack, attackFloor)
	player.PhysicalDefense = max64(player.PhysicalDefense+deltaDefense, defenseFloor)
	player.MagicDefense = max64(player.MagicDefense+deltaDefense, defenseFloor)
	player.MaxHealth = max64(player.MaxHealth+deltaHealth, healthFloor)
	player.Health = min64(max64(player.Health+deltaHealth, 1), max64(player.MaxHealth+skillBonus.Health, 1))
	player.MaxMana = max64(player.MaxMana+deltaMana, manaFloor)
	player.Mana = min64(max64(player.Mana+deltaMana, 0), max64(player.MaxMana+skillBonus.Mana, 1))
	player.Agility = max64(player.Agility+deltaSpeed, speedFloor)
	return player
}

func equipmentPlayerStatUpdates(player model.Player) map[string]any {
	return map[string]any{
		"physical_attack": player.PhysicalAttack, "magic_attack": player.MagicAttack,
		"physical_defense": player.PhysicalDefense, "magic_defense": player.MagicDefense,
		"health": player.Health, "max_health": player.MaxHealth,
		"mana": player.Mana, "max_mana": player.MaxMana,
		"agility": player.Agility,
	}
}

func (g *Game) starRefineArtifact(player *model.Player, raw string) (GameResult, bool, error) {
	row, err := g.ownedArtifact(player.ID, strings.TrimSpace(raw))
	if err != nil {
		return GameResult{Title: "🌟 引星淬器", Content: "请输入：`装备星化 装备名`。", Actions: []string{"装备背包"}}, true, nil
	}
	if row.StarLevel >= 20 {
		return GameResult{Title: "🌟 星阶圆满", Content: row.Name + "已达二十星。", Actions: []string{"装备详情 " + row.Name}}, true, nil
	}
	cost := int64((row.StarLevel + 1) * 2)
	if err := g.adjustNamedItem(player.ID, "星辰砂", -cost); err != nil {
		return GameResult{Title: "🌟 星砂不足", Content: fmt.Sprintf("淬炼至%d星需要星辰砂×%d。", row.StarLevel+1, cost), Actions: []string{"物品 星辰砂", "地图", "副本"}}, true, nil
	}
	before := g.equipmentStats(row)
	updated := row
	updated.StarLevel++
	after := g.equipmentStats(updated)
	if err := g.store.DB.Model(&row).Update("star_level", updated.StarLevel).Error; err != nil {
		_ = g.adjustNamedItem(player.ID, "星辰砂", cost)
		return GameResult{}, true, err
	}
	if row.Equipped {
		_ = g.applyEquipmentStatDifference(player.ID, before, after)
	}
	return GameResult{Title: "🌟 引星淬器完成", Content: fmt.Sprintf("装备：%s\n星阶：%d → %d/20\n消耗：星辰砂×%d\n每星使装备基础属性额外提高15%%，穿戴属性与战力已同步。", row.Name, row.StarLevel, updated.StarLevel, cost), Actions: []string{"装备星化 " + row.Name, "装备详情 " + row.Name, "状态"}}, true, nil
}

func (g *Game) moveArtifactSmelter(player *model.Player, raw string, put bool) (GameResult, bool, error) {
	row, err := g.ownedArtifact(player.ID, strings.TrimSpace(raw))
	if err != nil {
		return GameResult{Title: "🔥 玄火炉", Content: "请输入装备完整名称。", Actions: []string{"装备背包"}}, true, nil
	}
	if put && row.Equipped {
		return GameResult{Title: "🔥 无法投入", Content: "请先卸下“" + row.Name + "”。", Actions: []string{"卸下 " + row.Name}}, true, nil
	}
	if row.InSmelter == put {
		state := "炉中"
		if !put {
			state = "装备背包中"
		}
		return GameResult{Title: "🔥 玄火炉状态未变", Content: row.Name + "已经在" + state + "。", Actions: []string{"装备背包"}}, true, nil
	}
	if err := g.store.DB.Model(&row).Update("in_smelter", put).Error; err != nil {
		return GameResult{}, true, err
	}
	verb := "投入"
	if !put {
		verb = "取出"
	}
	return GameResult{Title: "🔥 玄火炉" + verb, Content: fmt.Sprintf("%s：%s\n玄火炉仅作融合与保护容器，不会自动吞噬装备。", verb, row.Name), Actions: []string{"装备背包", "融合装备", "投入熔炉", "熔炉取出"}}, true, nil
}

func (g *Game) fuseArtifacts(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return GameResult{Title: "🔥 法器融合", Content: "格式：`融合装备 主装备=副装备`。副装备会永久消耗；双方均须未穿戴且不在玄火炉。", Actions: []string{"装备背包", "装备帮助"}}, true, nil
	}
	main, errMain := g.ownedArtifact(player.ID, strings.TrimSpace(parts[0]))
	feed, errFeed := g.ownedArtifact(player.ID, strings.TrimSpace(parts[1]))
	if errMain != nil || errFeed != nil || main.ID == feed.ID {
		return GameResult{Title: "🔥 融合器物无效", Content: "必须选择自己拥有的两件不同装备。", Actions: []string{"装备背包"}}, true, nil
	}
	if main.Equipped || feed.Equipped || main.InSmelter || feed.InSmelter {
		return GameResult{Title: "🔥 融合保护生效", Content: "已穿戴或玄火炉中的装备不能作为融合材料。", Actions: []string{"当前装备", "装备背包"}}, true, nil
	}
	qualities := []string{"凡品", "灵品", "仙品", "神品"}
	quality := main.Quality
	mainRank, feedRank := 0, 0
	for i, name := range qualities {
		if name == main.Quality {
			mainRank = i
		}
		if name == feed.Quality {
			feedRank = i
		}
	}
	forgeGain := maxInt(1, feed.ForgeLevel/2+1)
	starGain := maxInt(1, feed.StarLevel/3+1)
	if feedRank >= mainRank && mainRank+1 < len(qualities) {
		quality = qualities[mainRank+1]
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&main).Updates(map[string]any{"forge_level": main.ForgeLevel + forgeGain, "star_level": minInt(main.StarLevel+starGain, 20), "quality": quality}).Error; err != nil {
			return err
		}
		return tx.Delete(&feed).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🔥 法器融合完成", Content: fmt.Sprintf("主器：%s\n祭器：%s（已消耗）\n品质：%s → %s\n锻造：%d → %d\n星阶：%d → %d\n━━━━━━━━━━━\n灵纹与已嵌宝石保留在主器；祭器内宝石随器胚一并炼化。", main.Name, feed.Name, main.Quality, quality, main.ForgeLevel, main.ForgeLevel+forgeGain, main.StarLevel, minInt(main.StarLevel+starGain, 20)), Actions: []string{"装备详情 " + main.Name, "装备背包", "穿戴 " + main.Name}}, true, nil
}

func (g *Game) equipmentGuide(player *model.Player) GameResult {
	return GameResult{Title: "🛡️ 百炼装备指南", Content: "【获取】仙盟器阁兑购凡品器胚，或学习器谱后按真实材料炼制；首领、副本与活动可产出高品质装备。\n【槽位】十个穿戴槽位互相独立，同槽替换会自动卸下旧装备；剑、钟、镜等只是器型。\n【养成】强化提升等级，玄火锻造提升品质，灵纹铭刻赋予道韵，引星淬器提高基础属性，灵孔可嵌入宝石。\n【套装】当前穿戴达到二、四、六件时逐层唤醒共鸣；只激活图鉴不会提供战斗属性。\n【流转】法器可传承给道友；分解返还玄铁；融合永久消耗副装备。已穿戴、入炉和高品质批量分解受保护。\n【查询】装备详情同时显示槽位、器型、定位、基础/实际属性、前置、材料和来源。", Actions: []string{"装备系统", "装备图鉴", "装备背包", "仙盟器阁", "套装大全", "宝石查询", "图鉴菜单"}}
}

func (g *Game) desiredEquipmentSetStats(playerID uint) equipmentStats {
	return g.desiredEquipmentSetStatsWithDB(g.store.DB, playerID)
}

func (g *Game) desiredEquipmentSetStatsWithDB(db *gorm.DB, playerID uint) equipmentStats {
	type row struct {
		SetName      string
		SetBonusJSON string
		Count        int
	}
	var rows []row
	_ = db.Table("player_artifacts").Select("artifact_templates.set_name, MAX(artifact_templates.set_bonus_json) AS set_bonus_json, COUNT(*) AS count").Joins("JOIN artifact_templates ON artifact_templates.id = player_artifacts.template_id").Where("player_artifacts.player_id = ? AND player_artifacts.equipped = ? AND artifact_templates.set_name <> ''", playerID, true).Group("artifact_templates.set_name").Scan(&rows).Error
	total := equipmentStats{}
	for _, row := range rows {
		var stages map[string]json.RawMessage
		if json.Unmarshal([]byte(row.SetBonusJSON), &stages) != nil {
			continue
		}
		for _, stage := range []struct {
			key   string
			count int
		}{{"two", 2}, {"four", 4}, {"six", 6}} {
			if row.Count < stage.count {
				continue
			}
			var stats equipmentStats
			_ = json.Unmarshal(stages[stage.key], &stats)
			total.Attack += stats.Attack
			total.Defense += stats.Defense
			total.Health += stats.Health
			total.Mana += stats.Mana
			total.Speed += stats.Speed
			total.Power += stats.Power
		}
	}
	return total
}

func (g *Game) syncEquipmentSetBonuses(playerID uint) (equipmentStats, error) {
	var player model.Player
	if err := g.store.DB.First(&player, playerID).Error; err != nil {
		return equipmentStats{}, err
	}
	skillBonus := g.activeSkillStatBonus(&player)
	var desired equipmentStats
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		desired, err = g.syncEquipmentSetBonusesTx(tx, playerID, skillBonus)
		return err
	})
	return desired, err
}

func (g *Game) syncEquipmentSetBonusesTx(tx *gorm.DB, playerID uint, skillBonus skillStatBonus) (equipmentStats, error) {
	desired := g.desiredEquipmentSetStatsWithDB(tx, playerID)
	applied := equipmentStats{}
	var ledger model.PlayerValue
	ledgerErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", playerID, "equipment.set.applied").First(&ledger).Error
	if ledgerErr == nil {
		if err := json.Unmarshal([]byte(ledger.Value), &applied); err != nil {
			return equipmentStats{}, fmt.Errorf("equipment set ledger is invalid: %w", err)
		}
	} else if !errors.Is(ledgerErr, gorm.ErrRecordNotFound) {
		return equipmentStats{}, ledgerErr
	}
	if desired == applied {
		return desired, nil
	}
	if err := applyEquipmentStatDifferenceTx(tx, playerID, applied, desired, skillBonus); err != nil {
		return equipmentStats{}, err
	}
	encoded, _ := json.Marshal(desired)
	if err := upsertPlayerValueTx(tx, playerID, "equipment.set.applied", string(encoded), nil); err != nil {
		return equipmentStats{}, err
	}
	return desired, nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists || value == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

var _ = strconv.Itoa
