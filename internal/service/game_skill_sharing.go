package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

var createdSkillStyleOrder = []string{"剑道", "术法", "炼体", "神魂", "遁法", "均衡"}

var errCreatedSkillResourcesChanged = errors.New("created skill resources changed")

type createdSkillRequirement struct {
	Authored    int64
	Perception  int64
	DaoHeart    int64
	Cultivation int64
	Scrolls     int64
	Tea         int64
}

func createdSkillRequirements(authored int64) createdSkillRequirement {
	if authored < 0 {
		authored = 0
	}
	requirement := createdSkillRequirement{
		Authored:    authored,
		Perception:  30 + authored*15,
		DaoHeart:    50 + authored*3,
		Cultivation: 200 * (authored + 1) * (authored + 1),
		Scrolls:     2 + authored,
		Tea:         3 + authored,
	}
	if requirement.DaoHeart > 100 {
		requirement.DaoHeart = 100
	}
	if requirement.Scrolls > 20 {
		requirement.Scrolls = 20
	}
	if requirement.Tea > 20 {
		requirement.Tea = 20
	}
	return requirement
}

func createdSkillLuckBonus(luck int64) int {
	luck = normalizedPlayerLuck(luck)
	return maxInt(int(luck-initialPlayerLuck)/2, 0)
}

func createdSkillSuccessRate(player *model.Player, requirement createdSkillRequirement) int {
	perceptionBonus := maxInt(int((player.Perception-requirement.Perception)/3), 0)
	if perceptionBonus > 25 {
		perceptionBonus = 25
	}
	heartBonus := maxInt(int((player.DaoHeart-requirement.DaoHeart)/5), 0)
	if heartBonus > 10 {
		heartBonus = 10
	}
	return minInt(35+perceptionBonus+heartBonus+createdSkillLuckBonus(player.Luck), 90)
}

func createdSkillStyle(argument string, authored int64) (string, string) {
	raw := strings.TrimSpace(argument)
	if raw == "" {
		return createdSkillStyleOrder[int(authored)%len(createdSkillStyleOrder)], ""
	}
	fields := strings.Fields(raw)
	for _, style := range createdSkillStyleOrder {
		if fields[0] == style {
			return style, strings.TrimSpace(strings.TrimPrefix(raw, fields[0]))
		}
	}
	return createdSkillStyleOrder[int(authored)%len(createdSkillStyleOrder)], raw
}

func createdSkillEffect(style string, player *model.Player, authored int64) (skillStatBonus, string) {
	scalingAuthored := min64(max64(authored, 0), 20)
	effectivePerception := min64(max64(player.Perception, 0), 1000)
	effectiveSpirit := min64(max64(player.Spirit, 0), 1000)
	base := 8 + effectivePerception/20 + effectiveSpirit/40 + (scalingAuthored+1)*2
	base = base * (100 + (normalizedPlayerLuck(player.Luck)-initialPlayerLuck)/4) / 100
	base = min64(max64(base, 8), 80+scalingAuthored*4)
	bonus := skillStatBonus{}
	var description string
	switch style {
	case "剑道":
		bonus.PhysicalAttack = base * 3
		bonus.Speed = max64(base/2, 2)
		bonus.CritRate = 0.01 + float64(normalizedPlayerLuck(player.Luck))/2500
		description = "凝神御剑，以物攻、身法与暴击抢占回合先机。"
	case "术法":
		bonus.MagicAttack = base * 3
		bonus.Mana = base * 8
		bonus.CritRate = 0.01 + float64(normalizedPlayerLuck(player.Luck))/2500
		description = "引灵成术，以法强、法力与暴击持续施法。"
	case "炼体":
		bonus.Defense = base * 2
		bonus.Health = base * 20
		bonus.DamageReduction = 0.02 + float64(normalizedPlayerLuck(player.Luck))/2500
		description = "锤炼肉身，以双防、气血与减伤承受正面攻势。"
	case "神魂":
		bonus.Attack = base * 2
		bonus.Defense = max64(base/2, 2)
		bonus.Mana = base * 10
		description = "神念化形，同时强化攻法、识海法力与护神之力。"
	case "遁法":
		bonus.PhysicalAttack = base
		bonus.Speed = base * 3
		bonus.DodgeRate = 0.02 + float64(normalizedPlayerLuck(player.Luck))/2500
		description = "步踏虚空，以身法、闪避与迅捷攻势周旋制敌。"
	default:
		bonus.Attack = base * 2
		bonus.Defense = base
		bonus.Health = base * 8
		bonus.Mana = base * 5
		description = "调和攻守，兼顾攻法、双防、气血与法力。"
	}
	return bonus, description
}

func (g *Game) createPlayerSkill(player *model.Player, argument string) (GameResult, bool, error) {
	var authored int64
	if err := g.store.DB.Model(&model.SkillPublication{}).Where("creator_player_id = ?", player.ID).Count(&authored).Error; err != nil {
		return GameResult{}, true, err
	}
	style, name := createdSkillStyle(argument, authored)
	generatedName := name == ""
	if generatedName {
		var err error
		name, err = g.nextCreatedSkillName(player)
		if err != nil {
			return GameResult{Title: "功名已满", Content: "你的无名推演篇章已经全部留名，请发送 `创功 流派 自定义功法名` 指定新的全服唯一名称。\n本次没有消耗任何资源。", Actions: []string{"我的创功", "创功 剑道 自定义功法名"}}, true, nil
		}
	} else {
		if invalid := validateCustomizationName(name, 2, 16); invalid != "" {
			return GameResult{Title: "功法名称不合规", Content: invalid + "\n本次没有消耗任何资源。", Actions: []string{"创功 剑道 自定义功法名", "我的创功"}}, true, nil
		}
		if rejected, blocked, err := g.rejectSensitiveContent("自创功法", player, name); err != nil || blocked {
			return rejected, true, err
		}
	}

	var existing model.Skill
	if err := g.store.DB.Where("name = ?", name).First(&existing).Error; err == nil {
		var owned int64
		_ = g.store.DB.Model(&model.PlayerSkill{}).Where("player_id = ? AND skill_id = ?", player.ID, existing.ID).Count(&owned).Error
		if owned > 0 {
			return GameResult{Title: "功法已经掌握", Content: fmt.Sprintf("%s已经收入你的功法道藏，无需重复创立。\n可切换为主修、继续精进或另创不同名称的功法。", existing.Name), Actions: []string{"换功 " + existing.Name, "功法", "精进", "我的创功"}}, true, nil
		}
		return GameResult{Title: "功法名称已被占用", Content: fmt.Sprintf("万法阁中已经存在“%s”，功法名称必须全服唯一。\n请更换名称后重试，本次没有消耗任何资源。", name), Actions: []string{"创功 " + style + " 自定义功法名", "功法分享"}}, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}

	requirement := createdSkillRequirements(authored)
	shortages := make([]string, 0, 5)
	if player.Perception < requirement.Perception {
		shortages = append(shortages, fmt.Sprintf("悟性%d/%d", player.Perception, requirement.Perception))
	}
	if player.DaoHeart < requirement.DaoHeart {
		shortages = append(shortages, fmt.Sprintf("道心%d/%d", player.DaoHeart, requirement.DaoHeart))
	}
	if player.Cultivation < requirement.Cultivation {
		shortages = append(shortages, fmt.Sprintf("修为%d/%d", player.Cultivation, requirement.Cultivation))
	}
	scrolls := g.createdSkillItemQuantity(player.ID, "功法残卷")
	if scrolls < requirement.Scrolls {
		shortages = append(shortages, fmt.Sprintf("功法残卷%d/%d", scrolls, requirement.Scrolls))
	}
	tea := g.createdSkillItemQuantity(player.ID, "灵茶")
	if tea < requirement.Tea {
		shortages = append(shortages, fmt.Sprintf("灵茶%d/%d", tea, requirement.Tea))
	}
	rate := createdSkillSuccessRate(player, requirement)
	if len(shortages) > 0 {
		return GameResult{
			Title:   "创作准备不足",
			Content: fmt.Sprintf("拟创：%s · %s\n这是你的第%d部自创功法，后续创作门槛和消耗会逐部提高。\n━━━━━━━━━━━\n不足：%s\n完整消耗：修为%d · 功法残卷×%d · 灵茶×%d\n当前推演率：%d%%（运气%d提供+%d%%）\n━━━━━━━━━━━\n材料与修为不足时不会开始推演，也不会扣除任何资源。", name, style, authored+1, strings.Join(shortages, "、"), requirement.Cultivation, requirement.Scrolls, requirement.Tea, rate, normalizedPlayerLuck(player.Luck), createdSkillLuckBonus(player.Luck)),
			Actions: []string{"状态", "修炼", "物品 功法残卷", "物品 灵茶", "我的创功"},
		}, true, nil
	}

	if randomPercent() > rate {
		failedCultivation := max64(requirement.Cultivation/4, 1)
		failedScrolls := max64(requirement.Scrolls/2, 1)
		failedTea := max64(requirement.Tea/2, 1)
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			return spendCreatedSkillResources(tx, player.ID, failedCultivation, failedScrolls, failedTea)
		}); err != nil {
			return g.createdSkillResourcesChanged(), true, nil
		}
		return GameResult{
			Title:   "推演未成",
			Content: fmt.Sprintf("%s的%s道纹在最后一关崩散，本次没有生成残缺功法。\n推演成功率：%d%% · 运气加成：+%d%%\n━━━━━━━━━━━\n失败消耗：修为%d · 功法残卷×%d · 灵茶×%d\n再次创作仍按第%d部难度计算。", name, style, rate, createdSkillLuckBonus(player.Luck), failedCultivation, failedScrolls, failedTea, authored+1),
			Actions: []string{"创功 " + style + " " + name, "状态", "我的创功"},
		}, true, nil
	}

	bonus, styleDescription := createdSkillEffect(style, player, authored)
	effectJSON, _ := json.Marshal(bonus)
	upgradeJSON, _ := json.Marshal(map[string]any{
		"mastery_per_level": 120 + authored*20,
		"max_level":         50,
	})
	skill := model.Skill{
		Name: name, Type: style, Rarity: "自创", RealmRequired: displayOr(player.RealmName, "炼气"),
		Description: "由" + player.DaoName + "依自身道途推演而成。" + styleDescription,
		EffectJSON:  string(effectJSON), UpgradeJSON: string(upgradeJSON),
	}
	learned := model.PlayerSkill{PlayerID: player.ID, Level: 1, Equipped: player.CurrentSkillID == 0}
	publication := model.SkillPublication{CreatorPlayerID: player.ID, CreatorName: player.DaoName, Published: false}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := spendCreatedSkillResources(tx, player.ID, requirement.Cultivation, requirement.Scrolls, requirement.Tea); err != nil {
			return err
		}
		if err := tx.Create(&skill).Error; err != nil {
			return err
		}
		learned.SkillID = skill.ID
		if err := tx.Create(&learned).Error; err != nil {
			return err
		}
		publication.SkillID = skill.ID
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		if player.CurrentSkillID == 0 {
			return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("current_skill_id", skill.ID).Error
		}
		return nil
	})
	if err != nil {
		if g.store.DB.Where("name = ?", name).First(&existing).Error == nil {
			return GameResult{Title: "功法名称刚被占用", Content: "另一位道友先一步使用了“" + name + "”，本次事务已经回滚，材料和修为没有扣除。", Actions: []string{"创功 " + style + " 自定义功法名", "我的创功"}}, true, nil
		}
		if errors.Is(err, errCreatedSkillResourcesChanged) || strings.Contains(err.Error(), "insufficient item quantity") {
			return g.createdSkillResourcesChanged(), true, nil
		}
		return GameResult{}, true, err
	}
	return GameResult{
		Title:   "自创功法",
		Content: fmt.Sprintf("灵光汇聚成篇，独门道法已经收入你的私人道藏。\n━━━━━━━━━━━\n功法：**%s**\n创功者：%s\n流派：%s · 品阶：自创\n真实道效：%s\n推演成功率：%d%% · 运气加成：+%d%%\n消耗：修为%d · 功法残卷×%d · 灵茶×%d\n━━━━━━━━━━━\n公开状态：私藏\n其他玩家现在无法查看或学习。确认分享后发送“上传功法 %s”；若不上传，它会一直只留在你的私人道藏。", skill.Name, player.DaoName, style, skillBonusText(bonus), rate, createdSkillLuckBonus(player.Luck), requirement.Cultivation, requirement.Scrolls, requirement.Tea, skill.Name),
		Actions: []string{"换功 " + skill.Name, "上传功法 " + skill.Name, "我的创功", "功法", "创功"},
	}, true, nil
}

func (g *Game) createdSkillItemQuantity(playerID uint, name string) int64 {
	item, err := g.itemByName(name)
	if err != nil {
		return 0
	}
	return g.itemQuantity(playerID, item.ID)
}

func spendCreatedSkillResources(tx *gorm.DB, playerID uint, cultivation, scrolls, tea int64) error {
	result := tx.Model(&model.Player{}).
		Where("id = ? AND cultivation >= ?", playerID, cultivation).
		Update("cultivation", gorm.Expr("cultivation - ?", cultivation))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errCreatedSkillResourcesChanged
	}
	if err := consumeNamedItemTx(tx, playerID, "功法残卷", scrolls); err != nil {
		return errCreatedSkillResourcesChanged
	}
	if err := consumeNamedItemTx(tx, playerID, "灵茶", tea); err != nil {
		return errCreatedSkillResourcesChanged
	}
	return nil
}

func (g *Game) createdSkillResourcesChanged() GameResult {
	return GameResult{Title: "创作准备已变化", Content: "推演结算前检测到修为或材料数量已经变化，本次事务已全部回滚，没有扣除资源。请查看当前状态和背包后重试。", Actions: []string{"状态", "背包", "我的创功"}}
}

func (g *Game) ensureSkillPublication(player *model.Player, skill model.Skill) (model.SkillPublication, error) {
	var publication model.SkillPublication
	err := g.store.DB.Where("skill_id = ?", skill.ID).First(&publication).Error
	if err == nil {
		return publication, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return publication, err
	}
	var firstOwner model.PlayerSkill
	if err := g.store.DB.Where("skill_id = ?", skill.ID).Order("created_at, id").First(&firstOwner).Error; err != nil {
		return publication, err
	}
	if firstOwner.PlayerID != player.ID {
		return publication, errors.New("not skill creator")
	}
	publication = model.SkillPublication{SkillID: skill.ID, CreatorPlayerID: player.ID, CreatorName: player.DaoName}
	return publication, g.store.DB.Create(&publication).Error
}

func (g *Game) setSkillPublication(player *model.Player, argument string, publish bool) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		action := "上传功法"
		if !publish {
			action = "撤下功法"
		}
		return GameResult{Title: "自创功法分享", Content: "请输入：`" + action + " 功法名`。", Actions: []string{"我的创功", "功法分享"}}, true, nil
	}
	skill, err := g.skillByName(name)
	if err != nil || skill.Rarity != "自创" {
		return GameResult{Title: "操作失败", Content: "没有找到这部自创功法。", Actions: []string{"我的创功"}}, true, nil
	}
	publication, err := g.ensureSkillPublication(player, skill)
	if err != nil || publication.CreatorPlayerID != player.ID {
		return GameResult{Title: "操作失败", Content: "只有这部功法的原作者可以改变公开状态；学习者和受传者不能代替作者上传。", Actions: []string{"我的创功", "功法分享"}}, true, nil
	}
	if publication.Published == publish {
		status := "已经公开"
		action := "撤下功法 " + skill.Name
		if !publish {
			status = "当前就是私藏状态"
			action = "上传功法 " + skill.Name
		}
		return GameResult{Title: "公开状态未变化", Content: fmt.Sprintf("%s：%s。", skill.Name, status), Actions: []string{action, "我的创功", "功法分享"}}, true, nil
	}
	updates := map[string]any{"published": publish, "creator_name": player.DaoName}
	if publish {
		now := time.Now()
		updates["published_at"] = &now
	} else {
		updates["published_at"] = nil
	}
	if err := g.store.DB.Model(&publication).Updates(updates).Error; err != nil {
		return GameResult{}, true, err
	}
	if publish {
		return GameResult{Title: "功法上传成功", Content: fmt.Sprintf("%s已公开收入全服功法分享阁。\n流派：%s · 真实道效：%s\n其他玩家现在可以查看并消耗功法残卷学习；著作者仍为%s。", skill.Name, skill.Type, skillBonusText(decodeSkillStatBonus(skill, 1)), player.DaoName), Actions: []string{"功法分享", "撤下功法 " + skill.Name, "我的创功"}}, true, nil
	}
	return GameResult{Title: "功法已撤下", Content: fmt.Sprintf("%s已从全服分享阁撤下。\n已经学会的道友会保留功法和等级，之后的新玩家不能再从分享阁学习。", skill.Name), Actions: []string{"上传功法 " + skill.Name, "我的创功", "功法分享"}}, true, nil
}

type sharedSkillRow struct {
	SkillID     uint
	Name        string
	Type        string
	Description string
	EffectJSON  string
	CreatorName string
	Published   bool
	PublishedAt *time.Time
	Learners    int64
}

func parseSkillLibraryQuery(argument string, allowFilter bool) (string, int) {
	fields := strings.Fields(strings.TrimSpace(argument))
	page := 1
	if len(fields) > 0 {
		if parsed, err := strconv.Atoi(fields[len(fields)-1]); err == nil && parsed > 0 {
			page = parsed
			fields = fields[:len(fields)-1]
		}
	}
	if !allowFilter || len(fields) == 0 {
		return "", page
	}
	return strings.Join(fields, " "), page
}

func (g *Game) sharedSkillLibrary(_ *model.Player, argument string) (GameResult, bool, error) {
	filter, page := parseSkillLibraryQuery(argument, true)
	const pageSize = 6
	query := g.store.DB.Table("skill_publications AS publications").
		Joins("JOIN skills ON skills.id = publications.skill_id").
		Where("publications.published = ?", true)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("skills.type = ? OR skills.name LIKE ? OR publications.creator_name LIKE ?", filter, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []sharedSkillRow
	err := query.Select("skills.id AS skill_id, skills.name, skills.type, skills.description, skills.effect_json, publications.creator_name, publications.published, publications.published_at, (SELECT COUNT(*) FROM player_skills WHERE player_skills.skill_id = skills.id) AS learners").
		Order("publications.published_at DESC, publications.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "功法分享阁", Content: "当前没有符合条件的玩家公开功法。自创成功后由作者发送“上传功法 功法名”，才会进入这里。", Actions: []string{"我的创功", "创功", "功法"}}, true, nil
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 已公开%d部玩家功法", page, pages, total), "只有作者主动上传的自创功法会显示在这里。", "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows)+4)
	for _, row := range rows {
		skill := model.Skill{Name: row.Name, Type: row.Type, EffectJSON: row.EffectJSON}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  著者：%s · 掌握%d人\n  道效：%s\n  %s", row.Name, row.Type, row.CreatorName, row.Learners, skillBonusText(decodeSkillStatBonus(skill, 1)), row.Description))
		actions = append(actions, "学功 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("功法分享 %s %d", filter, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("功法分享 %s %d", filter, page+1))
	}
	actions = append(actions, "我的创功", "功法")
	return GameResult{Title: "功法分享阁", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) createdSkills(player *model.Player, argument string) (GameResult, bool, error) {
	_, page := parseSkillLibraryQuery(argument, false)
	const pageSize = 6
	query := g.store.DB.Table("skill_publications AS publications").
		Joins("JOIN skills ON skills.id = publications.skill_id").
		Where("publications.creator_player_id = ?", player.ID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []sharedSkillRow
	err := query.Select("skills.id AS skill_id, skills.name, skills.type, skills.description, skills.effect_json, publications.creator_name, publications.published, publications.published_at, (SELECT COUNT(*) FROM player_skills WHERE player_skills.skill_id = skills.id) AS learners").
		Order("publications.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	next := createdSkillRequirements(total)
	if len(rows) == 0 {
		return GameResult{Title: "我的创功", Content: fmt.Sprintf("你还没有自创功法。\n首部前置：悟性%d · 道心%d\n完整消耗：修为%d · 功法残卷×%d · 灵茶×%d\n可选流派：%s", next.Perception, next.DaoHeart, next.Cultivation, next.Scrolls, next.Tea, strings.Join(createdSkillStyleOrder, "、")), Actions: []string{"创功", "创功 剑道 自定义功法名", "功法分享", "状态"}}, true, nil
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共创作%d部", page, pages, total), "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows)+4)
	for _, row := range rows {
		status := "私藏"
		action := "上传功法 " + row.Name
		if row.Published {
			status = "已公开"
			action = "撤下功法 " + row.Name
		}
		skill := model.Skill{Name: row.Name, Type: row.Type, EffectJSON: row.EffectJSON}
		lines = append(lines, fmt.Sprintf("- %s【%s · %s】\n  道效：%s\n  掌握人数：%d", row.Name, row.Type, status, skillBonusText(decodeSkillStatBonus(skill, 1)), row.Learners))
		actions = append(actions, action)
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("下一部前置：悟性%d · 道心%d", next.Perception, next.DaoHeart), fmt.Sprintf("下一部完整消耗：修为%d · 功法残卷×%d · 灵茶×%d", next.Cultivation, next.Scrolls, next.Tea), fmt.Sprintf("当前运气%d提供创功成功率+%d%%。", normalizedPlayerLuck(player.Luck), createdSkillLuckBonus(player.Luck)))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("我的创功 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("我的创功 %d", page+1))
	}
	actions = append(actions, "创功", "功法分享", "功法")
	return GameResult{Title: "我的创功", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) skillVisibleToPlayer(player *model.Player, skill model.Skill) bool {
	if skill.Rarity != "自创" {
		return true
	}
	var owned int64
	if player != nil {
		_ = g.store.DB.Model(&model.PlayerSkill{}).Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).Count(&owned).Error
	}
	if owned > 0 {
		return true
	}
	var publication model.SkillPublication
	return g.store.DB.Where("skill_id = ? AND published = ?", skill.ID, true).First(&publication).Error == nil
}
