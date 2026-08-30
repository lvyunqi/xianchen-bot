package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func (g *Game) arenaTierFor(rating int64) (model.ArenaTier, error) {
	var tier model.ArenaTier
	err := g.store.DB.Where("enabled = ? AND minimum_rating <= ?", true, rating).Order("minimum_rating DESC, sequence DESC").First(&tier).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = g.store.DB.Where("enabled = ?", true).Order("sequence").First(&tier).Error
	}
	return tier, err
}

func (g *Game) arenaProfile(player *model.Player) (GameResult, bool, error) {
	record, err := g.arenaRecord(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	tier, err := g.arenaTierFor(record.Rating)
	if err != nil {
		return GameResult{}, true, err
	}
	var tierCount int64
	_ = g.store.DB.Model(&model.ArenaTier{}).Where("enabled = ?", true).Count(&tierCount).Error
	total := record.Wins + record.Losses
	winRate := float64(0)
	if total > 0 {
		winRate = float64(record.Wins) * 100 / float64(total)
	}
	return GameResult{Title: "问剑竞技档案", Content: fmt.Sprintf("道号：%s\n段位：%s\n段位序位：%d/%d\n段位积分：%d\n战绩：%d胜 · %d负 · 共%d场\n胜率：%.1f%%\n竞技币：%d\n━━━━━━━━━━━\n胜利获得20竞技币与20积分；失败获得5竞技币并扣12积分。\n每日段位俸禄：竞技币%d · 银币%d\n段位道意：%s", player.DaoName, tier.Name, tier.Sequence, tierCount, record.Rating, record.Wins, record.Losses, total, winRate, player.ArenaCoins, tier.DailyCoin, tier.DailySilver, tier.Description), Actions: []string{"竞技", "竞技段位", "竞技奖励", "竞技商店", "竞榜"}}, true, nil
}

func (g *Game) arenaTierInfo(player *model.Player, raw string) (GameResult, bool, error) {
	record, err := g.arenaRecord(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	current, err := g.arenaTierFor(record.Rating)
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 10
	var total int64
	if err := g.store.DB.Model(&model.ArenaTier{}).Where("enabled = ?", true).Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 0)), 1)
	if strings.TrimSpace(raw) == "" {
		page = (current.Sequence-1)/pageSize + 1
	}
	if page > pages {
		page = pages
	}
	var tiers []model.ArenaTier
	if err := g.store.DB.Where("enabled = ?", true).Order("sequence").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tiers).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("当前：%s · %d积分 · 序位%d/%d", current.Name, record.Rating, current.Sequence, total), fmt.Sprintf("第%d/%d页 · 每页%d段", page, pages, pageSize), "━━━━━━━━━━━"}
	for _, tier := range tiers {
		mark := "○"
		if tier.ID == current.ID {
			mark = "●"
		}
		lines = append(lines, fmt.Sprintf("%s %s\n  晋阶积分%d · 日俸竞技币%d/银币%d\n  %s", mark, tier.Name, tier.MinimumRating, tier.DailyCoin, tier.DailySilver, tier.Description))
	}
	actions := []string{"竞技档案", "竞技奖励", "竞技", "竞榜"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("竞技段位 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("竞技段位 %d", page+1))
	}
	return GameResult{Title: "千阶问剑段位", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) claimArenaDailyReward(player *model.Player) (GameResult, bool, error) {
	today := time.Now().Format("2006-01-02")
	key := "arena.daily." + today
	if _, err := g.playerValue(player.ID, key); err == nil {
		return GameResult{Title: "竞技俸禄已领", Content: "今日段位俸禄已经领取，明日按届时段位重新结算。", Actions: []string{"竞技档案", "竞技商店"}}, true, nil
	}
	record, err := g.arenaRecord(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	tier, err := g.arenaTierFor(record.Rating)
	if err != nil {
		return GameResult{}, true, err
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		marker := model.PlayerValue{PlayerID: player.ID, Key: key, Value: tier.Name}
		if err := tx.Create(&marker).Error; err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"arena_coins": gorm.Expr("arena_coins + ?", tier.DailyCoin), "silver_coins": gorm.Expr("silver_coins + ?", tier.DailySilver)}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "竞技段位俸禄", Content: fmt.Sprintf("段位：%s\n竞技币：+%d\n银币：+%d\n结算积分：%d\n明日可再次领取。", tier.Name, tier.DailyCoin, tier.DailySilver, record.Rating), Actions: []string{"竞技商店", "竞技档案", "货币"}}, true, nil
}

func (g *Game) arenaGuide(player *model.Player) GameResult {
	return GameResult{Title: "问剑竞技规则", Content: "一、发送 `竞技` 匹配战力相近的对手。\n二、进入战斗后每回合可选择 `攻击`、`技能 功法名`、`防御` 或 `投降`。\n三、功法消耗法力，防御会降低本回合伤害，战斗状态会持续保存。\n四、胜利增加段位积分与20竞技币；失败扣除少量积分但仍得5竞技币。\n五、段位积分只用于排位，不会在商店被扣除。\n六、发送 `竞技奖励` 每日领取段位银币与竞技币。\n七、发送 `竞技商店` 用竞技币兑换物品。\n八、恶意中断不会复制奖励，所有胜负和货币变动均写入数据库事务。", Actions: []string{"竞技", "竞技档案", "竞技段位", "竞技奖励", "竞技商店", "竞榜"}}
}
