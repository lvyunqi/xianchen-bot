package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

const (
	birthdayDateKey          = "birthday.date"
	birthdayRegisteredKey    = "birthday.registered_at"
	birthdayTicketName       = "岁序福签"
	birthdayTitleCode        = "title_birthday_long_life"
	birthdayArtifactCode     = "birthday_longevity_pendant"
	birthdayArtifactName     = "岁序长生佩"
	birthdayLifetimeScoreKey = "birthday.score"
)

var (
	errBirthdayAlreadyClaimed   = errors.New("birthday reward already claimed")
	errBirthdayAlreadyBlessed   = errors.New("birthday blessing already sent")
	errBirthdayInventoryChanged = errors.New("birthday gift inventory changed")
	errBirthdayUniqueArtifact   = errors.New("birthday unique artifact already owned")
)

type birthdayProfile struct {
	Month int
	Day   int
}

type birthdayTaskDefinition struct {
	Code         string
	Name         string
	Requirement  string
	TicketReward int64
	SilverReward int64
}

var birthdayTasks = []birthdayTaskDefinition{
	{Code: "speak", Name: "星灯初明", Requirement: "生日当天在任意群首次发言，触发仙尘长篇祝福", TicketReward: 3, SilverReward: 88},
	{Code: "checkin", Name: "生辰留印", Requirement: "完成一次生辰签到", TicketReward: 3, SilverReward: 88},
	{Code: "gift", Name: "天赐长生", Requirement: "领取仙尘赠送的年度生日礼物", TicketReward: 5, SilverReward: 188},
	{Code: "blessings", Name: "万友同贺", Requirement: "收到三位不同道友的生日祝福", TicketReward: 8, SilverReward: 188},
	{Code: "present", Name: "礼承四海", Requirement: "收到至少一份道友赠礼", TicketReward: 10, SilverReward: 288},
}

func parseBirthdayProfile(raw string) (birthdayProfile, string, error) {
	normalized := strings.TrimSpace(raw)
	normalized = strings.NewReplacer("年", "-", "月", "-", "日", "", "/", "-", ".", "-").Replace(normalized)
	parts := strings.FieldsFunc(normalized, func(r rune) bool { return r == '-' || r == ' ' })
	if len(parts) == 3 {
		parts = parts[1:]
	}
	if len(parts) != 2 {
		return birthdayProfile{}, "", fmt.Errorf("生日格式不正确，请使用 `设置生日 07-23` 或 `设置生日 7月23日`")
	}
	month, monthErr := strconv.Atoi(parts[0])
	day, dayErr := strconv.Atoi(parts[1])
	if monthErr != nil || dayErr != nil {
		return birthdayProfile{}, "", fmt.Errorf("生日只能填写月和日")
	}
	check := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if month < 1 || month > 12 || day < 1 || check.Month() != time.Month(month) || check.Day() != day {
		return birthdayProfile{}, "", fmt.Errorf("该月日不存在，请核对后重新登记")
	}
	profile := birthdayProfile{Month: month, Day: day}
	return profile, fmt.Sprintf("%02d-%02d", month, day), nil
}

func decodeBirthdayProfile(value string) (birthdayProfile, bool) {
	profile, _, err := parseBirthdayProfile(value)
	return profile, err == nil
}

func birthdayDateText(profile birthdayProfile) string {
	return fmt.Sprintf("%d月%d日", profile.Month, profile.Day)
}

func isLeapYear(year int) bool {
	return year%400 == 0 || year%4 == 0 && year%100 != 0
}

func birthdayOccurrence(profile birthdayProfile, year int, location *time.Location) time.Time {
	day := profile.Day
	if profile.Month == 2 && profile.Day == 29 && !isLeapYear(year) {
		day = 28
	}
	return time.Date(year, time.Month(profile.Month), day, 0, 0, 0, 0, location)
}

func birthdayMatches(profile birthdayProfile, now time.Time) bool {
	occurrence := birthdayOccurrence(profile, now.Year(), now.Location())
	return occurrence.Month() == now.Month() && occurrence.Day() == now.Day()
}

func nextBirthdayOccurrence(profile birthdayProfile, now time.Time) time.Time {
	next := birthdayOccurrence(profile, now.Year(), now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if next.Before(today) {
		next = birthdayOccurrence(profile, now.Year()+1, now.Location())
	}
	return next
}

func (g *Game) playerBirthdayProfile(playerID uint) (birthdayProfile, bool, error) {
	value, err := g.playerValue(playerID, birthdayDateKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return birthdayProfile{}, false, nil
	}
	if err != nil {
		return birthdayProfile{}, false, err
	}
	profile, valid := decodeBirthdayProfile(value)
	return profile, valid, nil
}

func (g *Game) isPlayerBirthdayToday(playerID uint, now time.Time) bool {
	profile, exists, err := g.playerBirthdayProfile(playerID)
	return err == nil && exists && birthdayMatches(profile, now)
}

func birthdayYearKey(prefix string, year int) string {
	return fmt.Sprintf("birthday.%s.%d", prefix, year)
}

func birthdayEndOfYear(now time.Time) time.Time {
	return time.Date(now.Year()+1, 1, 2, 0, 0, 0, 0, now.Location())
}

func createPlayerValueOnceTx(tx *gorm.DB, playerID uint, key, value string, expiresAt *time.Time) (bool, error) {
	row := model.PlayerValue{PlayerID: playerID, Key: key, Value: value, ExpiresAt: expiresAt}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	return result.RowsAffected == 1, result.Error
}

func addPlayerValueIntTx(tx *gorm.DB, playerID uint, key string, delta int64) (int64, error) {
	current := playerValueIntTx(tx, playerID, key, 0)
	if delta > 0 && current > math.MaxInt64-delta || delta < 0 && current < math.MinInt64-delta {
		return current, fmt.Errorf("player value %s exceeds safe integer range", key)
	}
	current += delta
	return current, upsertPlayerValueTx(tx, playerID, key, strconv.FormatInt(current, 10), nil)
}

func grantNamedItemTx(tx *gorm.DB, playerID uint, itemName string, quantity int64) error {
	if quantity <= 0 {
		return nil
	}
	var item model.Item
	if err := tx.Where("name = ?", itemName).First(&item).Error; err != nil {
		return fmt.Errorf("物品%s不存在: %w", itemName, err)
	}
	return storage.NewPlayerRepository(tx).AdjustItem(playerID, item.ID, quantity)
}

func (g *Game) registerBirthday(player *model.Player, raw string) (GameResult, bool, error) {
	if existing, exists, err := g.playerBirthdayProfile(player.ID); err != nil {
		return GameResult{}, true, err
	} else if exists {
		return GameResult{Title: "🎂 生辰已经登记", Content: fmt.Sprintf("道号：%s\n生辰：%s\n━━━━━━━━━━━\n生日登记由玩家端永久锁定，不能通过反复修改日期领取年度奖励。如确实登记错误，只能联系主人核验后处理。", player.DaoName, birthdayDateText(existing)), Actions: []string{"状态", "角色菜单"}}, true, nil
	}
	profile, encoded, parseErr := parseBirthdayProfile(raw)
	if parseErr != nil {
		return GameResult{Title: "🎂 设置生日", Content: parseErr.Error() + "\n系统只保存月日，不保存出生年份。", Actions: []string{"设置生日 07-23", "角色菜单"}}, true, nil
	}
	now := time.Now()
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, birthdayDateKey).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errBirthdayAlreadyClaimed
		}
		if err := upsertPlayerValueTx(tx, player.ID, birthdayDateKey, encoded, nil); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, birthdayRegisteredKey, now.Format(time.RFC3339Nano), nil)
	})
	if errors.Is(err, errBirthdayAlreadyClaimed) {
		return GameResult{Title: "🎂 生辰已经登记", Content: "生日刚刚由另一条请求完成登记，本次没有重复改动。", Actions: []string{"生日菜单"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	next := nextBirthdayOccurrence(profile, now)
	content := fmt.Sprintf("道号：%s\n登记生辰：%s\n隐私：未保存出生年份\n━━━━━━━━━━━\n生日当天，完整生辰专属菜单会自动出现；你在每个群首次发言时，仙尘会送上一次长篇祝福。登记已永久锁定。", player.DaoName, birthdayDateText(profile))
	actions := []string{"状态", "角色菜单"}
	if birthdayMatches(profile, now) {
		content += "\n今日正逢生辰，专属庆典已经开启。"
		actions = []string{"生日菜单", "生辰签到", "领取生日礼物"}
	} else {
		content += fmt.Sprintf("\n下次生辰：%s · 还有%d天", next.Format("2006-01-02"), int(next.Sub(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())).Hours()/24))
	}
	return GameResult{Title: "🎂 生辰登记完成", Content: content, Actions: actions}, true, nil
}

func (g *Game) birthdayMenu(player *model.Player) (GameResult, bool, error) {
	profile, exists, err := g.playerBirthdayProfile(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if !exists {
		return GameResult{Title: "🎂 生辰尚未登记", Content: "先发送 `设置生日 月-日`。系统只保存月日；登记后由玩家端永久锁定。完整生日专属菜单只有本人生日当天才会显示。", Actions: []string{"设置生日 07-23", "角色菜单"}}, true, nil
	}
	now := time.Now()
	if !birthdayMatches(profile, now) {
		next := nextBirthdayOccurrence(profile, now)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return GameResult{Title: "🎂 生辰仙境尚未开启", Content: fmt.Sprintf("已登记：%s\n下次开启：%s\n倒计时：%d天\n━━━━━━━━━━━\n完整生日菜单、签到、礼物、任务、抽奖和兑换只在生日当天显示与开放。", birthdayDateText(profile), next.Format("2006-01-02"), int(next.Sub(today).Hours()/24)), Actions: []string{"状态", "角色菜单"}}, true, nil
	}
	year := now.Year()
	tickets := g.birthdayTicketQuantity(player.ID)
	checkin := map[bool]string{true: "已签到", false: "待签到"}[g.playerValueExists(player.ID, birthdayYearKey("checkin", year))]
	gift := map[bool]string{true: "已领取", false: "待领取"}[g.playerValueExists(player.ID, birthdayYearKey("claim", year))]
	completedTasks := 0
	for _, task := range birthdayTasks {
		if g.playerValueExists(player.ID, birthdayTaskClaimKey(year, task.Code)) {
			completedTasks++
		}
	}
	content := fmt.Sprintf("寿星：%s · %s\n岁序福签：%d\n收到祝福：%d位 · 收到赠礼：%d份\n━━━━━━━━━━━\n【今日寿礼】生辰签到：%s · 仙尘礼物：%s\n【生辰任务】已领奖%d/%d项\n【庆典玩法】限定抽奖 · 福签兑换 · 寿星福缘榜\n【道友同贺】生日祝福 · 生日赠礼\n━━━━━━━━━━━\n本菜单只在生日当天显示，所有年度奖励在当年只能结算一次。", player.DaoName, birthdayDateText(profile), tickets, g.playerValueInt(player.ID, birthdayReceivedBlessingKey(year), 0), g.playerValueInt(player.ID, birthdayReceivedGiftKey(year), 0), checkin, gift, completedTasks, len(birthdayTasks))
	return GameResult{Title: "🎂 仙尘生辰专属菜单", Content: content, Actions: []string{"生辰签到", "领取生日礼物", "生日任务", "生辰抽奖", "生辰兑换", "今日寿星", "寿星榜", "我的称号", "背包"}}, true, nil
}

func (g *Game) birthdayClosed(player *model.Player) (GameResult, bool, error) {
	profile, exists, err := g.playerBirthdayProfile(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if !exists {
		return GameResult{Title: "🎂 生辰功能未开启", Content: "请先发送 `设置生日 月-日`；完整功能只在本人生日当天开放。", Actions: []string{"设置生日 07-23"}}, true, nil
	}
	return GameResult{Title: "🎂 今日并非你的生辰", Content: fmt.Sprintf("已登记：%s\n生日专属功能只在当天开放，不会提前消耗福签或领取次数。", birthdayDateText(profile)), Actions: []string{"生日菜单", "状态"}}, true, nil
}

func (g *Game) birthdayCheckin(player *model.Player) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	year := now.Year()
	var levelProgress model.PlayerLevelProgress
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		created, err := createPlayerValueOnceTx(tx, player.ID, birthdayYearKey("checkin", year), now.Format(time.RFC3339Nano), nil)
		if err != nil {
			return err
		}
		if !created {
			return errBirthdayAlreadyClaimed
		}
		if err := grantNamedItemTx(tx, player.ID, birthdayTicketName, 8); err != nil {
			return err
		}
		if err := grantNamedItemTx(tx, player.ID, "灵果", 3); err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", gorm.Expr("silver_coins + ?", 188)).Error; err != nil {
			return err
		}
		levelProgress, err = grantCultivationExperienceTx(tx, player.ID, 188)
		return err
	})
	if errors.Is(err, errBirthdayAlreadyClaimed) {
		return GameResult{Title: "🎂 生辰已经签到", Content: "今年生日的生辰签到已经完成，不能重复领取。", Actions: []string{"生日菜单", "生日任务"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	result := GameResult{Title: "🎂 生辰签到完成", Content: "寿星在岁序簿上留下道印。\n━━━━━━━━━━━\n银币+188 · 修为+188\n岁序福签×8 · 灵果×3\n今年生辰签到：已完成", Actions: []string{"生日任务", "领取生日礼物", "生辰抽奖", "生日菜单"}}
	appendPlayerLevelSettlement(&result, latest, levelProgress)
	return result, true, nil
}

func (g *Game) claimBirthdayGift(player *model.Player) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	year := now.Year()
	var levelProgress model.PlayerLevelProgress
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		created, err := createPlayerValueOnceTx(tx, player.ID, birthdayYearKey("claim", year), now.Format(time.RFC3339Nano), nil)
		if err != nil {
			return err
		}
		if !created {
			return errBirthdayAlreadyClaimed
		}
		for itemName, quantity := range map[string]int64{
			birthdayTicketName: 30, "长生蟠桃": 3, "生辰许愿灯": 1, "阵基石": 8,
			"雷灵晶": 3, "龙血芝": 2, "造化仙壤": 2, "万福同心礼匣": 1,
		} {
			if err := grantNamedItemTx(tx, player.ID, itemName, quantity); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"silver_coins":  gorm.Expr("silver_coins + ?", 888),
			"spirit_stones": gorm.Expr("spirit_stones + ?", 8888),
			"merit":         gorm.Expr("merit + ?", 88),
		}).Error; err != nil {
			return err
		}
		levelProgress, err = grantCultivationExperienceTx(tx, player.ID, 888)
		if err != nil {
			return err
		}
		var title model.Title
		if err := tx.Where("code = ? AND enabled = ?", birthdayTitleCode, true).First(&title).Error; err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, titleUnlockKey(title), "birthday", nil)
	})
	if errors.Is(err, errBirthdayAlreadyClaimed) {
		return GameResult{Title: "🎂 仙尘生日礼物已领取", Content: "今年的仙尘生辰礼已经收入道籍，不能重复领取。", Actions: []string{"生日菜单", "背包", "我的称号"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	broadcast := fmt.Sprintf("【仙尘生辰礼】今日星河为%s点亮长明灯。仙尘谨贺寿星生辰吉乐、道途长青，并赐尊号【岁序长生】！", player.DaoName)
	_ = g.publishWorldBroadcast("生辰", player.DaoName+"领取仙尘生辰礼", broadcast)
	result := GameResult{Title: "🎁 仙尘生日礼物", Content: "仙尘为你启封年度生辰礼。\n━━━━━━━━━━━\n银币+888 · 灵石+8888 · 修为+888 · 功德+88\n岁序福签×30 · 长生蟠桃×3 · 生辰许愿灯×1\n阵基石×8 · 雷灵晶×3 · 龙血芝×2 · 造化仙壤×2\n万福同心礼匣×1\n专属称号：【岁序长生】已收入尊号玉册\n━━━━━━━━━━━\n以上均为独立生日新增奖励，不替换任何原签到、任务或活动奖励。", Actions: []string{"佩戴称号 岁序长生", "背包", "生日任务", "生辰抽奖", "生日菜单"}, BroadcastContent: broadcast}
	appendPlayerLevelSettlement(&result, latest, levelProgress)
	return result, true, nil
}

func (g *Game) birthdayTicketQuantity(playerID uint) int64 {
	item, err := g.itemByName(birthdayTicketName)
	if err != nil {
		return 0
	}
	return g.itemQuantity(playerID, item.ID)
}

func birthdayReceivedBlessingKey(year int) string { return birthdayYearKey("recv.bless", year) }
func birthdayReceivedGiftKey(year int) string     { return birthdayYearKey("recv.gift", year) }
func birthdayReceivedItemsKey(year int) string    { return birthdayYearKey("recv.items", year) }
func birthdayTaskClaimKey(year int, code string) string {
	return fmt.Sprintf("birthday.task.%d.%s", year, code)
}

func (g *Game) todayBirthdayPlayers(now time.Time) ([]model.Player, error) {
	var dates []model.PlayerValue
	if err := g.store.DB.Where("key = ?", birthdayDateKey).Find(&dates).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(dates))
	for _, row := range dates {
		profile, valid := decodeBirthdayProfile(row.Value)
		if valid && birthdayMatches(profile, now) {
			ids = append(ids, row.PlayerID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var players []model.Player
	if err := g.store.DB.Where("id IN ? AND banned = ?", ids, false).Find(&players).Error; err != nil {
		return nil, err
	}
	sort.Slice(players, func(left, right int) bool { return players[left].ID < players[right].ID })
	return players, nil
}

func (g *Game) todayBirthdayList(player *model.Player) (GameResult, bool, error) {
	rows, err := g.todayBirthdayPlayers(time.Now())
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "🎂 今日暂无寿星", Content: "今天没有已登记生日的道友，生辰公开功能不会显示；等待寿星生日当天再开启。", Actions: []string{"状态"}}, true, nil
	}
	lines := []string{fmt.Sprintf("今日共有%d位寿星", len(rows)), "━━━━━━━━━━━"}
	actions := []string{"寿星榜"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("- %s · %s · 收到祝福%d · 赠礼%d", row.DaoName, row.RealmName, g.playerValueInt(row.ID, birthdayReceivedBlessingKey(time.Now().Year()), 0), g.playerValueInt(row.ID, birthdayReceivedGiftKey(time.Now().Year()), 0)))
		if row.ID != player.ID {
			actions = append(actions, "生日祝福 @"+row.DaoName+" 生日快乐", "生日赠礼 @"+row.DaoName)
		}
	}
	return GameResult{Title: "🎂 今日寿星", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) BirthdayAmbientGreeting(groupID, accountID string) (GameResult, bool, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return GameResult{}, false, nil
	}
	player, err := g.players.GetByAccount(accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) || err == nil && player.Banned {
		return GameResult{}, false, nil
	}
	if err != nil {
		return GameResult{}, false, err
	}
	now := time.Now()
	profile, exists, err := g.playerBirthdayProfile(player.ID)
	if err != nil || !exists || !birthdayMatches(profile, now) {
		return GameResult{}, false, err
	}
	markerKey := fmt.Sprintf("birthday.greet.%d.%016x", now.Year(), rotatingShopScore(groupID))
	expires := birthdayEndOfYear(now)
	created := false
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var createErr error
		created, createErr = createPlayerValueOnceTx(tx, player.ID, markerKey, groupID, &expires)
		if createErr != nil || !created {
			return createErr
		}
		return upsertPlayerValueTx(tx, player.ID, birthdayYearKey("spoke", now.Year()), now.Format(time.RFC3339Nano), &expires)
	})
	if err != nil || !created {
		return GameResult{}, false, err
	}
	content := fmt.Sprintf("今日群星移位，岁序为%s停驻一刻。\n仙尘循着道籍中的生辰印记而来，为寿星点亮长明仙灯。\n━━━━━━━━━━━\n愿你旧岁所行，皆化作脚下青云；\n愿你新岁所求，皆有清风明月相迎；\n愿你气血长盈，法力不竭，道心澄明；\n愿你所炼之丹皆成，所锻之器皆鸣；\n愿你入秘境得真传，渡雷劫有故人护道；\n愿山河万里皆可往，星海诸界皆可归；\n愿相识的道友常在，同修的灯火不熄；\n愿每一次选择都不负本心，每一重境界都有所得。\n━━━━━━━━━━━\n今日本群第一次听见寿星发言，仙尘谨以诸界清光相贺：\n%s，生辰吉乐，岁岁长安，道途无量。\n完整生辰专属菜单已为你开启。", player.DaoName, player.DaoName)
	return GameResult{Title: "🎂 仙尘长篇生辰祝福", Content: content, Actions: []string{"生日菜单", "生日祝福 @" + player.DaoName + " 生日快乐", "生日赠礼 @" + player.DaoName, "今日寿星", "寿星榜"}}, true, nil
}

func (g *Game) birthdayBlessing(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return GameResult{Title: "🎂 生日祝福", Content: "请输入：`生日祝福 @寿星 祝福语`。每位道友每年可为同一寿星送出一次计分祝福。", Actions: []string{"今日寿星"}}, true, nil
	}
	target, err := g.findPlayer(parts[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🎂 祝福对象无效", Content: "请选择今日过生日的另一位道友。", Actions: []string{"今日寿星"}}, true, nil
	}
	now := time.Now()
	if !g.isPlayerBirthdayToday(target.ID, now) {
		return GameResult{Title: "🎂 今日并非对方生辰", Content: target.DaoName + "今天没有开启生辰庆典，本次祝福未计入寿星榜。", Actions: []string{"今日寿星"}}, true, nil
	}
	blessing := strings.TrimSpace(strings.Join(parts[1:], " "))
	if blessing == "" {
		blessing = "生日快乐，愿你岁岁长安，道途常明"
	}
	if utf8.RuneCountInString(blessing) < 2 || utf8.RuneCountInString(blessing) > 80 {
		return GameResult{Title: "🎂 祝福长度不符", Content: "祝福语需为2至80个字符。", Actions: []string{"生日祝福 @" + target.DaoName + " 生日快乐"}}, true, nil
	}
	if rejected, blocked, reviewErr := g.rejectSensitiveContent("生辰祝福", player, blessing); reviewErr != nil || blocked {
		return rejected, true, reviewErr
	}
	year := now.Year()
	markerKey := fmt.Sprintf("birthday.blessed.%d.%d", target.ID, year)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		created, createErr := createPlayerValueOnceTx(tx, player.ID, markerKey, target.DaoName, nil)
		if createErr != nil {
			return createErr
		}
		if !created {
			return errBirthdayAlreadyBlessed
		}
		if _, err := addPlayerValueIntTx(tx, target.ID, birthdayReceivedBlessingKey(year), 1); err != nil {
			return err
		}
		if _, err := addPlayerValueIntTx(tx, target.ID, birthdayLifetimeScoreKey, 10); err != nil {
			return err
		}
		if err := grantNamedItemTx(tx, player.ID, birthdayTicketName, 2); err != nil {
			return err
		}
		return grantNamedItemTx(tx, target.ID, birthdayTicketName, 1)
	})
	if errors.Is(err, errBirthdayAlreadyBlessed) {
		return GameResult{Title: "🎂 今年已经祝福过", Content: "你今年已经为" + target.DaoName + "送过一次计分祝福，不能重复获得福签或寿星榜分数。", Actions: []string{"生日赠礼 @" + target.DaoName, "寿星榜"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.ID, "生辰祝福", fmt.Sprintf("%s送来生日祝福：%s", player.DaoName, blessing))
	return GameResult{Title: "🎉 道友生辰同贺", Content: fmt.Sprintf("祝福者：%s\n寿星：%s\n祝福：%s\n━━━━━━━━━━━\n你获得岁序福签×2，寿星获得岁序福签×1；寿星榜祝福值+10。", player.DaoName, target.DaoName, blessing), Actions: []string{"生日赠礼 @" + target.DaoName, "今日寿星", "寿星榜"}}, true, nil
}

func (g *Game) birthdayPresent(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) < 2 {
		return GameResult{Title: "🎁 生日赠礼", Content: "请输入：`生日赠礼 @寿星 物品名*数量`。只能赠送乾坤袋中可交易且未绑定的真实物品。", Actions: []string{"今日寿星", "背包"}}, true, nil
	}
	target, err := g.findPlayer(parts[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🎁 赠礼对象无效", Content: "请选择今日过生日的另一位道友。", Actions: []string{"今日寿星"}}, true, nil
	}
	now := time.Now()
	if !g.isPlayerBirthdayToday(target.ID, now) {
		return GameResult{Title: "🎁 生辰赠礼未开启", Content: target.DaoName + "今天并非已登记的生日，本次没有转移物品。", Actions: []string{"今日寿星"}}, true, nil
	}
	itemName, quantity, parseErr := parseShopPurchase(parts[1:])
	if parseErr != nil {
		return GameResult{Title: "🎁 赠礼数量错误", Content: parseErr.Error(), Actions: []string{"背包"}}, true, nil
	}
	var item model.Item
	if err := g.store.DB.Where("name = ?", itemName).First(&item).Error; err != nil || !item.Tradable {
		return GameResult{Title: "🎁 该物品不可赠送", Content: "只能赠送乾坤袋中标记为可交易的物品；绑定物、福签、定制凭证和权限物品均不可转移。", Actions: []string{"物品 " + itemName, "背包"}}, true, nil
	}
	year := now.Year()
	scored := false
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var owned model.PlayerItem
		if err := tx.Where("player_id = ? AND item_id = ?", player.ID, item.ID).First(&owned).Error; err != nil || owned.Bound || owned.Quantity < quantity {
			return errBirthdayInventoryChanged
		}
		repo := storage.NewPlayerRepository(tx)
		if err := repo.AdjustItem(player.ID, item.ID, -quantity); err != nil {
			return errBirthdayInventoryChanged
		}
		if err := repo.AdjustItem(target.ID, item.ID, quantity); err != nil {
			return err
		}
		if _, err := addPlayerValueIntTx(tx, target.ID, birthdayReceivedItemsKey(year), quantity); err != nil {
			return err
		}
		markerKey := fmt.Sprintf("birthday.gifted.%d.%d", target.ID, year)
		var markerErr error
		scored, markerErr = createPlayerValueOnceTx(tx, player.ID, markerKey, item.Name, nil)
		if markerErr != nil {
			return markerErr
		}
		if !scored {
			return nil
		}
		if _, err := addPlayerValueIntTx(tx, target.ID, birthdayReceivedGiftKey(year), 1); err != nil {
			return err
		}
		if _, err := addPlayerValueIntTx(tx, target.ID, birthdayLifetimeScoreKey, 20); err != nil {
			return err
		}
		if err := grantNamedItemTx(tx, player.ID, birthdayTicketName, 2); err != nil {
			return err
		}
		return grantNamedItemTx(tx, target.ID, birthdayTicketName, 1)
	})
	if errors.Is(err, errBirthdayInventoryChanged) {
		return GameResult{Title: "🎁 赠礼失败", Content: fmt.Sprintf("你的%s不足%d件、物品已绑定，或库存刚刚发生变化。本次双方背包均未改动。", item.Name, quantity), Actions: []string{"背包", "生日赠礼 @" + target.DaoName}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	bonus := "本次礼物已经送达；同一赠礼者今年对该寿星的榜单加分只计算首次。"
	if scored {
		bonus = "首次年度赠礼：你获得岁序福签×2，寿星获得岁序福签×1；寿星榜祝福值+20。"
	}
	_ = g.createPlayerNotification(target.ID, "生辰赠礼", fmt.Sprintf("%s赠予你%s×%d。", player.DaoName, item.Name, quantity))
	return GameResult{Title: "🎁 生辰赠礼送达", Content: fmt.Sprintf("赠礼者：%s\n寿星：%s\n礼物：%s×%d\n━━━━━━━━━━━\n%s", player.DaoName, target.DaoName, item.Name, quantity, bonus), Actions: []string{"生日祝福 @" + target.DaoName + " 生日快乐", "今日寿星", "寿星榜", "背包"}}, true, nil
}

func (g *Game) birthdayTaskList(player *model.Player) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	year := now.Year()
	lines := []string{"生日专属任务只在今日开放，完成条件后需手动领取；不替换原任务。", "━━━━━━━━━━━"}
	actions := []string{"生日菜单"}
	for _, task := range birthdayTasks {
		complete := g.birthdayTaskComplete(player.ID, year, task.Code)
		claimed := g.playerValueExists(player.ID, birthdayTaskClaimKey(year, task.Code))
		state := "未完成"
		if complete {
			state = "可领取"
		}
		if claimed {
			state = "已领取"
		}
		lines = append(lines, fmt.Sprintf("【%s】%s\n条件：%s\n奖励：岁序福签×%d · 银币%d", task.Name, state, task.Requirement, task.TicketReward, task.SilverReward), "━━━━━━━")
		if complete && !claimed {
			actions = append(actions, "领取生日任务 "+task.Name)
		}
	}
	actions = append(actions, "生辰签到", "领取生日礼物", "生辰抽奖", "生辰兑换")
	return GameResult{Title: "🎂 生日专属任务", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) birthdayTaskComplete(playerID uint, year int, code string) bool {
	switch code {
	case "speak":
		return g.playerValueExists(playerID, birthdayYearKey("spoke", year))
	case "checkin":
		return g.playerValueExists(playerID, birthdayYearKey("checkin", year))
	case "gift":
		return g.playerValueExists(playerID, birthdayYearKey("claim", year))
	case "blessings":
		return g.playerValueInt(playerID, birthdayReceivedBlessingKey(year), 0) >= 3
	case "present":
		return g.playerValueInt(playerID, birthdayReceivedGiftKey(year), 0) >= 1
	default:
		return false
	}
}

func birthdayTaskCompleteTx(tx *gorm.DB, playerID uint, year int, code string) bool {
	markerExists := func(key string) bool {
		var count int64
		return tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", playerID, key).Count(&count).Error == nil && count > 0
	}
	switch code {
	case "speak":
		return markerExists(birthdayYearKey("spoke", year))
	case "checkin":
		return markerExists(birthdayYearKey("checkin", year))
	case "gift":
		return markerExists(birthdayYearKey("claim", year))
	case "blessings":
		return playerValueIntTx(tx, playerID, birthdayReceivedBlessingKey(year), 0) >= 3
	case "present":
		return playerValueIntTx(tx, playerID, birthdayReceivedGiftKey(year), 0) >= 1
	default:
		return false
	}
}

func (g *Game) claimBirthdayTask(player *model.Player, raw string) (GameResult, bool, error) {
	now := time.Now()
	if !g.isPlayerBirthdayToday(player.ID, now) {
		return g.birthdayClosed(player)
	}
	name := strings.TrimSpace(raw)
	var selected birthdayTaskDefinition
	found := false
	for _, task := range birthdayTasks {
		if task.Name == name || task.Code == name {
			selected, found = task, true
			break
		}
	}
	if !found {
		return GameResult{Title: "🎂 生日任务不存在", Content: "请从生日任务菜单的蓝字中选择完整任务名。", Actions: []string{"生日任务"}}, true, nil
	}
	year := now.Year()
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if !birthdayTaskCompleteTx(tx, player.ID, year, selected.Code) {
			return errBirthdayInventoryChanged
		}
		created, err := createPlayerValueOnceTx(tx, player.ID, birthdayTaskClaimKey(year, selected.Code), now.Format(time.RFC3339Nano), nil)
		if err != nil {
			return err
		}
		if !created {
			return errBirthdayAlreadyClaimed
		}
		if err := grantNamedItemTx(tx, player.ID, birthdayTicketName, selected.TicketReward); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", gorm.Expr("silver_coins + ?", selected.SilverReward)).Error
	})
	if errors.Is(err, errBirthdayInventoryChanged) {
		return GameResult{Title: "🎂 任务尚未完成", Content: selected.Requirement, Actions: []string{"生日任务"}}, true, nil
	}
	if errors.Is(err, errBirthdayAlreadyClaimed) {
		return GameResult{Title: "🎂 任务奖励已领取", Content: selected.Name + "今年已经结算，不能重复领取。", Actions: []string{"生日任务"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🎂 生辰任务完成", Content: fmt.Sprintf("任务：%s\n获得：岁序福签×%d · 银币+%d", selected.Name, selected.TicketReward, selected.SilverReward), Actions: []string{"生日任务", "生辰抽奖", "生辰兑换", "生日菜单"}}, true, nil
}
