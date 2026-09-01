package service

import (
	"encoding/json"
	"errors"
	"fmt"
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

const (
	activitySevenGoals       = "xianchen_activity_seven_goals"
	activityRealmSprint      = "xianchen_activity_realm_sprint"
	activitySevenBenefit     = "xianchen_activity_seven_benefits"
	activityOpeningCodes     = "xianchen_activity_opening_codes"
	activityCodeQuests       = "xianchen_activity_code_quests"
	activityInvitation       = "xianchen_activity_invitation"
	activityRookieRank       = "xianchen_activity_rookie_rank"
	activityFortune          = "xianchen_activity_fortune"
	activityPrayer           = "xianchen_activity_prayer"
	activityFestivalSale     = "xianchen_activity_festival_sale"
	activityV221Compensation = "xianchen_activity_v221_compensation"
)

var (
	errActivityClaimed = errors.New("activity reward already claimed")
	errActivityClosed  = errors.New("activity is not active")
)

type activityReward struct {
	Cultivation      int64
	SpiritStones     int64
	SilverCoins      int64
	ImmortalJade     int64
	ArenaCoins       int64
	Merit            int64
	Reputation       int64
	DaoHeart         int64
	ImmortalAffinity int64
	Items            map[string]int64
}

func (g *Game) executeActivity(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 1086:
		return g.activityMenu(player), true, nil
	case 1087:
		if command.Spec.Command == "领取七日目标" {
			return g.claimSevenDayGoal(player, command.RawArguments)
		}
		return g.sevenDayGoals(player, command.RawArguments)
	case 1088:
		if command.Spec.Command == "领取境界冲刺" {
			return g.claimRealmSprint(player, command.RawArguments)
		}
		return g.realmSprint(player, command.RawArguments)
	case 1089:
		if command.Spec.Command == "领取七日福利" {
			return g.claimSevenDayBenefit(player, command.RawArguments)
		}
		return g.sevenDayBenefits(player)
	case 1090:
		return g.openingCodeMenu(player), true, nil
	case 1091:
		return g.redeemActivityCode(player, command.RawArguments)
	case 1092:
		return g.limitedBenefitCodes(player)
	case 1093:
		return g.secretCodeQuests(player)
	case 1094:
		return g.daoistRecruitmentMenu(player), true, nil
	case 1095:
		if command.Spec.Command == "接受邀请" {
			return g.acceptActivityInvitation(player, command.RawArguments)
		}
		return g.inviteDaoist(player)
	case 1096:
		return g.companionInvitationRewards(player, command.RawArguments)
	case 1097:
		return g.assistDaoistCultivation(player, command.RawArguments)
	case 1098:
		return g.rookieRanking(player, command.RawArguments, command.Spec.Command == "领取新秀奖励")
	case 1099:
		return g.festivalMenu(player), true, nil
	case 1100:
		return g.heavenlyFortune(player)
	case 1101:
		return g.limitedPrayer(player, command.RawArguments)
	case 1102:
		if command.Spec.Command == "庆典购买" {
			return g.buyFestivalGood(player, command.Arguments)
		}
		return g.festivalSale(player, command.RawArguments)
	case 1103:
		return g.activityOverview(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) activityMenu(player *model.Player) GameResult {
	active, upcoming, ended := g.activityStateCounts()
	content := fmt.Sprintf("道友：%s · %s%d层\n当前活动：进行中%d项 · 即将开启%d项 · 已结束%d项\n━━━━━━━━━━━\n🎁 版本补偿\n全服补偿 ┆ 补偿公告\n领取全服补偿\n\n🎯 七日成长\n七日目标 ┆ 境界冲刺\n七日福利 ┆ 活动总览\n\n🔑 开服密令\n密令兑换 ┆ 限时福利码\n密令任务 ┆ 开服密令\n\n👥 道友召集\n邀请道友 ┆ 结伴奖励\n助力修炼 ┆ 新秀榜\n\n🎊 庆典专属\n天降鸿运 ┆ 限时祈福\n庆典特卖 ┆ 活动总览\n━━━━━━━━━━━\n所有领取、兑换与邀请绑定均实时入账且不可重复；活动总览可查看开放时间、剩余时间和个人参与状态。", player.DaoName, player.RealmName, player.RealmLevel, active, upcoming, ended)
	actions := []string{"全服补偿", "补偿公告", "领取全服补偿", "七日目标", "境界冲刺", "七日福利", "开服密令", "密令兑换", "限时福利码", "密令任务", "道友召集", "邀请道友", "结伴奖励", "助力修炼", "新秀榜", "庆典专属", "天降鸿运", "限时祈福", "庆典特卖", "活动总览"}
	return GameResult{Title: "仙尘活动中心", Content: content, Actions: actions}
}

func (g *Game) activityStateCounts() (int, int, int) {
	var rows []model.Activity
	_ = g.store.DB.Where("code LIKE ?", "xianchen_activity_%").Find(&rows).Error
	active, upcoming, ended := 0, 0, 0
	for _, row := range rows {
		switch activityState(row, time.Now()) {
		case "进行中":
			active++
		case "即将开启":
			upcoming++
		default:
			ended++
		}
	}
	return active, upcoming, ended
}

func activityState(row model.Activity, now time.Time) string {
	status := strings.TrimSpace(row.Status)
	if status == "停用" || status == "关闭" || status == "已停用" {
		return "已关闭"
	}
	if !row.StartsAt.IsZero() && now.Before(row.StartsAt) {
		return "即将开启"
	}
	if !row.EndsAt.IsZero() && !now.Before(row.EndsAt) {
		return "已结束"
	}
	return "进行中"
}

func activityWindowText(row model.Activity, now time.Time) string {
	state := activityState(row, now)
	switch state {
	case "进行中":
		return "剩余" + preciseActivityDuration(time.Until(row.EndsAt))
	case "即将开启":
		return preciseActivityDuration(time.Until(row.StartsAt)) + "后开启"
	case "已关闭":
		return "活动已暂停"
	default:
		return "已于" + row.EndsAt.Format("01月02日 15:04") + "结束"
	}
}

func preciseActivityDuration(value time.Duration) string {
	if value <= 0 {
		return "不足一分钟"
	}
	days := int(value.Hours()) / 24
	hours := int(value.Hours()) % 24
	minutes := int(value.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", maxInt(minutes, 1))
}

func (g *Game) requireActiveActivity(code string) (model.Activity, error) {
	var row model.Activity
	if err := g.store.DB.Where("code = ?", code).First(&row).Error; err != nil {
		return row, err
	}
	if activityState(row, time.Now()) != "进行中" {
		return row, errActivityClosed
	}
	return row, nil
}

func activityUnavailableResult(row model.Activity, title string) (GameResult, bool, error) {
	state := activityState(row, time.Now())
	content := fmt.Sprintf("当前状态：%s\n开放：%s\n结束：%s\n%s", state, row.StartsAt.Format("2006-01-02 15:04"), row.EndsAt.Format("2006-01-02 15:04"), activityWindowText(row, time.Now()))
	return GameResult{Title: title + "暂不可参与", Content: content, Actions: []string{"活动总览", "活动菜单"}}, true, nil
}

func (g *Game) claimActivityReward(playerID uint, key, value string, reward activityReward) error {
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", playerID, key).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errActivityClaimed
		}
		marker := model.PlayerValue{PlayerID: playerID, Key: key, Value: value}
		if err := tx.Create(&marker).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return errActivityClaimed
			}
			return err
		}
		return grantActivityRewardTx(tx, playerID, reward)
	})
}

func grantActivityRewardTx(tx *gorm.DB, playerID uint, reward activityReward) error {
	updates := map[string]any{}
	for _, field := range []struct {
		name  string
		value int64
	}{
		{"cultivation", reward.Cultivation}, {"spirit_stones", reward.SpiritStones},
		{"silver_coins", reward.SilverCoins}, {"immortal_jade", reward.ImmortalJade},
		{"arena_coins", reward.ArenaCoins}, {"merit", reward.Merit},
		{"reputation", reward.Reputation}, {"dao_heart", reward.DaoHeart},
		{"immortal_affinity", reward.ImmortalAffinity},
	} {
		if field.value != 0 {
			updates[field.name] = gorm.Expr(field.name+" + ?", field.value)
		}
	}
	if len(updates) > 0 {
		if err := tx.Model(&model.Player{}).Where("id = ?", playerID).Updates(updates).Error; err != nil {
			return err
		}
	}
	repo := storage.NewPlayerRepository(tx)
	for itemName, quantity := range reward.Items {
		if quantity <= 0 {
			continue
		}
		var item model.Item
		if err := tx.Where("name = ?", itemName).First(&item).Error; err != nil {
			return fmt.Errorf("活动奖励物品%s未配置: %w", itemName, err)
		}
		if err := repo.AdjustItem(playerID, item.ID, quantity); err != nil {
			return err
		}
	}
	return nil
}

func activityRewardText(reward activityReward) string {
	parts := make([]string, 0, 12)
	for _, row := range []struct {
		name  string
		value int64
	}{
		{"修为", reward.Cultivation}, {"灵石", reward.SpiritStones}, {"银币", reward.SilverCoins},
		{"仙金", reward.ImmortalJade}, {"竞技币", reward.ArenaCoins}, {"功德", reward.Merit},
		{"声望", reward.Reputation}, {"道心", reward.DaoHeart}, {"仙缘", reward.ImmortalAffinity},
	} {
		if row.value != 0 {
			parts = append(parts, fmt.Sprintf("%s%+d", row.name, row.value))
		}
	}
	for name, quantity := range reward.Items {
		if quantity > 0 {
			parts = append(parts, fmt.Sprintf("%s×%d", name, quantity))
		}
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, "、")
}

func (g *Game) countPlayerValuePrefix(playerID uint, prefix string) int64 {
	var count int64
	_ = g.store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key LIKE ?", playerID, prefix+"%").Count(&count).Error
	return count
}

type sevenDayGoalDefinition struct {
	ID, Name, Description, ProgressType, ProgressKey, Action string
	Day                                                      int
	Target                                                   int64
	Reward                                                   activityReward
}

var sevenDayGoalDefinitions = []sevenDayGoalDefinition{
	{"dao_record", "道籍初成", "完成入道并拥有全服唯一道号。", "registered", "", "状态", 1, 1, activityReward{SilverCoins: 60, Items: map[string]int64{"灵果": 2}}},
	{"first_explore", "山门问路", "完成一次大世界探索，认识当前地点。", "stat", "stats.explores", "探索", 1, 1, activityReward{SpiritStones: 80, Items: map[string]int64{"仙露": 1}}},
	{"first_win", "初试锋芒", "在逐回合战斗中击败一只妖灵。", "stat", "stats.wins", "位置", 1, 1, activityReward{Cultivation: 30, SilverCoins: 40}},
	{"quiet_cultivation", "静室凝神", "累计完成半个时辰的有效闭关。", "stat", "stats.cultivation_minutes", "修炼", 2, 30, activityReward{Cultivation: 60, Items: map[string]int64{"灵茶": 2}}},
	{"herb_friend", "草木知心", "在地图或灵田完成两次采集收获。", "sum", "stats.collects,farm.harvested", "位置", 2, 2, activityReward{SpiritStones: 100, Items: map[string]int64{"凝露草籽": 2}}},
	{"learn_skill", "道法初窥", "学会至少一门可在战斗中施展的功法。", "skills", "", "功法", 2, 1, activityReward{SilverCoins: 80, Items: map[string]int64{"功法残卷": 1}}},
	{"alchemy_flame", "丹炉生烟", "完成一次有效炼丹。", "stat", "stats.alchemy", "丹方", 3, 1, activityReward{SpiritStones: 120, Items: map[string]int64{"灵茶": 2}}},
	{"farm_sprout", "灵田新芽", "从自己的灵田收获一株成熟灵植。", "stat", "farm.harvested", "灵田", 3, 1, activityReward{SilverCoins: 100, Items: map[string]int64{"云雾茶籽": 2}}},
	{"demon_hunter", "妖丹满囊", "累计赢得五场逐回合战斗。", "stat", "stats.wins", "位置", 3, 5, activityReward{Cultivation: 80, Items: map[string]int64{"妖兽内丹": 2}}},
	{"mansion_foundation", "洞天安基", "开辟仙府，使修行拥有长久根基。", "mansion", "", "仙府", 4, 1, activityReward{SpiritStones: 180, Items: map[string]int64{"仙府材料": 2}}},
	{"first_forge", "百炼成锋", "完成一次真实装备锻造。", "stat", "stats.forges", "装备系统", 4, 1, activityReward{SilverCoins: 120, Items: map[string]int64{"玄铁": 3}}},
	{"arena_peer", "问剑同辈", "赢得一场玩家逐回合竞技。", "stat", "stats.arena_wins", "竞技", 4, 1, activityReward{ArenaCoins: 20, Items: map[string]int64{"仙露": 2}}},
	{"meet_friend", "同道相逢", "成功邀请一名新道友结伴入道。", "invites", "", "邀请道友", 5, 1, activityReward{SilverCoins: 150, ImmortalAffinity: 2}},
	{"heart_growth", "道心日进", "将道心稳固至五十五。", "field", "dao_heart", "道心", 5, 55, activityReward{Cultivation: 100, Items: map[string]int64{"清心丹": 1}}},
	{"leyline_meditation", "灵脉入定", "在修仙界灵脉累计打坐十分钟。", "stat", "stats.leyline_minutes", "寻脉", 5, 10, activityReward{SpiritStones: 160, Items: map[string]int64{"灵果": 3}}},
	{"dungeon_clear", "秘境破阵", "亲自完成一次副本逐回合通关。", "stat", "stats.dungeons", "副本", 6, 1, activityReward{Cultivation: 120, Items: map[string]int64{"扫荡券": 1}}},
	{"boss_victory", "镇域扬名", "击败一位地图区域首领。", "stat", "stats.boss_wins", "首领", 6, 1, activityReward{Merit: 8, Items: map[string]int64{"避劫符": 1}}},
	{"sect_merit", "宗门有功", "完成一次宗门巡查任务。", "stat", "stats.sect_patrol", "宗务", 6, 1, activityReward{SilverCoins: 160, Reputation: 5}},
	{"seven_active", "七曜圆成", "累计完成十五项七日问道目标。", "goal_claims", "", "七日目标", 7, 15, activityReward{SilverCoins: 300, Items: map[string]int64{"双倍修为卡": 1}}},
	{"journey_active", "仙途不息", "探索、战斗、副本与采集累计活跃二十次。", "sum", "stats.explores,stats.wins,stats.dungeons,stats.collects", "活动总览", 7, 20, activityReward{SpiritStones: 300, Merit: 10}},
	{"minor_cycle", "周天圆满", "当前大境至少修至第二层。", "realm_level", "", "突破", 7, 2, activityReward{Cultivation: 150, Items: map[string]int64{"淬脉丹": 1}}},
}

func playerActivityDay(player *model.Player) int {
	days := int(time.Since(player.CreatedAt).Hours()/24) + 1
	return minInt(maxInt(days, 1), 7)
}

func (g *Game) sevenDayGoalProgress(player *model.Player, goal sevenDayGoalDefinition) int64 {
	switch goal.ProgressType {
	case "registered":
		return 1
	case "stat":
		return g.playerValueInt(player.ID, goal.ProgressKey, 0)
	case "sum":
		var total int64
		for _, key := range strings.Split(goal.ProgressKey, ",") {
			total += g.playerValueInt(player.ID, key, 0)
		}
		return total
	case "skills":
		var count int64
		_ = g.store.DB.Model(&model.PlayerSkill{}).Where("player_id = ?", player.ID).Count(&count).Error
		return count
	case "mansion":
		if player.MansionID > 0 {
			return 1
		}
	case "invites":
		return g.successfulInvitationCount(player.ID)
	case "field":
		if goal.ProgressKey == "dao_heart" {
			return player.DaoHeart
		}
	case "goal_claims":
		return g.countPlayerValuePrefix(player.ID, "activity.goal.")
	case "realm_level":
		return int64(player.RealmLevel)
	}
	return 0
}

func (g *Game) sevenDayGoals(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activitySevenGoals)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "七日目标")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 6
	currentDay := playerActivityDay(player)
	page := (currentDay-1)*3/pageSize + 1
	if strings.TrimSpace(raw) != "" {
		page = maxInt(int(parsePositiveInt(raw, int64(page))), 1)
	}
	pages := (len(sevenDayGoalDefinitions) + pageSize - 1) / pageSize
	page = minInt(page, pages)
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(sevenDayGoalDefinitions))
	lines := []string{fmt.Sprintf("入道第%d日 · 第%d/%d页 · %s", currentDay, page, pages, activityWindowText(activity, time.Now())), "目标按入道日从低到高逐步解锁，高阶目标不会提前压给新人。", "━━━━━━━━━━━"}
	actions := []string{"活动菜单", "七日福利"}
	for _, goal := range sevenDayGoalDefinitions[start:end] {
		progress := g.sevenDayGoalProgress(player, goal)
		claimed := g.playerValueExists(player.ID, "activity.goal."+goal.ID)
		state := "进行中"
		if goal.Day > currentDay {
			state = "待解锁"
		} else if claimed {
			state = "已领取"
		} else if progress >= goal.Target {
			state = "可领取"
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n%s\n进度：%d/%d · 奖励：%s", state, goal.Name, goal.Description, min64(progress, goal.Target), goal.Target, activityRewardText(goal.Reward)))
		if state == "可领取" {
			actions = append(actions, "领取七日目标 "+goal.Name)
		} else if state == "进行中" {
			actions = append(actions, goal.Action)
		}
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("七日目标 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("七日目标 %d", page+1))
	}
	return GameResult{Title: "七曜问道目标", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) claimSevenDayGoal(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activitySevenGoals)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "七日目标")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	name := strings.TrimSpace(raw)
	var selected *sevenDayGoalDefinition
	for index := range sevenDayGoalDefinitions {
		if sevenDayGoalDefinitions[index].Name == name {
			selected = &sevenDayGoalDefinitions[index]
			break
		}
	}
	if selected == nil {
		return GameResult{Title: "目标不存在", Content: "请从七日目标页面点击可领取的目标名，不需要输入编号。", Actions: []string{"七日目标"}}, true, nil
	}
	if selected.Day > playerActivityDay(player) {
		return GameResult{Title: "目标尚未解锁", Content: fmt.Sprintf("%s将在入道第%d日解锁。当前是第%d日，请先完成已开放目标。", selected.Name, selected.Day, playerActivityDay(player)), Actions: []string{"七日目标"}}, true, nil
	}
	if time.Now().After(player.CreatedAt.AddDate(0, 0, 14)) {
		return GameResult{Title: "七日目标补领期已结束", Content: "七日目标在入道前七日解锁，并保留至第十四日补领。", Actions: []string{"活动总览", "境界冲刺"}}, true, nil
	}
	progress := g.sevenDayGoalProgress(player, *selected)
	if progress < selected.Target {
		return GameResult{Title: "目标尚未完成", Content: fmt.Sprintf("目标：%s\n当前进度：%d/%d\n还需：%d\n完成方式：%s", selected.Name, progress, selected.Target, selected.Target-progress, selected.Description), Actions: []string{selected.Action, "七日目标"}}, true, nil
	}
	err = g.claimActivityReward(player.ID, "activity.goal."+selected.ID, selected.Name, selected.Reward)
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "目标奖励已领取", Content: selected.Name + "不可重复领取。", Actions: []string{"七日目标"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "七日目标达成", Content: fmt.Sprintf("目标：%s\n进度：%d/%d\n━━━━━━━━━━━\n获得：%s\n奖励已经写入道籍与乾坤袋。", selected.Name, progress, selected.Target, activityRewardText(selected.Reward)), Actions: []string{"七日目标", "背包", "状态"}}, true, nil
}

type sevenDayBenefitDefinition struct {
	ID, Name string
	Day      int
	Reward   activityReward
}

var sevenDayBenefitDefinitions = []sevenDayBenefitDefinition{
	{"guidance", "青云引路礼", 1, activityReward{SpiritStones: 120, SilverCoins: 80, Items: map[string]int64{"灵果": 3, "仙露": 2}}},
	{"meridian", "洗尘养脉礼", 2, activityReward{Cultivation: 60, Items: map[string]int64{"淬脉丹": 1, "灵茶": 2}}},
	{"spring", "灵田播春礼", 3, activityReward{SilverCoins: 120, Items: map[string]int64{"凝露草籽": 3, "云雾茶籽": 2}}},
	{"forge", "百炼开炉礼", 4, activityReward{SpiritStones: 180, Items: map[string]int64{"玄铁": 5, "星辰砂": 2}}},
	{"companion", "同道问心礼", 5, activityReward{SilverCoins: 160, ImmortalAffinity: 3, Items: map[string]int64{"清心丹": 1}}},
	{"dungeon", "秘境护行礼", 6, activityReward{Merit: 6, Items: map[string]int64{"扫荡券": 2, "避劫符": 1}}},
	{"completion", "七星圆满礼", 7, activityReward{SpiritStones: 500, SilverCoins: 300, Items: map[string]int64{"双倍修为卡": 1, "功法残卷": 2}}},
}

func (g *Game) sevenDayBenefits(player *model.Player) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activitySevenBenefit)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "七日福利")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	day := playerActivityDay(player)
	lines := []string{fmt.Sprintf("当前入道第%d日 · %s", day, activityWindowText(activity, time.Now())), "福利按入道天数逐份解锁，漏领可在入道第十四日前补领。", "━━━━━━━━━━━"}
	actions := []string{"领取七日福利", "七日目标", "活动菜单"}
	for _, benefit := range sevenDayBenefitDefinitions {
		state := "待解锁"
		if g.playerValueExists(player.ID, "activity.benefit."+benefit.ID) {
			state = "已领取"
		} else if benefit.Day <= day {
			state = "可领取"
			actions = append(actions, "领取七日福利 "+benefit.Name)
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n解锁：入道第%s日 · %s", state, benefit.Name, chineseOrdinal(benefit.Day), activityRewardText(benefit.Reward)))
	}
	return GameResult{Title: "青云七曜福利", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func chineseOrdinal(value int) string {
	values := []string{"一", "二", "三", "四", "五", "六", "七", "八", "九", "十"}
	if value >= 1 && value <= len(values) {
		return values[value-1]
	}
	return strconv.Itoa(value)
}

func (g *Game) claimSevenDayBenefit(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activitySevenBenefit)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "七日福利")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if time.Now().After(player.CreatedAt.AddDate(0, 0, 14)) {
		return GameResult{Title: "七日福利补领期已结束", Content: "未领取福利保留至入道第十四日，之后福缘归入天地。", Actions: []string{"活动总览"}}, true, nil
	}
	name := strings.TrimSpace(raw)
	var selected *sevenDayBenefitDefinition
	for index := range sevenDayBenefitDefinitions {
		row := &sevenDayBenefitDefinitions[index]
		if name == row.Name || (name == "" && row.Day <= playerActivityDay(player) && !g.playerValueExists(player.ID, "activity.benefit."+row.ID)) {
			selected = row
			break
		}
	}
	if selected == nil {
		return GameResult{Title: "暂无可领福利", Content: "当前已解锁的七日福利均已领取，下一份会随入道天数自动解锁。", Actions: []string{"七日福利", "七日目标"}}, true, nil
	}
	if selected.Day > playerActivityDay(player) {
		return GameResult{Title: "福利尚未解锁", Content: fmt.Sprintf("%s将在入道第%s日解锁。", selected.Name, chineseOrdinal(selected.Day)), Actions: []string{"七日福利"}}, true, nil
	}
	err = g.claimActivityReward(player.ID, "activity.benefit."+selected.ID, selected.Name, selected.Reward)
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "福利已经领取", Content: selected.Name + "每名道友仅能领取一次。", Actions: []string{"七日福利"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "七日福利已领取", Content: fmt.Sprintf("福缘：%s\n获得：%s\n━━━━━━━━━━━\n物品已进入乾坤袋，货币与修为已写入道籍。", selected.Name, activityRewardText(selected.Reward)), Actions: []string{"七日福利", "背包", "货币"}}, true, nil
}

type realmSprintMilestone struct {
	Key, Name string
	Sequence  int
	Level     int
	Reward    activityReward
}

func (g *Game) realmSprintMilestones() ([]realmSprintMilestone, error) {
	var realms []model.Realm
	if err := g.store.DB.Order("sequence,id").Find(&realms).Error; err != nil {
		return nil, err
	}
	milestones := make([]realmSprintMilestone, 0, len(realms)+4)
	for _, realm := range realms {
		levels := []int{10}
		suffixes := []string{"十层圆满"}
		if realm.Sequence == 1 {
			levels = []int{2, 4, 6, 8, 10}
			suffixes = []string{"引气成周", "经脉初定", "灵台澄明", "道基将成", "十层圆满"}
		}
		for index, level := range levels {
			cultivation := max64(realm.RequiredCultivation/200, int64(20+level*5))
			cultivation = min64(cultivation, 50000)
			silver := min64(int64(50+realm.Sequence*8+level*4), 20000)
			item := "破境丹"
			if realm.Sequence == 1 && level <= 4 {
				item = "灵果"
			} else if realm.Sequence <= 2 || level <= 6 {
				item = "淬脉丹"
			} else if realm.Sequence <= 10 || level <= 8 {
				item = "凝元丹"
			}
			milestones = append(milestones, realmSprintMilestone{
				Key: fmt.Sprintf("%d.%d", realm.Sequence, level), Name: realm.Name + "·" + suffixes[index],
				Sequence: realm.Sequence, Level: level,
				Reward: activityReward{Cultivation: cultivation, SilverCoins: silver, Merit: int64(1 + realm.Sequence/25), Items: map[string]int64{item: 1}},
			})
		}
	}
	return milestones, nil
}

func (g *Game) reachedRealmMilestone(player *model.Player, milestone realmSprintMilestone) bool {
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return false
	}
	return sequence > milestone.Sequence || sequence == milestone.Sequence && player.RealmLevel >= milestone.Level
}

func (g *Game) realmSprint(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityRealmSprint)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "境界冲刺")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	milestones, err := g.realmSprintMilestones()
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 6
	page := 1
	for index, milestone := range milestones {
		if !g.reachedRealmMilestone(player, milestone) {
			page = index/pageSize + 1
			break
		}
	}
	if strings.TrimSpace(raw) != "" {
		page = maxInt(int(parsePositiveInt(raw, int64(page))), 1)
	}
	pages := maxInt((len(milestones)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start := minInt((page-1)*pageSize, len(milestones))
	end := minInt(start+pageSize, len(milestones))
	lines := []string{fmt.Sprintf("当前：%s·%d层 · 第%d/%d页 · 共%d道里程碑", player.RealmName, player.RealmLevel, page, pages, len(milestones)), "里程碑严格依照境界图鉴从最低到最高排列；默认页自动定位当前下一目标。", "━━━━━━━━━━━"}
	actions := []string{"活动菜单", "修炼", "突破"}
	for _, milestone := range milestones[start:end] {
		claimed := g.playerValueExists(player.ID, "activity.realm."+milestone.Key)
		state := "未达成"
		if claimed {
			state = "已领取"
		} else if g.reachedRealmMilestone(player, milestone) {
			state = "可领取"
			actions = append(actions, "领取境界冲刺 "+milestone.Name)
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n条件：抵达%s·%d层\n奖励：%s", state, milestone.Name, strings.Split(milestone.Name, "·")[0], milestone.Level, activityRewardText(milestone.Reward)))
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("境界冲刺 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("境界冲刺 %d", page+1))
	}
	return GameResult{Title: "万境登仙冲刺", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) claimRealmSprint(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityRealmSprint)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "境界冲刺")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	milestones, err := g.realmSprintMilestones()
	if err != nil {
		return GameResult{}, true, err
	}
	name := strings.TrimSpace(raw)
	var selected *realmSprintMilestone
	for index := range milestones {
		if milestones[index].Name == name {
			selected = &milestones[index]
			break
		}
	}
	if selected == nil {
		return GameResult{Title: "里程碑不存在", Content: "请从境界冲刺页面点击里程碑名称领取，不需要输入境界编号。", Actions: []string{"境界冲刺"}}, true, nil
	}
	if !g.reachedRealmMilestone(player, *selected) {
		return GameResult{Title: "境界尚未达成", Content: fmt.Sprintf("目标：%s\n需要抵达第%d境·%d层\n当前：%s·%d层\n必须逐层修至圆满，不能越境领取。", selected.Name, selected.Sequence, selected.Level, player.RealmName, player.RealmLevel), Actions: []string{"修炼", "突破", "境界冲刺"}}, true, nil
	}
	err = g.claimActivityReward(player.ID, "activity.realm."+selected.Key, selected.Name, selected.Reward)
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "冲刺奖励已领取", Content: selected.Name + "的里程碑奖励不可重复领取。", Actions: []string{"境界冲刺"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "境界里程碑达成", Content: fmt.Sprintf("里程碑：%s\n获得：%s\n━━━━━━━━━━━\n下一境仍需逐层修炼并准备对应突破丹药。", selected.Name, activityRewardText(selected.Reward)), Actions: []string{"境界冲刺", "背包", "修炼", "突破"}}, true, nil
}

func (g *Game) openingCodeMenu(player *model.Player) GameResult {
	redeemed := g.countPlayerValuePrefix(player.ID, "activity.code.")
	content := fmt.Sprintf("道友：%s · 已成功兑换%d道密令\n━━━━━━━━━━━\n【密令兑换】输入完整密令，验证有效期、全服次数与个人领取记录后一次性发奖。\n【限时福利码】查看当前公开、仍在有效期内的福利码。\n【密令任务】完成探索、镇妖和连续签到后揭示隐藏密令。\n━━━━━━━━━━━\n同一密令每名道友只能兑换一次；兑换失败不会消耗使用次数，也不会发放部分奖励。", player.DaoName, redeemed)
	return GameResult{Title: "太虚开服密令", Content: content, Actions: []string{"密令兑换", "限时福利码", "密令任务", "活动总览", "背包"}}
}

type redemptionRewardEntry struct {
	Item     string `json:"item"`
	Currency string `json:"currency"`
	Count    int64  `json:"count"`
}

func decodeRedemptionReward(raw string) ([]redemptionRewardEntry, error) {
	var rows []redemptionRewardEntry
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("密令没有配置奖励")
	}
	return rows, nil
}

func redemptionActivityReward(rows []redemptionRewardEntry) (activityReward, error) {
	reward := activityReward{Items: make(map[string]int64)}
	for _, row := range rows {
		if row.Count <= 0 {
			continue
		}
		if strings.TrimSpace(row.Item) != "" {
			reward.Items[row.Item] += row.Count
			continue
		}
		switch strings.TrimSpace(row.Currency) {
		case "灵石":
			reward.SpiritStones += row.Count
		case "银币":
			reward.SilverCoins += row.Count
		case "仙金":
			reward.ImmortalJade += row.Count
		case "竞技币":
			reward.ArenaCoins += row.Count
		case "修为":
			reward.Cultivation += row.Count
		case "功德":
			reward.Merit += row.Count
		case "声望":
			reward.Reputation += row.Count
		default:
			return reward, fmt.Errorf("不支持的密令货币：%s", row.Currency)
		}
	}
	return reward, nil
}

func (g *Game) redeemActivityCode(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityOpeningCodes)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "密令兑换")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		return GameResult{Title: "密令兑换", Content: "请输入：`密令兑换 完整密令`。可发送限时福利码查看公开密令，或完成密令任务揭示隐藏密令。", Actions: []string{"限时福利码", "密令任务", "开服密令"}}, true, nil
	}
	var granted activityReward
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var row model.RedemptionCode
		if findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("UPPER(code) = ?", code).First(&row).Error; findErr != nil {
			return findErr
		}
		if row.Status != "有效" && row.Status != "启用" && !strings.EqualFold(row.Status, "active") {
			return errors.New("密令已经停用")
		}
		if row.ExpiresAt != nil && !time.Now().Before(*row.ExpiresAt) {
			return errors.New("密令已经过期")
		}
		if row.MaxUses > 0 && row.UsedCount >= row.MaxUses {
			return errors.New("密令的全服兑换次数已经用尽")
		}
		markerKey := fmt.Sprintf("activity.code.%d", row.ID)
		var claimed int64
		if countErr := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, markerKey).Count(&claimed).Error; countErr != nil {
			return countErr
		}
		if claimed > 0 {
			return errActivityClaimed
		}
		entries, decodeErr := decodeRedemptionReward(row.RewardJSON)
		if decodeErr != nil {
			return decodeErr
		}
		granted, decodeErr = redemptionActivityReward(entries)
		if decodeErr != nil {
			return decodeErr
		}
		marker := model.PlayerValue{PlayerID: player.ID, Key: markerKey, Value: code}
		if createErr := tx.Create(&marker).Error; createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return errActivityClaimed
			}
			return createErr
		}
		if grantErr := grantActivityRewardTx(tx, player.ID, granted); grantErr != nil {
			return grantErr
		}
		result := tx.Model(&model.RedemptionCode{}).Where("id = ? AND (max_uses <= 0 OR used_count < max_uses)", row.ID).Update("used_count", gorm.Expr("used_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("密令的全服兑换次数刚刚用尽")
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "密令无效", Content: "没有找到这道密令。请检查字母是否完整，密令不区分大小写。", Actions: []string{"限时福利码", "密令任务"}}, true, nil
	}
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "密令已经兑换", Content: "该密令已记入你的道籍，不能重复兑换。", Actions: []string{"开服密令", "背包"}}, true, nil
	}
	if err != nil {
		return GameResult{Title: "密令兑换失败", Content: err.Error() + "。本次没有扣除次数或发放部分奖励。", Actions: []string{"限时福利码", "密令任务"}}, true, nil
	}
	return GameResult{Title: "密令兑换成功", Content: fmt.Sprintf("密令：%s\n获得：%s\n━━━━━━━━━━━\n奖励已完整写入道籍与乾坤袋，该密令不可再次兑换。", code, activityRewardText(granted)), Actions: []string{"背包", "货币", "开服密令"}}, true, nil
}

func (g *Game) limitedBenefitCodes(player *model.Player) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityOpeningCodes)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "限时福利码")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	publicCodes := []string{"XIANLV666", "DAOYOU2026", "QINGYUAN"}
	var rows []model.RedemptionCode
	if err := g.store.DB.Where("code IN ?", publicCodes).Order("id").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"以下为当前公开福利码；隐藏密令不会在这里提前泄露。", "━━━━━━━━━━━"}
	actions := []string{"密令任务", "开服密令"}
	for _, row := range rows {
		if row.ExpiresAt != nil && !time.Now().Before(*row.ExpiresAt) {
			continue
		}
		entries, _ := decodeRedemptionReward(row.RewardJSON)
		reward, _ := redemptionActivityReward(entries)
		claimed := g.playerValueExists(player.ID, fmt.Sprintf("activity.code.%d", row.ID))
		state := "可兑换"
		if claimed {
			state = "已兑换"
		} else if row.MaxUses > 0 && row.UsedCount >= row.MaxUses {
			state = "已兑完"
		}
		expires := "长期有效"
		if row.ExpiresAt != nil {
			expires = row.ExpiresAt.Format("2006-01-02 15:04") + "截止"
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n奖励：%s\n期限：%s · 全服已兑%d次", state, row.Code, activityRewardText(reward), expires, row.UsedCount))
		if state == "可兑换" {
			actions = append(actions, "密令兑换 "+row.Code)
		}
	}
	return GameResult{Title: "限时福利码", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

type secretCodeQuestDefinition struct {
	Name, Description, StatKey, Code, Action string
	Target                                   int64
}

var secretCodeQuestDefinitions = []secretCodeQuestDefinition{
	{"踏遍青山", "累计完成三次大世界探索。", "stats.explores", "SHANHEWENDAO", "探索", 3},
	{"剑下除妖", "累计赢得五场逐回合战斗。", "stats.wins", "ZHENYAOTIANXIA", "位置", 5},
	{"七日守约", "连续签到达到三日。", "checkin.streak", "QIRIXIUXING", "签到", 3},
}

func (g *Game) secretCodeQuests(player *model.Player) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityCodeQuests)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "密令任务")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"完成目标后密令会直接显现，再前往密令兑换领取奖励。", "━━━━━━━━━━━"}
	actions := []string{"开服密令", "密令兑换"}
	for _, quest := range secretCodeQuestDefinitions {
		progress := g.playerValueInt(player.ID, quest.StatKey, 0)
		state, code := "进行中", "尚未显现"
		if progress >= quest.Target {
			state, code = "已揭示", quest.Code
			actions = append(actions, "密令兑换 "+quest.Code)
		} else {
			actions = append(actions, quest.Action)
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n%s\n进度：%d/%d\n天机密令：%s", state, quest.Name, quest.Description, min64(progress, quest.Target), quest.Target, code))
	}
	return GameResult{Title: "天机密令悬卷", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) daoistRecruitmentMenu(player *model.Player) GameResult {
	count := g.successfulInvitationCount(player.ID)
	code, _ := g.personalInvitationCode(player)
	content := fmt.Sprintf("召集人：%s · 邀请码：%s\n成功结伴：%d名新道友\n━━━━━━━━━━━\n【邀请道友】查看自己的专属邀请码与绑定规则。\n【结伴奖励】成功邀请达到不同人数后领取额外福缘。\n【助力修炼】每日为其他道友护持周天，双方都能获得回馈。\n━━━━━━━━━━━\n受邀者须入道未满七日，每个道籍只能绑定一位邀请人；自己不能邀请自己。", player.DaoName, code, count)
	return GameResult{Title: "四海道友召集令", Content: content, Actions: []string{"邀请道友", "结伴奖励", "助力修炼", "新秀榜", "活动总览"}}
}

func alphaInvitationCode(id uint) string {
	value := uint64(id)
	letters := make([]byte, 6)
	for index := len(letters) - 1; index >= 0; index-- {
		letters[index] = byte('A' + value%26)
		value /= 26
	}
	return "XC" + string(letters)
}

func (g *Game) personalInvitationCode(player *model.Player) (string, error) {
	var persistent model.ReferralCode
	if err := g.store.DB.Where("account_id = ?", player.AccountID).First(&persistent).Error; err == nil {
		if persistent.CurrentPlayerID != player.ID {
			_ = g.store.DB.Model(&persistent).Update("current_player_id", player.ID).Error
		}
		return strings.ToUpper(strings.TrimSpace(persistent.Code)), nil
	}
	code := ""
	if value, err := g.playerValue(player.ID, "activity.invite.code"); err == nil {
		code = strings.ToUpper(strings.TrimSpace(value))
	}
	if code == "" {
		code = alphaInvitationCode(player.ID)
	}
	row := model.ReferralCode{AccountID: player.AccountID, CurrentPlayerID: player.ID, Code: code}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return "", err
	}
	return code, nil
}

func (g *Game) successfulInvitationCount(playerID uint) int64 {
	var player model.Player
	if g.store.DB.Unscoped().First(&player, playerID).Error == nil && player.AccountID != "" {
		var count int64
		_ = g.store.DB.Model(&model.ReferralBinding{}).Where("inviter_account_id = ? AND rewarded = ?", player.AccountID, true).Count(&count).Error
		return count
	}
	var count int64
	_ = g.store.DB.Model(&model.PlayerValue{}).Where("key = ? AND value = ?", "activity.invite.inviter", strconv.FormatUint(uint64(playerID), 10)).Count(&count).Error
	return count
}

func (g *Game) inviteDaoist(player *model.Player) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityInvitation)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "邀请道友")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	code, err := g.personalInvitationCode(player)
	if err != nil {
		return GameResult{}, true, err
	}
	content := fmt.Sprintf("专属邀请码：%s\n已成功邀请：%d名\n━━━━━━━━━━━\n邀请流程：\n一、把邀请码发给准备入道的新道友。\n二、对方入道后七日内发送“接受邀请 %s”。\n三、绑定成功时，你获得银币一百二十八与灵果一枚；对方获得银币八十八与灵果两枚。\n四、邀请人数达到结伴里程碑后，再前往结伴奖励领取。\n━━━━━━━━━━━\n邀请码只用于活动绑定，不需要填写QQ号或玩家ID。", code, g.successfulInvitationCount(player.ID), code)
	return GameResult{Title: "邀请道友", Content: content, Actions: []string{"接受邀请 " + code, "结伴奖励", "道友召集", "活动总览"}}, true, nil
}

func (g *Game) acceptActivityInvitation(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityInvitation)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "接受邀请")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if time.Since(player.CreatedAt) > 7*24*time.Hour {
		return GameResult{Title: "已过邀请绑定期", Content: "只有入道未满七日的新道友可以绑定邀请关系。你仍可生成自己的邀请码召集后来者。", Actions: []string{"邀请道友", "道友召集"}}, true, nil
	}
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		return GameResult{Title: "接受邀请", Content: "请输入：`接受邀请 邀请码`。邀请码由邀请你的道友在“邀请道友”页面查看。", Actions: []string{"道友召集"}}, true, nil
	}
	var codeRow model.ReferralCode
	if err := g.store.DB.Where("UPPER(code) = ?", code).First(&codeRow).Error; err != nil {
		return GameResult{Title: "邀请码无效", Content: "没有找到这个邀请码，请让邀请人重新打开邀请道友页面核对。", Actions: []string{"道友召集"}}, true, nil
	}
	if codeRow.AccountID == player.AccountID {
		return GameResult{Title: "不能邀请自己", Content: "道友召集必须由另一名真实道友发起。", Actions: []string{"邀请道友"}}, true, nil
	}
	var inviter model.Player
	if err := g.store.DB.Where("account_id = ?", codeRow.AccountID).First(&inviter).Error; err != nil {
		return GameResult{Title: "邀请道籍暂未建立", Content: "邀请码所属账号当前没有有效道籍，请邀请人重新入道并打开一次“邀请道友”后再绑定。", Actions: []string{"道友召集"}}, true, nil
	}
	inviteeReward := activityReward{SilverCoins: 88, Items: map[string]int64{"灵果": 2}}
	inviterReward := activityReward{SilverCoins: 128, Items: map[string]int64{"灵果": 1}}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var bound int64
		if countErr := tx.Model(&model.ReferralBinding{}).Where("invitee_account_id = ?", player.AccountID).Count(&bound).Error; countErr != nil {
			return countErr
		}
		if bound > 0 {
			return errActivityClaimed
		}
		binding := model.ReferralBinding{InviteeAccountID: player.AccountID, InviteePlayerID: player.ID, InviterAccountID: inviter.AccountID, InviterPlayerID: inviter.ID, InvitationCode: code, Rewarded: true}
		if createErr := tx.Create(&binding).Error; createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return errActivityClaimed
			}
			return createErr
		}
		// Keep the legacy player marker for old reports; eligibility is enforced by
		// the account-scoped binding above and therefore survives character deletion.
		marker := model.PlayerValue{PlayerID: player.ID, Key: "activity.invite.inviter", Value: strconv.FormatUint(uint64(inviter.ID), 10)}
		if createErr := tx.Create(&marker).Error; createErr != nil {
			return createErr
		}
		if grantErr := grantActivityRewardTx(tx, player.ID, inviteeReward); grantErr != nil {
			return grantErr
		}
		return grantActivityRewardTx(tx, inviter.ID, inviterReward)
	})
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "邀请关系已绑定", Content: "每个平台账号终身只能领取一次受邀奖励；删号、改名或重新入道都不会重置。", Actions: []string{"道友召集"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	count := g.successfulInvitationCount(inviter.ID)
	return GameResult{Title: "结伴入道成功", Content: fmt.Sprintf("邀请人：%s\n受邀者：%s\n━━━━━━━━━━━\n你获得：%s\n%s获得：%s\n%s累计成功邀请%d名新道友。", inviter.DaoName, player.DaoName, activityRewardText(inviteeReward), inviter.DaoName, activityRewardText(inviterReward), inviter.DaoName, count), Actions: []string{"道友召集", "助力修炼 " + inviter.DaoName, "背包"}}, true, nil
}

type companionRewardDefinition struct {
	ID, Name string
	Required int64
	Reward   activityReward
}

var companionRewardDefinitions = []companionRewardDefinition{
	{"first", "初识同道", 1, activityReward{SilverCoins: 188, Items: map[string]int64{"灵茶": 2}}},
	{"three", "三人成行", 3, activityReward{SpiritStones: 300, SilverCoins: 388, Items: map[string]int64{"功法残卷": 1}}},
	{"five", "五友问道", 5, activityReward{Merit: 12, SilverCoins: 688, Items: map[string]int64{"避劫符": 1}}},
	{"ten", "十方来朝", 10, activityReward{SpiritStones: 1000, SilverCoins: 1288, Items: map[string]int64{"双倍修为卡": 1, "引劫玉符": 1}}},
}

func (g *Game) referralClaimed(accountID, key string) bool {
	var count int64
	_ = g.store.DB.Model(&model.ReferralClaim{}).Where("account_id = ? AND claim_key = ?", accountID, key).Count(&count).Error
	return count > 0
}

func (g *Game) claimReferralReward(player *model.Player, key string, reward activityReward) error {
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		claim := model.ReferralClaim{AccountID: player.AccountID, ClaimKey: key}
		if err := tx.Create(&claim).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return errActivityClaimed
			}
			return err
		}
		if err := grantActivityRewardTx(tx, player.ID, reward); err != nil {
			return err
		}
		marker := model.PlayerValue{PlayerID: player.ID, Key: key, Value: "claimed"}
		return tx.Where("player_id = ? AND key = ?", player.ID, key).FirstOrCreate(&marker).Error
	})
}

func (g *Game) companionInvitationRewards(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityInvitation)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "结伴奖励")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	count := g.successfulInvitationCount(player.ID)
	name := strings.TrimSpace(raw)
	if name != "" {
		for _, milestone := range companionRewardDefinitions {
			if milestone.Name != name {
				continue
			}
			if count < milestone.Required {
				return GameResult{Title: "结伴人数不足", Content: fmt.Sprintf("奖励：%s\n需要成功邀请%d名新道友\n当前：%d名\n还需：%d名", milestone.Name, milestone.Required, count, milestone.Required-count), Actions: []string{"邀请道友", "结伴奖励"}}, true, nil
			}
			claimKey := "activity.companion." + milestone.ID
			err = g.claimReferralReward(player, claimKey, milestone.Reward)
			if errors.Is(err, errActivityClaimed) {
				return GameResult{Title: "结伴奖励已领取", Content: milestone.Name + "不可重复领取。", Actions: []string{"结伴奖励"}}, true, nil
			}
			if err != nil {
				return GameResult{}, true, err
			}
			return GameResult{Title: "结伴奖励已领取", Content: fmt.Sprintf("里程碑：%s\n成功邀请：%d名\n获得：%s", milestone.Name, count, activityRewardText(milestone.Reward)), Actions: []string{"结伴奖励", "邀请道友", "背包"}}, true, nil
		}
		return GameResult{Title: "结伴奖励不存在", Content: "请从结伴奖励页面点击里程碑名称领取。", Actions: []string{"结伴奖励"}}, true, nil
	}
	lines := []string{fmt.Sprintf("成功邀请：%d名新道友", count), "━━━━━━━━━━━"}
	actions := []string{"邀请道友", "道友召集"}
	for _, milestone := range companionRewardDefinitions {
		claimed := g.referralClaimed(player.AccountID, "activity.companion."+milestone.ID)
		state := "未达成"
		if claimed {
			state = "已领取"
		} else if count >= milestone.Required {
			state = "可领取"
			actions = append(actions, "结伴奖励 "+milestone.Name)
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n需要：成功邀请%d名 · 当前%d名\n奖励：%s", state, milestone.Name, milestone.Required, count, activityRewardText(milestone.Reward)))
	}
	return GameResult{Title: "结伴奖励", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) assistDaoistCultivation(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityInvitation)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "助力修炼")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if strings.TrimSpace(raw) == "" {
		today := time.Now().Format("2006-01-02")
		sent := g.countPlayerValuePrefix(player.ID, "activity.help.out."+today+".")
		received := g.countPlayerValuePrefix(player.ID, "activity.help.in."+today+".")
		return GameResult{Title: "助力修炼", Content: fmt.Sprintf("今日已助力：%d/3名\n今日获助：%d/5次\n━━━━━━━━━━━\n发送“助力修炼 道号”可为其他道友护持一次周天。每个目标每日只能助力一次，自己不能给自己助力；受助者获得与当前层次相称的少量修为，助力者获得银币回馈。", sent, received), Actions: []string{"好友", "道友召集", "新秀榜"}}, true, nil
	}
	target, err := g.findPlayer(raw)
	if err != nil {
		return GameResult{Title: "未找到道友", Content: "请输入真实道号，例如：`助力修炼 青玄真人`。", Actions: []string{"好友", "道友召集"}}, true, nil
	}
	if target.ID == player.ID {
		return GameResult{Title: "不能助力自己", Content: "助力修炼需要另一位道友互相护持周天。", Actions: []string{"好友"}}, true, nil
	}
	today := time.Now().Format("2006-01-02")
	sentPrefix := "activity.help.out." + today + "."
	receivedPrefix := "activity.help.in." + today + "."
	if g.countPlayerValuePrefix(player.ID, sentPrefix) >= 3 {
		return GameResult{Title: "今日助力次数已用尽", Content: "每名道友每日最多为三人护持周天，明日再来。", Actions: []string{"道友召集", "活动总览"}}, true, nil
	}
	if g.countPlayerValuePrefix(target.ID, receivedPrefix) >= 5 {
		return GameResult{Title: "对方今日获助已满", Content: target.DaoName + "今日已经获得五次修炼助力，请换一位道友。", Actions: []string{"好友"}}, true, nil
	}
	gain := min64(max64(target.CultivationRequired/50, 20), 2000)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		outMarker := model.PlayerValue{PlayerID: player.ID, Key: sentPrefix + strconv.FormatUint(uint64(target.ID), 10), Value: target.DaoName}
		inMarker := model.PlayerValue{PlayerID: target.ID, Key: receivedPrefix + strconv.FormatUint(uint64(player.ID), 10), Value: player.DaoName}
		if createErr := tx.Create(&outMarker).Error; createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return errActivityClaimed
			}
			return createErr
		}
		if createErr := tx.Create(&inMarker).Error; createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return errActivityClaimed
			}
			return createErr
		}
		if _, updateErr := grantCultivationExperienceTx(tx, target.ID, gain); updateErr != nil {
			return updateErr
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", gorm.Expr("silver_coins + ?", 20)).Error
	})
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "今日已经助力过", Content: "你今天已经为" + target.DaoName + "护持过周天，不能重复助力同一人。", Actions: []string{"好友", "助力修炼"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if refreshed, loadErr := g.players.Get(target.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&refreshed)
	}
	return GameResult{Title: "助力修炼完成", Content: fmt.Sprintf("你为%s护住周天灵息，使其当前层次修为+%d。\n你的回馈：银币+20\n━━━━━━━━━━━\n今日剩余助力次数：%d\n对方也可以发送“助力修炼 %s”为你回礼。", target.DaoName, gain, max64(3-g.countPlayerValuePrefix(player.ID, sentPrefix), 0), player.DaoName), Actions: []string{"助力修炼", "道友召集", "货币"}}, true, nil
}

type rookieRankingRow struct {
	PlayerID      uint   `gorm:"column:player_id"`
	Name          string `gorm:"column:name"`
	RealmName     string `gorm:"column:realm_name"`
	RealmSequence int    `gorm:"column:realm_sequence"`
	RealmLevel    int    `gorm:"column:realm_level"`
	CombatPower   int64  `gorm:"column:combat_power"`
}

func (g *Game) loadRookieRanking() ([]rookieRankingRow, error) {
	var rows []rookieRankingRow
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	err := g.store.DB.Raw(`SELECT p.id AS player_id, p.dao_name AS name, p.realm_name AS realm_name,
		COALESCE(r.sequence, 0) AS realm_sequence, p.realm_level AS realm_level, p.combat_power AS combat_power
		FROM players p LEFT JOIN realms r ON r.id = p.realm_id
		WHERE p.deleted_at IS NULL AND p.banned = ? AND p.created_at >= ?
		ORDER BY COALESCE(r.sequence,0) DESC, p.realm_level DESC, p.combat_power DESC, p.created_at ASC`, false, cutoff).Scan(&rows).Error
	return rows, err
}

func rookiePlayerRank(rows []rookieRankingRow, playerID uint) int {
	for index, row := range rows {
		if row.PlayerID == playerID {
			return index + 1
		}
	}
	return 0
}

func (g *Game) rookieRanking(player *model.Player, raw string, claim bool) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityRookieRank)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "新秀榜")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	rows, err := g.loadRookieRanking()
	if err != nil {
		return GameResult{}, true, err
	}
	rank := rookiePlayerRank(rows, player.ID)
	if claim {
		if rank == 0 || rank > 10 {
			return GameResult{Title: "暂无新秀俸禄", Content: "只有入道未满七日且位列新秀榜前十的道友可以领取今日俸禄。", Actions: []string{"新秀榜", "七日目标"}}, true, nil
		}
		reward := activityReward{Cultivation: min64(max64(player.CultivationRequired/50, 30), 3000), SilverCoins: int64(11-rank) * 60, Merit: int64(maxInt(4-rank/3, 1)), Items: map[string]int64{"灵果": int64(maxInt(4-rank/3, 1))}}
		if rank <= 3 {
			reward.Items["功法残卷"] = 1
		}
		key := "activity.rookie." + time.Now().Format("2006-01-02")
		err = g.claimActivityReward(player.ID, key, strconv.Itoa(rank), reward)
		if errors.Is(err, errActivityClaimed) {
			return GameResult{Title: "今日新秀俸禄已领取", Content: "新秀榜每日结算一次，明日可按新名次再次领取。", Actions: []string{"新秀榜"}}, true, nil
		}
		if err != nil {
			return GameResult{}, true, err
		}
		broadcast := ""
		if rank <= 3 {
			broadcast = fmt.Sprintf("【青云新秀】%s位列今日新秀榜第%d名，获赐%s。", player.DaoName, rank, activityRewardText(reward))
			_ = g.publishWorldBroadcast("新秀", player.DaoName+"名列青云榜", broadcast)
		}
		return GameResult{Title: "新秀俸禄已领取", Content: fmt.Sprintf("今日名次：第%d名\n获得：%s\n━━━━━━━━━━━\n榜单明日重新结算，境界层数优先于战力。", rank, activityRewardText(reward)), Actions: []string{"新秀榜", "背包", "状态"}, BroadcastContent: broadcast}, true, nil
	}
	const pageSize = 10
	page := maxInt(int(parsePositiveInt(raw, 1)), 1)
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start := minInt((page-1)*pageSize, len(rows))
	end := minInt(start+pageSize, len(rows))
	lines := []string{fmt.Sprintf("仅统计入道未满七日的道友 · 第%d/%d页", page, pages), "排序：大境界 → 当前层数 → 实时战力", "━━━━━━━━━━━"}
	for index, row := range rows[start:end] {
		position := start + index + 1
		lines = append(lines, fmt.Sprintf("%s 第%d名 · %s\n%s·%d层 · 战力%d", rankingMarker(position), position, row.Name, row.RealmName, row.RealmLevel, row.CombatPower))
	}
	if len(rows) == 0 {
		lines = append(lines, "当前还没有符合条件的新秀，第一位入道者将成为开榜者。")
	}
	personal := "你不在新秀统计期内"
	if rank > 0 {
		personal = fmt.Sprintf("你的名次：第%d名", rank)
	}
	lines = append(lines, "━━━━━━━━━━━", personal, "前十每日可领取新秀俸禄，前三名领奖时会触发全区通报。")
	actions := []string{"领取新秀奖励", "七日目标", "活动菜单"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("新秀榜 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("新秀榜 %d", page+1))
	}
	return GameResult{Title: "青云新秀榜", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) festivalMenu(player *model.Player) GameResult {
	today := time.Now().Format("2006-01-02")
	fortune := map[bool]string{true: "已承接", false: "可承接"}[g.playerValueExists(player.ID, "activity.fortune."+today)]
	prayer := map[bool]string{true: "已祈福", false: "可祈福"}[g.playerValueExists(player.ID, "activity.prayer."+today)]
	content := fmt.Sprintf("道友：%s\n━━━━━━━━━━━\n【天降鸿运】今日%s · 每日一次，稀有福缘全区通报\n【限时祈福】今日%s · 问道、护脉、纳福三签择一\n【庆典特卖】银币购买突破、疗伤、炼器与护劫物资\n━━━━━━━━━━━\n活动时间与个人领取状态以活动总览为准；庆典购买数量不设活动限购，只受银币余额约束。", player.DaoName, fortune, prayer)
	return GameResult{Title: "庆典专属", Content: content, Actions: []string{"天降鸿运", "限时祈福", "庆典特卖", "活动总览", "货币"}}
}

func (g *Game) heavenlyFortune(player *model.Player) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityFortune)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "天降鸿运")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	type fortuneOutcome struct {
		Name, Description string
		Reward            activityReward
		Rare              bool
	}
	roll := rand.Intn(1000)
	outcome := fortuneOutcome{"紫气盈门", "东方紫气越过洞府，为今日财运添上一笔。", activityReward{SpiritStones: 188, SilverCoins: 88}, false}
	switch {
	case roll < 40:
		outcome = fortuneOutcome{"天道垂青", "九霄金光落在道籍之上，护劫福缘随之凝成。", activityReward{SilverCoins: 888, Merit: 18, Items: map[string]int64{"避劫符": 1}}, true}
	case roll < 260:
		outcome = fortuneOutcome{"丹霞落袋", "一缕丹霞化作药香，落入乾坤袋中。", activityReward{Items: map[string]int64{"仙露": 2, "灵果": 3}}, false}
	case roll < 560:
		outcome = fortuneOutcome{"灵泉润脉", "无根灵泉洗过经络，为当前层次补上一缕修为。", activityReward{Cultivation: min64(max64(player.CultivationRequired/50, 20), 2000), Items: map[string]int64{"灵茶": 1}}, false}
	}
	key := "activity.fortune." + time.Now().Format("2006-01-02")
	err = g.claimActivityReward(player.ID, key, outcome.Name, outcome.Reward)
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "今日鸿运已经承接", Content: "天机一日只落一次，明日可再次承接新的福缘。", Actions: []string{"庆典专属", "活动总览"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	broadcast := ""
	if outcome.Rare {
		broadcast = fmt.Sprintf("【天降鸿运】%s承接稀世福缘“%s”，获得%s。", player.DaoName, outcome.Name, activityRewardText(outcome.Reward))
		_ = g.publishWorldBroadcast("鸿运", player.DaoName+"得天道垂青", broadcast)
	}
	return GameResult{Title: "天降鸿运·" + outcome.Name, Content: fmt.Sprintf("%s\n━━━━━━━━━━━\n获得：%s\n今日次数：已使用\n下一次：明日零时后", outcome.Description, activityRewardText(outcome.Reward)), Actions: []string{"庆典专属", "背包", "货币", "活动总览"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) limitedPrayer(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityPrayer)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "限时祈福")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	choice := strings.TrimSpace(raw)
	if choice == "" {
		used := g.playerValueExists(player.ID, "activity.prayer."+time.Now().Format("2006-01-02"))
		state := "今日尚可祈福"
		if used {
			state = "今日已经祈福"
		}
		content := fmt.Sprintf("%s · 当前灵石%d\n每次消耗灵石20，每日三签择一且只能祈福一次。\n━━━━━━━━━━━\n【问道签】补充与当前层次相称的修为，并得灵茶。\n【护脉签】获得仙露与清心丹，用于疗伤、稳心。\n【纳福签】获得银币与灵石回馈，用于庆典和常设货铺。", state, player.SpiritStones)
		return GameResult{Title: "太一限时祈福", Content: content, Actions: []string{"限时祈福 问道", "限时祈福 护脉", "限时祈福 纳福", "庆典专属", "活动总览"}}, true, nil
	}
	var reward activityReward
	description := ""
	switch choice {
	case "问道", "问道签":
		choice, description = "问道签", "香火升入识海，一段晦涩经义随钟声化开。"
		reward = activityReward{Cultivation: min64(max64(player.CultivationRequired/40, 30), 2500), Items: map[string]int64{"灵茶": 1}}
	case "护脉", "护脉签":
		choice, description = "护脉签", "青莲法水沿经络流转，为接下来的疗伤与问心备好药引。"
		reward = activityReward{Items: map[string]int64{"仙露": 2, "清心丹": 1}}
	case "纳福", "纳福签":
		choice, description = "纳福签", "金色签文落入钱庄，今日的商会福缘已经记账。"
		reward = activityReward{SpiritStones: 80, SilverCoins: 120}
	default:
		return GameResult{Title: "祈福签不存在", Content: "请选择问道、护脉或纳福，不需要输入编号。", Actions: []string{"限时祈福 问道", "限时祈福 护脉", "限时祈福 纳福"}}, true, nil
	}
	key := "activity.prayer." + time.Now().Format("2006-01-02")
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var claimed int64
		if countErr := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, key).Count(&claimed).Error; countErr != nil {
			return countErr
		}
		if claimed > 0 {
			return errActivityClaimed
		}
		result := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, 20).Update("spirit_stones", gorm.Expr("spirit_stones - ?", 20))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("祈福需要灵石20，当前余额不足")
		}
		marker := model.PlayerValue{PlayerID: player.ID, Key: key, Value: choice}
		if createErr := tx.Create(&marker).Error; createErr != nil {
			if strings.Contains(strings.ToLower(createErr.Error()), "unique") {
				return errActivityClaimed
			}
			return createErr
		}
		return grantActivityRewardTx(tx, player.ID, reward)
	})
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "今日已经祈福", Content: "太一法会每日只能择一签，明日再来。", Actions: []string{"庆典专属", "活动总览"}}, true, nil
	}
	if err != nil {
		return GameResult{Title: "祈福未成", Content: err.Error() + "。本次没有消耗祈福次数。", Actions: []string{"货币", "签到", "限时祈福"}}, true, nil
	}
	return GameResult{Title: "限时祈福·" + choice, Content: fmt.Sprintf("%s\n━━━━━━━━━━━\n消耗：灵石20\n获得：%s\n今日祈福次数：已使用", description, activityRewardText(reward)), Actions: []string{"庆典专属", "背包", "货币", "活动总览"}}, true, nil
}

func (g *Game) festivalSale(player *model.Player, raw string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityFestivalSale)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "庆典特卖")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(raw, 1)), 1)
	query := g.store.DB.Model(&model.ShopEntry{}).Where("enabled = ? AND code LIKE ?", true, "event_sale_%")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	page = minInt(page, pages)
	var rows []model.ShopEntry
	if err := query.Order("sort,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("银币余额：%d · 第%d/%d页 · %s", player.SilverCoins, page, pages, activityWindowText(activity, time.Now())), "活动期内不限购，购买数量只受银币余额与安全整数范围约束。", "━━━━━━━━━━━"}
	actions := []string{"庆典专属", "货币", "签到"}
	for _, row := range rows {
		var item model.Item
		_ = g.store.DB.First(&item, row.ItemID).Error
		lines = append(lines, fmt.Sprintf("%s · 活动价%d银币\n%s\n用途：%s", row.ItemName, row.Price, displayOr(item.Description, "庆典修行物资。"), displayOr(item.EffectType, item.CategoryName)))
		actions = append(actions, "庆典购买 "+row.ItemName)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("庆典特卖 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("庆典特卖 %d", page+1))
	}
	return GameResult{Title: "万宝庆典特卖", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) buyFestivalGood(player *model.Player, arguments []string) (GameResult, bool, error) {
	activity, err := g.requireActiveActivity(activityFestivalSale)
	if errors.Is(err, errActivityClosed) {
		return activityUnavailableResult(activity, "庆典购买")
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if len(arguments) == 0 {
		return GameResult{Title: "庆典购买", Content: "请输入：`庆典购买 物品名 [数量]`，或直接点击庆典特卖中的蓝字。", Actions: []string{"庆典特卖"}}, true, nil
	}
	name, quantity, parseErr := parseShopPurchase(arguments)
	if parseErr != nil {
		return GameResult{Title: "购买数量错误", Content: parseErr.Error(), Actions: []string{"庆典特卖"}}, true, nil
	}
	var row model.ShopEntry
	if err := g.store.DB.Where("enabled = ? AND code LIKE ? AND item_name = ?", true, "event_sale_%", name).First(&row).Error; err != nil {
		return GameResult{Title: "庆典商品不存在", Content: "当前特卖没有“" + name + "”，请从庆典特卖页面选择。", Actions: []string{"庆典特卖"}}, true, nil
	}
	if row.Price > 0 && quantity > int64(^uint64(0)>>1)/row.Price {
		return GameResult{Title: "购买数量过大", Content: "总价超过安全计算范围，请拆分购买。", Actions: []string{"庆典特卖"}}, true, nil
	}
	total := row.Price * quantity
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND silver_coins >= ?", player.ID, total).Update("silver_coins", gorm.Expr("silver_coins - ?", total))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("银币不足")
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, row.ItemID, quantity); err != nil {
			return err
		}
		purchased := playerValueIntTx(tx, player.ID, "activity.sale.purchased", 0) + quantity
		return upsertPlayerValueTx(tx, player.ID, "activity.sale.purchased", strconv.FormatInt(purchased, 10), nil)
	})
	if err != nil {
		return GameResult{Title: "庆典购买失败", Content: fmt.Sprintf("购买%s×%d需要银币%d。%s，本次没有扣款。", name, quantity, total, err.Error()), Actions: []string{"庆典特卖", "签到", "货币"}}, true, nil
	}
	return GameResult{Title: "庆典购买成功", Content: fmt.Sprintf("购入：%s×%d\n支付：银币%d\n活动限购：无\n━━━━━━━━━━━\n物品已经放入乾坤袋。", name, quantity, total), Actions: []string{"背包", "庆典特卖", "货币"}}, true, nil
}

type activityOverviewFeature struct {
	Code, Name, Command string
}

var activityOverviewFeatures = []activityOverviewFeature{
	{activitySevenGoals, "七日目标", "七日目标"}, {activityRealmSprint, "境界冲刺", "境界冲刺"},
	{activitySevenBenefit, "七日福利", "七日福利"}, {activityOpeningCodes, "开服密令", "开服密令"},
	{activityCodeQuests, "密令任务", "密令任务"}, {activityInvitation, "道友召集", "道友召集"},
	{activityRookieRank, "新秀榜", "新秀榜"}, {activityFortune, "天降鸿运", "天降鸿运"},
	{activityPrayer, "限时祈福", "限时祈福"}, {activityFestivalSale, "庆典特卖", "庆典特卖"},
	{activityV221Compensation, "万象归元全服补偿", "全服补偿"},
}

func (g *Game) activityPersonalState(player *model.Player, feature activityOverviewFeature) string {
	today := time.Now().Format("2006-01-02")
	switch feature.Code {
	case activitySevenGoals:
		return fmt.Sprintf("入道第%d日 · 已领%d/%d项目标", playerActivityDay(player), g.countPlayerValuePrefix(player.ID, "activity.goal."), len(sevenDayGoalDefinitions))
	case activityRealmSprint:
		return fmt.Sprintf("%s·%d层 · 已领%d道里程碑", player.RealmName, player.RealmLevel, g.countPlayerValuePrefix(player.ID, "activity.realm."))
	case activitySevenBenefit:
		return fmt.Sprintf("已领%d/%d份福利", g.countPlayerValuePrefix(player.ID, "activity.benefit."), len(sevenDayBenefitDefinitions))
	case activityOpeningCodes:
		return fmt.Sprintf("已兑换%d道密令", g.countPlayerValuePrefix(player.ID, "activity.code."))
	case activityCodeQuests:
		completed := 0
		for _, quest := range secretCodeQuestDefinitions {
			if g.playerValueInt(player.ID, quest.StatKey, 0) >= quest.Target {
				completed++
			}
		}
		return fmt.Sprintf("已揭示%d/%d道隐藏密令", completed, len(secretCodeQuestDefinitions))
	case activityInvitation:
		return fmt.Sprintf("已邀请%d名 · 今日获助%d/5次", g.successfulInvitationCount(player.ID), g.countPlayerValuePrefix(player.ID, "activity.help.in."+today+"."))
	case activityRookieRank:
		if time.Since(player.CreatedAt) <= 7*24*time.Hour {
			rows, _ := g.loadRookieRanking()
			rank := rookiePlayerRank(rows, player.ID)
			claimState := "今日未领奖"
			if g.playerValueExists(player.ID, "activity.rookie."+today) {
				claimState = "今日已领奖"
			}
			return fmt.Sprintf("新秀统计期内 · 当前第%d名 · %s", rank, claimState)
		}
		return "入道已满七日 · 不再计入新秀榜"
	case activityFortune:
		if g.playerValueExists(player.ID, "activity.fortune."+today) {
			return "今日已承接"
		}
		return "今日可承接"
	case activityPrayer:
		if value, err := g.playerValue(player.ID, "activity.prayer."+today); err == nil {
			return "今日已选" + value
		}
		return "今日尚可祈福"
	case activityFestivalSale:
		return fmt.Sprintf("累计购入%d件庆典物资", g.playerValueInt(player.ID, "activity.sale.purchased", 0))
	case activityV221Compensation:
		if g.v221CompensationClaimed(player) {
			return "已经领取"
		}
		if eligibleForV221Compensation(player) {
			return "符合范围 · 尚未领取"
		}
		return "建立道籍晚于补偿截止时间"
	default:
		return "尚未参与"
	}
}

func (g *Game) activityOverview(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 5
	page := maxInt(int(parsePositiveInt(raw, 1)), 1)
	pages := (len(activityOverviewFeatures) + pageSize - 1) / pageSize
	page = minInt(page, pages)
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(activityOverviewFeatures))
	lines := []string{fmt.Sprintf("道友：%s · 第%d/%d页", player.DaoName, page, pages), "状态由实际开放时间实时计算，不使用过期的静态标签。", "━━━━━━━━━━━"}
	actions := []string{"活动菜单"}
	for _, feature := range activityOverviewFeatures[start:end] {
		var row model.Activity
		if err := g.store.DB.Where("code = ?", feature.Code).First(&row).Error; err != nil {
			lines = append(lines, fmt.Sprintf("【未配置】%s\n当前没有可运行的活动数据。", feature.Name))
			continue
		}
		state := activityState(row, time.Now())
		lines = append(lines, fmt.Sprintf("【%s】%s\n时间：%s 至 %s\n倒计时：%s\n个人：%s", state, row.Name, row.StartsAt.Format("01-02 15:04"), row.EndsAt.Format("01-02 15:04"), activityWindowText(row, time.Now()), g.activityPersonalState(player, feature)))
		actions = append(actions, feature.Command)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("活动总览 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("活动总览 %d", page+1))
	}
	return GameResult{Title: "活动总览", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) playerValueExists(playerID uint, key string) bool {
	var count int64
	_ = g.store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", playerID, key).Count(&count).Error
	return count > 0
}
