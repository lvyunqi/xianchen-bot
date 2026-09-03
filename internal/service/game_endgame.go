package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeDungeon(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 121:
		return g.dungeonList(player)
	case 122:
		return g.runDungeon(player, command.RawArguments, false)
	case 123:
		if player.CoupleID == 0 {
			return GameResult{Title: "组队失败", Content: "需要仙侣或好友组队。", Actions: []string{"寻缘"}}, true, nil
		}
		return g.runDungeon(player, command.RawArguments, true)
	case 124:
		return g.sweepDungeon(player, command.RawArguments)
	case 125:
		return g.dungeonRanking()
	case 126:
		return g.resetDungeon(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) dungeonByName(name string) (model.Dungeon, error) {
	var row model.Dungeon
	err := g.store.DB.Where("name = ? AND enabled = ?", strings.TrimSpace(name), true).First(&row).Error
	return row, err
}

func (g *Game) dungeonList(player *model.Player) (GameResult, bool, error) {
	var rows []model.Dungeon
	if err := g.store.DB.Where("enabled = ?", true).Order("recommended_power,id").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	maximum, maximumErr := g.staminaMaximum(player.ID)
	if maximumErr != nil {
		return GameResult{}, true, maximumErr
	}
	recovery, recoveryErr := g.staminaRecoveryPerMinute(player.ID)
	if recoveryErr != nil {
		return GameResult{}, true, recoveryErr
	}
	lines := []string{"次数规则：普通20次 · 困难12次 · 噩梦8次 · 地狱5次。", fmt.Sprintf("体力规则：炼气期基础上限%d；每提升一个大境上限+%d、每分钟恢复+%d；你当前上限%d、每分钟自动恢复%d点，无需打坐；普通副本每次只消耗3点。", g.settingInt("player.daily_stamina", 100), g.settingInt("player.stamina_growth_per_realm", 100), g.settingInt("player.stamina_recovery_growth_per_realm", 10), maximum, recovery), "━━━━━━━━━━━"}
	actions := make([]string, 0, len(rows))
	if len(rows) == 0 {
		return GameResult{Title: "副本列表", Content: "当前没有开放副本，请主人检查副本数据；恢复开放后可立即查看。", Actions: []string{"功能菜单", "地图", "帮助"}}, true, nil
	}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("- %s【%s】· 推荐战力%d · 体力%d · 每日%d次", row.Name, row.Difficulty, row.RecommendedPower, row.StaminaCost, row.DailyLimit))
		actions = append(actions, "进入 "+row.Name)
	}
	return GameResult{Title: "副本列表", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) runDungeon(player *model.Player, name string, team bool) (GameResult, bool, error) {
	dungeon, err := g.dungeonByName(name)
	if err != nil {
		return GameResult{Title: "进入副本", Content: "副本不存在。发送 `副本` 查看列表。"}, true, nil
	}
	date := time.Now().Format("2006-01-02")
	var count int64
	g.store.DB.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND run_date = ?", player.ID, dungeon.ID, date).Count(&count)
	bonusUses := g.playerValueInt(player.ID, "dungeon.reset."+date, 0)
	if count >= int64(dungeon.DailyLimit)+bonusUses {
		return GameResult{Title: "副本次数耗尽", Content: "可发送 `重副` 消耗灵石重置一次。"}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		if player.State == model.PlayerStateBattling {
			return GameResult{Title: "已在战斗中", Content: "请完成当前回合或发送 `投降` 后再进入副本。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
		}
		return GameResult{Title: "当前无法进入", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	if player.Health <= 1 {
		return GameResult{Title: "重伤难行", Content: "当前气血过低，无法踏入副本。", Actions: []string{"疗伤", "状态"}}, true, nil
	}
	remaining, staminaErr := g.useStamina(player.ID, int64(dungeon.StaminaCost))
	if staminaErr != nil {
		return GameResult{Title: "无法进入", Content: staminaErr.Error()}, true, nil
	}
	enemyPower := max64(dungeon.RecommendedPower, 20)
	enemyName := dungeonGuardianName(dungeon)
	enemyHP := max64(enemyPower*3, 80)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{
		DungeonID: dungeon.ID, BattleKind: "副本", Round: 1, EnemyName: enemyName,
		EnemyPower: enemyPower, PlayerHP: effective.Health, PlayerMana: effective.Mana,
		EnemyHP: enemyHP, EnemyMaxHP: enemyHP, Team: team, StartedAt: time.Now().UnixMilli(),
	}
	if err := g.beginPVEBattle(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	mode := "单人闯境"
	if team {
		mode = "仙侣组队 · 伤害提升50%"
	}
	content := fmt.Sprintf("道友「%s」踏入%s！\n━━━━━━━━━━━\n【敌方阵位】\nD1：%s【战力%d】\n气血：%d/%d\n\n【我方阵位】\nA1：%s【战力%d】\n气血：%d/%d · 法力：%d/%d\n\n模式：%s\n消耗体力：%d · 剩余体力：%d\n━━━━━━━━━━━\n轮到你行动：可普通攻击、施展已学功法、防御或投降。\n守境者低于35%%气血后可能狂暴。", player.DaoName, dungeon.Name, enemyName, enemyPower, enemyHP, enemyHP, player.DaoName, player.CombatPower, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana, mode, dungeon.StaminaCost, remaining)
	return GameResult{Title: "副本挑战开始", Content: content, ImageURL: dungeon.ImageURL, Actions: []string{"技能", "攻击", "防御", "投降", "功法", "状态"}}, true, nil
}

func dungeonGuardianName(dungeon model.Dungeon) string {
	guardians := map[string]string{"普通": "守境灵将", "困难": "镇关妖帅", "噩梦": "噬界魔君", "地狱": "劫狱道主"}
	return dungeon.Name + "·" + displayOr(guardians[dungeon.Difficulty], "镇境守卫")
}

func (g *Game) finishDungeonBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	var dungeon model.Dungeon
	if err := g.store.DB.First(&dungeon, state.DungeonID).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.clearMapMonsterBattle(player.ID)
	_, _ = g.addPlayerValueInt(player.ID, "stats.battles", 1)
	remainingHP := max64(state.PlayerHP, 1)
	_ = g.store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": max64(state.PlayerMana, 0)}).Error
	duration := max64(time.Now().UnixMilli()-state.StartedAt, 1000)
	mode := "单人闯境"
	if state.Team {
		mode = "仙侣组队"
	}
	if !won {
		run := model.DungeonRun{PlayerID: player.ID, DungeonID: dungeon.ID, RunDate: time.Now().Format("2006-01-02"), DurationMS: duration, Success: false}
		if err := g.store.DB.Create(&run).Error; err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "副本挑战失败", Content: fmt.Sprintf("副本：%s · %s\n%s\n━━━━━━━━━━━\n我方已经失去战斗能力。\n剩余气血：%d/%d\n本次没有获得战利，副本次数已结算。", dungeon.Name, mode, logLine, remainingHP, effective.MaxHealth), Actions: []string{"回城复活", "疗伤", "状态", "副本"}}, true, nil
	}
	var reward map[string]any
	if json.Unmarshal([]byte(dungeon.RewardPoolJSON), &reward) != nil {
		reward = make(map[string]any)
	}
	if itemName, ok := reward["item"].(string); ok && strings.TrimSpace(itemName) != "" {
		reward["items"] = map[string]any{itemName: float64(1)}
		delete(reward, "item")
	}
	if itemList, ok := reward["items"].([]any); ok {
		items := make(map[string]any)
		for _, raw := range itemList {
			if name, ok := raw.(string); ok && strings.TrimSpace(name) != "" {
				items[name] = float64(1)
			}
		}
		reward["items"] = items
	}
	if rewardNumber(reward, "cultivation") <= 0 {
		reward["cultivation"] = float64(max64(dungeon.RecommendedPower/2, 50))
	}
	if rewardNumber(reward, "spirit_stones") <= 0 {
		reward["spirit_stones"] = float64(max64(dungeon.RecommendedPower/10, 10))
	}
	if err := g.applyConfiguredEventReward(player, reward); err != nil {
		return GameResult{}, true, err
	}
	score := max64(player.CombatPower*1000/int64(maxInt(state.Round, 1)), 1)
	run := model.DungeonRun{PlayerID: player.ID, DungeonID: dungeon.ID, RunDate: time.Now().Format("2006-01-02"), DurationMS: duration, Score: score, Success: true}
	if err := g.store.DB.Create(&run).Error; err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.dungeons", 1)
	_, _ = g.addPlayerValueInt(player.ID, "stats.wins", 1)
	content := fmt.Sprintf("副本：%s · %s\n%s\n━━━━━━━━━━━\n【战果展示】\n通关回合：%d · 用时：%.1f秒 · 评分：%d\n获得：%s\n\n副本奖励已经实际写入道籍与乾坤袋，首次通关后可使用扫荡券。", dungeon.Name, mode, logLine, state.Round, float64(duration)/1000, score, eventRewardText(reward))
	return GameResult{Title: "副本通关", Content: content, ImageURL: dungeon.ImageURL, Actions: []string{"扫荡 " + dungeon.Name, "背包", "副榜", "副本", "状态"}}, true, nil
}

var (
	errDungeonDailyLimit        = errors.New("dungeon daily limit reached")
	errDungeonStaminaChanged    = errors.New("dungeon stamina insufficient")
	errDungeonManualClearNeeded = errors.New("dungeon manual clear required")
	errDungeonInventoryChanged  = errors.New("dungeon sweep inventory changed")
)

func dungeonRemainingRunsTx(tx *gorm.DB, playerID uint, dungeon model.Dungeon, date string) (int64, int64, error) {
	var used int64
	if err := tx.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND run_date = ?", playerID, dungeon.ID, date).Count(&used).Error; err != nil {
		return 0, 0, err
	}
	bonusUses := playerValueIntTx(tx, playerID, "dungeon.reset."+date, 0)
	maximum := int64(dungeon.DailyLimit) + max64(bonusUses, 0)
	return max64(maximum-used, 0), maximum, nil
}

func consumeStaminaTx(tx *gorm.DB, playerID uint, refreshed, cost int64) (int64, error) {
	var row model.PlayerValue
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", playerID, "stamina.value").First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return refreshed, err
	}
	current := refreshed
	if err == nil {
		if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(row.Value), 10, 64); parseErr == nil {
			current = parsed
		}
	}
	if current < cost {
		return current, errDungeonStaminaChanged
	}
	remaining := current - cost
	return remaining, upsertPlayerValueTx(tx, playerID, "stamina.value", strconv.FormatInt(remaining, 10), nil)
}

func (g *Game) sweepDungeon(player *model.Player, raw string) (GameResult, bool, error) {
	name, quantity, parseErr := parseShopPurchase(strings.Fields(strings.TrimSpace(raw)))
	if parseErr != nil {
		return GameResult{Title: "扫荡数量错误", Content: "请输入：`扫荡 副本名*次数`。" + parseErr.Error(), Actions: []string{"副本"}}, true, nil
	}
	dungeon, err := g.dungeonByName(name)
	if err != nil {
		return GameResult{Title: "扫荡失败", Content: "副本不存在。", Actions: []string{"副本"}}, true, nil
	}
	var clears int64
	if err := g.store.DB.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND success = ? AND duration_ms > ?", player.ID, dungeon.ID, true, 0).Count(&clears).Error; err != nil {
		return GameResult{}, true, err
	}
	if clears == 0 {
		return GameResult{Title: "扫荡未解锁", Content: "先手动逐回合通关该副本；挂机和其他扫荡记录不能代替首次通关。", Actions: []string{"进入 " + dungeon.Name, "副本"}}, true, nil
	}
	if dungeon.StaminaCost <= 0 || quantity > math.MaxInt64/int64(dungeon.StaminaCost) {
		return GameResult{Title: "扫荡数量过大", Content: "体力消耗超过安全计算范围，请拆分扫荡。", Actions: []string{"副本"}}, true, nil
	}
	refreshedStamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	today := time.Now().Format("2006-01-02")
	rewardPerRun := max64(dungeon.RecommendedPower/2, 50)
	if quantity > math.MaxInt64/rewardPerRun {
		return GameResult{Title: "扫荡数量过大", Content: "修为奖励超过安全计算范围，请拆分扫荡。", Actions: []string{"副本"}}, true, nil
	}
	staminaCost := int64(dungeon.StaminaCost) * quantity
	reward := rewardPerRun * quantity
	remainingStamina, remainingDaily, dailyMaximum := int64(0), int64(0), int64(0)
	var levelProgress model.PlayerLevelProgress
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var manualClears int64
		if err := tx.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND success = ? AND duration_ms > ?", player.ID, dungeon.ID, true, 0).Count(&manualClears).Error; err != nil {
			return err
		}
		if manualClears == 0 {
			return errDungeonManualClearNeeded
		}
		var err error
		remainingDaily, dailyMaximum, err = dungeonRemainingRunsTx(tx, player.ID, dungeon, today)
		if err != nil {
			return err
		}
		if quantity > remainingDaily {
			return errDungeonDailyLimit
		}
		var ticket model.Item
		if err := tx.Where("name = ?", "扫荡券").First(&ticket).Error; err != nil {
			return err
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, ticket.ID, -quantity); err != nil {
			return errInsufficientCurrency
		}
		remainingStamina, err = consumeStaminaTx(tx, player.ID, refreshedStamina, staminaCost)
		if err != nil {
			return err
		}
		levelProgress, err = grantCultivationExperienceTx(tx, player.ID, reward)
		if err != nil {
			return err
		}
		runs := make([]model.DungeonRun, 0, int(quantity))
		for index := int64(0); index < quantity; index++ {
			runs = append(runs, model.DungeonRun{PlayerID: player.ID, DungeonID: dungeon.ID, RunDate: today, DurationMS: 0, Score: player.CombatPower, Success: true})
		}
		if err := tx.Create(&runs).Error; err != nil {
			return err
		}
		_, err = addPlayerValueIntTx(tx, player.ID, "stats.dungeons", quantity)
		return err
	})
	if errors.Is(err, errDungeonManualClearNeeded) {
		return GameResult{Title: "扫荡未解锁", Content: "首次手动通关记录已经变化，本次没有扣除任何资源。", Actions: []string{"进入 " + dungeon.Name}}, true, nil
	}
	if errors.Is(err, errDungeonDailyLimit) {
		return GameResult{Title: "副本次数不足", Content: fmt.Sprintf("%s今日仅剩%d/%d次，无法一次扫荡%d次。本次没有扣除扫荡券或体力。", dungeon.Name, remainingDaily, dailyMaximum, quantity), Actions: []string{"副本", "重副"}}, true, nil
	}
	if errors.Is(err, errInsufficientCurrency) {
		return GameResult{Title: "扫荡券不足", Content: fmt.Sprintf("扫荡%s×%d需要扫荡券×%d，本次没有扣除体力或次数。", dungeon.Name, quantity, quantity), Actions: []string{"物品 扫荡券", "合成列表", "神秘商城", "限时商城"}}, true, nil
	}
	if errors.Is(err, errDungeonStaminaChanged) {
		return GameResult{Title: "体力不足", Content: fmt.Sprintf("扫荡%s×%d需要体力%d，当前%d。本次没有扣除扫荡券或次数。", dungeon.Name, quantity, staminaCost, remainingStamina), Actions: []string{"体力", "副本"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	result := GameResult{Title: "副本批量扫荡", Content: fmt.Sprintf("副本：%s\n本次扫荡：%d次\n消耗扫荡券：%d\n消耗体力：%d · 剩余%d\n修为：+%d\n今日剩余次数：%d/%d", dungeon.Name, quantity, quantity, staminaCost, remainingStamina, reward, remainingDaily-quantity, dailyMaximum), Actions: []string{"扫荡 " + dungeon.Name, "副本", "背包", "状态"}}
	appendPlayerLevelSettlement(&result, latest, levelProgress)
	return result, true, nil
}

func (g *Game) dungeonRanking() (GameResult, bool, error) {
	type row struct {
		DaoName     string
		DungeonName string
		Score       int64
		DurationMS  int64
	}
	var rows []row
	err := g.store.DB.Table("dungeon_runs").Select("players.dao_name, dungeons.name AS dungeon_name, dungeon_runs.score, dungeon_runs.duration_ms").Joins("JOIN players ON players.id = dungeon_runs.player_id").Joins("JOIN dungeons ON dungeons.id = dungeon_runs.dungeon_id").Where("dungeon_runs.success = ?", true).Order("dungeon_runs.score DESC").Limit(10).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf("%d. %s · %s · 评分%d", i+1, row.DaoName, row.DungeonName, row.Score))
	}
	if len(lines) == 0 {
		lines = append(lines, "暂无通关记录。")
	}
	return GameResult{Title: "副本排行", Content: strings.Join(lines, "\n")}, true, nil
}

func (g *Game) resetDungeon(player *model.Player) (GameResult, bool, error) {
	cost := int64(50)
	if player.SpiritStones < cost {
		return GameResult{Title: "重置失败", Content: "需要50灵石。"}, true, nil
	}
	date := time.Now().Format("2006-01-02")
	used := g.playerValueInt(player.ID, "dungeon.reset."+date, 0)
	if used >= 3 {
		return GameResult{Title: "重置上限", Content: "今日最多重置3次。"}, true, nil
	}
	if err := g.store.DB.Model(player).Update("spirit_stones", gorm.Expr("spirit_stones - ?", cost)).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.setPlayerValueInt(player.ID, "dungeon.reset."+date, used+1)
	return GameResult{Title: "副本重置", Content: fmt.Sprintf("消耗灵石：50\n今日额外次数：%d/3", used+1), Actions: []string{"副本"}}, true, nil
}

func (g *Game) executeArena(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 127:
		return g.arenaMatch(player)
	case 128:
		return g.arenaFight(player)
	case 129:
		return g.arenaRanking()
	case 130:
		return g.arenaStore(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) arenaRecord(playerID uint) (model.ArenaRecord, error) {
	row := model.ArenaRecord{PlayerID: playerID, Rating: 1000}
	err := g.store.DB.Where("player_id = ?", playerID).FirstOrCreate(&row).Error
	return row, err
}

func (g *Game) arenaMatch(player *model.Player) (GameResult, bool, error) {
	rangeFactor := g.settingFloat("arena.match_range", .2)
	minPower := int64(float64(player.CombatPower) * (1 - rangeFactor))
	maxPower := int64(float64(player.CombatPower) * (1 + rangeFactor))
	var candidates []model.Player
	_ = g.store.DB.Where("id <> ? AND combat_power BETWEEN ? AND ? AND banned = ?", player.ID, minPower, maxPower, false).Limit(20).Find(&candidates).Error
	if len(candidates) == 0 {
		_ = g.store.DB.Where("id <> ? AND banned = ?", player.ID, false).Order("ABS(combat_power - " + fmt.Sprint(player.CombatPower) + ")").Limit(5).Find(&candidates).Error
	}
	if len(candidates) == 0 {
		return GameResult{Title: "竞技匹配", Content: "暂无其他玩家可匹配。"}, true, nil
	}
	target := candidates[rand.Intn(len(candidates))]
	_ = g.setPlayerValueInt(player.ID, "arena.target", int64(target.ID))
	return GameResult{Title: "竞技匹配", Content: fmt.Sprintf("对手：%s\n境界：%s\n战力：%d\n你的战力：%d\n\n竞技已进入回合制：每回合选择普通攻击、功法技能或防御。", target.DaoName, target.RealmName, target.CombatPower, player.CombatPower), Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
}

func (g *Game) arenaFight(player *model.Player) (GameResult, bool, error) {
	return g.pvpTurn(player, "attack", "")
	/*
		targetID := g.playerValueInt(player.ID, "arena.target", 0)
		if targetID <= 0 {
			return GameResult{Title: "尚未匹配", Content: "先发送 `竞技` 匹配对手。", Actions: []string{"竞技"}}, true, nil
		}
		target, err := g.players.Get(uint(targetID))
		if err != nil {
			return GameResult{Title: "对手已离开", Content: "请重新匹配。", Actions: []string{"竞技"}}, true, nil
		}
		mine, _ := g.arenaRecord(player.ID)
		theirs, _ := g.arenaRecord(target.ID)
		myStats := g.playerCombatStats(player)
		theirStats := g.playerCombatStats(&target)
		myStats.Health = myStats.MaxHealth
		theirStats.Health = theirStats.MaxHealth
		outcome := resolveCombat(myStats, theirStats, g.configuredCombatRules(), rand.New(rand.NewSource(time.Now().UnixNano()+int64(player.ID)+int64(target.ID))))
		won := outcome.PlayerWon && !outcome.Draw
		delta := int64(20)
		result := "胜利"
		if outcome.Draw {
			delta = 3
			result = "战平"
		} else if !won {
			delta = -12
			result = "失败"
		}
		err = g.store.DB.Transaction(func(tx *gorm.DB) error {
			mineUpdates := map[string]any{"rating": gorm.Expr("MAX(rating + ?, 0)", delta)}
			if outcome.Draw {
				// 平局不计入胜负场。
			} else if won {
				mineUpdates["wins"] = gorm.Expr("wins + 1")
			} else {
				mineUpdates["losses"] = gorm.Expr("losses + 1")
			}
			if err := tx.Model(&mine).Updates(mineUpdates).Error; err != nil {
				return err
			}
			targetDelta := int64(-2)
			if !outcome.Draw {
				targetDelta = -delta / 2
			}
			if outcome.Draw {
				return tx.Model(&theirs).Update("rating", gorm.Expr("MAX(rating + ?, 0)", targetDelta)).Error
			}
			if won {
				return tx.Model(&theirs).Updates(map[string]any{"rating": gorm.Expr("MAX(rating + ?, 0)", targetDelta), "losses": gorm.Expr("losses + 1")}).Error
			}
			return tx.Model(&theirs).Updates(map[string]any{"rating": gorm.Expr("rating + ?", targetDelta), "wins": gorm.Expr("wins + 1")}).Error
		})
		if err != nil {
			return GameResult{}, true, err
		}
		_ = g.setPlayerValueInt(player.ID, "arena.target", 0)
		return GameResult{Title: "竞技" + result, Content: fmt.Sprintf("对手：%s\n%s\n积分变化：%+d", target.DaoName, formatCombatOutcome(outcome), delta), Actions: []string{"竞技", "竞榜"}}, true, nil
	*/
}

func (g *Game) arenaRanking() (GameResult, bool, error) {
	type row struct {
		DaoName string
		Rating  int64
		Wins    int64
		Losses  int64
	}
	var rows []row
	err := g.store.DB.Table("arena_records").Select("players.dao_name, arena_records.rating, arena_records.wins, arena_records.losses").Joins("JOIN players ON players.id = arena_records.player_id").Order("arena_records.rating DESC").Limit(10).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf("%d. %s · %d分 · %d胜%d负", i+1, row.DaoName, row.Rating, row.Wins, row.Losses))
	}
	if len(lines) == 0 {
		lines = append(lines, "暂无竞技记录。")
	}
	return GameResult{Title: "竞技排行", Content: strings.Join(lines, "\n")}, true, nil
}

func (g *Game) arenaStore(player *model.Player, argument string) (GameResult, bool, error) {
	record, _ := g.arenaRecord(player.ID)
	raw := strings.TrimSpace(argument)
	page := int(parsePositiveInt(raw, 0))
	if raw == "" || page > 0 && strconv.Itoa(page) == raw {
		if page <= 0 {
			page = 1
		}
		const pageSize = 8
		query := g.store.DB.Where("enabled = ? AND currency = ?", true, "竞技币")
		var total int64
		if err := query.Model(&model.ShopEntry{}).Count(&total).Error; err != nil {
			return GameResult{}, true, err
		}
		pages := maxInt(int((total+pageSize-1)/pageSize), 1)
		if page > pages {
			page = pages
		}
		var rows []model.ShopEntry
		if err := query.Order("sort,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			return GameResult{}, true, err
		}
		lines := []string{fmt.Sprintf("段位积分：%d · 竞技币：%d", record.Rating, player.ArenaCoins), fmt.Sprintf("第%d/%d页 · 共%d件", page, pages, total), "竞技积分决定段位，竞技币只用于兑换，二者不再混扣。", "━━━━━━━━━━━"}
		actions := []string{"竞技档案", "竞技奖励"}
		for _, row := range rows {
			var item model.Item
			_ = g.store.DB.Where("id = ? OR name = ?", row.ItemID, row.ItemName).First(&item).Error
			lines = append(lines, fmt.Sprintf("- %s · %d竞技币 · 常设不限购\n  %s", row.ItemName, row.Price, displayOr(item.Description, "问剑台兑换物资。")))
			actions = append(actions, "竞商 "+row.ItemName)
		}
		if len(rows) == 0 {
			lines = append(lines, "竞技货架尚未开放，请主人检查竞技币商品配置。")
		}
		if page > 1 {
			actions = append(actions, fmt.Sprintf("竞技商店 %d", page-1))
		}
		if page < pages {
			actions = append(actions, fmt.Sprintf("竞技商店 %d", page+1))
		}
		return GameResult{Title: "问剑竞技商店", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(strings.Fields(raw))
	if parseErr != nil {
		return GameResult{Title: "兑换数量错误", Content: parseErr.Error(), Actions: []string{"竞技商店"}}, true, nil
	}
	var shop model.ShopEntry
	if err := g.store.DB.Where("item_name = ? AND currency = ? AND enabled = ?", name, "竞技币", true).Order("sort,id").First(&shop).Error; err != nil {
		return GameResult{Title: "兑换失败", Content: "竞技商店没有上架“" + name + "”。", Actions: []string{"竞技商店"}}, true, nil
	}
	if shop.Price < 0 || shop.Price > 0 && quantity > int64(^uint64(0)>>1)/shop.Price {
		return GameResult{Title: "兑换数量过大", Content: "兑换总价超过系统可安全计算范围，请拆分兑换。", Actions: []string{"竞技商店"}}, true, nil
	}
	total := shop.Price * quantity
	item, err := g.itemByName(name)
	if err != nil {
		return GameResult{Title: "兑换失败", Content: "商品关联物品不存在。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND arena_coins >= ?", player.ID, total).Update("arena_coins", gorm.Expr("arena_coins - ?", total))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		return storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity)
	})
	if err == errInsufficientCurrency {
		return GameResult{Title: "竞技币不足", Content: fmt.Sprintf("兑换%s×%d需要%d竞技币，当前持有%d。", item.Name, quantity, total, player.ArenaCoins), Actions: []string{"竞技", "竞技奖励", "竞技商店"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "竞技兑换", Content: fmt.Sprintf("获得：%s×%d\n支付：%d竞技币\n剩余竞技币：%d\n段位积分保持：%d\n购买规则：常设不限购\n物品用途：%s", item.Name, quantity, total, player.ArenaCoins-total, record.Rating, displayOr(item.Description, "可在对应玩法中使用。")), Actions: []string{"物品 " + item.Name, "背包", "竞技商店"}}, true, nil
}

func (g *Game) executeEncounter(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 131:
		var rows []model.SocialMessage
		_ = g.store.DB.Where("receiver_id = ? AND type = ?", player.ID, "encounter").Order("id DESC").Limit(20).Find(&rows).Error
		if len(rows) == 0 {
			return GameResult{Title: "奇遇录", Content: "尚无奇遇记录。", Actions: []string{"抽签", "福地"}}, true, nil
		}
		lines := make([]string, 0, len(rows))
		for _, row := range rows {
			lines = append(lines, "- "+row.CreatedAt.Format("2006-01-02")+" · "+row.Content)
		}
		return GameResult{Title: "奇遇录", Content: strings.Join(lines, "\n")}, true, nil
	case 132:
		return g.dailyFortune(player)
	case 133:
		if player.SpiritStones < 20 {
			return GameResult{Title: "天机阁", Content: "窥探天机需要20灵石。"}, true, nil
		}
		_ = g.store.DB.Model(player).Update("spirit_stones", gorm.Expr("spirit_stones - 20")).Error
		hints := []string{"今日北行易得机缘。", "雷云将聚，宜提前备劫。", "旧友来访，可多行论道。", "灵脉潮汐将至，修炼收益上升。"}
		return GameResult{Title: "天机一线", Content: hints[rand.Intn(len(hints))] + "\n灵石：-20"}, true, nil
	case 134:
		_, next, err := g.nextRealm(player)
		if err != nil {
			return GameResult{Title: "渡劫预兆", Content: "天机尽头已无更高凡境。"}, true, nil
		}
		missing := max64(next.RequiredCultivation-player.Cultivation, 0)
		return GameResult{Title: "渡劫预兆", Content: fmt.Sprintf("下一劫：%s\n还需修为：%d\n道心：%d\n建议成功率：%.0f%%", next.Name, missing, player.DaoHeart, (g.settingFloat("tribulation.base_rate", .7)+float64(player.DaoHeart-50)/500)*100), Actions: []string{"备劫"}}, true, nil
	case 135:
		if player.Mana < 5 {
			return GameResult{Title: "灵脉探测", Content: "需要5点法力。"}, true, nil
		}
		_ = g.store.DB.Model(player).Update("mana", gorm.Expr("mana - 5")).Error
		quality := []string{"微弱", "普通", "上品", "极品"}[rand.Intn(4)]
		return GameResult{Title: "灵脉探测", Content: fmt.Sprintf("神识向地下延伸，发现%s灵脉。\n位置：%s附近\n法力：-5", quality, player.Location)}, true, nil
	case 136:
		remaining, err := g.useStamina(player.ID, 8)
		if err != nil {
			return GameResult{Title: "寻找福地", Content: err.Error()}, true, nil
		}
		if randomPercent() > 30 {
			return GameResult{Title: "福地难寻", Content: fmt.Sprintf("寻遍群山，未见洞天门户。\n剩余体力：%d", remaining)}, true, nil
		}
		expires := time.Now().Add(60 * time.Minute)
		_ = g.setPlayerValue(player.ID, "buff.blessed_land", "cultivation:1.5", &expires)
		row := model.SocialMessage{ReceiverID: player.ID, Type: "encounter", Content: "发现福地洞天，修炼收益提升50%一小时", Read: true}
		_ = g.social.Create(&row)
		return GameResult{Title: "福地洞天", Content: fmt.Sprintf("你找到一处灵雾环绕的洞府。\n一小时内修炼收益+50%%\n剩余体力：%d", remaining), Actions: []string{"修炼"}}, true, nil
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) dailyFortune(player *model.Player) (GameResult, bool, error) {
	today := time.Now().Format("2006-01-02")
	if date, _ := g.playerValue(player.ID, "fortune.date"); date == today {
		value, _ := g.playerValue(player.ID, "fortune.text")
		return GameResult{Title: "今日仙缘签", Content: value + "\n今日已经抽过签。"}, true, nil
	}
	roll := randomPercent()
	text := "下签 · 宜静守道心"
	dailyText := "今日宜避险，永久运气不变"
	if roll > 30 {
		text = "中签 · 平稳无波"
		dailyText = "今日因果平稳，永久运气不变"
	}
	if roll > 80 {
		text = "上签 · 紫气东来"
		dailyText = "今日宜寻缘访仙，永久运气不变"
	}
	_ = g.setPlayerValue(player.ID, "fortune.date", today, nil)
	_ = g.setPlayerValue(player.ID, "fortune.text", text, nil)
	return GameResult{Title: "仙缘抽签", Content: fmt.Sprintf("**%s**\n%s\n━━━━━━━━━━━\n运气：%d/%d\n抽签只揭示今日宜忌，不会增减永久运气；永久运气需在仙缘奇遇中概率凝成。", text, dailyText, normalizedPlayerLuck(player.Luck), maximumPlayerLuck), Actions: []string{"仙缘", "占卜", "探索", "仙遇"}}, true, nil
}

func (g *Game) executeCareer(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 137:
		var messages []model.SocialMessage
		_ = g.store.DB.Where("sender_id = ? OR receiver_id = ?", player.ID, player.ID).Order("created_at").Limit(30).Find(&messages).Error
		lines := []string{fmt.Sprintf("- %s · %s入道，灵根%s", player.CreatedAt.Format("2006-01-02"), player.DaoName, player.SpiritualRoot)}
		for _, row := range messages {
			if row.Type == "diary" || row.Type == "encounter" {
				lines = append(lines, "- "+row.CreatedAt.Format("2006-01-02")+" · "+row.Content)
			}
		}
		return GameResult{Title: "修仙年表", Content: strings.Join(lines, "\n")}, true, nil
	case 138:
		return g.careerStats(player), true, nil
	case 139:
		goal := strings.TrimSpace(command.RawArguments)
		if goal == "" {
			value, err := g.playerValue(player.ID, "career.goal")
			if err != nil {
				value = "尚未设定"
			}
			return GameResult{Title: "修仙目标", Content: value}, true, nil
		}
		_ = g.setPlayerValue(player.ID, "career.goal", goal, nil)
		return GameResult{Title: "目标已定", Content: goal + "\n愿你道心不改。"}, true, nil
	case 140:
		return g.careerEvaluation(player), true, nil
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) careerStats(player *model.Player) GameResult {
	days := int64(time.Since(player.CreatedAt).Hours() / 24)
	return GameResult{Title: "修仙统计", Content: fmt.Sprintf(
		"入道天数：%d\n累计闭关：%d分钟\n累计修为收益：%d\n探索次数：%d\n战斗次数：%d\n战斗胜利：%d\n突破次数：%d\n渡劫成功：%d\n当前战力：%d",
		days,
		g.playerValueInt(player.ID, "stats.cultivation_minutes", 0),
		g.playerValueInt(player.ID, "stats.cultivation_gain", 0),
		g.playerValueInt(player.ID, "stats.explores", 0),
		g.playerValueInt(player.ID, "stats.battles", 0),
		g.playerValueInt(player.ID, "stats.wins", 0),
		g.playerValueInt(player.ID, "stats.breakthroughs", 0),
		g.playerValueInt(player.ID, "stats.tribulation_successes", 0),
		player.CombatPower,
	)}
}

func (g *Game) careerEvaluation(player *model.Player) GameResult {
	score := int64(player.RealmID)*20 + player.DaoHeart/5 + player.ImmortalAffinity/5 + g.playerValueInt(player.ID, "stats.wins", 0)
	level := "初窥门径"
	advice := "多闭关修炼，并通过探索积累资源。"
	if score >= 80 {
		level = "道基稳固"
		advice = "可以挑战秘境并精进主修功法。"
	}
	if score >= 150 {
		level = "一方真人"
		advice = "建立宗门、培养仙侣默契，准备更高天劫。"
	}
	if score >= 240 {
		level = "超凡入圣"
		advice = "你的凡间根基已成，向飞升与大道终点进发。"
	}
	return GameResult{Title: "修仙评价", Content: fmt.Sprintf("评价：**%s**\n综合进度：%d\n境界：%s\n道心：%d\n仙缘：%d\n建议：%s", level, score, player.RealmName, player.DaoHeart, player.ImmortalAffinity, advice)}
}
