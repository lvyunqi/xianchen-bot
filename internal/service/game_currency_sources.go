package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

var errSilverJobBlocked = errors.New("silver job blocked")

func (g *Game) silverIncomeGuide(player *model.Player) (GameResult, bool, error) {
	checkinReward := max64(g.settingInt("checkin.silver_reward", 120), 0)
	checkinState := "今日可领"
	if last, _ := g.playerValue(player.ID, "checkin.last_date"); strings.TrimSpace(last) == time.Now().Format("2006-01-02") {
		checkinState = "今日已领"
	}

	sequence, _ := g.playerRealmSequence(player)
	var tasks []model.TaskTemplate
	if err := g.store.DB.Where("enabled = ? AND type NOT IN ?", true, []string{"成就", "称号"}).Find(&tasks).Error; err != nil {
		return GameResult{}, true, err
	}
	availableTasks := 0
	minimumTaskSilver, maximumTaskSilver := int64(0), int64(0)
	for _, task := range tasks {
		realm, level, _ := taskRealmTier(task)
		if realm > sequence || realm == sequence && level > player.RealmLevel {
			continue
		}
		silver := model.TaskSilverReward(task)
		if silver <= 0 {
			continue
		}
		availableTasks++
		if minimumTaskSilver == 0 || silver < minimumTaskSilver {
			minimumTaskSilver = silver
		}
		if silver > maximumTaskSilver {
			maximumTaskSilver = silver
		}
	}
	taskIncome := "当前境界暂无可领委托"
	if availableTasks > 0 {
		taskIncome = fmt.Sprintf("当前境界%d项，每项%d-%d", availableTasks, minimumTaskSilver, maximumTaskSilver)
	}

	arenaIncome := "完成问剑定级后可领"
	if record, err := g.arenaRecord(player.ID); err == nil {
		if tier, tierErr := g.arenaTierFor(record.Rating); tierErr == nil {
			state := "今日可领"
			if _, claimedErr := g.playerValue(player.ID, "arena.daily."+time.Now().Format("2006-01-02")); claimedErr == nil {
				state = "今日已领"
			}
			arenaIncome = fmt.Sprintf("%s每日%d（%s）", tier.Name, tier.DailySilver, state)
		}
	}

	content := fmt.Sprintf("当前银币：%d\n━━━━━━━━━━━\n一、每日签到：+%d（%s）\n二、日常/悬赏/地图任务：%s\n三、竞技段位俸禄：%s\n四、仙盟差事：发送“赚银币”，消耗少量体力完成一趟真实差事\n五、七日目标、境界冲刺、天降鸿运与密令：按各页标明数额领取\n六、邀请、助力修炼、新秀榜：完成真实社交目标后领取\n七、有效BUG/建议：初审通过后按反馈规则发放\n━━━━━━━━━━━\n钱庄借款只能应急且需要还本付息，不计作收益；仙金不能兑换或生成银币。", player.SilverCoins, checkinReward, checkinState, taskIncome, arenaIncome)
	actions := []string{"赚银币", "签到", "日常", "悬赏", "任务菜单", "竞技奖励", "七日目标", "活动总览", "邀请道友", "反馈菜单", "钱庄", "银币商城", "货币"}
	return GameResult{Title: "🪙 银币来源", Content: content, Actions: actions}, true, nil
}

func (g *Game) earnSilverCoins(player *model.Player) (GameResult, bool, error) {
	if player.Health <= 1 {
		return GameResult{Title: "仙盟差事无法承接", Content: "元神离体时不能承接仙盟差事，请先返回地脉复生阵。", Actions: []string{"回城复活", "状态"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "仙盟差事无法承接", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	cooldownMinutes := max64(g.settingInt("silver_job.cooldown_minutes", 10), 1)
	cooldownDuration := time.Duration(cooldownMinutes) * time.Minute
	cooldownKey := "cooldown.silver_job"
	if value, err := g.playerValue(player.ID, cooldownKey); err == nil {
		if until, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil && until.After(time.Now()) {
			return silverJobCooldownResult(time.Until(until)), true, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}

	staminaCost := max64(g.settingInt("silver_job.stamina_cost", 4), 1)
	currentStamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	staminaMaximum, err := g.staminaMaximum(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if currentStamina < staminaCost {
		return silverJobStaminaResult(currentStamina, staminaMaximum, staminaCost), true, nil
	}
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return GameResult{}, true, err
	}

	var silverReward, cultivationReward, remainingStamina, balance int64
	var jobName string
	var blocked GameResult
	now := time.Now()
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var cooldown model.PlayerValue
		cooldownErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", player.ID, cooldownKey).First(&cooldown).Error
		if cooldownErr == nil {
			until, parseErr := time.Parse(time.RFC3339Nano, cooldown.Value)
			if parseErr == nil && until.After(now) {
				blocked = silverJobCooldownResult(time.Until(until))
				return errSilverJobBlocked
			}
		} else if !errors.Is(cooldownErr, gorm.ErrRecordNotFound) {
			return cooldownErr
		}

		var latest model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&latest, player.ID).Error; err != nil {
			return err
		}
		stamina := currentStamina
		var staminaRow model.PlayerValue
		staminaErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", player.ID, "stamina.value").First(&staminaRow).Error
		if staminaErr == nil {
			if parsed, parseErr := strconv.ParseInt(staminaRow.Value, 10, 64); parseErr == nil {
				stamina = parsed
			}
		} else if !errors.Is(staminaErr, gorm.ErrRecordNotFound) {
			return staminaErr
		}
		if stamina < staminaCost {
			blocked = silverJobStaminaResult(stamina, staminaMaximum, staminaCost)
			return errSilverJobBlocked
		}

		level := int64(maxInt(latest.Level, 1))
		silverReward = 40 + int64(maxInt(sequence, 1))*8 + int64(maxInt(latest.RealmLevel, 1))*2 + level/5
		cultivationReward = 8 + int64(maxInt(sequence, 1))*2 + int64(maxInt(latest.RealmLevel, 1)) + level/20
		if latest.SilverCoins > math.MaxInt64-silverReward || latest.Cultivation > math.MaxInt64-cultivationReward {
			blocked = GameResult{Title: "仙盟差事结算受阻", Content: "当前银币或修为已经达到安全数值上限，本次没有扣除体力，也没有写入冷却。", Actions: []string{"货币", "状态"}}
			return errSilverJobBlocked
		}
		jobs := []string{"抄录护山阵图", "清点灵药库藏", "巡查山门地脉", "押送仙盟灵材", "修缮接引阵基"}
		jobName = jobs[int((int64(latest.ID)+now.Unix()/int64(cooldownDuration/time.Second))%int64(len(jobs)))]
		remainingStamina = stamina - staminaCost
		balance = latest.SilverCoins + silverReward
		if err := tx.Model(&model.Player{}).Where("id = ?", latest.ID).Updates(map[string]any{
			"silver_coins": balance,
			"cultivation":  latest.Cultivation + cultivationReward,
		}).Error; err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, latest.ID, "stamina.value", strconv.FormatInt(remainingStamina, 10), nil); err != nil {
			return err
		}
		until := now.Add(cooldownDuration)
		return upsertPlayerValueTx(tx, latest.ID, cooldownKey, until.Format(time.RFC3339Nano), &until)
	})
	if errors.Is(err, errSilverJobBlocked) {
		return blocked, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	content := fmt.Sprintf("差事：%s\n你替仙盟完成一趟短差，报酬已经记入钱庄。\n━━━━━━━━━━━\n银币：+%d（当前%d）\n修为：+%d\n体力：-%d（剩余%d/%d）\n━━━━━━━━━━━\n报酬会随角色等级、当前大境和层数缓慢增长；%d分钟后可再次承接。", jobName, silverReward, balance, cultivationReward, staminaCost, remainingStamina, staminaMaximum, cooldownMinutes)
	return GameResult{Title: "🪙 仙盟差事完成", Content: content, Actions: []string{"赚银币", "等级", "银币来源", "货币", "体力"}}, true, nil
}

func silverJobCooldownResult(remaining time.Duration) GameResult {
	if remaining < time.Second {
		remaining = time.Second
	}
	return GameResult{Title: "仙盟差事尚未刷新", Content: "下一批仙盟差事还需" + formatDuration(remaining) + "刷新。本次没有扣除体力，也没有重复写入冷却。", Actions: []string{"银币来源", "日常", "悬赏", "体力"}}
}

func silverJobStaminaResult(current, maximum, cost int64) GameResult {
	return GameResult{Title: "仙盟差事体力不足", Content: fmt.Sprintf("承接差事需要体力%d，当前%d/%d。本次没有写入冷却，体力恢复后可直接重试。", cost, current, maximum), Actions: []string{"体力", "状态", "银币来源"}}
}
