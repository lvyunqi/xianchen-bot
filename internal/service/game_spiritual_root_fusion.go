package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const pendingSpiritualRootKey = "spiritual_root.fusion.pending"

type spiritualRootFusionResult struct {
	Mode         string    `json:"mode,omitempty"`
	ParentA      string    `json:"parent_a"`
	ParentB      string    `json:"parent_b"`
	SourcePlayer string    `json:"source_player,omitempty"`
	SourceStage  int       `json:"source_stage,omitempty"`
	Result       string    `json:"result"`
	Quality      int       `json:"quality"`
	CreatedAt    time.Time `json:"created_at"`
}

func (g *Game) fuseSpiritualRoots(player *model.Player, arguments []string) (GameResult, bool, error) {
	if len(arguments) < 2 {
		return GameResult{Title: "🌱 灵根随机重铸（替换型）", Content: "这不是属性叠加功能：候选道种不叠加当前灵根属性，也不会把两条父系灵根加到当前灵根上。\n选择两种不同的图鉴灵根作为重铸道纹，随机凝成一枚非父系的候选道种。\n格式：`灵根合成 灵根A 灵根B`\n消耗：灵根精粹×2、阵基石×1、灵石×500。\n结果：道种凝成后，只有发送“吸收灵根”才会替换当前灵根；放弃时材料不返还。", Actions: []string{"灵根图鉴", "物品 阵基石", "庆典特卖 2", "灵根道种", "灵根进化菜单"}}, true, nil
	}
	if _, err := g.playerValue(player.ID, pendingSpiritualRootKey); err == nil {
		return GameResult{Title: "🌱 已有待定灵根", Content: "识海中已有一枚尚未处理的合成道种。请先查看并选择吸收或放弃，系统不会覆盖上一次结果。", Actions: []string{"灵根道种", "吸收灵根", "放弃灵根"}}, true, nil
	}
	firstName, secondName := strings.TrimSpace(arguments[0]), strings.TrimSpace(arguments[1])
	var first, second model.SpiritualRootTemplate
	if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, firstName, firstName).First(&first).Error != nil {
		return GameResult{Title: "🌱 第一灵根未收录", Content: "图鉴中没有找到“" + firstName + "”，请从灵根图鉴蓝字选择。", Actions: []string{"灵根图鉴"}}, true, nil
	}
	if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, secondName, secondName).First(&second).Error != nil {
		return GameResult{Title: "🌱 第二灵根未收录", Content: "图鉴中没有找到“" + secondName + "”，请从灵根图鉴蓝字选择。", Actions: []string{"灵根图鉴"}}, true, nil
	}
	if first.ID == second.ID {
		return GameResult{Title: "🌱 父系道纹相同", Content: "两种输入灵根必须不同；相同道纹只能进化，不能进行随机合成。本次没有消耗材料。", Actions: []string{"灵根图鉴", "灵进", "灵根合成"}}, true, nil
	}
	if player.RealmLevel < 3 && player.RealmName == "炼气" {
		return GameResult{Title: "🌱 灵根合成未解锁", Content: fmt.Sprintf("当前：%s·%d层\n前置：炼气三层或更高境界\n原因：肉身尚不能承受两条本源道纹互相冲击。", player.RealmName, player.RealmLevel), Actions: []string{"修炼", "突破", "状态"}}, true, nil
	}
	if player.RootQuality < 50 {
		return GameResult{Title: "🌱 灵根纯度不足", Content: fmt.Sprintf("灵根合成要求当前纯度至少50，当前%d。先淬炼本源，材料不会提前扣除。", player.RootQuality), Actions: []string{"灵淬", "灵检", "灵根进化菜单"}}, true, nil
	}
	essence, essenceErr := g.itemByName("灵根精粹")
	stone, stoneErr := g.itemByName("阵基石")
	if essenceErr != nil || stoneErr != nil {
		return GameResult{}, true, fmt.Errorf("灵根合成材料未载入")
	}
	essenceOwned := g.itemQuantity(player.ID, essence.ID)
	stoneOwned := g.itemQuantity(player.ID, stone.ID)
	if essenceOwned < 2 || stoneOwned < 1 || player.SpiritStones < 500 {
		return GameResult{Title: "🌱 灵根合成材料不足", Content: fmt.Sprintf("需要：灵根精粹×2、阵基石×1、灵石×500\n持有：灵根精粹×%d、阵基石×%d、灵石×%d\n━━━━━━━━━━━\n灵根精粹可通过“合成 太初灵根精粹”获得；点击材料可查询其余来源。", essenceOwned, stoneOwned, player.SpiritStones), Actions: []string{"合成 太初灵根精粹", "配方 太初灵根精粹", "物品 灵根精粹", "物品 阵基石", "背包"}}, true, nil
	}
	resultRoot, err := g.randomFusionRoot(first.Name, second.Name, player.SpiritualRoot, player.Luck)
	if err != nil {
		return GameResult{}, true, err
	}
	quality := (first.BaseQuality + second.BaseQuality + resultRoot.BaseQuality) / 3
	quality += rand.Intn(7) - 3
	quality = minInt(maxInt(quality, 35), 100)
	pending := spiritualRootFusionResult{Mode: "fusion", ParentA: first.Name, ParentB: second.Name, Result: resultRoot.Name, Quality: quality, CreatedAt: time.Now()}
	encoded, _ := json.Marshal(pending)
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeNamedItemTx(tx, player.ID, "灵根精粹", 2); err != nil {
			return err
		}
		if err := consumeNamedItemTx(tx, player.ID, "阵基石", 1); err != nil {
			return err
		}
		payment := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, 500).Update("spirit_stones", gorm.Expr("spirit_stones - ?", 500))
		if payment.Error != nil {
			return payment.Error
		}
		if payment.RowsAffected != 1 {
			return fmt.Errorf("灵石不足")
		}
		return upsertPlayerValueTx(tx, player.ID, pendingSpiritualRootKey, string(encoded), nil)
	}); err != nil {
		return GameResult{}, true, err
	}
	bonus := g.spiritualRootBonuses(resultRoot.Name, quality)
	content := fmt.Sprintf("重铸道纹：%s × %s\n━━━━━━━━━━━\n随机凝成：%s\n品阶：%s · 本源：%s · 纯度%d\n修炼加成：+%s\n主加成：%s\n副加成：%s\n运气：%d/%d（已参与稀有灵根权重判定）\n━━━━━━━━━━━\n消耗：灵根精粹×2、阵基石×1、灵石×500\n此道种不会与当前灵根叠加属性。当前灵根尚未改变；发送“吸收灵根”才会替换，发送“放弃灵根”会销毁道种且材料不返还。", first.Name, second.Name, resultRoot.Name, resultRoot.Grade, resultRoot.Element, quality, bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, normalizedPlayerLuck(player.Luck), maximumPlayerLuck)
	return GameResult{Title: "🌱 替换型灵根道种已凝成", Content: content, ImageURL: resultRoot.ImageURL, Actions: []string{"吸收灵根", "放弃灵根", "灵根道种", "灵根详情 " + resultRoot.Name, "状态"}}, true, nil
}

func (g *Game) transferSpiritualRoot(player *model.Player, arguments []string) (GameResult, bool, error) {
	if len(arguments) == 0 {
		return GameResult{Title: "🌱 灵根传承", Content: "将自身当前真实灵根凝成一枚传承道种，传承者不会失去原灵根。\n格式：`灵传 @对方`\n消耗：灵根精粹×1、灵石×300\n承接者收到道种后，需自行选择“吸收灵根”或“放弃灵根”。", Actions: []string{"灵检", "物品 灵根精粹", "合成 太初灵根精粹", "灵根进化菜单"}}, true, nil
	}
	target, err := g.findPlayer(arguments[0])
	if err != nil {
		return GameResult{Title: "🌱 灵根传承失败", Content: "没有找到承接者。请使用对方的全服唯一道号或直接@对方。", Actions: []string{"灵根进化菜单", "灵检"}}, true, nil
	}
	if target.ID == player.ID {
		return GameResult{Title: "🌱 灵根传承失败", Content: "不能把自身灵根传承给自己。本次没有扣除材料或灵石。", Actions: []string{"灵根合成", "灵检"}}, true, nil
	}
	if _, err := g.playerValue(target.ID, pendingSpiritualRootKey); err == nil {
		return GameResult{Title: "🌱 对方已有待定道种", Content: fmt.Sprintf("%s的识海中已有一枚尚未处理的灵根道种。必须由对方先吸收或放弃，系统不会覆盖原结果。\n━━━━━━━━━━━\n本次没有扣除灵根精粹、灵石或传承次数。", target.DaoName), Actions: []string{"灵根道种", "吸收灵根", "放弃灵根", "灵检"}}, true, nil
	}
	var root model.SpiritualRootTemplate
	if err := g.store.DB.Where("name = ?", player.SpiritualRoot).First(&root).Error; err != nil {
		return GameResult{Title: "🌱 当前灵根道藏失联", Content: fmt.Sprintf("当前灵根“%s”尚未收录于灵根图鉴，无法安全凝成传承道种。请先联系主人修复图鉴；本次没有扣除任何资源。", player.SpiritualRoot), Actions: []string{"灵检", "灵根图鉴", "反馈菜单"}}, true, nil
	}
	essence, err := g.itemByName("灵根精粹")
	if err != nil {
		return GameResult{}, true, fmt.Errorf("灵根精粹未载入: %w", err)
	}
	owned := g.itemQuantity(player.ID, essence.ID)
	if owned < 1 || player.SpiritStones < 300 {
		return GameResult{Title: "🌱 传承材料不足", Content: fmt.Sprintf("需要：灵根精粹×1、灵石×300\n持有：灵根精粹×%d、灵石×%d\n━━━━━━━━━━━\n灵根精粹可通过“合成 太初灵根精粹”获得，点击物品可查看完整地图、副本与配方来源。", owned, player.SpiritStones), Actions: []string{"物品 灵根精粹", "合成 太初灵根精粹", "配方 太初灵根精粹", "背包"}}, true, nil
	}
	stage := spiritualRootStage(g.spiritualRootEvolutionValue(player.ID, "evolve"))
	pending := spiritualRootFusionResult{
		Mode: "transfer", SourcePlayer: player.DaoName, SourceStage: stage,
		ParentA: player.SpiritualRoot, Result: player.SpiritualRoot,
		Quality: minInt(maxInt(player.RootQuality, 35), 100), CreatedAt: time.Now(),
	}
	encoded, _ := json.Marshal(pending)
	var transferCount int64
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", target.ID, pendingSpiritualRootKey).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return fmt.Errorf("target pending spiritual root already exists")
		}
		if err := consumeNamedItemTx(tx, player.ID, "灵根精粹", 1); err != nil {
			return err
		}
		payment := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, 300).Update("spirit_stones", gorm.Expr("spirit_stones - ?", 300))
		if payment.Error != nil {
			return payment.Error
		}
		if payment.RowsAffected != 1 {
			return fmt.Errorf("灵石不足")
		}
		if err := tx.Create(&model.PlayerValue{PlayerID: target.ID, Key: pendingSpiritualRootKey, Value: string(encoded)}).Error; err != nil {
			return err
		}
		transferCount = playerValueIntTx(tx, player.ID, "spiritual_root.transfer.count", 0) + 1
		return upsertPlayerValueTx(tx, player.ID, "spiritual_root.transfer.count", fmt.Sprintf("%d", transferCount), nil)
	})
	if err != nil {
		if strings.Contains(err.Error(), "pending spiritual root") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return GameResult{Title: "🌱 对方已有待定道种", Content: "对方的识海刚刚收到了另一枚道种，本次事务已回滚，没有扣除材料或灵石。", Actions: []string{"灵检", "背包"}}, true, nil
		}
		return GameResult{}, true, err
	}
	bonus := g.spiritualRootBonuses(root.Name, pending.Quality)
	content := fmt.Sprintf("传承者：%s\n承接者：%s\n━━━━━━━━━━━\n传承灵根：%s\n本源道阶：%s（第%d/10重）\n传承纯度：%d\n修炼加成：+%s\n主加成：%s\n副加成：%s\n━━━━━━━━━━━\n实际消耗：灵根精粹×1、灵石×300\n累计传承：%d次\n传承者的原灵根、纯度和本源进度均未减少。承接者必须自行确认吸收；传承道阶会化为本源感悟，但不会让对方跳过自身境界、觉醒与进化天关。", player.DaoName, target.DaoName, root.Name, spiritualRootStageName(stage), stage, pending.Quality, bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, transferCount)
	return GameResult{Title: "🌱 灵根传承道种已送达", Content: content, ImageURL: root.ImageURL, Actions: []string{"灵根道种", "吸收灵根", "放弃灵根", "灵根详情 " + root.Name, "灵检"}}, true, nil
}

func (g *Game) randomFusionRoot(parentA, parentB, current string, luck int64) (model.SpiritualRootTemplate, error) {
	var rows []model.SpiritualRootTemplate
	if err := g.store.DB.Where("enabled = ? AND name NOT IN ?", true, []string{parentA, parentB, current}).Find(&rows).Error; err != nil {
		return model.SpiritualRootTemplate{}, err
	}
	if len(rows) == 0 {
		return model.SpiritualRootTemplate{}, fmt.Errorf("图鉴中没有可用的第三种灵根")
	}
	// Higher luck gently improves rare-root odds without making top roots
	// deterministic. RarityWeight is an inverse rarity score in this catalogue.
	total := 0
	weights := make([]int, len(rows))
	for index, row := range rows {
		weight := maxInt(row.RarityWeight, 1)
		rarityBoost := int(clampInt64(luck, 10, 50)-10) * maxInt(101-weight, 1) / 100
		weights[index] = weight + rarityBoost
		total += weights[index]
	}
	roll := rand.Intn(maxInt(total, 1))
	for index, row := range rows {
		roll -= weights[index]
		if roll < 0 {
			return row, nil
		}
	}
	return rows[len(rows)-1], nil
}

func (g *Game) pendingSpiritualRoot(player *model.Player) (GameResult, bool, error) {
	pending, err := g.loadPendingSpiritualRoot(player.ID)
	if err != nil {
		return GameResult{Title: "🌱 暂无合成道种", Content: "识海中没有待处理的随机灵根。选择两条不同图鉴灵根发送“灵根合成 灵根A 灵根B”。", Actions: []string{"灵根合成", "灵根图鉴", "合成 太初灵根精粹"}}, true, nil
	}
	var root model.SpiritualRootTemplate
	if g.store.DB.Where("name = ? AND enabled = ?", pending.Result, true).First(&root).Error != nil {
		return GameResult{Title: "🌱 道种图鉴失联", Content: "合成结果已不在启用图鉴中，可安全放弃后重新合成。", Actions: []string{"放弃灵根", "灵根图鉴"}}, true, nil
	}
	bonus := g.spiritualRootBonuses(root.Name, pending.Quality)
	if pending.Mode == "transfer" {
		return GameResult{Title: "🌱 待承接灵根道种", Content: fmt.Sprintf("传承者：%s\n传承灵根：%s · %s · 纯度%d\n传承道阶：%s（第%d/10重）\n修炼加成：+%s\n主加成：%s\n副加成：%s\n传承时间：%s\n━━━━━━━━━━━\n吸收会替换当前灵根并重算基础属性，另获得与传承道阶对应的本源感悟；不会复制对方境界、觉醒次数或跳过自身天关。", displayOr(pending.SourcePlayer, "未知道友"), root.Name, root.Grade, pending.Quality, spiritualRootStageName(maxInt(pending.SourceStage, 1)), maxInt(pending.SourceStage, 1), bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, pending.CreatedAt.Format("01-02 15:04")), ImageURL: root.ImageURL, Actions: []string{"吸收灵根", "放弃灵根", "灵根详情 " + root.Name, "灵检"}}, true, nil
	}
	return GameResult{Title: "🌱 待吸收替换型灵根道种", Content: fmt.Sprintf("父系：%s × %s\n结果：%s · %s · 纯度%d\n修炼加成：+%s\n主加成：%s\n副加成：%s\n凝成时间：%s\n━━━━━━━━━━━\n这是替换型道种，不会与当前灵根叠加属性。\n发送“吸收灵根”会替换当前灵根并按新灵根重算基础属性；已有境界、进化永久成长、物品与技能不会清空。\n发送“放弃灵根”会销毁道种，已经消耗的灵根精粹、阵基石与灵石不会返还。", pending.ParentA, pending.ParentB, root.Name, root.Grade, pending.Quality, bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, pending.CreatedAt.Format("01-02 15:04")), ImageURL: root.ImageURL, Actions: []string{"吸收灵根", "放弃灵根", "灵根详情 " + root.Name, "灵检"}}, true, nil
}

func (g *Game) absorbFusedSpiritualRoot(player *model.Player) (GameResult, bool, error) {
	pending, err := g.loadPendingSpiritualRoot(player.ID)
	if err != nil {
		return GameResult{Title: "🌱 没有可吸收灵根", Content: "请先完成一次灵根合成。", Actions: []string{"灵根合成", "灵根图鉴"}}, true, nil
	}
	var root model.SpiritualRootTemplate
	if g.store.DB.Where("name = ? AND enabled = ?", pending.Result, true).First(&root).Error != nil {
		return GameResult{Title: "🌱 灵根已失效", Content: "该道种对应图鉴已停用，请放弃后重新合成。", Actions: []string{"放弃灵根"}}, true, nil
	}
	previousName, previousQuality := player.SpiritualRoot, player.RootQuality
	result := g.rebalanceCustomizedRoot(*player, root.Name)
	result = g.rebalanceRootQuality(result, pending.Quality)
	skillBonus := g.activeSkillStatBonus(player)
	result.Health = rebalanceCustomizedCurrent(player.Health, max64(player.MaxHealth+skillBonus.Health, 1), max64(result.MaxHealth+skillBonus.Health, 1))
	result.Mana = rebalanceCustomizedCurrent(player.Mana, max64(player.MaxMana+skillBonus.Mana, 0), max64(result.MaxMana+skillBonus.Mana, 0))
	result.CombatPower = calculateCombatPower(result)
	updates := map[string]any{
		"spiritual_root": result.SpiritualRoot, "root_quality": result.RootQuality,
		"health": result.Health, "max_health": result.MaxHealth, "mana": result.Mana, "max_mana": result.MaxMana,
		"physical_attack": result.PhysicalAttack, "magic_attack": result.MagicAttack,
		"physical_defense": result.PhysicalDefense, "magic_defense": result.MagicDefense,
		"agility": result.Agility, "crit_rate": result.CritRate, "crit_damage": result.CritDamage,
		"damage_reduction": result.DamageReduction, "combat_power": result.CombatPower,
	}
	inheritedInsight := int64(0)
	if pending.Mode == "transfer" {
		inheritedInsight = int64(maxInt(pending.SourceStage, 1) * 10)
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
			return err
		}
		if inheritedInsight > 0 {
			value := playerValueIntTx(tx, player.ID, "spiritual_root.origin_power", 0) + inheritedInsight
			if err := upsertPlayerValueTx(tx, player.ID, "spiritual_root.origin_power", fmt.Sprintf("%d", value), nil); err != nil {
				return err
			}
		}
		return tx.Where("player_id = ? AND key = ?", player.ID, pendingSpiritualRootKey).Delete(&model.PlayerValue{}).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	broadcast := fmt.Sprintf("【灵根合道】%s将%s与%s两道本源合炼，最终吸收%s，灵根由%s蜕变为%s。", player.DaoName, pending.ParentA, pending.ParentB, root.Name, previousName, root.Name)
	resultTitle := "🌱 新灵根吸收完成"
	originLine := ""
	if pending.Mode == "transfer" {
		broadcast = fmt.Sprintf("【灵根传承】%s承接%s留下的%s道种，灵根由%s蜕变为%s，本源道脉由此续接。", player.DaoName, displayOr(pending.SourcePlayer, "前辈"), root.Name, previousName, root.Name)
		resultTitle = "🌱 传承灵根吸收完成"
		originLine = fmt.Sprintf("\n传承本源感悟：+%d", inheritedInsight)
	}
	_ = g.publishWorldBroadcast("灵根", player.DaoName+"合成新灵根", broadcast)
	return GameResult{Title: resultTitle, Content: fmt.Sprintf("原灵根：%s · 纯度%d\n新灵根：%s · 纯度%d%s\n━━━━━━━━━━━\n气血上限：%d → %d\n法力上限：%d → %d\n攻击：%d/%d → %d/%d\n防御：%d/%d → %d/%d\n基础战力：%d → %d\n━━━━━━━━━━━\n境界、已有进化永久成长、功法、装备和灵兽均已保留。", previousName, previousQuality, root.Name, result.RootQuality, originLine, player.MaxHealth, result.MaxHealth, player.MaxMana, result.MaxMana, player.PhysicalAttack, player.MagicAttack, result.PhysicalAttack, result.MagicAttack, player.PhysicalDefense, player.MagicDefense, result.PhysicalDefense, result.MagicDefense, player.CombatPower, result.CombatPower), Actions: []string{"灵检", "状态", "灵根详情 " + root.Name, "灵进", "灵根合成"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) discardFusedSpiritualRoot(player *model.Player) (GameResult, bool, error) {
	pending, err := g.loadPendingSpiritualRoot(player.ID)
	if err != nil {
		return GameResult{Title: "🌱 暂无道种", Content: "没有需要放弃的合成灵根。", Actions: []string{"灵根合成", "灵根图鉴"}}, true, nil
	}
	if err := g.store.DB.Where("player_id = ? AND key = ?", player.ID, pendingSpiritualRootKey).Delete(&model.PlayerValue{}).Error; err != nil {
		return GameResult{}, true, err
	}
	title := "🌱 合成道种已散去"
	source := "合成消耗的材料不会返还。"
	if pending.Mode == "transfer" {
		title = "🌱 传承道种已散去"
		source = fmt.Sprintf("来自%s的传承已经婉拒；传承者不会失去原灵根，已消耗的传承材料不返还。", displayOr(pending.SourcePlayer, "前辈"))
	}
	return GameResult{Title: title, Content: fmt.Sprintf("已放弃：%s（纯度%d）\n当前灵根保持%s不变。\n%s", pending.Result, pending.Quality, player.SpiritualRoot, source), Actions: []string{"灵根合成", "灵根图鉴", "灵检"}}, true, nil
}

func (g *Game) loadPendingSpiritualRoot(playerID uint) (spiritualRootFusionResult, error) {
	value, err := g.playerValue(playerID, pendingSpiritualRootKey)
	if err != nil {
		return spiritualRootFusionResult{}, err
	}
	var pending spiritualRootFusionResult
	if json.Unmarshal([]byte(value), &pending) != nil || pending.Result == "" {
		return spiritualRootFusionResult{}, gorm.ErrRecordNotFound
	}
	return pending, nil
}
