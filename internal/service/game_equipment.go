package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

var equipmentSlots = []string{"本命法器", "冠冕", "道袍", "护腕", "腰佩", "灵靴", "戒指", "项链", "护符", "阵盘"}

type equipmentStats struct {
	Attack  int64 `json:"attack"`
	Defense int64 `json:"defense"`
	Health  int64 `json:"health"`
	Mana    int64 `json:"mana"`
	Speed   int64 `json:"speed"`
	Power   int64 `json:"power"`
}

func (g *Game) equipmentMenu(player *model.Player) (GameResult, bool, error) {
	var templateCount, ownedCount, equippedCount, activatedCount int64
	if err := g.store.DB.Model(&model.ArtifactTemplate{}).Where("enabled = ?", true).Count(&templateCount).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ?", player.ID).Count(&ownedCount).Error
	_ = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND equipped = ?", player.ID, true).Count(&equippedCount).Error
	_ = g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND activated = ?", player.ID, true).Count(&activatedCount).Error
	lines := []string{
		fmt.Sprintf("道友：%s · 当前战力%d", player.DaoName, player.CombatPower),
		fmt.Sprintf("器谱收录%d件 · 持有%d件 · 穿戴%d/%d · 图鉴激活%d件", templateCount, ownedCount, equippedCount, len(equipmentSlots), activatedCount),
		"━━━━━━━━━━━",
		"【穿戴与道藏】",
		"穿戴法器 · 卸下法器 · 当前装备 · 装备背包",
		"装备图鉴 · 器谱详情 · 套装大全 · 当前套装",
		"【百炼养器】",
		"强化法器 · 玄火锻造 · 灵纹铭刻 · 引星淬器",
		"开辟灵孔 · 宝石图鉴 · 嵌灵宝石 · 取下宝石",
		"【器物流转】",
		"仙盟器阁 · 器阁兑购 · 法器传承 · 法器分解",
		"玄火炉投入 · 玄火炉取出 · 法器融合",
		"━━━━━━━━━━━",
		"槽位与器型严格分开：本命法器、冠冕、道袍等是穿戴位置；剑、钟、镜、鼎等是器型。",
	}
	return GameResult{Title: "🛡️ 仙尘百炼装备", Content: strings.Join(lines, "\n"), Actions: []string{
		"当前装备", "装备背包", "装备图鉴", "装备激活", "套装大全", "当前套装",
		"强化装备", "锻造装备", "装备铭刻", "装备星化", "装备开孔", "宝石查询",
		"仙盟器阁", "装备分解", "传送装备", "投入熔炉", "熔炉取出", "融合装备", "装备帮助",
	}}, true, nil
}

func (g *Game) equipmentCatalog(player *model.Player, raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterPage(raw)
	const pageSize = 5
	query := g.store.DB.Model(&model.ArtifactTemplate{}).Where("enabled = ?", true)
	if filter != "" && filter != "全部" {
		like := "%" + filter + "%"
		query = query.Where("slot = ? OR archetype = ? OR name LIKE ? OR positioning LIKE ? OR set_name LIKE ? OR type LIKE ?", filter, filter, like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.ArtifactTemplate
	if err := query.Order("minimum_realm_sequence,minimum_realm_level,slot,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d件器谱", page, pages, total), "筛选可填槽位、器型、定位、套装或名称。", "━━━━━━━━━━━"}
	actions := []string{"装备系统", "当前装备", "装备背包"}
	for _, row := range rows {
		slot := artifactTemplateSlot(row)
		lines = append(lines, fmt.Sprintf("⚔️ %s【%s · %s】\n定位：%s · 套装：%s\n基础属性：%s\n炼制：%s\n前置：%s", row.Name, slot, artifactTemplateArchetype(row), displayOr(row.Positioning, "均衡养器"), displayOr(row.SetName, "散修器物"), artifactTemplateStatsText(row.AttributeJSON), displayConfigText(row.MaterialsJSON), g.artifactRequirementText(row)), "━━━━━━━")
		actions = append(actions, "装备详情 "+row.Name)
	}
	if len(rows) == 0 {
		lines = append(lines, "没有找到符合筛选条件的器谱。")
	}
	if page > 1 {
		actions = append(actions, catalogPageAction("装备图鉴", filter, page-1))
	}
	if page < pages {
		actions = append(actions, catalogPageAction("装备图鉴", filter, page+1))
	}
	for _, slot := range equipmentSlots {
		actions = append(actions, "装备图鉴 "+slot)
	}
	return GameResult{Title: "🛡️ 万器图鉴", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func artifactTemplateSlot(row model.ArtifactTemplate) string {
	return storage.ArtifactTemplateSlot(row)
}

func artifactTemplateArchetype(row model.ArtifactTemplate) string {
	if strings.TrimSpace(row.Archetype) != "" {
		return row.Archetype
	}
	if row.Type != "" && artifactSlot(row.Type) != row.Type {
		return row.Type
	}
	return "法器"
}

func artifactTemplateStatsText(raw string) string {
	var values map[string]any
	if json.Unmarshal([]byte(raw), &values) != nil || len(values) == 0 {
		return "尚未配置"
	}
	labels := map[string]string{"attack": "攻击", "power": "道力", "defense": "防御", "health": "气血", "mana": "法力", "speed": "身法", "spirit": "神识", "perception": "悟性", "crit_rate": "暴击", "dodge": "闪避"}
	order := []string{"attack", "power", "defense", "health", "mana", "speed", "spirit", "perception", "crit_rate", "dodge"}
	parts := make([]string, 0, len(values))
	for _, key := range order {
		value, exists := values[key]
		if !exists {
			continue
		}
		parts = append(parts, labels[key]+"+"+displayConfigValue(value))
	}
	if len(parts) == 0 {
		return displayConfigText(raw)
	}
	return strings.Join(parts, " · ")
}

func (g *Game) artifactRequirementText(row model.ArtifactTemplate) string {
	parts := []string{}
	if row.MinimumRealmSequence > 0 {
		name := fmt.Sprintf("第%d境", row.MinimumRealmSequence)
		var realm model.Realm
		if g.store.DB.Where("sequence = ?", row.MinimumRealmSequence).First(&realm).Error == nil {
			name = realm.Name
		}
		parts = append(parts, fmt.Sprintf("%s·%d层", name, maxInt(row.MinimumRealmLevel, 1)))
	}
	if row.MinimumCombatPower > 0 {
		parts = append(parts, fmt.Sprintf("战力%d", row.MinimumCombatPower))
	}
	if len(parts) == 0 {
		return "入道即可使用"
	}
	return strings.Join(parts, " · ")
}

func (g *Game) artifactRequirementStatus(player *model.Player, row model.ArtifactTemplate) []string {
	unmet := []string{}
	sequence, _ := g.playerRealmSequence(player)
	if row.MinimumRealmSequence > 0 && sequence < row.MinimumRealmSequence {
		unmet = append(unmet, fmt.Sprintf("境界不足：需要%s", g.artifactRequirementText(row)))
	} else if row.MinimumRealmSequence > 0 && sequence == row.MinimumRealmSequence && player.RealmLevel < maxInt(row.MinimumRealmLevel, 1) {
		unmet = append(unmet, fmt.Sprintf("层数不足：需要%d层，当前%d层", row.MinimumRealmLevel, player.RealmLevel))
	}
	if row.MinimumCombatPower > 0 && player.CombatPower < row.MinimumCombatPower {
		unmet = append(unmet, fmt.Sprintf("战力不足：需要%d，当前%d", row.MinimumCombatPower, player.CombatPower))
	}
	return unmet
}

func (g *Game) equipmentDetails(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(strings.TrimPrefix(raw, "装备·"))
	if name == "" {
		return GameResult{Title: "🛡️ 器谱详情", Content: "请输入：`装备详情 装备名`，或从装备图鉴点击蓝字。", Actions: []string{"装备图鉴", "装备背包"}}, true, nil
	}
	var template model.ArtifactTemplate
	var owned model.PlayerArtifact
	_ = g.store.DB.Where("player_id = ? AND name = ?", player.ID, name).Order("level DESC,id DESC").First(&owned).Error
	queryName := name
	if owned.ID != 0 {
		if g.store.DB.First(&template, owned.TemplateID).Error != nil {
			return GameResult{Title: "🛡️ 器谱失联", Content: "这件自定义装备对应的器谱已经失联，请提交BUG；装备不会被删除。", Actions: []string{"提交BUG 装备详情：器谱失联；现象：自定义装备无法查询；期望：恢复器谱关联", "装备背包"}}, true, nil
		}
		queryName = owned.Name
	} else if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&template).Error != nil {
		return GameResult{Title: "🛡️ 器谱未收录", Content: "万器图鉴没有找到“" + name + "”。可按槽位或器型筛选。", Actions: []string{"装备图鉴", "装备图鉴 本命法器", "装备系统"}}, true, nil
	}
	status := "尚未持有"
	instance := ""
	if owned.ID != 0 {
		state := "未穿戴"
		if owned.Equipped {
			state = "已穿戴"
		} else if owned.InSmelter {
			state = "玄火炉中"
		}
		status = state
		stats := g.equipmentStats(owned)
		instance = fmt.Sprintf("\n【当前器物】\n品质：%s · 强化+%d · 锻造%d · 星阶%d\n灵孔：%d · 灵纹：%s\n实际属性：攻击+%d · 防御+%d · 气血+%d · 法力+%d · 身法+%d", owned.Quality, owned.Level, owned.ForgeLevel, owned.StarLevel, owned.SocketCount, displayOr(owned.Inscription, "未铭刻"), stats.Attack+stats.Power, stats.Defense, stats.Health, stats.Mana, stats.Speed)
	}
	sources, sourceActions := g.craftingMaterialGuide(template.MaterialsJSON)
	content := fmt.Sprintf("器名：%s · %s\n槽位：%s（穿戴位置）\n器型：%s（形制）\n定位：%s\n套装：%s\n━━━━━━━━━━━\n基础属性：%s\n强化上限：+%d\n穿戴前置：%s\n炼制材料：%s\n器物说明：%s\n图鉴来源：%s%s\n━━━━━━━━━━━\n【材料去向】\n%s", queryName, status, artifactTemplateSlot(template), artifactTemplateArchetype(template), displayOr(template.Positioning, "均衡养器"), displayOr(template.SetName, "散修器物"), artifactTemplateStatsText(template.AttributeJSON), template.MaxLevel, g.artifactRequirementText(template), displayConfigText(template.MaterialsJSON), displayOr(template.Description, "尚无器物志"), displayConfigText(template.SourceJSON), instance, sources)
	actions := []string{"装备图鉴", "学器 " + template.Name, "炼器 " + template.Name, "装备背包", "套装查询 " + displayOr(template.SetName, "散修器物")}
	if owned.ID != 0 {
		actions = append([]string{"穿戴 " + owned.Name, "卸下 " + owned.Name, "强宝 " + owned.Name, "锻造 " + owned.Name, "装备铭刻 " + owned.Name}, actions...)
	}
	actions = append(actions, sourceActions...)
	return GameResult{Title: "🛡️ 器谱详情", Content: content, Actions: actions}, true, nil
}

func (g *Game) equipmentOverview(player *model.Player) (GameResult, bool, error) {
	setStats, err := g.syncEquipmentSetBonuses(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
		*player = latest
	}
	var rows []model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ? AND equipped = ?", player.ID, true).Order("slot,id").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	bySlot := make(map[string]model.PlayerArtifact, len(rows))
	total := equipmentStats{}
	for _, row := range rows {
		slot := g.ensureArtifactSlot(&row)
		bySlot[slot] = row
		stats := g.equipmentStats(row)
		total.Attack += stats.Attack
		total.Defense += stats.Defense
		total.Health += stats.Health
		total.Mana += stats.Mana
		total.Speed += stats.Speed
		total.Power += stats.Power
	}
	lines := []string{fmt.Sprintf("道友：%s · 当前战力%d", player.DaoName, player.CombatPower), "━━━━━━━━━━━"}
	for _, slot := range equipmentSlots {
		if row, ok := bySlot[slot]; ok {
			rune := "未篆刻"
			if row.Inscription != "" {
				rune = row.Inscription
			}
			lines = append(lines, fmt.Sprintf("%s：%s · %s +%d · 锻造%d · %s", slot, row.Name, row.Quality, row.Level, row.ForgeLevel, rune))
		} else {
			lines = append(lines, slot+"：未装备")
		}
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("散件总加成：攻击+%d · 防御+%d · 气血+%d · 法力+%d · 身法+%d", total.Attack+total.Power, total.Defense, total.Health, total.Mana, total.Speed), fmt.Sprintf("套装共鸣：攻击+%d · 防御+%d · 气血+%d · 法力+%d · 身法+%d", setStats.Attack+setStats.Power, setStats.Defense, setStats.Health, setStats.Mana, setStats.Speed), "流程：获取器谱 → 炼器/兑购 → 穿戴 → 强化/锻造 → 星化/嵌灵 → 套装共鸣")
	return GameResult{Title: "当前装备", Content: strings.Join(lines, "\n"), Actions: []string{"装备系统", "装备背包", "装备图鉴", "当前套装", "锻造装备", "装备铭刻", "一键卸下"}}, true, nil
}

func (g *Game) equipmentBag(player *model.Player, argument string) (GameResult, bool, error) {
	const pageSize = 8
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(argument), 1)), 1)
	query := g.store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ?", player.ID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	var rows []model.PlayerArtifact
	if err := query.Order("equipped DESC,quality DESC,level DESC,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "装备背包", Content: "尚无装备。新手可开启青云入道礼匣，后续通过器谱炼制、首领掉落和副本获得装备。", Actions: []string{"礼包", "法宝", "副本", "首领"}}, true, nil
	}
	lines := []string{"装备名称不带编号，点击蓝字即可穿戴或卸下。", "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows)+3)
	for _, row := range rows {
		slot := g.ensureArtifactSlot(&row)
		state := "未装备"
		action := "穿戴 " + row.Name
		if row.Equipped {
			state = "已装备"
			action = "卸下 " + row.Name
		}
		stats := g.equipmentStats(row)
		var template model.ArtifactTemplate
		_ = g.store.DB.First(&template, row.TemplateID).Error
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  槽位：%s · 器型：%s · %s +%d\n  锻造%d · 星阶%d · 灵孔%d\n  攻击+%d · 防御+%d · 气血+%d · 法力+%d", row.Name, state, slot, artifactTemplateArchetype(template), row.Quality, row.Level, row.ForgeLevel, row.StarLevel, row.SocketCount, stats.Attack+stats.Power, stats.Defense, stats.Health, stats.Mana))
		actions = append(actions, action)
		actions = append(actions, "装备详情 "+row.Name)
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d件装备", page, pages, total))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("装备背包 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("装备背包 %d", page+1))
	}
	actions = append(actions, "当前装备")
	return GameResult{Title: "装备背包", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

var errEquipmentStateChanged = errors.New("equipment state changed during update")

func (g *Game) changeEquipment(player *model.Player, argument string, equip bool) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "装备操作", Content: "请发送 `装备背包` 后点击对应蓝字；无需输入序号。", Actions: []string{"装备背包", "当前装备"}}, true, nil
	}
	var artifact model.PlayerArtifact
	query := g.store.DB.Where("player_id = ?", player.ID)
	if equip {
		query = query.Where("name = ?", name)
	} else {
		query = query.Where("equipped = ? AND (name = ? OR slot = ?)", true, name, name)
	}
	if err := query.Order("level DESC,id DESC").First(&artifact).Error; err != nil {
		return GameResult{Title: "装备不存在", Content: "没有找到“" + name + "”，请从装备背包选择。", Actions: []string{"装备背包", "当前装备"}}, true, nil
	}
	slot := g.ensureArtifactSlot(&artifact)
	if artifact.InSmelter {
		return GameResult{Title: "玄火炉封存", Content: artifact.Name + "仍在玄火炉中，请先取出再穿戴。", Actions: []string{"熔炉取出 " + artifact.Name, "装备背包"}}, true, nil
	}
	var template model.ArtifactTemplate
	if g.store.DB.First(&template, artifact.TemplateID).Error == nil && equip {
		if unmet := g.artifactRequirementStatus(player, template); len(unmet) > 0 {
			return GameResult{Title: "穿戴前置未满", Content: fmt.Sprintf("装备：%s\n槽位：%s · 器型：%s\n━━━━━━━━━━━\n- %s", artifact.Name, slot, artifactTemplateArchetype(template), strings.Join(unmet, "\n- ")), Actions: []string{"装备详情 " + artifact.Name, "修炼", "突破", "当前装备"}}, true, nil
		}
	}
	if artifact.Equipped == equip {
		return GameResult{Title: "装备状态未变", Content: fmt.Sprintf("%s当前已经%s。", artifact.Name, map[bool]string{true: "穿戴", false: "卸下"}[equip]), Actions: []string{"当前装备"}}, true, nil
	}
	old := model.PlayerArtifact{}
	delta := equipmentStats{}
	skillBonus := g.activeSkillStatBonus(player)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.PlayerArtifact
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND player_id = ?", artifact.ID, player.ID).First(&current).Error; err != nil {
			return err
		}
		if current.Equipped == equip {
			return errEquipmentStateChanged
		}
		selected := g.equipmentStatsWithDB(tx, current)
		oldStats := equipmentStats{}
		if equip {
			findOld := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND equipped = ? AND slot = ? AND id <> ?", player.ID, true, slot, current.ID).First(&old)
			if findOld.Error != nil && !errors.Is(findOld.Error, gorm.ErrRecordNotFound) {
				return findOld.Error
			}
			if old.ID != 0 {
				oldStats = g.equipmentStatsWithDB(tx, old)
			}
		}
		sign := int64(1)
		if !equip {
			sign = -1
		}
		delta = equipmentStats{
			Attack:  (selected.Attack+selected.Power)*sign - oldStats.Attack - oldStats.Power,
			Defense: selected.Defense*sign - oldStats.Defense,
			Health:  selected.Health*sign - oldStats.Health,
			Mana:    selected.Mana*sign - oldStats.Mana,
			Speed:   selected.Speed*sign - oldStats.Speed,
		}
		beforeStats, afterStats := equipmentStats{}, selected
		if equip {
			beforeStats = oldStats
		} else {
			beforeStats, afterStats = selected, equipmentStats{}
		}
		if old.ID != 0 {
			updated := tx.Model(&model.PlayerArtifact{}).Where("id = ? AND player_id = ? AND equipped = ?", old.ID, player.ID, true).Update("equipped", false)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errEquipmentStateChanged
			}
		}
		updated := tx.Model(&model.PlayerArtifact{}).Where("id = ? AND player_id = ? AND equipped = ?", current.ID, player.ID, current.Equipped).
			Updates(map[string]any{"equipped": equip, "slot": slot})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errEquipmentStateChanged
		}
		artifact = current
		if err := applyEquipmentStatDifferenceTx(tx, player.ID, beforeStats, afterStats, skillBonus); err != nil {
			return err
		}
		_, err := g.syncEquipmentSetBonusesTx(tx, player.ID, skillBonus)
		return err
	})
	if errors.Is(err, errEquipmentStateChanged) {
		return GameResult{Title: "装备状态已变化", Content: "该装备的穿戴状态刚刚发生变化，本次没有重复增减任何属性，请重新查看。", Actions: []string{"当前装备", "装备背包"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	verb := "穿戴"
	if !equip {
		verb = "卸下"
	}
	replaced := ""
	if old.ID != 0 {
		replaced = "\n自动卸下同槽装备：" + old.Name
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	latest, _ = g.players.Get(player.ID)
	powerDelta := latest.CombatPower - player.CombatPower
	return GameResult{Title: verb + "成功", Content: fmt.Sprintf("%s：%s\n槽位：%s · 器型：%s%s\n━━━━━━━━━━━\n攻击变化：%+d\n防御变化：%+d\n气血变化：%+d\n法力变化：%+d\n身法变化：%+d\n战力变化：%+d（当前%d）", verb, artifact.Name, slot, artifactTemplateArchetype(template), replaced, delta.Attack, delta.Defense, delta.Health, delta.Mana, delta.Speed, powerDelta, latest.CombatPower), Actions: []string{"当前装备", "装备背包", "装备详情 " + artifact.Name, "锻造 " + artifact.Name, "装备铭刻 " + artifact.Name}}, true, nil
}

func (g *Game) unequipAllEquipment(player *model.Player) (GameResult, bool, error) {
	count := 0
	clearedSetBonus := false
	skillBonus := g.activeSkillStatBonus(player)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, player.ID).Error; err != nil {
			return err
		}
		var realm model.Realm
		if err := tx.First(&realm, current.RealmID).Error; err != nil {
			return err
		}
		var rows []model.PlayerArtifact
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND equipped = ?", player.ID, true).Find(&rows).Error; err != nil {
			return err
		}
		removed := equipmentStats{}
		for _, row := range rows {
			stats := g.equipmentStatsWithDB(tx, row)
			removed.Attack += stats.Attack
			removed.Defense += stats.Defense
			removed.Health += stats.Health
			removed.Mana += stats.Mana
			removed.Speed += stats.Speed
			removed.Power += stats.Power
		}
		var appliedSet equipmentStats
		var ledger model.PlayerValue
		ledgerErr := tx.Where("player_id = ? AND key = ?", player.ID, "equipment.set.applied").First(&ledger).Error
		if ledgerErr == nil {
			if err := json.Unmarshal([]byte(ledger.Value), &appliedSet); err != nil {
				return fmt.Errorf("equipment set ledger is invalid: %w", err)
			}
			removed.Attack += appliedSet.Attack
			removed.Defense += appliedSet.Defense
			removed.Health += appliedSet.Health
			removed.Mana += appliedSet.Mana
			removed.Speed += appliedSet.Speed
			removed.Power += appliedSet.Power
			clearedSetBonus = appliedSet != (equipmentStats{})
		} else if !errors.Is(ledgerErr, gorm.ErrRecordNotFound) {
			return ledgerErr
		}
		if len(rows) == 0 && !clearedSetBonus {
			return nil
		}
		if len(rows) > 0 {
			updated := tx.Model(&model.PlayerArtifact{}).Where("player_id = ? AND equipped = ?", player.ID, true).Update("equipped", false)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != int64(len(rows)) {
				return errors.New("equipped artifact state changed during unequip all")
			}
		}
		adjusted := playerAfterEquipmentStatDifference(current, realm, removed, equipmentStats{}, skillBonus)
		adjusted.CombatPower = calculateCombatPower(adjusted)
		updates := equipmentPlayerStatUpdates(adjusted)
		updates["combat_power"] = adjusted.CombatPower
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("player_id = ? AND key = ?", player.ID, "equipment.set.applied").Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		count = len(rows)
		return nil
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if count == 0 && !clearedSetBonus {
		return GameResult{Title: "一键卸下", Content: "当前没有穿戴任何装备。", Actions: []string{"装备背包"}}, true, nil
	}
	latest, err := g.players.Get(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if err := g.syncPlayerCombatPower(&latest); err != nil {
		return GameResult{}, true, err
	}
	content := fmt.Sprintf("已卸下%d件装备，所有装备均保留在装备背包。", count)
	if clearedSetBonus {
		content += "\n对应套装共鸣已同步解除。"
	}
	return GameResult{Title: "一键卸下", Content: content, Actions: []string{"当前装备", "装备背包"}}, true, nil
}

var (
	errForgeMaterials  = errors.New("forge materials insufficient")
	errForgeConcurrent = errors.New("forge state changed")
)

func (g *Game) forgeEquipment(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "装备锻造", Content: "请输入：`锻造 装备名`，或从装备背包选择。", Actions: []string{"装备背包"}}, true, nil
	}
	const forgeLimit = 30
	var artifact, upgraded model.PlayerArtifact
	var beforeStats, afterStats, delta equipmentStats
	var oldForge int
	var oldQuality string
	var cost int64
	missing, capped := false, false
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.PlayerArtifact
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND name = ?", player.ID, name).Order("level DESC,id DESC").First(&current).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			missing = true
			return nil
		}
		if findErr != nil {
			return findErr
		}
		artifact = current
		oldForge, oldQuality = current.ForgeLevel, current.Quality
		if oldForge >= forgeLimit {
			capped = true
			return nil
		}
		cost = int64(maxInt(oldForge+1, 1) * 2)
		newForge := oldForge + 1
		newQuality := oldQuality
		if newForge%3 == 0 {
			qualities := []string{"凡品", "灵品", "仙品", "神品"}
			for index, quality := range qualities {
				if quality == oldQuality && index+1 < len(qualities) {
					newQuality = qualities[index+1]
					break
				}
			}
		}
		beforeStats = g.equipmentStatsWithDB(tx, current)
		upgraded = current
		upgraded.ForgeLevel, upgraded.Quality = newForge, newQuality
		afterStats = g.equipmentStatsWithDB(tx, upgraded)
		delta = equipmentStats{
			Attack:  afterStats.Attack + afterStats.Power - beforeStats.Attack - beforeStats.Power,
			Defense: afterStats.Defense - beforeStats.Defense,
			Health:  afterStats.Health - beforeStats.Health,
			Mana:    afterStats.Mana - beforeStats.Mana,
			Speed:   afterStats.Speed - beforeStats.Speed,
		}
		if err := consumeNamedItemTx(tx, player.ID, "玄铁", cost); err != nil {
			return errForgeMaterials
		}
		updated := tx.Model(&model.PlayerArtifact{}).Where("id = ? AND player_id = ? AND forge_level = ?", current.ID, player.ID, oldForge).
			Updates(map[string]any{"forge_level": newForge, "quality": newQuality})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errForgeConcurrent
		}
		if current.Equipped {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
				"physical_attack":  gorm.Expr("MAX(physical_attack + ?, 1)", delta.Attack),
				"magic_attack":     gorm.Expr("MAX(magic_attack + ?, 1)", delta.Attack),
				"physical_defense": gorm.Expr("MAX(physical_defense + ?, 1)", delta.Defense),
				"magic_defense":    gorm.Expr("MAX(magic_defense + ?, 1)", delta.Defense),
				"max_health":       gorm.Expr("MAX(max_health + ?, 1)", delta.Health),
				"health":           gorm.Expr("MIN(MAX(health + ?, 1), MAX(max_health + ?, 1))", delta.Health, delta.Health),
				"max_mana":         gorm.Expr("MAX(max_mana + ?, 1)", delta.Mana),
				"mana":             gorm.Expr("MIN(MAX(mana + ?, 0), MAX(max_mana + ?, 1))", delta.Mana, delta.Mana),
				"agility":          gorm.Expr("MAX(agility + ?, 1)", delta.Speed),
			}).Error; err != nil {
				return err
			}
		}
		_, err := addPlayerValueIntTx(tx, player.ID, "stats.forges", 1)
		return err
	})
	if missing {
		return GameResult{Title: "装备锻造", Content: "没有找到“" + name + "”，请从装备背包选择。", Actions: []string{"装备背包"}}, true, nil
	}
	if capped {
		return GameResult{Title: "锻造已达上限", Content: fmt.Sprintf("%s已达当前锻造上限%d重。本次未扣除玄铁；仍可继续强化、星化、铭刻与开孔。", artifact.Name, forgeLimit), Actions: []string{"装备详情 " + artifact.Name, "强化 " + artifact.Name, "装备星化 " + artifact.Name}}, true, nil
	}
	if errors.Is(err, errForgeMaterials) {
		return GameResult{Title: "锻造材料不足", Content: fmt.Sprintf("%s锻造至下一重需要玄铁×%d，本次没有扣除材料。", artifact.Name, cost), Actions: []string{"货铺", "探索", "装备背包"}}, true, nil
	}
	if errors.Is(err, errForgeConcurrent) {
		return GameResult{Title: "锻造状态已变化", Content: artifact.Name + "的锻造状态刚刚发生变化，本次事务已回滚且未扣玄铁，请重新查看后再试。", Actions: []string{"装备详情 " + artifact.Name, "装备背包"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.syncEquipmentSetBonuses(player.ID)
	bonusText := "装备未穿戴，属性将在穿戴时计入角色战力。"
	if artifact.Equipped {
		bonusText = fmt.Sprintf("出战加成已同步：攻击%+d · 防御%+d · 气血%+d · 法力%+d。", delta.Attack, delta.Defense, delta.Health, delta.Mana)
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	slot := g.ensureArtifactSlot(&artifact)
	var template model.ArtifactTemplate
	_ = g.store.DB.First(&template, artifact.TemplateID).Error
	result := GameResult{Title: "玄火锻造", Content: fmt.Sprintf("装备：%s\n槽位：%s · 器型：%s\n锻造：%d → %d\n消耗：玄铁×%d\n品质：%s → %s\n%s\n每三重锻造提升一次品质，装备加成与总战力实时同步。", artifact.Name, slot, artifactTemplateArchetype(template), oldForge, upgraded.ForgeLevel, cost, oldQuality, upgraded.Quality, bonusText), Actions: []string{"锻造 " + artifact.Name, "装备背包", "当前装备", "状态"}}
	if upgraded.Quality != oldQuality && (upgraded.Quality == "仙品" || upgraded.Quality == "神品") {
		broadcast := fmt.Sprintf("【百炼升品】道友%s凭苦修毅力反复淬炼，使%s晋升为%s至宝，器鸣响彻诸天！", player.DaoName, artifact.Name, upgraded.Quality)
		_ = g.publishWorldBroadcast("珍宝", artifact.Name+"晋升"+upgraded.Quality, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func (g *Game) inscribeEquipment(player *model.Player, argument string) (GameResult, bool, error) {
	var artifact model.PlayerArtifact
	name := strings.TrimSpace(argument)
	if name == "" || g.store.DB.Where("player_id = ? AND name = ?", player.ID, name).Order("level DESC").First(&artifact).Error != nil {
		return GameResult{Title: "装备篆刻", Content: "请输入：`篆刻 装备名`，或从装备背包选择。", Actions: []string{"装备背包"}}, true, nil
	}
	if err := g.adjustNamedItem(player.ID, "阵基石", -1); err != nil {
		return GameResult{Title: "篆刻材料不足", Content: "篆刻需要阵基石×1。", Actions: []string{"探索", "货铺"}}, true, nil
	}
	runes := []string{"庚金破军纹", "乙木回春纹", "玄水护魂纹", "离火焚天纹", "厚土镇岳纹", "风雷神行纹"}
	rune := runes[(int(artifact.ID)+artifact.ForgeLevel+artifact.Level)%len(runes)]
	if err := g.store.DB.Model(&artifact).Update("inscription", rune).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵纹篆成", Content: fmt.Sprintf("装备：%s\n消耗：阵基石×1\n原灵纹：%s\n新灵纹：%s\n灵纹会参与后续套装、克制和战斗效果计算。", artifact.Name, displayOr(artifact.Inscription, "无"), rune), Actions: []string{"当前装备", "装备背包", "篆刻 " + artifact.Name}}, true, nil
}

func (g *Game) ensureArtifactSlot(row *model.PlayerArtifact) string {
	if strings.TrimSpace(row.Slot) != "" {
		return row.Slot
	}
	var template model.ArtifactTemplate
	_ = g.store.DB.First(&template, row.TemplateID).Error
	slot := artifactTemplateSlot(template)
	_ = g.store.DB.Model(row).Update("slot", slot).Error
	row.Slot = slot
	return slot
}

func (g *Game) artifactDisplayIdentity(row *model.PlayerArtifact) (string, string) {
	slot := g.ensureArtifactSlot(row)
	var template model.ArtifactTemplate
	_ = g.store.DB.First(&template, row.TemplateID).Error
	return slot, artifactTemplateArchetype(template)
}

func artifactSlot(kind string) string {
	return storage.ArtifactSlot(kind)
}

// repairMigratedArtifactSlots resolves collisions created when legacy owned
// artifacts move from their fallback slot to the template's real slot. The
// strongest item remains equipped; every other item stays intact in the bag.
func (g *Game) repairMigratedArtifactSlots() error {
	var playerIDs []uint
	if err := g.store.DB.Model(&model.PlayerValue{}).Where("key = ?", storage.ArtifactSlotSyncMigrationKey).
		Distinct("player_id").Pluck("player_id", &playerIDs).Error; err != nil {
		return err
	}
	var firstErr error
	for _, playerID := range playerIDs {
		if err := g.repairMigratedArtifactSlotsForPlayer(playerID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := g.store.DB.Where("player_id = ? AND key = ?", playerID, storage.ArtifactSlotSyncMigrationKey).Delete(&model.PlayerValue{}).Error; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (g *Game) repairMigratedArtifactSlotsForPlayer(playerID uint) error {
	var equipped []model.PlayerArtifact
	if err := g.store.DB.Where("player_id = ? AND equipped = ?", playerID, true).Order("slot,id").Find(&equipped).Error; err != nil {
		return err
	}
	winners := make(map[string]model.PlayerArtifact, len(equipped))
	losers := make([]model.PlayerArtifact, 0)
	for _, row := range equipped {
		winner, exists := winners[row.Slot]
		if !exists {
			winners[row.Slot] = row
			continue
		}
		if g.preferArtifactForSlot(row, winner) {
			losers = append(losers, winner)
			winners[row.Slot] = row
		} else {
			losers = append(losers, row)
		}
	}
	if len(losers) > 0 {
		// Bring the pre-migration set ledger current before changing its count.
		if _, err := g.syncEquipmentSetBonuses(playerID); err != nil {
			return err
		}
		removed := equipmentStats{}
		ids := make([]uint, 0, len(losers))
		for _, row := range losers {
			stats := g.equipmentStats(row)
			removed.Attack += stats.Attack
			removed.Defense += stats.Defense
			removed.Health += stats.Health
			removed.Mana += stats.Mana
			removed.Speed += stats.Speed
			removed.Power += stats.Power
			ids = append(ids, row.ID)
		}
		var player model.Player
		if err := g.store.DB.First(&player, playerID).Error; err != nil {
			return err
		}
		skillBonus := g.activeSkillStatBonus(&player)
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			updated := tx.Model(&model.PlayerArtifact{}).Where("id IN ? AND player_id = ? AND equipped = ?", ids, playerID, true).Update("equipped", false)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != int64(len(ids)) {
				return errors.New("equipped artifact state changed during slot repair")
			}
			return applyEquipmentStatDifferenceTx(tx, playerID, removed, equipmentStats{}, skillBonus)
		}); err != nil {
			return err
		}
	}
	if _, err := g.syncEquipmentSetBonuses(playerID); err != nil {
		return err
	}
	player, err := g.players.Get(playerID)
	if err != nil {
		return err
	}
	return g.syncPlayerCombatPower(&player)
}

func (g *Game) preferArtifactForSlot(candidate, current model.PlayerArtifact) bool {
	candidatePower := artifactStatsPower(g.equipmentStats(candidate))
	currentPower := artifactStatsPower(g.equipmentStats(current))
	if candidatePower != currentPower {
		return candidatePower > currentPower
	}
	if candidate.ForgeLevel != current.ForgeLevel {
		return candidate.ForgeLevel > current.ForgeLevel
	}
	if candidate.Level != current.Level {
		return candidate.Level > current.Level
	}
	return candidate.ID < current.ID
}

func (g *Game) equipmentStats(row model.PlayerArtifact) equipmentStats {
	return g.equipmentStatsWithDB(g.store.DB, row)
}

func (g *Game) equipmentStatsWithDB(db *gorm.DB, row model.PlayerArtifact) equipmentStats {
	var template model.ArtifactTemplate
	if db.First(&template, row.TemplateID).Error != nil {
		return equipmentStats{Attack: int64(maxInt(row.Level, 1) * 2)}
	}
	stats := equipmentStats{}
	_ = json.Unmarshal([]byte(template.AttributeJSON), &stats)
	var gems []string
	if json.Unmarshal([]byte(row.SocketJSON), &gems) == nil {
		for _, gemName := range gems {
			var gem model.Item
			if db.Where("name = ? AND effect_func = ?", gemName, "equipment_gem").First(&gem).Error != nil {
				continue
			}
			var bonus equipmentStats
			if json.Unmarshal([]byte(gem.EffectParams), &bonus) == nil {
				stats.Attack += bonus.Attack
				stats.Defense += bonus.Defense
				stats.Health += bonus.Health
				stats.Mana += bonus.Mana
				stats.Speed += bonus.Speed
				stats.Power += bonus.Power
			}
		}
	}
	quality := map[string]int64{"凡品": 100, "灵品": 125, "仙品": 165, "神品": 220}[row.Quality]
	if quality == 0 {
		quality = 100
	}
	multiplier := quality + int64(maxInt(row.Level-1, 0)*8+row.ForgeLevel*12)
	multiplier += int64(maxInt(row.StarLevel, 0) * 15)
	stats.Attack = stats.Attack * multiplier / 100
	stats.Defense = stats.Defense * multiplier / 100
	stats.Health = stats.Health * multiplier / 100
	stats.Mana = stats.Mana * multiplier / 100
	stats.Speed = stats.Speed * multiplier / 100
	stats.Power = stats.Power * multiplier / 100
	return stats
}
