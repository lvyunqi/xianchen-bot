package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

const v221CompensationClaimKey = "compensation.v2.2.2.attribute-and-runtime-recovery"

var (
	compensationClaimMu       sync.Mutex
	v221CompensationCutoff    = time.Date(2026, time.July, 24, 23, 59, 59, 0, time.FixedZone("UTC+8", 8*60*60))
	errCompensationIneligible = errors.New("account is not eligible for compensation")
	v221CompensationReward    = activityReward{
		SpiritStones: 88888,
		SilverCoins:  3888,
		Merit:        88,
		Reputation:   88,
		Items: map[string]int64{
			"万象归元纪念令": 1,
			"月华问道礼匣":  2,
			"玄铁":      188,
			"阵基石":     88,
			"星辰砂":     66,
			"雷灵晶":     36,
			"妖兽内丹":    128,
			"龙血芝":     8,
			"龙血芝孢子":   8,
			"灵根精粹":    12,
			"功法残卷":    30,
			"扫荡券":     50,
			"回元散":     99,
			"回灵丹":     99,
			"聚灵丹":     50,
			"淬脉丹":     30,
			"凝元丹":     25,
			"破境丹":     20,
			"九转还魂丹":   3,
			"轮回丹":     1,
			"双倍修为卡":   8,
			"避劫符":     8,
			"引劫玉符":    5,
			"传送符":     30,
			"造化仙壤":    20,
			"地脉灵肥":    50,
			"灵壤肥":     99,
			"九霄雷罡石":   5,
			"星河道力核":   5,
			"混元五炁珠":   3,
		},
	}
)

func eligibleForV221Compensation(player *model.Player) bool {
	return player != nil && strings.TrimSpace(player.AccountID) != "" && !player.CreatedAt.IsZero() && !player.CreatedAt.After(v221CompensationCutoff)
}

func (g *Game) v221CompensationClaimed(player *model.Player) bool {
	if player == nil {
		return false
	}
	var count int64
	_ = g.store.DB.Model(&model.AccountRewardClaim{}).
		Where("claim_key = ? AND (account_id = ? OR player_id = ?)", v221CompensationClaimKey, player.AccountID, player.ID).
		Count(&count).Error
	return count > 0
}

func v221CompensationRewardDescription() string {
	return strings.Join([]string{
		"货币：灵石×88888、银币×3888、功德×88、声望×88",
		"珍稀：万象归元纪念令×1、月华问道礼匣×2、灵根精粹×12、龙血芝×8、龙血芝孢子×8",
		"炼器：玄铁×188、阵基石×88、星辰砂×66、雷灵晶×36、妖兽内丹×128",
		"宝石：九霄雷罡石×5、星河道力核×5、混元五炁珠×3",
		"修行：功法残卷×30、双倍修为卡×8、扫荡券×50、传送符×30",
		"丹药：回元散×99、回灵丹×99、聚灵丹×50、淬脉丹×30、凝元丹×25、破境丹×20、九转还魂丹×3、轮回丹×1",
		"护道灵田：避劫符×8、引劫玉符×5、造化仙壤×20、地脉灵肥×50、灵壤肥×99",
	}, "\n")
}

func (g *Game) v221ServerCompensation(player *model.Player) GameResult {
	state := "不符合领取范围"
	actions := []string{"补偿公告", "活动菜单", "世界公告"}
	switch {
	case g.v221CompensationClaimed(player):
		state = "已领取，不可重复领取"
		actions = []string{"背包", "货币", "活动菜单"}
	case eligibleForV221Compensation(player):
		state = "可领取"
		actions = append([]string{"领取全服补偿"}, actions...)
	}
	content := fmt.Sprintf("批次：仙尘 v2.2.2 万象归元礼\n当前状态：%s\n领取范围：2026-07-24 23:59:59（北京时间）前已经建立道籍的玩家\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n本批次是独立补偿，是否领取过旧版补偿都不影响。补偿只增加上述物资，不发仙金、不把异常属性写成新的养成基线；等级基础属性由程序单独自动回正。领取凭证与全部奖励在同一事务结算，任何一项失败都会整体撤回。符合范围且仍保留原道籍的玩家长期拥有补领资格；领取成功后，删号、换OpenID或重复点击均不能再次领取。", state, v221CompensationRewardDescription())
	return GameResult{Title: "万象归元全服补偿", Content: content, Actions: actions}
}

func (g *Game) v221CompensationNotice() (GameResult, bool, error) {
	var notice model.Notice
	if err := g.store.DB.Where("code = ? AND published = ?", "world_notice_v222_compensation_20260724", true).First(&notice).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{
		Title:   notice.Title,
		Content: notice.Content,
		Actions: []string{"全服补偿", "领取全服补偿", "活动菜单", "世界公告"},
	}, true, nil
}

func (g *Game) claimV221ServerCompensation(player *model.Player) (GameResult, bool, error) {
	compensationClaimMu.Lock()
	defer compensationClaimMu.Unlock()

	rewardSnapshot, err := json.Marshal(v221CompensationReward)
	if err != nil {
		return GameResult{}, true, err
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, player.ID).Error; err != nil {
			return err
		}
		if !eligibleForV221Compensation(&current) {
			return errCompensationIneligible
		}
		var existing int64
		if err := tx.Model(&model.AccountRewardClaim{}).
			Where("claim_key = ? AND (account_id = ? OR player_id = ?)", v221CompensationClaimKey, current.AccountID, current.ID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errActivityClaimed
		}
		now := time.Now()
		receipt := model.AccountRewardClaim{
			AccountID: current.AccountID, ClaimKey: v221CompensationClaimKey, PlayerID: current.ID,
			RewardJSON: string(rewardSnapshot), ClaimedAt: now,
		}
		if err := tx.Create(&receipt).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return errActivityClaimed
			}
			return err
		}
		return grantActivityRewardTx(tx, current.ID, v221CompensationReward)
	})
	if errors.Is(err, errActivityClaimed) {
		return GameResult{Title: "全服补偿已经领取", Content: "该账号已经领取过仙尘 v2.2.2 万象归元礼。领取记录会跟随原道籍与角色ID保留，重复发送不会再次增加任何货币或物品。", Actions: []string{"背包", "货币", "活动菜单"}}, true, nil
	}
	if errors.Is(err, errCompensationIneligible) {
		return GameResult{Title: "不在本次补偿范围", Content: "本次补偿面向2026-07-24 23:59:59（北京时间）前已经建立道籍的玩家。此后新建立的道籍不受本次属性异常影响，因此不能领取。", Actions: []string{"活动菜单", "七日福利", "签到"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{
		Title:   "全服补偿领取成功",
		Content: "仙尘 v2.2.2 万象归元礼已经完整写入道籍。\n━━━━━━━━━━━\n" + v221CompensationRewardDescription() + "\n━━━━━━━━━━━\n本批次每个平台账号仅限一次；物品已进入乾坤袋，货币、功德与声望已经入账。",
		Actions: []string{"背包", "货币", "状态", "活动菜单"},
	}, true, nil
}
