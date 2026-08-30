package service

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) executeExplore(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 23:
		return g.explore(player)
	case 24:
		return g.secretRealm(player)
	case 25:
		return g.treasureHunt(player)
	case 26:
		if strings.TrimSpace(command.RawArguments) != "" {
			return g.gatherLocalMapResource(player, command.RawArguments)
		}
		return g.collectHerbs(player)
	case 27:
		return g.visitFriend(player, command.RawArguments)
	case 28:
		return g.travelMountain(player)
	case 29:
		return g.exploreRuins(player)
	case 30:
		return g.huntMonster(player)
	case 31:
		return g.dharmaAssembly(player)
	case 32:
		return g.meetImmortal(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) useStamina(playerID uint, amount int64) (int64, error) {
	current, err := g.currentStamina(playerID)
	if err != nil {
		return 0, err
	}
	if current < amount {
		maximum, maximumErr := g.staminaMaximum(playerID)
		if maximumErr != nil {
			return current, maximumErr
		}
		return current, fmt.Errorf("体力不足：当前%d/%d，本次需要%d", current, maximum, amount)
	}
	current -= amount
	return current, g.setPlayerValueInt(playerID, "stamina.value", current)
}

func (g *Game) staminaMaximum(playerID uint) (int64, error) {
	base := max64(g.settingInt("player.daily_stamina", 100), 0)
	growth := max64(g.settingInt("player.stamina_growth_per_realm", 100), 0)
	var player model.Player
	if err := g.store.DB.Select("id", "realm_id", "realm_name").First(&player, playerID).Error; err != nil {
		return 0, err
	}
	sequence, err := g.playerRealmSequence(&player)
	if err != nil {
		return 0, err
	}
	if sequence < 1 {
		sequence = 1
	}
	return base + int64(sequence-1)*growth, nil
}

func (g *Game) staminaRecoveryForSequence(sequence int) int64 {
	if sequence < 1 {
		sequence = 1
	}
	base := max64(g.settingInt("player.stamina_recovery_per_minute", 10), 0)
	growth := max64(g.settingInt("player.stamina_recovery_growth_per_realm", 10), 0)
	return base + int64(sequence-1)*growth
}

func (g *Game) staminaRecoveryPerMinute(playerID uint) (int64, error) {
	var player model.Player
	if err := g.store.DB.Select("id", "realm_id", "realm_name").First(&player, playerID).Error; err != nil {
		return 0, err
	}
	sequence, err := g.playerRealmSequence(&player)
	if err != nil {
		return 0, err
	}
	return g.staminaRecoveryForSequence(sequence), nil
}

func (g *Game) currentStamina(playerID uint) (int64, error) {
	today := time.Now().Format("2006-01-02")
	maximum, err := g.staminaMaximum(playerID)
	if err != nil {
		return 0, err
	}
	recovery, err := g.staminaRecoveryPerMinute(playerID)
	if err != nil {
		return 0, err
	}
	date, _ := g.playerValue(playerID, "stamina.date")
	if date != today {
		if err := g.setPlayerValue(playerID, "stamina.date", today, nil); err != nil {
			return 0, err
		}
		if err := g.setPlayerValueInt(playerID, "stamina.value", maximum); err != nil {
			return 0, err
		}
		return maximum, nil
	}
	var row model.PlayerValue
	err = g.store.DB.Where("player_id = ? AND key = ?", playerID, "stamina.value").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := g.setPlayerValueInt(playerID, "stamina.value", maximum); err != nil {
			return 0, err
		}
		return maximum, nil
	}
	if err != nil {
		return 0, err
	}
	current, parseErr := strconv.ParseInt(row.Value, 10, 64)
	if parseErr != nil {
		current = maximum
	}
	if current < 0 {
		current = 0
	}
	if current > maximum {
		current = maximum
	}
	minutes := int64(time.Since(row.UpdatedAt) / time.Minute)
	if recovery > 0 && minutes > 0 && current < maximum {
		current += minutes * recovery
		if current > maximum {
			current = maximum
		}
		if err := g.setPlayerValueInt(playerID, "stamina.value", current); err != nil {
			return 0, err
		}
	}
	return current, nil
}

func (g *Game) explore(player *model.Player) (GameResult, bool, error) {
	if player.Health <= 1 {
		effective := g.playerWithActiveSkillStats(player)
		return GameResult{Title: "👻 元神离体，无法游历", Content: fmt.Sprintf("道友%s当前气血仅余%d/%d，肉身已经失去行动能力。\n━━━━━━━━━━━\n阵亡状态不能探索、采集、挑战妖兽或获取任何游历收益。请先返回最近的地脉复生阵，再疗伤恢复气血。", player.DaoName, effective.Health, effective.MaxHealth), Actions: []string{"回城复活", "状态"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		if player.State == model.PlayerStateBattling {
			return GameResult{Title: "⚔️ 战斗尚未结束", Content: "你仍在当前战局中，不能同时探索大世界。请继续行动或投降脱离战斗。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
		}
		return GameResult{Title: "🧭 当前无法探索", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	location, locationErr := g.currentWorldLocation(player)
	if locationErr != nil {
		return GameResult{}, true, locationErr
	}
	if location.ID == 0 {
		return GameResult{Title: "🧭 世界尚未勘定", Content: "当前没有可踏足的地图地点，请主人检查世界地图数据。", Actions: []string{"地图"}}, true, nil
	}
	remaining, err := g.useStamina(player.ID, 2)
	if err != nil {
		return GameResult{Title: "🧭 无法探索", Content: err.Error(), Actions: []string{"状态", "位置"}}, true, nil
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.explores", 1)
	if eventResult, triggered, eventErr := g.triggerConfiguredEvent(player, remaining); eventErr != nil {
		return GameResult{}, true, eventErr
	} else if triggered {
		eventResult.Content = fmt.Sprintf("所在：%s · %s\n━━━━━━━━━━━\n%s", location.Region, location.Name, eventResult.Content)
		if eventResult.ImageURL == "" {
			eventResult.ImageURL = location.ImageURL
		}
		eventResult.Actions = append(eventResult.Actions, "位置", "地图")
		return eventResult, true, nil
	}
	roll := randomPercent()
	switch {
	case roll <= 12:
		return g.discoverPetEncounter(player, location, remaining)
	case roll <= 25:
		reward := max64(5+int64(location.MinimumRealmSequence*4)+int64(player.RealmLevel), 8)
		_ = g.store.DB.Model(player).Update("cultivation", gorm.Expr("cultivation + ?", reward)).Error
		return GameResult{Title: "💧 " + location.Name + "·地泉问迹", Content: fmt.Sprintf("你沿%s的地脉纹路行至一处隐泉，泉眼与%s地下灵机相连。护住经脉后，你只截取一缕温和灵气，没有强行掠夺泉脉本源。\n━━━━━━━━━━━\n所在州域：%s · %s\n游历所得：修为 +%d\n体力消耗：2 · 剩余体力：%d\n探索记录：已计入此地游历次数\n━━━━━━━━━━━\n此类野外灵机只提供少量修为；稳定成长仍需闭关、灵脉打坐与完成地图任务。", location.Description, location.Name, location.Region, location.Name, reward, remaining), ImageURL: location.ImageURL, Actions: []string{"位置", "灵脉地图", "修炼", "探索", "任务"}}, true, nil
	case roll <= 50:
		resource := strings.TrimSpace(location.ResourceName)
		if resource != "" {
			if item, itemErr := g.itemByName(resource); itemErr == nil {
				_ = g.players.AdjustItem(player.ID, item.ID, 1)
				return GameResult{Title: "🌿 " + location.Name + "·灵材寻踪", Content: fmt.Sprintf("神识扫过%s的山川草木，你在一处被岩影遮蔽的灵穴中辨出%s。采下外围成熟灵材后，你保留根脉，等待此地再次孕育。\n━━━━━━━━━━━\n区域资源：%s\n本次所得：%s × 1\n常规采集量：%d · 刷新约%d分钟\n体力消耗：2 · 剩余体力：%d\n━━━━━━━━━━━\n需要更多材料可发送“采集 %s”；该资源同时会在物品查询中标明地图出处。", location.Description, resource, resource, resource, maxInt(location.ResourceQuantity, 1), maxInt(location.ResourceCooldownMin, 1), remaining, resource), ImageURL: location.ImageURL, Actions: []string{"物品 " + resource, "采集 " + resource, "背包", "位置", "探索"}}, true, nil
			}
		}
		return GameResult{Title: "🌿 " + location.Name + "·灵痕初现", Content: fmt.Sprintf("你在%s发现尚未完全成形的灵材气息，但此地资源尚未在物品图鉴中完成登记。\n━━━━━━━━━━━\n体力消耗：2 · 剩余体力：%d\n本次没有凭空生成随机物品，避免出现与地图无关的掉落。", location.Name, remaining), ImageURL: location.ImageURL, Actions: []string{"位置", "地图", "探索"}}, true, nil
	case roll <= 75:
		return g.huntMonsterWithStamina(player, remaining)
	case roll <= 90:
		npcs := decodeTextList(location.NPCJSON)
		if len(npcs) > 0 {
			npc := npcs[rand.Intn(len(npcs))]
			tasks := decodeTextList(location.TasksJSON)
			taskHint := "此地暂无可接取任务。"
			actions := []string{"对话 " + npc, "位置", "探索", "任务"}
			if len(tasks) > 0 {
				taskHint = "附近因果：" + tasks[0]
				actions = append(actions, "接任务 "+tasks[0])
			}
			return GameResult{Title: "📜 " + location.Name + "·人间问道", Content: fmt.Sprintf("行至%s深处时，你遇见%s。对方似乎知晓当地灵脉、妖兽与旧事，却没有直接将机缘交到你手中。\n━━━━━━━━━━━\n所在：%s · %s\n可对话人物：%s\n%s\n体力消耗：2 · 剩余体力：%d\n━━━━━━━━━━━\n与NPC对话后才能获知委托、前置和奖励；探索本身不会替玩家自动接取。", location.Name, npc, location.Region, location.Name, npc, taskHint, remaining), ImageURL: location.ImageURL, Actions: actions}, true, nil
		}
		fallthrough
	default:
		return GameResult{Title: "🧭 " + location.Name + "·山河游历", Content: fmt.Sprintf("你沿%s缓步勘察，记下灵气流向、妖兽足迹和地势变化。此行没有强求机缘，却补全了对当地山河的认识。\n━━━━━━━━━━━\n区域：%s · 地点：%s\n地图见闻：%s\n体力消耗：2 · 剩余体力：%d\n本次所得：无直接物品或修为\n━━━━━━━━━━━\n可继续与当地人物交谈、采集区域灵材，或主动挑战此地妖兽。", location.Name, location.Region, location.Name, location.Description, remaining), ImageURL: location.ImageURL, Actions: explorationLocationActions(location)}, true, nil
	}
}

func explorationLocationActions(location model.WorldLocation) []string {
	actions := []string{"位置", "探索", "地图"}
	if location.ResourceName != "" {
		actions = append(actions, "采集 "+location.ResourceName)
	}
	if location.MonsterName != "" {
		actions = append(actions, "挑战 "+location.MonsterName)
	}
	if npcs := decodeTextList(location.NPCJSON); len(npcs) > 0 {
		actions = append(actions, "对话 "+npcs[0])
	}
	return actions
}

func (g *Game) resurrectPlayer(player *model.Player) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	if player.Health > 1 {
		return GameResult{Title: "🕯️ 无需复生", Content: fmt.Sprintf("你的元神仍在肉身之中，当前气血%d/%d。若只是重伤，可服用疗伤丹药恢复。", effective.Health, effective.MaxHealth), Actions: []string{"疗伤", "状态", "位置"}}, true, nil
	}
	location, _ := g.currentWorldLocation(player)
	reviveHP := max64(effective.MaxHealth*3/10, 1)
	reviveMana := max64(effective.MaxMana*3/10, 0)
	// Death should matter without erasing hours of low-realm progression. The
	// loss is 1% of held cultivation and is capped by a quarter of the current
	// stage requirement, with a small floor for over-stocked early characters.
	cultivationLoss := player.Cultivation / 100
	stageCap := max64(player.CultivationRequired/4, 20)
	cultivationLoss = min64(max64(cultivationLoss, 0), stageCap)
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key = ?", player.ID, "pve.battle").Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"health": reviveHP, "mana": reviveMana, "cultivation": gorm.Expr("MAX(cultivation - ?, 0)", cultivationLoss), "state": model.PlayerStateIdle, "cultivation_started_at": nil,
		}).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.revivals", 1)
	return GameResult{Title: "👻 地脉复生", Content: fmt.Sprintf("一道接引灵光将%s散落的元神牵回肉身，最近的地脉复生阵随之熄灭。\n━━━━━━━━━━━\n复生地点：%s · %s\n气血：%d/%d\n法力：%d/%d\n修为惩罚：-%d\n累计复生：%d次\n━━━━━━━━━━━\n惩罚规则：扣除当前修为1%%，但不超过本层突破需求的25%%；低境界单次上限至少20点。\n你已恢复行动，但气血仍低，可先服用回元散与回灵丹再继续挑战。", player.DaoName, displayOr(location.Region, "当前州域"), displayOr(location.Name, player.Location), reviveHP, effective.MaxHealth, reviveMana, effective.MaxMana, cultivationLoss, g.playerValueInt(player.ID, "stats.revivals", 0)), ImageURL: location.ImageURL, Actions: []string{"使用 回元散", "使用 回灵丹", "疗伤", "状态", "位置", "背包"}}, true, nil
}

func (g *Game) secretRealm(player *model.Player) (GameResult, bool, error) {
	result, handled, err := g.dungeonList(player)
	if err != nil || !handled {
		return result, handled, err
	}
	result.Title = "秘境入口"
	result.Content = "秘境不会自动战斗。请从下列副本选择，进入后逐回合发送攻击、技能 功法名或防御。\n━━━━━━━━━━━\n" + result.Content
	return result, true, nil
}

func (g *Game) treasureHunt(player *model.Player) (GameResult, bool, error) {
	baseChance := .30 + float64(player.ImmortalAffinity)/1000
	if baseChance > .95 {
		baseChance = .95
	}
	chance, luckBonus := probabilityWithLuck(baseChance, player.Luck, luckTreasureBonusCap)
	if rand.Float64() > chance {
		return GameResult{Title: "寻宝未果", Content: fmt.Sprintf("你循着地脉与星盘指向搜寻半日，只找到一处早已坍塌的洞穴。\n━━━━━━━━━━━\n寻宝率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n运气：%d/%d（本次不会消耗）", baseChance*100, luckBonus*100, chance*100, normalizedPlayerLuck(player.Luck), maximumPlayerLuck), Actions: []string{"寻宝", "仙缘", "占卜", "位置"}}, true, nil
	}
	item, err := g.randomEnabledItem()
	if err != nil {
		return GameResult{}, true, err
	}
	if err := g.players.AdjustItem(player.ID, item.ID, 1); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "气运寻宝", Content: fmt.Sprintf("地脉灵光与星盘卦线在断崖下交汇，你从禁制残隙中取出一只古修宝匣。\n━━━━━━━━━━━\n获得：%s×1\n寻宝率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n运气：%d/%d（永久属性，本次不会消耗）", item.Name, baseChance*100, luckBonus*100, chance*100, normalizedPlayerLuck(player.Luck), maximumPlayerLuck), Actions: []string{"物品 " + item.Name, "背包", "寻宝", "仙缘"}}, true, nil
}

func (g *Game) collectHerbs(player *model.Player) (GameResult, bool, error) {
	remaining, err := g.useStamina(player.ID, 3)
	if err != nil {
		return GameResult{Title: "无法采集", Content: err.Error()}, true, nil
	}
	chance := 55 + int(player.Spirit/5)
	if randomPercent() > chance {
		return GameResult{Title: "采集落空", Content: fmt.Sprintf("灵草气息消散，只留下折断的根须。\n剩余体力：%d", remaining)}, true, nil
	}
	quantity := int64(1 + rand.Intn(3))
	item, err := g.itemByName("灵茶")
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.players.AdjustItem(player.ID, item.ID, quantity)
	_, _ = g.addPlayerValueInt(player.ID, "stats.collects", 1)
	return GameResult{Title: "采集灵草", Content: fmt.Sprintf("神识捕捉到草木灵韵。\n获得：灵茶×%d\n剩余体力：%d", quantity, remaining), Actions: []string{"背包", "采集"}}, true, nil
}

func (g *Game) visitFriend(player *model.Player, argument string) (GameResult, bool, error) {
	target, err := g.findPlayer(argument)
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "访友", Content: "请输入存在的道友：`访友 @对方`"}, true, nil
	}
	reward := int64(10)
	if player.RealmID < target.RealmID {
		reward = 15
	}
	_ = g.store.DB.Model(player).Updates(map[string]any{"cultivation": gorm.Expr("cultivation + ?", reward), "reputation": gorm.Expr("reputation + 1")}).Error
	return GameResult{Title: "访友论道", Content: fmt.Sprintf("你拜访%s，对坐论道良久。\n修为：+%d\n声望：+1", target.DaoName, reward), Actions: []string{"加友 " + target.AccountID, "论道 " + target.AccountID}}, true, nil
}

func (g *Game) travelMountain(player *model.Player) (GameResult, bool, error) {
	remaining, ok, err := g.cooldown(player.ID, "travel_mountain", 30*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "石刻沉寂", Content: "还需" + formatDuration(remaining) + "才能再次参悟。"}, true, nil
	}
	_ = g.store.DB.Model(player).Update("perception", gorm.Expr("perception + 1")).Error
	return GameResult{Title: "游历名山", Content: "绝壁石刻蕴含前人道韵。\n悟性：+1"}, true, nil
}

func (g *Game) exploreRuins(player *model.Player) (GameResult, bool, error) {
	remaining, err := g.useStamina(player.ID, 5)
	if err != nil {
		return GameResult{Title: "无法探幽", Content: err.Error()}, true, nil
	}
	if randomPercent() > 35 {
		return GameResult{Title: "古迹空寂", Content: fmt.Sprintf("残碑字迹早已风化。\n剩余体力：%d", remaining)}, true, nil
	}
	_ = g.adjustNamedItem(player.ID, "功法残卷", 1)
	return GameResult{Title: "探幽访古", Content: fmt.Sprintf("你从石匣中取出功法残卷×1。\n剩余体力：%d", remaining), Actions: []string{"背包", "学功"}}, true, nil
}

func (g *Game) huntMonster(player *model.Player) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if location.MonsterName == "" {
		return GameResult{Title: "此地无妖", Content: "当前位置没有可挑战妖兽，请按地图路线前往其他区域。", Actions: []string{"位置", "地图"}}, true, nil
	}
	return g.startMapMonsterBattle(player, location.MonsterName)
}

func (g *Game) huntMonsterWithStamina(player *model.Player, remaining int64) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if location.MonsterName == "" {
		return GameResult{Title: "👾 妖气散去", Content: fmt.Sprintf("你在%s察觉到一缕陌生妖气，追至山坳时只见残留爪痕。当前地点没有已登记妖兽，因此不会凭空开启战斗。\n━━━━━━━━━━━\n体力消耗：2 · 剩余体力：%d", location.Name, remaining), ImageURL: location.ImageURL, Actions: []string{"位置", "地图", "探索"}}, true, nil
	}
	return GameResult{Title: "👾 " + location.Name + "·妖踪乍现", Content: fmt.Sprintf("林间灵禽忽然噤声，泥土中浮出尚未散尽的妖煞。你循迹望去，%s正盘踞在%s的必经之路。\n━━━━━━━━━━━\n遭遇目标：%s\n妖兽战力：%d\n你的战力：%d\n体力消耗：2 · 剩余体力：%d\n战斗规则：探索只发现目标，不会替你自动攻击。\n━━━━━━━━━━━\n发送“挑战 %s”后进入逐回合战斗，再由你选择普通攻击、功法、技能、防御或投降。", location.MonsterName, location.Name, location.MonsterName, location.MonsterPower, player.CombatPower, remaining, location.MonsterName), ImageURL: location.ImageURL, Actions: []string{"挑战 " + location.MonsterName, "查询 " + location.MonsterName, "功法", "状态", "位置"}}, true, nil
}

func (g *Game) dharmaAssembly(player *model.Player) (GameResult, bool, error) {
	remaining, ok, err := g.cooldown(player.ID, "dharma", 60*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "法会未开", Content: "下一场法会还需" + formatDuration(remaining) + "。"}, true, nil
	}
	reward := int64(10 * maxInt(player.RealmLevel, 1))
	_ = g.store.DB.Model(player).Update("cultivation", gorm.Expr("cultivation + ?", reward)).Error
	return GameResult{Title: "参加法会", Content: fmt.Sprintf("高人讲法，字字珠玑。\n获得修为：+%d", reward)}, true, nil
}

func (g *Game) meetImmortal(player *model.Player) (GameResult, bool, error) {
	remaining, err := g.useStamina(player.ID, 5)
	if err != nil {
		return GameResult{Title: "无缘遇仙", Content: err.Error()}, true, nil
	}
	baseChance := .02 + float64(player.ImmortalAffinity)/10000
	if baseChance > .90 {
		baseChance = .90
	}
	chance, luckBonus := probabilityWithLuck(baseChance, player.Luck, luckMeetImmortalBonusCap)
	if rand.Float64() > chance {
		return GameResult{Title: "仙踪渺渺", Content: fmt.Sprintf("云海深处似有人影，转瞬无踪。\n遇仙率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n剩余体力：%d", baseChance*100, luckBonus*100, chance*100, remaining), Actions: []string{"遇仙", "仙缘", "探索"}}, true, nil
	}
	_ = g.store.DB.Model(player).Updates(map[string]any{"immortal_affinity": gorm.Expr("immortal_affinity + 20"), "root_quality": gorm.Expr("MIN(root_quality + 2, 100)")}).Error
	luckLine, luckErr := g.tryGrowLuckFromEncounter(player)
	if luckErr != nil {
		return GameResult{}, true, luckErr
	}
	return GameResult{Title: "遇仙奇缘", Content: fmt.Sprintf("云中仙人于崖前点出一式运气法门，旋即踏霞而去。\n━━━━━━━━━━━\n遇仙率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n仙缘：+20\n灵根纯度：+2\n%s", baseChance*100, luckBonus*100, chance*100, luckLine), Actions: []string{"仙缘", "状态", "功法", "遇仙"}}, true, nil
}

func (g *Game) executeBattle(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 47:
		return g.huntMonster(player)
	case 48:
		if player.CoupleID == 0 {
			return GameResult{Title: "合战失败", Content: "你尚未结缘，无法发动双人合战。", Actions: []string{"寻缘"}}, true, nil
		}
		location, err := g.currentWorldLocation(player)
		if err != nil {
			return GameResult{}, true, err
		}
		if location.MonsterName == "" {
			return GameResult{Title: "此地无妖", Content: "当前位置没有合战目标。", Actions: []string{"位置", "地图"}}, true, nil
		}
		return g.startMapMonsterBattleMode(player, location.MonsterName, true, 5)
	case 49:
		if player.State != model.PlayerStateBattling {
			return GameResult{Title: "无需逃跑", Content: "当前不在战斗中。"}, true, nil
		}
		if randomPercent() <= 70 {
			_ = g.store.DB.Model(player).Update("state", model.PlayerStateIdle).Error
			return GameResult{Title: "逃跑成功", Content: "你借遁光脱离战场。"}, true, nil
		}
		return GameResult{Title: "逃跑失败", Content: "退路被妖兽截断。"}, true, nil
	case 50:
		return g.heal(player)
	case 51:
		return g.battlePill(player, command.RawArguments)
	case 52:
		petPower := g.activePetCombatPower(player)
		effective := g.playerWithActiveSkillStats(player)
		return GameResult{Title: "综合战力", Content: fmt.Sprintf("总战力：**%d**\n━━━━━━━━━━━\n角色基础与装备：%d\n出战灵兽：%d\n攻击：物攻%d · 法攻%d\n防御：物防%d · 法防%d\n气血：%d · 法力：%d\n━━━━━━━━━━━\n装备穿戴/锻造、灵根成长、属性提升和灵兽出战后都会重新计算总战力；未出战灵兽只计入灵兽榜，不计入角色战力。", player.CombatPower, max64(player.CombatPower-petPower, 1), petPower, effective.PhysicalAttack, effective.MagicAttack, effective.PhysicalDefense, effective.MagicDefense, effective.MaxHealth, effective.MaxMana), Actions: []string{"状态", "当前装备", "灵兽", "灵检"}}, true, nil
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) heal(player *model.Player) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	maximumHealth, maximumMana := effective.MaxHealth, effective.MaxMana
	currentHealth, currentMana := effective.Health, effective.Mana
	if currentHealth >= maximumHealth {
		return GameResult{Title: "气血充盈", Content: "无需疗伤。"}, true, nil
	}
	// 濒死→引导复生
	if currentHealth <= 1 {
		return g.resurrectPlayer(player)
	}
	item, err := g.itemByName("仙露")
	hasDew := err == nil && g.itemQuantity(player.ID, item.ID) >= 1
	if hasDew {
		recover := max64(int64(item.EffectValue), max64(maximumHealth*35/100, 20))
		newHealth := min64(currentHealth+recover, maximumHealth)
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := consumeNamedItemTx(tx, player.ID, "仙露", 1); err != nil {
				return err
			}
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("health", newHealth).Error; err != nil {
				return err
			}
			return syncPVEBattleVitalsTx(tx, player.ID, &newHealth, nil)
		}); err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "💧 仙露疗伤", Content: fmt.Sprintf("服下仙露，药力沿十二正经化开，受损气脉重新续合。\n━━━━━━━━━━━\n气血恢复：+%d（至少恢复最大气血35%%）\n当前：%d/%d\n消耗：仙露×1\n━━━━━━━━━━━\n当前主修功法的气血上限已计入恢复结算。", newHealth-currentHealth, newHealth, maximumHealth), Actions: []string{"状态", "修炼", "探索", "物品 仙露"}}, true, nil
	}
	// 无仙露时消耗法力和灵石运功止血
	if currentMana < 10 || player.SpiritStones < 5 {
		return GameResult{Title: "💧 疗伤条件不足", Content: fmt.Sprintf("乾坤袋中没有仙露，运功止血还需要法力10与灵石5。\n━━━━━━━━━━━\n当前：仙露×0 · 法力%d/%d · 灵石%d\n可通过签到、货铺、地图妖兽和副本获取疗伤资源。", currentMana, maximumMana, player.SpiritStones), Actions: []string{"签到", "物品 仙露", "货铺", "位置", "状态"}}, true, nil
	}
	recover := max64(maximumHealth*25/100, 20)
	newHealth := min64(currentHealth+recover, maximumHealth)
	newMana := currentMana - 10
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Updates(map[string]any{
			"health": newHealth, "mana": newMana, "spirit_stones": gorm.Expr("spirit_stones - ?", 5),
		}).Error; err != nil {
			return err
		}
		return syncPVEBattleVitalsTx(tx, player.ID, &newHealth, &newMana)
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "运功疗伤", Content: fmt.Sprintf("丹田余息在经脉中流转，灵石灵炁随法力注入伤处，伤势开始愈合。\n━━━━━━━━━━━\n气血恢复：+%d（最大气血25%%）\n当前：%d/%d\n消耗：法力10 · 灵石5\n当前主修功法的气血上限已计入恢复结算。", newHealth-currentHealth, newHealth, maximumHealth), Actions: []string{"物品 仙露", "集市", "状态", "修炼"}}, true, nil
}

func (g *Game) battlePill(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		name = "灵果"
	}
	item, err := g.itemByName(name)
	if err != nil || g.itemQuantity(player.ID, item.ID) < 1 {
		return GameResult{Title: "丹战失败", Content: "背包中没有可用的" + name + "。"}, true, nil
	}
	_ = g.players.AdjustItem(player.ID, item.ID, -1)
	expires := time.Now().Add(15 * time.Minute)
	_ = g.setPlayerValue(player.ID, "buff.battle", name+":攻击+10%", &expires)
	return GameResult{Title: "丹战辅助", Content: fmt.Sprintf("服用%s，15分钟内攻击提升10%%。", name)}, true, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
