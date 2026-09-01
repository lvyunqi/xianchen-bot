package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeTribulation(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 53:
		return g.attemptTribulation(player, false)
	case 54:
		if player.CoupleID == 0 {
			return GameResult{Title: "共渡失败", Content: "你尚未结缘。", Actions: []string{"寻缘"}}, true, nil
		}
		return g.attemptTribulation(player, true)
	case 55:
		return g.tribulationChecklist(player)
	case 58:
		return g.ascend(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) nextRealm(player *model.Player) (model.Realm, model.Realm, error) {
	var current model.Realm
	if err := g.store.DB.First(&current, player.RealmID).Error; err != nil {
		return model.Realm{}, model.Realm{}, err
	}
	var next model.Realm
	err := g.store.DB.Where("sequence > ?", current.Sequence).Order("sequence").First(&next).Error
	return current, next, err
}

func (g *Game) tribulationChecklist(player *model.Player) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	current, next, err := g.nextRealm(player)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "备劫", Content: "已无更高凡间境界，可尝试飞升。", Actions: []string{"飞升"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	cost := realmStageCost(current, next)
	medicineBonus := g.activeItemBonuses(player.ID)
	effectiveDaoHeart := player.DaoHeart + medicineBonus.DaoHeart
	titleTribulationBonus := g.activeTitleProbabilityBonus(player, "tribulation_percent")
	rate := current.TribulationBaseRate + float64(effectiveDaoHeart-50)/500 + medicineBonus.TribulationRate + titleTribulationBonus
	if current.TribulationBaseRate <= 0 {
		rate = g.settingFloat("tribulation.base_rate", .7) + float64(effectiveDaoHeart-50)/500 + medicineBonus.TribulationRate + titleTribulationBonus
	}
	if rate > .99 {
		rate = .99
	}
	ready := player.RealmLevel >= realmStageCount && player.Cultivation >= cost
	talisman, talismanErr := g.itemByName("引劫玉符")
	if talismanErr != nil {
		return GameResult{}, true, talismanErr
	}
	talismanOwned := g.itemQuantity(player.ID, talisman.ID)
	minimumDaoHeart := tribulationDaoHeartRequirement(current.Sequence)
	healthReady := effective.Health*100 >= effective.MaxHealth*80
	manaReady := effective.Mana*100 >= effective.MaxMana*60
	ready = ready && effectiveDaoHeart >= minimumDaoHeart && healthReady && manaReady && talismanOwned >= 1
	status := "未满足"
	if ready {
		status = "已满足"
		expires := time.Now().Add(30 * time.Minute)
		_ = g.setPlayerValue(player.ID, "tribulation.prepared", strconv.Itoa(current.Sequence), &expires)
	}
	profile := realmTribulationProfile(current.Sequence)
	return GameResult{Title: "备劫清单", Content: fmt.Sprintf("当前：%s · %d/%d层\n目标：%s · 一层\n圆满条件：当前境界十层（%s）\n本次需修为：%d · 当前%d\n道心门槛：%d · 当前%d（本体%d · 药力+%d）\n气血门槛：80%% · 当前%d/%d\n法力门槛：60%% · 当前%d/%d\n引劫玉符：需要1枚 · 当前%d枚\n准备判定：%s\n━━━━━━━━━━━\n劫名：%s\n三道劫关：%s\n主要威胁：%s\n规则：三关分别判定，后一关成功率更低；任一关失败即渡劫失败。\n失败代价：引劫玉符照常消耗，并损失修为、陷入重伤；闯过的劫关越多反噬越重。\n━━━━━━━━━━━\n首劫综合成功率：%.0f%%\n丹药护劫加成：+%.1f%%\n称号护劫加成：+%.1f%%\n仙侣共渡加成：%.0f%%\n准备有效期：满足全部条件后30分钟", current.Name, player.RealmLevel, realmStageCount, next.Name, map[bool]string{true: "已圆满", false: "未圆满"}[player.RealmLevel >= realmStageCount], cost, player.Cultivation, minimumDaoHeart, effectiveDaoHeart, player.DaoHeart, medicineBonus.DaoHeart, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana, talismanOwned, status, profile.Name, strings.Join(profile.Trials, " → "), profile.Hazard, rate*100, medicineBonus.TribulationRate*100, titleTribulationBonus*100, g.settingFloat("tribulation.couple_bonus", .3)*100), Actions: []string{"引劫", "共渡", "合成 引劫玉符", "物品 引劫玉符", "疗伤", "冥想", "修炼", "状态"}}, true, nil
}

func (g *Game) attemptTribulation(player *model.Player, together bool) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	current, next, err := g.nextRealm(player)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "天门已现", Content: "你已越过诸劫，发送 `飞升` 叩问天门。", Actions: []string{"飞升"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	profile := realmTribulationProfile(current.Sequence)
	medicineBonus := g.activeItemBonuses(player.ID)
	effectiveDaoHeart := player.DaoHeart + medicineBonus.DaoHeart
	titleTribulationBonus := g.activeTitleProbabilityBonus(player, "tribulation_percent")
	if player.RealmLevel < realmStageCount {
		return GameResult{Title: "道基未圆满", Content: fmt.Sprintf("当前：%s · %d层\n引动%s前必须先修至十层圆满。\n还需完成：%d次小境突破\n规则：修为再多也不能越层进入下一境界。", current.Name, player.RealmLevel, profile.Name, realmStageCount-player.RealmLevel), Actions: []string{"突破", "修炼", "备劫"}}, true, nil
	}
	cost := realmStageCost(current, next)
	if player.Cultivation < cost {
		return GameResult{Title: "天劫未至", Content: fmt.Sprintf("劫名：%s\n目标：%s · 一层\n本次需要修为：%d\n当前修为：%d\n尚缺：%d", profile.Name, next.Name, cost, player.Cultivation, cost-player.Cultivation), Actions: []string{"修炼", "备劫"}}, true, nil
	}
	minimumDaoHeart := tribulationDaoHeartRequirement(current.Sequence)
	if effectiveDaoHeart < minimumDaoHeart || effective.Health*100 < effective.MaxHealth*80 || effective.Mana*100 < effective.MaxMana*60 {
		return GameResult{Title: "备劫条件不足", Content: fmt.Sprintf("引动%s需要：道心%d、气血至少80%%、法力至少60%%。\n当前：道心%d（本体%d · 药力+%d）· 气血%d/%d · 法力%d/%d。", profile.Name, minimumDaoHeart, effectiveDaoHeart, player.DaoHeart, medicineBonus.DaoHeart, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana), Actions: []string{"备劫", "疗伤", "冥想", "状态"}}, true, nil
	}
	talisman, talismanErr := g.itemByName("引劫玉符")
	if talismanErr != nil {
		return GameResult{}, true, talismanErr
	}
	if g.itemQuantity(player.ID, talisman.ID) < 1 {
		return GameResult{Title: "引劫法器不足", Content: fmt.Sprintf("引动%s必须消耗引劫玉符×1。\n当前持有：0\n无论渡劫成功或失败，玉符都会在牵引劫云时耗尽。", profile.Name), Actions: []string{"合成 引劫玉符", "合成图鉴", "物品 引劫玉符", "背包", "备劫"}}, true, nil
	}
	prepared, preparedErr := g.playerValue(player.ID, "tribulation.prepared")
	if preparedErr != nil || prepared != strconv.Itoa(current.Sequence) {
		return GameResult{Title: "尚未完成备劫", Content: "天劫不可仓促引动。请先发送 `备劫` 完成当前状态校验；备劫有效期为30分钟。", Actions: []string{"备劫", "状态"}}, true, nil
	}
	rate := current.TribulationBaseRate + float64(effectiveDaoHeart-50)/500 + medicineBonus.TribulationRate + titleTribulationBonus
	if current.TribulationBaseRate <= 0 {
		rate = g.settingFloat("tribulation.base_rate", .7) + float64(effectiveDaoHeart-50)/500 + medicineBonus.TribulationRate + titleTribulationBonus
	}
	mode := tribulationModeText(together, medicineBonus, titleTribulationBonus)
	if together {
		rate += g.settingFloat("tribulation.couple_bonus", .3)
	}
	guardBonus := 0.0
	if _, guardErr := g.playerValue(player.ID, "buff.tribulation_guard"); guardErr == nil {
		guardBonus = .12
		rate += guardBonus
		_ = g.store.DB.Where("player_id = ? AND key = ?", player.ID, "buff.tribulation_guard").Delete(&model.PlayerValue{}).Error
		mode += " · 紫府天灯护劫"
	}
	if rate > .99 {
		rate = .99
	}
	trialLogs := make([]string, 0, len(profile.Trials))
	failedTrial := ""
	failedIndex := -1
	for index, trial := range profile.Trials {
		trialRate := rate - float64(index)*.05
		if trialRate < .15 {
			trialRate = .15
		}
		if trialRate > .99 {
			trialRate = .99
		}
		if rand.Float64() > trialRate {
			failedTrial, failedIndex = trial, index
			trialLogs = append(trialLogs, fmt.Sprintf("第%d劫·%s：失败（%.0f%%）", index+1, trial, trialRate*100))
			break
		}
		trialLogs = append(trialLogs, fmt.Sprintf("第%d劫·%s：通过（%.0f%%）", index+1, trial, trialRate*100))
	}
	_ = g.store.DB.Where("player_id = ? AND key = ?", player.ID, "tribulation.prepared").Delete(&model.PlayerValue{}).Error
	if failedIndex >= 0 {
		lossPercent := int64(20 + failedIndex*10)
		loss := max64(cost*lossPercent/100, 1)
		health := max64(effective.MaxHealth/2, 1)
		err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := consumeNamedItemTx(tx, player.ID, "引劫玉符", 1); err != nil {
				return err
			}
			return tx.Model(player).Updates(map[string]any{"cultivation": gorm.Expr("MAX(cultivation - ?, 0)", loss), "health": health}).Error
		})
		if err != nil {
			return GameResult{}, true, err
		}
		_, _ = g.addPlayerValueInt(player.ID, "stats.tribulation_failures", 1)
		broadcast := fmt.Sprintf("【劫罚】%s引动%s，却在%s一关失守，损失修为%d。诸位道友当以此为鉴，先固道心再叩天劫。", player.DaoName, profile.Name, failedTrial, loss)
		_ = g.publishWorldBroadcast("劫罚", player.DaoName+"渡劫失利", broadcast)
		return GameResult{Title: profile.Name + "未渡", Content: fmt.Sprintf("%s在“%s”一关失守，劫力倒灌经脉。\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n消耗：引劫玉符×1\n护劫加成：%.0f%%\n损失修为：%d（%d%%）\n气血：%d/%d\n境界仍为：%s · 十层\n再次引劫前必须重新备劫。", mode, failedTrial, strings.Join(trialLogs, "\n"), guardBonus*100, loss, lossPercent, health, effective.MaxHealth, current.Name), Actions: []string{"疗伤", "修炼", "合成 引劫玉符", "备劫"}, BroadcastContent: broadcast}, true, nil
	}
	cultivationRequired := realmStageCost(next, model.Realm{})
	var following model.Realm
	if queryErr := g.store.DB.Where("sequence > ?", next.Sequence).Order("sequence").First(&following).Error; queryErr == nil {
		cultivationRequired = realmStageCost(next, following)
	}
	transitioned, err := g.advanceRealmAfterTribulation(player.ID, current, next, cost, cultivationRequired)
	if err != nil {
		return GameResult{}, true, err
	}
	transitionedEffective := g.playerWithActiveSkillStats(&transitioned)
	_, _ = g.addPlayerValueInt(player.ID, "stats.tribulation_successes", 1)
	broadcast := fmt.Sprintf("【境界天赐】%s历经%s三重劫程，由%s十层破境踏入%s一层，天地为之降下玄光。", player.DaoName, profile.Name, current.Name, next.Name)
	_ = g.publishWorldBroadcast("境界", player.DaoName+"破境功成", broadcast)
	return GameResult{Title: profile.Name + "功成", Content: fmt.Sprintf("%s连续渡过三道劫关，道基在毁灭中重铸。\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n境界：%s · 十层 → **%s · 一层**\n消耗：引劫玉符×1\n消耗修为：%d\n护劫加成：%.0f%%\n气血：%d/%d\n法力：%d/%d\n新的大境仍须逐层修满十层。", mode, strings.Join(trialLogs, "\n"), current.Name, next.Name, cost, guardBonus*100, transitionedEffective.Health, transitionedEffective.MaxHealth, transitionedEffective.Mana, transitionedEffective.MaxMana), Actions: []string{"状态", "修炼", "境界查询 " + next.Name}, BroadcastContent: broadcast}, true, nil
}

var errTribulationStateChanged = errors.New("player realm state changed during tribulation settlement")

func (g *Game) advanceRealmAfterTribulation(playerID uint, current, next model.Realm, cost, cultivationRequired int64) (model.Player, error) {
	var transitioned model.Player
	skillBonus := skillStatBonus{}
	if player, err := g.players.Get(playerID); err == nil {
		skillBonus = g.activeSkillStatBonus(&player)
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var player model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, playerID).Error; err != nil {
			return err
		}
		if player.RealmID != current.ID || player.RealmLevel < realmStageCount || player.Cultivation < cost {
			return errTribulationStateChanged
		}
		if err := consumeNamedItemTx(tx, player.ID, "引劫玉符", 1); err != nil {
			return err
		}

		healthDelta := next.BaseHealth - current.BaseHealth
		manaDelta := next.BaseMana - current.BaseMana
		attackDelta := next.BaseAttack - current.BaseAttack
		defenseDelta := next.BaseDefense - current.BaseDefense
		speedDelta := next.BaseSpeed - current.BaseSpeed
		levelFloor := model.PlayerLevelStats(player.Level)
		lifespanDelta := next.BaseLifespan - current.BaseLifespan
		dodgeDelta := next.BaseDodge - current.BaseDodge

		player.RealmID, player.RealmName, player.RealmLevel = next.ID, next.Name, 1
		player.Cultivation -= cost
		player.CultivationRequired = cultivationRequired
		player.MaxHealth = max64(player.MaxHealth+healthDelta, max64(next.BaseHealth, levelFloor.MaxHealth))
		player.Health = min64(max64(player.Health+healthDelta, 1), max64(player.MaxHealth+skillBonus.Health, 1))
		player.MaxMana = max64(player.MaxMana+manaDelta, max64(next.BaseMana, levelFloor.MaxMana))
		player.Mana = min64(max64(player.Mana+manaDelta, 0), max64(player.MaxMana+skillBonus.Mana, 1))
		player.PhysicalAttack = max64(player.PhysicalAttack+attackDelta, max64(next.BaseAttack, levelFloor.PhysicalAttack))
		player.MagicAttack = max64(player.MagicAttack+attackDelta, max64(next.BaseAttack, levelFloor.MagicAttack))
		player.PhysicalDefense = max64(player.PhysicalDefense+defenseDelta, max64(next.BaseDefense, levelFloor.PhysicalDefense))
		player.MagicDefense = max64(player.MagicDefense+defenseDelta, max64(next.BaseDefense, levelFloor.MagicDefense))
		player.Agility = max64(player.Agility+speedDelta, max64(next.BaseSpeed, levelFloor.Agility))
		player.DodgeRate += dodgeDelta
		if player.DodgeRate < next.BaseDodge {
			player.DodgeRate = next.BaseDodge
		}
		player.MaxLifespan = max64(player.MaxLifespan+lifespanDelta, max64(next.BaseLifespan, 1))
		player.Lifespan = min64(max64(player.Lifespan+lifespanDelta, 1), player.MaxLifespan)
		player.CombatPower = calculateCombatPower(player)

		updates := map[string]any{
			"realm_id": player.RealmID, "realm_name": player.RealmName, "realm_level": player.RealmLevel,
			"cultivation": player.Cultivation, "cultivation_required": player.CultivationRequired,
			"health": player.Health, "max_health": player.MaxHealth, "mana": player.Mana, "max_mana": player.MaxMana,
			"physical_attack": player.PhysicalAttack, "magic_attack": player.MagicAttack,
			"physical_defense": player.PhysicalDefense, "magic_defense": player.MagicDefense,
			"agility": player.Agility, "dodge_rate": player.DodgeRate,
			"lifespan": player.Lifespan, "max_lifespan": player.MaxLifespan,
			"combat_power": player.CombatPower,
		}
		updated := tx.Model(&model.Player{}).Where("id = ? AND realm_id = ?", player.ID, current.ID).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errTribulationStateChanged
		}
		transitioned = player
		return nil
	})
	if err != nil {
		return model.Player{}, err
	}
	if err := g.syncPlayerCombatPower(&transitioned); err != nil {
		return model.Player{}, err
	}
	return transitioned, nil
}

func tribulationModeText(together bool, medicineBonus activeItemBonus, titleTribulationBonus float64) string {
	mode := "单人渡劫"
	if together {
		mode = "仙侣共渡"
	}
	if medicineBonus.TribulationRate > 0 || medicineBonus.DaoHeart > 0 {
		mode += fmt.Sprintf(" · 丹药护劫+%.1f%%", medicineBonus.TribulationRate*100)
	}
	if titleTribulationBonus > 0 {
		mode += fmt.Sprintf(" · 称号护劫+%.1f%%", titleTribulationBonus*100)
	}
	return mode
}

func tribulationDaoHeartRequirement(realmSequence int) int64 {
	return min64(50+int64(maxInt(realmSequence-1, 0))/20, 90)
}

func (g *Game) ascend(player *model.Player) (GameResult, bool, error) {
	if player.RealmName == "飞升" {
		return GameResult{Title: "仙门已过", Content: "你已褪去凡胎。飞升并非境界终点，请继续将飞升境逐层修满，再冲击后续诸天道境。", Actions: []string{"状态", "修炼", "突破"}}, true, nil
	}
	if player.RealmName != "大乘" {
		return GameResult{Title: "飞升条件不足", Content: "只有大乘圆满或飞升境修士才能叩问天门。", Actions: []string{"备劫", "修炼"}}, true, nil
	}
	if player.RealmLevel < realmStageCount {
		return GameResult{Title: "飞升条件不足", Content: fmt.Sprintf("大乘境必须达到十层圆满，当前为%d层。", player.RealmLevel), Actions: []string{"突破", "修炼"}}, true, nil
	}
	return g.attemptTribulation(player, false)
}

type tribulationProfile struct {
	Name   string
	Hazard string
	Trials []string
}

func realmTribulationProfile(sequence int) tribulationProfile {
	profiles := []tribulationProfile{
		{"九霄紫雷劫", "九重紫雷逐次淬体，后一道威能远胜前一道", []string{"紫雷炼体", "雷海问心", "天门审道"}},
		{"玄阴蚀魂劫", "玄阴劫气直入识海，专破神魂与旧日执念", []string{"阴风洗魂", "旧忆噬心", "元神归一"}},
		{"赤霄业火劫", "业火会点燃因果与杀业，道心不稳者形神俱损", []string{"业火焚身", "因果照心", "火中生莲"}},
		{"天罡裂空劫", "天罡风刃割裂护体灵光，虚空裂隙封锁退路", []string{"罡风锻骨", "裂空迷途", "踏虚归真"}},
		{"五行灭神劫", "金木水火土五劫相生相克，需随劫势调转灵力", []string{"五行轮转", "逆克问心", "万法归元"}},
		{"无相心魔劫", "心魔化作最深执念，修为越高，幻境越接近真实", []string{"照见妄念", "斩却旧我", "道心无相"}},
		{"虚空裂界劫", "界壁崩裂产生空间乱流，肉身与元神会被强行分离", []string{"界风裂体", "虚海寻魂", "重开天地"}},
		{"星陨镇魂劫", "周天星辰投下镇魂之力，每次星陨都会封禁一处经脉", []string{"星火坠身", "周天锁魂", "命星重明"}},
		{"因果斩业劫", "过往因果化作斩业天刀，未了宿债皆会成为劫力", []string{"因果显化", "斩业偿债", "命线新生"}},
		{"岁月枯荣劫", "寿元在刹那间经历万载枯荣，考验长生道果是否真实", []string{"万载枯身", "一念守真", "岁月回春"}},
		{"混沌归墟劫", "灵力、感知与方位尽归混沌，只能凭本心寻找生门", []string{"诸法沉寂", "归墟问道", "混沌初开"}},
		{"万道寂灭劫", "既有术法暂时失效，唯有自证大道才能重掌法则", []string{"万法皆空", "孤道独行", "一念生万法"}},
	}
	index := (maxInt(sequence, 1) - 1) % len(profiles)
	return profiles[index]
}

func (g *Game) executeMansion(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 59:
		return g.viewMansion(player)
	case 60:
		return g.upgradeMansion(player)
	case 61:
		return g.plantCrop(player, command.RawArguments)
	case 62:
		return g.harvestCrops(player)
	case 63:
		return g.brewBasicPill(player, command.RawArguments, 1)
	case 64:
		return g.upgradeAlchemyRoom(player)
	case 65:
		return g.buildFormation(player)
	case 66:
		return g.tameAtMansion(player)
	case 67:
		return g.mansionWarehouse(player)
	case 68:
		return g.visitMansion(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) getOrCreateMansion(player *model.Player) (model.Mansion, bool, error) {
	var mansion model.Mansion
	err := g.store.DB.Where("player_id = ?", player.ID).First(&mansion).Error
	if err == nil {
		return mansion, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return mansion, false, err
	}
	mansion = model.Mansion{PlayerID: player.ID, Name: player.DaoName + "的仙府", Level: 1, FarmLevel: 1, AlchemyRoomLevel: 1, FormationLevel: 0, BeastRoomLevel: 1, WarehouseLevel: 1, Prosperity: 10, LayoutJSON: `{}`}
	if err := g.store.DB.Create(&mansion).Error; err != nil {
		return mansion, false, err
	}
	_ = g.store.DB.Model(player).Update("mansion_id", mansion.ID).Error
	return mansion, true, nil
}

func (g *Game) viewMansion(player *model.Player) (GameResult, bool, error) {
	mansion, created, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	createdText := ""
	if created {
		createdText = "\n仙府已为你开辟。"
	}
	var growing int64
	g.store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Count(&growing)
	return GameResult{Title: mansion.Name, Content: fmt.Sprintf("仙府等级：%d\n灵田等级：%d\n丹房等级：%d\n阵法等级：%d\n兽室等级：%d\n府库等级：%d\n繁荣度：%d\n生长中灵植：%d%s", mansion.Level, mansion.FarmLevel, mansion.AlchemyRoomLevel, mansion.FormationLevel, mansion.BeastRoomLevel, mansion.WarehouseLevel, mansion.Prosperity, growing, createdText), Actions: []string{"种田", "收获", "炼丹", "升级府"}}, true, nil
}

func (g *Game) upgradeMansion(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	cost := int64(float64(mansion.Level*3) * g.settingFloat("mansion.material_factor", 1))
	if err := g.adjustNamedItem(player.ID, "仙府材料", -cost); err != nil {
		return GameResult{Title: "仙府升级失败", Content: fmt.Sprintf("需要仙府材料×%d。", cost)}, true, nil
	}
	_ = g.store.DB.Model(&mansion).Updates(map[string]any{"level": gorm.Expr("level + 1"), "prosperity": gorm.Expr("prosperity + 20")}).Error
	return GameResult{Title: "仙府升级", Content: fmt.Sprintf("消耗仙府材料×%d\n仙府：%d级 → %d级\n繁荣度：+20", cost, mansion.Level, mansion.Level+1), Actions: []string{"仙府"}}, true, nil
}

func (g *Game) plantCrop(player *model.Player, argument string) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	parts := strings.Fields(strings.TrimSpace(argument))
	requestedPlot := 0
	if len(parts) > 1 {
		if parsed, parseErr := strconv.Atoi(parts[len(parts)-1]); parseErr == nil {
			requestedPlot = parsed
			parts = parts[:len(parts)-1]
		}
	}
	seed := strings.Join(parts, " ")
	if seed == "" {
		return GameResult{Title: "灵田播种", Content: "请先在种子商店购买灵种，再发送 `种田 种子名`。不同种子具有独立生长时间和基础产量。", Actions: []string{"种子商店", "仙府"}}, true, nil
	}
	seedItem, err := g.itemByName(seed)
	if err != nil {
		return GameResult{Title: "种田失败", Content: "没有找到名为“" + seed + "”的灵种，请从种子商店蓝字中选择。", Actions: []string{"种子商店"}}, true, nil
	}
	var seedConfig struct {
		Crop        string `json:"crop"`
		GrowMinutes int    `json:"grow_minutes"`
		Yield       int64  `json:"yield"`
	}
	if seedItem.EffectFunc != "plant_seed" || json.Unmarshal([]byte(seedItem.EffectParams), &seedConfig) != nil || seedConfig.Crop == "" {
		return GameResult{Title: "不能播种", Content: seedItem.Name + "不是灵田种子。请发送 `种子商店` 查看可播种灵种。", Actions: []string{"种子商店"}}, true, nil
	}
	cropItem, err := g.itemByName(seedConfig.Crop)
	if err != nil {
		return GameResult{Title: "种子配置错误", Content: seedItem.Name + "对应的成熟灵植不存在，请管理员检查种子效果参数。"}, true, nil
	}
	if g.itemQuantity(player.ID, seedItem.ID) < 1 {
		return GameResult{Title: "种田失败", Content: "乾坤袋中没有" + seedItem.Name + "。", Actions: []string{"购买种子 " + seedItem.Name, "种子商店"}}, true, nil
	}
	var growing int64
	g.store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Count(&growing)
	maxPlots := maxInt(mansion.FarmLevel*2, 2)
	if growing >= int64(maxPlots) {
		return GameResult{Title: "灵田已满", Content: "请先收获成熟灵植。"}, true, nil
	}
	var active []model.MansionCrop
	_ = g.store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Find(&active).Error
	occupied := make(map[int]bool, len(active))
	for _, row := range active {
		occupied[row.Plot] = true
	}
	plot := requestedPlot
	if plot != 0 && (plot < 1 || plot > maxPlots || occupied[plot]) {
		return GameResult{Title: "地块不可用", Content: fmt.Sprintf("第%d号地块不存在或已有灵植。当前灵田开放%d块土地。", plot, maxPlots), Actions: []string{"土地详情", "灵田"}}, true, nil
	}
	if plot == 0 {
		for candidate := 1; candidate <= maxPlots; candidate++ {
			if !occupied[candidate] {
				plot = candidate
				break
			}
		}
	}
	_ = g.players.AdjustItem(player.ID, seedItem.ID, -1)
	minutes := maxInt(seedConfig.GrowMinutes-mansion.FarmLevel*2, 5)
	now := time.Now()
	yield := max64(seedConfig.Yield+int64(mansion.FarmLevel/2), 1)
	if strings.Contains(player.SpiritualRoot, "木") {
		yield = max64(yield*115/100, yield+1)
	}
	crop := model.MansionCrop{MansionID: mansion.ID, ItemID: cropItem.ID, SeedName: seedItem.Name, Plot: plot, Quantity: yield, Protected: player.ActivePetID != 0, PlantedAt: now, ReadyAt: now.Add(time.Duration(minutes) * time.Minute)}
	if err := g.store.DB.Create(&crop).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵田播种", Content: fmt.Sprintf("灵田：第%d号田垄\n消耗：%s × 1\n生长灵植：%s\n预计成熟：%s\n预计收获：%s × %d\n灵田等级：%d（已缩短生长时间）", crop.Plot, seedItem.Name, cropItem.Name, formatDuration(time.Duration(minutes)*time.Minute), cropItem.Name, crop.Quantity, mansion.FarmLevel), Actions: []string{"仙府", "收获", "种子商店"}}, true, nil
}

func (g *Game) harvestCrops(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var crops []model.MansionCrop
	err = g.store.DB.Where("mansion_id = ? AND harvested = ? AND ready_at <= ?", mansion.ID, false, time.Now()).Find(&crops).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(crops) == 0 {
		var next model.MansionCrop
		if queryErr := g.store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Order("ready_at").First(&next).Error; queryErr == nil {
			return GameResult{Title: "尚未成熟", Content: "最近一株还需" + formatDuration(time.Until(next.ReadyAt)) + "。"}, true, nil
		}
		return GameResult{Title: "无物可收", Content: "灵田中没有作物。", Actions: []string{"种田"}}, true, nil
	}
	var lines []string
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		for _, crop := range crops {
			var item model.Item
			if err := tx.First(&item, crop.ItemID).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).FirstOrCreate(&model.PlayerItem{PlayerID: player.ID, ItemID: item.ID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).Update("quantity", gorm.Expr("quantity + ?", crop.Quantity)).Error; err != nil {
				return err
			}
			if err := tx.Model(&crop).Update("harvested", true).Error; err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("- %s×%d", item.Name, crop.Quantity))
		}
		return tx.Model(&mansion).Update("prosperity", gorm.Expr("prosperity + ?", len(crops))).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	var harvested int64
	for _, crop := range crops {
		harvested += crop.Quantity
	}
	_, _ = g.addPlayerValueInt(player.ID, "farm.harvested", harvested)
	return GameResult{Title: "灵田收获", Content: strings.Join(lines, "\n") + fmt.Sprintf("\n━━━━━━━━━━━\n本次共收获%d株，已收入灵田仓库。", harvested), Actions: []string{"种田", "灵田仓库", "出售灵植", "灵田"}}, true, nil
}

func (g *Game) upgradeAlchemyRoom(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	cost := int64(mansion.AlchemyRoomLevel * 2)
	if err := g.adjustNamedItem(player.ID, "仙府材料", -cost); err != nil {
		return GameResult{Title: "丹房升级失败", Content: fmt.Sprintf("需要仙府材料×%d。", cost)}, true, nil
	}
	_ = g.store.DB.Model(&mansion).Update("alchemy_room_level", gorm.Expr("alchemy_room_level + 1")).Error
	return GameResult{Title: "丹房升级", Content: fmt.Sprintf("丹房：%d级 → %d级\n炼丹成功率提升。", mansion.AlchemyRoomLevel, mansion.AlchemyRoomLevel+1)}, true, nil
}

func (g *Game) buildFormation(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	cost := int64((mansion.FormationLevel + 1) * 2)
	if err := g.adjustNamedItem(player.ID, "仙府材料", -cost); err != nil {
		return GameResult{Title: "布阵失败", Content: fmt.Sprintf("需要仙府材料×%d。", cost)}, true, nil
	}
	_ = g.store.DB.Model(&mansion).Update("formation_level", gorm.Expr("formation_level + 1")).Error
	return GameResult{Title: "护府大阵", Content: fmt.Sprintf("消耗仙府材料×%d\n阵法等级：%d → %d\n仙府防御提升。", cost, mansion.FormationLevel, mansion.FormationLevel+1)}, true, nil
}

func (g *Game) tameAtMansion(player *model.Player) (GameResult, bool, error) {
	var pet model.Pet
	if err := g.store.DB.Where("player_id = ? AND active = ?", player.ID, true).First(&pet).Error; err != nil {
		return GameResult{Title: "驯兽", Content: "当前没有出战灵兽。", Actions: []string{"捕获", "灵兽"}}, true, nil
	}
	remaining, ok, err := g.cooldown(player.ID, "tame_pet", 30*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "驯养休息", Content: "还需" + formatDuration(remaining) + "。"}, true, nil
	}
	_ = g.store.DB.Model(&pet).Update("loyalty", gorm.Expr("MIN(loyalty + 5, 100)")).Error
	return GameResult{Title: "仙府驯兽", Content: fmt.Sprintf("%s的忠诚提升5点。", pet.Name), Actions: []string{"灵兽"}}, true, nil
}

func (g *Game) mansionWarehouse(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙府府库", Content: fmt.Sprintf("府库等级：%d\n容量：%d格\n当前版本中府库与角色乾坤袋共享物品索引，物品不会因切换界面丢失。", mansion.WarehouseLevel, mansion.WarehouseLevel*20), Actions: []string{"背包", "仙府"}}, true, nil
}

func (g *Game) visitMansion(player *model.Player, argument string) (GameResult, bool, error) {
	target, err := g.findPlayer(argument)
	if err != nil {
		return GameResult{Title: "参观仙府", Content: "请输入：`参观 @对方`"}, true, nil
	}
	var mansion model.Mansion
	if err := g.store.DB.Where("player_id = ?", target.ID).First(&mansion).Error; err != nil {
		return GameResult{Title: "洞门紧闭", Content: target.DaoName + "尚未开辟仙府。"}, true, nil
	}
	return GameResult{Title: mansion.Name, Content: fmt.Sprintf("主人：%s\n等级：%d\n灵田：%d\n丹房：%d\n阵法：%d\n繁荣：%d", target.DaoName, mansion.Level, mansion.FarmLevel, mansion.AlchemyRoomLevel, mansion.FormationLevel, mansion.Prosperity)}, true, nil
}

func (g *Game) brewBasicPill(player *model.Player, recipeName string, quantity int64) (GameResult, bool, error) {
	if quantity < 1 {
		quantity = 1
	}
	if strings.TrimSpace(recipeName) == "" {
		recipeName = "回元散"
	}
	var recipe model.AlchemyRecipe
	if err := g.store.DB.Where("name = ? AND enabled = ?", strings.TrimSpace(recipeName), true).First(&recipe).Error; err != nil {
		return GameResult{Title: "丹方不存在", Content: "没有找到“" + strings.TrimSpace(recipeName) + "”，请从丹方列表蓝字中选择。", Actions: []string{"丹方"}}, true, nil
	}
	materials := make(map[string]int64)
	if err := json.Unmarshal([]byte(recipe.MaterialsJSON), &materials); err != nil || len(materials) == 0 {
		return GameResult{Title: "丹方道纹紊乱", Content: recipe.Name + "的材料道纹无法解析，本次没有扣除药材，请主人检查丹方配置。"}, true, nil
	}
	missing := []string{}
	scaledMaterials := make(map[string]int64, len(materials))
	for name, amount := range materials {
		if amount <= 0 || quantity > math.MaxInt64/amount {
			return GameResult{Title: "炼丹数量过大", Content: "材料总量超过安全整数范围，请拆分本次炼制。", Actions: []string{"丹方", "背包"}}, true, nil
		}
		item, itemErr := g.itemByName(name)
		need := amount * quantity
		scaledMaterials[name] = need
		owned := int64(0)
		if itemErr == nil {
			owned = g.itemQuantity(player.ID, item.ID)
		}
		if itemErr != nil || owned < need {
			missing = append(missing, fmt.Sprintf("%s需要%d，现有%d", name, need, owned))
		}
	}
	if len(missing) > 0 {
		return GameResult{Title: "炼丹材料不足", Content: fmt.Sprintf("丹方：%s\n缺少：\n- %s\n材料可通过地图采集、灵田种植、首领掉落或集市获得。", recipe.Name, strings.Join(missing, "\n- ")), Actions: []string{"灵田", "地图", "集市", "丹方"}}, true, nil
	}
	mansion, _, _ := g.getOrCreateMansion(player)
	baseRate := recipe.SuccessRate + float64(maxInt(mansion.AlchemyRoomLevel-1, 0))*.015
	guaranteed := recipe.SuccessRate >= 1
	titleBonus := 0.0
	if guaranteed {
		baseRate = 1
	} else if baseRate > .98 {
		baseRate = .98
	}
	if !guaranteed {
		titleBonus = g.activeTitleProbabilityBonus(player, "alchemy_percent", "pill_percent")
		if baseRate+titleBonus > .98 {
			titleBonus = maxFloat(.98-baseRate, 0)
		}
	}
	rate, luckBonus := probabilityWithLuck(baseRate+titleBonus, player.Luck, luckAlchemyBonusCap)
	if guaranteed {
		rate = 1
		luckBonus = 0
	} else if rate > .98 {
		rate = .98
		luckBonus = rate - baseRate
	}
	success := synthesisSuccessCount(quantity, rate)
	var output model.Item
	if err := g.store.DB.Where("id = ? OR name = ?", recipe.OutputItemID, recipe.OutputName).Order("id").First(&output).Error; err != nil {
		return GameResult{Title: "丹方配置错误", Content: "产物“" + recipe.OutputName + "”不存在，请管理员检查丹方关联。"}, true, nil
	}
	if ownedOutput := g.itemQuantity(player.ID, output.ID); success > math.MaxInt64-ownedOutput {
		return GameResult{Title: "成丹数量过大", Content: "产物总数超过乾坤袋可安全记录范围，本次没有扣除材料。", Actions: []string{"背包", "丹方"}}, true, nil
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		repo := storage.NewPlayerRepository(tx)
		for name, amount := range scaledMaterials {
			var item model.Item
			if err := tx.Where("name = ?", name).First(&item).Error; err != nil {
				return err
			}
			if err := repo.AdjustItem(player.ID, item.ID, -amount); err != nil {
				return err
			}
		}
		if success > 0 {
			return repo.AdjustItem(player.ID, output.ID, success)
		}
		return nil
	}); err != nil {
		return GameResult{}, true, err
	}
	if success > 0 {
		_, _ = g.addPlayerValueInt(player.ID, "stats.alchemy", success)
	}
	materialText := make([]string, 0, len(materials))
	actions := []string{"使用 " + output.Name, "药效 " + output.Name, "物品 " + output.Name, "背包", "丹方", "炼药 " + recipe.Name}
	for name, amount := range scaledMaterials {
		materialText = append(materialText, fmt.Sprintf("%s×%d", name, amount))
		actions = append(actions, "物品 "+name)
	}
	sort.Strings(materialText)
	totalMedicine := "本炉没有成丹，因此没有可服用药力。"
	if success > 0 {
		totalMedicine = itemEffectSummary(output, success)
	}
	return GameResult{Title: "丹火凝药", Content: fmt.Sprintf("丹方：%s\n丹房：%d级\n━━━━━━━━━━━\n投入材料：%s\n开炉：%d次\n成功率：基础与丹房%.1f%% · 称号+%.1f%% · 运气+%.1f%% · 实际%.1f%%\n成丹：%d · 失败：%d\n获得：%s×%d\n━━━━━━━━━━━\n单颗药效：%s\n本炉总药力：%s\n丹诀说明：%s\n━━━━━━━━━━━\n每份丹方独立判定；炼制数量不设玩法上限，只受材料库存与安全整数范围约束。", recipe.Name, mansion.AlchemyRoomLevel, strings.Join(materialText, "、"), quantity, baseRate*100, titleBonus*100, luckBonus*100, rate*100, success, quantity-success, output.Name, success, itemEffectSummary(output, 1), totalMedicine, recipe.Description), Actions: actions}, true, nil
}

var _ = rand.Intn
