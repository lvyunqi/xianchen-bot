package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func (g *Game) checkIn(player *model.Player) (GameResult, bool, error) {
	today := time.Now().Format("2006-01-02")
	last, _ := g.playerValue(player.ID, "checkin.last_date")
	if strings.TrimSpace(last) == today {
		streak := g.playerValueInt(player.ID, "checkin.streak", 0)
		return GameResult{Title: "今日签到", Content: fmt.Sprintf("今日道印已经落下，不能重复签到。\n连续签到：%d天\n明日再来，可领取下一份七日签到奖励。", streak), Actions: []string{"签到记录", "任务菜单"}}, true, nil
	}

	streak := g.playerValueInt(player.ID, "checkin.streak", 0)
	if lastDate, err := time.Parse("2006-01-02", strings.TrimSpace(last)); err != nil || !sameCalendarDay(lastDate, time.Now().AddDate(0, 0, -1)) {
		streak = 0
	}
	streak++
	day := (streak-1)%7 + 1
	var reward model.CheckinReward
	if err := g.store.DB.Where("day = ?", day).First(&reward).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return GameResult{Title: "签到道印缺失", Content: fmt.Sprintf("七日签到第%d天的道印尚未载入，请主人检查签到配置。", day), Actions: []string{"签到记录", "任务菜单"}}, true, nil
		}
		return GameResult{}, true, err
	}

	itemText := strings.TrimSpace(reward.ItemName)
	if itemText != "" && reward.Quantity > 0 {
		var item model.Item
		if err := g.store.DB.Where("name = ? OR code = ?", itemText, itemText).First(&item).Error; err != nil {
			return GameResult{Title: "签到奖励暂缺", Content: fmt.Sprintf("第%d天奖励“%s”尚未在物品库配置，签到未扣除次数。", day, itemText), Actions: []string{"签到记录", "背包"}}, true, nil
		}
		if err := g.players.AdjustItem(player.ID, item.ID, reward.Quantity); err != nil {
			return GameResult{}, true, err
		}
	}
	if err := g.setPlayerValue(player.ID, "checkin.last_date", today, nil); err != nil {
		return GameResult{}, true, err
	}
	if err := g.setPlayerValueInt(player.ID, "checkin.streak", streak); err != nil {
		return GameResult{}, true, err
	}
	silverReward := g.settingInt("checkin.silver_reward", int64(100+day*20))
	if silverReward < 0 {
		silverReward = 0
	}
	if _, err := g.players.UpdateColumnWhere(player.ID, "silver_coins", gorm.Expr("silver_coins + ?", silverReward), ""); err != nil {
		return GameResult{}, true, err
	}
	nextDay := streak%7 + 1
	content := fmt.Sprintf("🌄 签到成功 · 第%d天\n━━━━━━━━━━━\n连续签到：%d天\n银币：+%d\n今日物品：%s × %d", day, streak, silverReward, displayOr(itemText, "暂无物品"), maxInt64(reward.Quantity, 0))
	if strings.TrimSpace(reward.SpecialReward) != "" {
		content += "\n特殊奖励：" + reward.SpecialReward
	}
	content += fmt.Sprintf("\n明日签到：第%d天\n奖励不会因查看记录而消失。", nextDay)
	return GameResult{Title: "每日签到", Content: content, Actions: []string{"签到记录", "银币商城", "货币", "背包", "任务菜单"}}, true, nil
}

func (g *Game) checkInRecord(player *model.Player) (GameResult, bool, error) {
	var rewards []model.CheckinReward
	if err := g.store.DB.Where("day BETWEEN ? AND ?", 1, 7).Order("day").Find(&rewards).Error; err != nil {
		return GameResult{}, true, err
	}
	sort.SliceStable(rewards, func(i, j int) bool { return rewards[i].Day < rewards[j].Day })
	streak := g.playerValueInt(player.ID, "checkin.streak", 0)
	last, _ := g.playerValue(player.ID, "checkin.last_date")
	lines := []string{fmt.Sprintf("📜 七日签到记录\n连续签到：%d天\n上次签到：%s\n━━━━━━━━━━━", streak, displayOr(last, "尚未签到"))}
	for _, reward := range rewards {
		marker := "○"
		if streak > 0 && int64(reward.Day) <= (streak-1)%7+1 {
			marker = "●"
		}
		rewardText := fmt.Sprintf("%s 第%d天：%s × %d", marker, reward.Day, displayOr(reward.ItemName, "暂无物品"), reward.Quantity)
		if strings.TrimSpace(reward.SpecialReward) != "" {
			rewardText += " · " + reward.SpecialReward
		}
		lines = append(lines, rewardText)
	}
	if len(rewards) == 0 {
		lines = append(lines, "今日签到道印尚未载入。")
	}
	lines = append(lines, "━━━━━━━━━━━", "发送 `签到` 领取今日奖励。")
	return GameResult{Title: "签到记录", Content: strings.Join(lines, "\n"), Actions: []string{"签到", "背包", "任务菜单"}}, true, nil
}

func sameCalendarDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
