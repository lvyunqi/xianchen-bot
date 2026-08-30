package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeBattlefieldExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	switch action {
	case "enter":
		return g.enterImmortalDemonBattlefield(player, command, system)
	case "choose":
		return g.chooseBattlefieldFaction(player, command.RawArguments)
	case "battle":
		return g.beginBattlefieldBattle(player, command, system)
	case "task":
		return g.completeBattlefieldTask(player)
	case "ranking":
		return g.battlefieldRanking(player, command.RawArguments)
	case "shop":
		return g.battlefieldShop(player, command.RawArguments)
	default:
		return GameResult{}, false, fmt.Errorf("未知战场动作: %s", action)
	}
}

func (g *Game) enterImmortalDemonBattlefield(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	config, result, ok, err := g.resolveExtendedRuntimeConfig(player, command, system, "enter")
	if err != nil || !ok {
		return result, true, err
	}
	requirement, unmet, err := g.prerequisiteStatus(player, config.Prerequisite)
	if err != nil {
		return GameResult{Title: "战场道纹错误", Content: "战区前置无法解析，本次没有扣除资源。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: "战区尚未开放", Content: strings.Join(unmet, "\n"), Actions: append(g.prerequisiteActions(unmet), "战场")}, true, nil
	}
	effect := decodeExtendedEffect(config)
	var progress model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙魔战场", config.Code).First(&progress).Error != nil {
		progress = model.PlayerExtendedProgress{PlayerID: player.ID, System: "仙魔战场", ConfigCode: config.Code, ConfigName: config.Name, State: "已经入场", Level: maxInt(config.Level, 1), Power: effect.Power, MetadataJSON: `{}`}
	} else {
		progress.State = "已经入场"
		progress.Uses++
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, "battlefield.active", config.Code, nil)
	}); err != nil {
		return GameResult{}, true, err
	}
	faction, _ := g.playerValue(player.ID, "battlefield.faction")
	if faction == "" {
		faction = "尚未选择"
	}
	return GameResult{Title: "进入仙魔战场", Content: fmt.Sprintf("战区：%s\n类型：%s · 战区阶位%d\n阵营：%s\n战功：%d\n前置：%s\n━━━━━━━━━━━\n入场只登记战区，不会自动战斗。选择阵营后可逐回合出战。", config.Name, config.Type, config.Level, faction, g.playerValueInt(player.ID, "battlefield.contribution", 0), requirement), Actions: []string{"阵营 仙", "阵营 魔", "战战", "战务", "战榜", "战商"}}, true, nil
}

func normalizeBattlefieldFaction(raw string) string {
	value := strings.TrimSpace(raw)
	switch value {
	case "仙", "仙盟", "仙道":
		return "仙盟"
	case "魔", "魔域", "魔道":
		return "魔域"
	default:
		return ""
	}
}

func (g *Game) chooseBattlefieldFaction(player *model.Player, raw string) (GameResult, bool, error) {
	faction := normalizeBattlefieldFaction(raw)
	if faction == "" {
		return GameResult{Title: "选择战场阵营", Content: "请输入 `阵营 仙` 或 `阵营 魔`。", Actions: []string{"阵营 仙", "阵营 魔", "战场"}}, true, nil
	}
	if _, err := g.playerValue(player.ID, "battlefield.active"); err != nil {
		return GameResult{Title: "尚未进入战区", Content: "先发送 `战场` 登记当前可进入战区。", Actions: []string{"战场"}}, true, nil
	}
	current, _ := g.playerValue(player.ID, "battlefield.faction")
	if current == faction {
		return GameResult{Title: "阵营没有变化", Content: "你已经属于" + faction + "，不会重置战功。", Actions: []string{"战战", "战务", "战榜"}}, true, nil
	}
	if current != "" && g.playerValueInt(player.ID, "battlefield.contribution", 0) > 0 {
		return GameResult{Title: "阵营契约已定", Content: fmt.Sprintf("当前阵营：%s\n已有战功：%d\n本赛季不能携带战功改投另一阵营。", current, g.playerValueInt(player.ID, "battlefield.contribution", 0)), Actions: []string{"战战", "战务", "战榜"}}, true, nil
	}
	if err := g.setPlayerValue(player.ID, "battlefield.faction", faction, nil); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "战场阵营已定", Content: fmt.Sprintf("阵营：%s\n初始战功：0\n━━━━━━━━━━━\n战斗、战务、排行与战场商店现在均按此阵营和真实战功结算。", faction), Actions: []string{"战战", "战务", "战榜", "战商"}}, true, nil
}

func (g *Game) activeBattlefieldConfig(playerID uint, system extendedSystem) (model.GameplayConfigBase, model.PlayerExtendedProgress, error) {
	code, err := g.playerValue(playerID, "battlefield.active")
	if err != nil {
		return model.GameplayConfigBase{}, model.PlayerExtendedProgress{}, err
	}
	config, err := g.extendedConfig(system.Table, code)
	if err != nil {
		return config, model.PlayerExtendedProgress{}, err
	}
	var progress model.PlayerExtendedProgress
	err = g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", playerID, "仙魔战场", config.Code).First(&progress).Error
	return config, progress, err
}

func (g *Game) beginBattlefieldBattle(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	faction, _ := g.playerValue(player.ID, "battlefield.faction")
	if faction == "" {
		return GameResult{Title: "尚未选择阵营", Content: "先选择仙盟或魔域。", Actions: []string{"阵营 仙", "阵营 魔", "战场"}}, true, nil
	}
	config, _, err := g.activeBattlefieldConfig(player.ID, system)
	if err != nil {
		return GameResult{Title: "尚未进入战区", Content: "先发送 `战场`。", Actions: []string{"战场"}}, true, nil
	}
	effect := decodeExtendedEffect(config)
	costText, missing, err := g.extendedCostStatus(player, config.CostMaterials)
	if err != nil || len(missing) > 0 {
		return GameResult{Title: "出征物资不足", Content: fmt.Sprintf("战区：%s\n需要：%s\n缺少：%s", config.Name, costText, strings.Join(missing, "、")), Actions: []string{"背包", "战商", "地图", "副本"}}, true, nil
	}
	return g.beginExtendedRuntimeBattle(player, command, system, "battle", config, effect, costText)
}

func (g *Game) completeBattlefieldTask(player *model.Player) (GameResult, bool, error) {
	faction, _ := g.playerValue(player.ID, "battlefield.faction")
	if faction == "" {
		return GameResult{Title: "战务尚未开放", Content: "先进入战场并选择阵营。", Actions: []string{"战场", "阵营 仙", "阵营 魔"}}, true, nil
	}
	today := time.Now().Format("2006-01-02")
	if value, _ := g.playerValue(player.ID, "battlefield.task.date"); value == today {
		return GameResult{Title: "今日战务已结", Content: fmt.Sprintf("阵营：%s\n当前战功：%d\n每日战务只结算一次。", faction, g.playerValueInt(player.ID, "battlefield.contribution", 0)), Actions: []string{"战战", "战榜", "战商"}}, true, nil
	}
	reward := int64(20 + normalizedPlayerLuck(player.Luck)/5)
	current := g.playerValueInt(player.ID, "battlefield.contribution", 0)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"silver_coins": gorm.Expr("silver_coins + 80"), "merit": gorm.Expr("merit + 2")}).Error; err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "battlefield.contribution", strconv.FormatInt(current+reward, 10), nil); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, "battlefield.task.date", today, nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "战场军务完成", Content: fmt.Sprintf("阵营：%s\n巡查阵线、补全阵纹并护送战备完成。\n战功+%d · 银币+80 · 功德+2\n当前战功：%d", faction, reward, current+reward), Actions: []string{"战战", "战榜", "战商", "钱庄"}}, true, nil
}

func (g *Game) finishBattlefieldExtendedBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	remainingHP, remainingMana := max64(state.PlayerHP, 1), max64(state.PlayerMana, 0)
	system := extendedSystems["仙魔战场"]
	config, err := g.extendedConfig(system.Table, state.ExtendedConfigCode)
	if err != nil {
		return GameResult{}, true, err
	}
	var progress model.PlayerExtendedProgress
	if err := g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙魔战场", config.Code).First(&progress).Error; err != nil {
		return GameResult{}, true, err
	}
	contribution := int64(3)
	coins, cultivation := int64(5), max64(state.EnemyPower/4, 20)
	stateText := "战场失利"
	if state.Surrendered {
		contribution, coins, cultivation = 0, 0, 0
		stateText = "主动撤离战场"
	} else if won {
		contribution = max64(state.EnemyPower/10, 15)
		coins = max64(state.EnemyPower/20, 10)
		cultivation = max64(state.EnemyPower, 80)
		stateText = "战场凯旋"
	}
	current := g.playerValueInt(player.ID, "battlefield.contribution", 0)
	progress.State, progress.Uses, progress.Mastery, progress.Experience = stateText, progress.Uses+1, progress.Mastery+contribution, progress.Experience+cultivation
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key IN ?", player.ID, []string{"pve.battle", "extended.pending.system"}).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": remainingMana, "state": model.PlayerStateIdle, "cultivation": gorm.Expr("cultivation + ?", cultivation), "arena_coins": gorm.Expr("arena_coins + ?", coins)}).Error; err != nil {
			return err
		}
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, "battlefield.contribution", strconv.FormatInt(current+contribution, 10), nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	faction, _ := g.playerValue(player.ID, "battlefield.faction")
	return GameResult{Title: stateText, Content: fmt.Sprintf("战区：%s · 阵营：%s\n%s\n━━━━━━━━━━━\n战功+%d · 竞技币+%d · 修为+%d\n当前战功：%d\n剩余气血：%d/%d · 法力：%d/%d", config.Name, faction, logLine, contribution, coins, cultivation, current+contribution, remainingHP, effective.MaxHealth, remainingMana, effective.MaxMana), Actions: []string{"战战", "战务", "战榜", "战商", "状态"}}, true, nil
}

func (g *Game) battlefieldRanking(player *model.Player, raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 10
	type row struct {
		PlayerID     uint
		DaoName      string
		Contribution int64
		Faction      string
	}
	var rows []row
	err := g.store.DB.Raw(`SELECT players.id AS player_id,players.dao_name,CAST(points.value AS INTEGER) AS contribution,COALESCE(faction.value,'未定') AS faction
		FROM player_values points JOIN players ON players.id=points.player_id
		LEFT JOIN player_values faction ON faction.player_id=players.id AND faction.key='battlefield.faction'
		WHERE points.key='battlefield.contribution' AND CAST(points.value AS INTEGER)>0 AND players.deleted_at IS NULL
		ORDER BY contribution DESC,players.id`).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start, end := minInt((page-1)*pageSize, len(rows)), minInt(page*pageSize, len(rows))
	lines := []string{fmt.Sprintf("第%d/%d页 · 按真实累计战功排序", page, pages), "━━━━━━━━━━━"}
	for index, row := range rows[start:end] {
		lines = append(lines, fmt.Sprintf("%d. %s【%s】· 战功%d", start+index+1, row.DaoName, row.Faction, row.Contribution))
	}
	if len(rows) == 0 {
		lines = append(lines, "尚无人获得战功。")
	}
	actions := []string{"战场", "战战", "战务", "战商"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("战榜 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("战榜 %d", page+1))
	}
	return GameResult{Title: "仙魔战功榜", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

var battlefieldGoods = []struct {
	Name string
	Cost int64
}{{"回灵丹", 30}, {"仙露", 25}, {"玄铁", 45}, {"星辰砂", 80}}

func (g *Game) battlefieldShop(player *model.Player, raw string) (GameResult, bool, error) {
	name, quantity, parseErr := parseStackQuantity(strings.TrimSpace(raw))
	points := g.playerValueInt(player.ID, "battlefield.contribution", 0)
	if strings.TrimSpace(raw) == "" || parseErr != nil {
		lines := []string{fmt.Sprintf("当前战功：%d", points), "战场物资会真实进入乾坤袋，支持 `战商 物品名*数量`。", "━━━━━━━━━━━"}
		actions := []string{"战战", "战务", "战榜"}
		for _, good := range battlefieldGoods {
			lines = append(lines, fmt.Sprintf("- %s · %d战功", good.Name, good.Cost))
			actions = append(actions, "战商 "+good.Name, "物品 "+good.Name)
		}
		return GameResult{Title: "仙魔战场军需", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	cost := int64(0)
	for _, good := range battlefieldGoods {
		if good.Name == name {
			cost = good.Cost
			break
		}
	}
	if cost == 0 || quantity <= 0 {
		return GameResult{Title: "战场物资未上架", Content: "请从战商蓝字选择有效物资。", Actions: []string{"战商"}}, true, nil
	}
	total := cost * quantity
	if total < 0 || points < total {
		return GameResult{Title: "战功不足", Content: fmt.Sprintf("兑换%s×%d需要%d战功，当前%d。", name, quantity, total, points), Actions: []string{"战战", "战务", "战榜"}}, true, nil
	}
	item, err := g.itemByName(name)
	if err != nil {
		return GameResult{Title: "战场物资异常", Content: "物品道藏尚未载入，本次没有扣除战功。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		current := playerValueIntTx(tx, player.ID, "battlefield.contribution", 0)
		if current < total {
			return errors.New("战功不足")
		}
		if err := upsertPlayerValueTx(tx, player.ID, "battlefield.contribution", strconv.FormatInt(current-total, 10), nil); err != nil {
			return err
		}
		return storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "战场物资兑换", Content: fmt.Sprintf("获得：%s×%d\n消耗战功：%d\n剩余战功：%d\n物资已进入乾坤袋。", name, quantity, total, points-total), Actions: []string{"背包", "物品 " + name, "战商", "战榜"}}, true, nil
}
