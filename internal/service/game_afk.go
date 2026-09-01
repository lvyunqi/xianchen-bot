package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type afkJob struct {
	Type          string    `json:"type"`
	Target        string    `json:"target"`
	StartedAt     time.Time `json:"started_at"`
	Interval      int       `json:"interval_minutes"`
	RequestedRuns int64     `json:"requested_runs"`
	CompletedRuns int64     `json:"completed_runs"`
}

func (g *Game) executeAFK(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	if command.Spec.ID == 246 {
		return g.startAFK(player, command.RawArguments)
	}
	return g.claimAFK(player, isAFKStopCommand(command.Spec.Command))
}

func isAFKStopCommand(command string) bool {
	switch strings.TrimSpace(command) {
	case "结束挂机", "停止挂机", "挂机结束", "挂机停止":
		return true
	default:
		return false
	}
}

func (g *Game) startAFK(player *model.Player, raw string) (GameResult, bool, error) {
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "无法挂机", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	if value, err := g.playerValue(player.ID, "afk.job"); err == nil && strings.TrimSpace(value) != "" {
		var active afkJob
		if json.Unmarshal([]byte(value), &active) == nil {
			queue := "持续挂机，直到手动收获"
			if active.RequestedRuns > 0 {
				queue = fmt.Sprintf("计划%d轮 · 已完成%d轮", active.RequestedRuns, active.CompletedRuns)
			}
			interval := active.Interval
			if interval < 1 {
				interval = maxInt(int(g.settingInt("afk.interval_minutes", 10)), 1)
			}
			return GameResult{Title: "挂机进行中", Content: fmt.Sprintf("目标：%s\n队列：%s\n已运行：%s\n每%d分钟积累一轮。", active.Target, queue, formatDuration(time.Since(active.StartedAt)), interval), Actions: []string{"领取挂机", "结束挂机", "系统"}}, true, nil
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GameResult{Title: "挂机设置", Content: "按次数：`挂机 猎妖*99` 或 `挂机 副本名*99`。\n持续挂机：`挂机 猎妖`。\n数量不设游戏上限，每轮仍需经过实际时间并在领取时扣除体力。", Actions: []string{"挂机 猎妖", "地图", "副本", "系统"}}, true, nil
	}
	interval := int(g.settingInt("afk.interval_minutes", 10))
	if interval < 1 {
		interval = 1
	}
	target := raw
	requestedRuns := int64(0)
	if strings.Contains(strings.ReplaceAll(raw, "×", "*"), "*") {
		var parseErr error
		target, requestedRuns, parseErr = parseStackQuantity(raw)
		if parseErr != nil {
			return GameResult{Title: "挂机数量错误", Content: parseErr.Error(), Actions: []string{"挂机", "副本"}}, true, nil
		}
	} else {
		parts := strings.Fields(raw)
		if len(parts) > 1 {
			if legacyMinutes, parseErr := strconv.ParseInt(parts[len(parts)-1], 10, 64); parseErr == nil && legacyMinutes > 0 {
				target = strings.Join(parts[:len(parts)-1], " ")
				requestedRuns = (legacyMinutes + int64(interval) - 1) / int64(interval)
			}
		}
	}
	job := afkJob{Target: target, StartedAt: time.Now(), Interval: interval, RequestedRuns: requestedRuns}
	if target == "猎妖" || target == "怪物" {
		job.Type = "monster"
	} else {
		dungeon, err := g.dungeonByName(target)
		if err != nil {
			return GameResult{Title: "挂机目标不存在", Content: "请输入当前已开放的副本名称，或使用 `挂机 猎妖`。", Actions: []string{"副本", "地图"}}, true, nil
		}
		var manualClears int64
		if err := g.store.DB.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND success = ? AND duration_ms > ?", player.ID, dungeon.ID, true, 0).Count(&manualClears).Error; err != nil {
			return GameResult{}, true, err
		}
		if manualClears == 0 {
			return GameResult{Title: "挂机副本未解锁", Content: "必须先手动逐回合通关“" + dungeon.Name + "”，挂机记录和其他扫荡不能代替首次通关。", Actions: []string{"进入 " + dungeon.Name, "副本"}}, true, nil
		}
		job.Type = "dungeon"
	}
	data, err := json.Marshal(job)
	if err != nil {
		return GameResult{}, true, err
	}
	if err := g.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		return GameResult{}, true, err
	}
	queueText := "持续挂机，无轮数上限"
	if job.RequestedRuns > 0 {
		queueText = fmt.Sprintf("计划%d轮 · 预计%d分钟", job.RequestedRuns, job.RequestedRuns*int64(job.Interval))
	}
	rule := "轮数不设输入上限，但领取时必须拥有足够体力。"
	if job.Type == "dungeon" {
		rule = "每轮消耗1张扫荡券，并计入副本原每日次数；输入轮数不能绕过首次通关、体力或次数限制。"
	}
	return GameResult{Title: "挂机开始", Content: fmt.Sprintf("目标：%s\n模式：%s\n队列：%s\n每%d分钟积累一轮\n规则：%s", job.Target, map[string]string{"monster": "地点猎妖", "dungeon": "副本扫荡"}[job.Type], queueText, job.Interval, rule), Actions: []string{"领取挂机", "结束挂机", "系统"}}, true, nil
}

func (g *Game) claimAFK(player *model.Player, stop bool) (GameResult, bool, error) {
	if value, err := g.playerValue(player.ID, "afk.job"); err != nil || strings.TrimSpace(value) == "" {
		return GameResult{Title: "没有挂机", Content: "先发送 `挂机 猎妖` 或 `挂机 副本名` 开始挂机。", Actions: []string{"挂机 猎妖", "系统"}}, true, nil
	}
	defaultInterval := maxInt(int(g.settingInt("afk.interval_minutes", 10)), 1)
	staminaMaximum, err := g.staminaMaximum(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	refreshedStamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}

	var job afkJob
	var status string
	var interval, waitMinutes int
	var dueRuns, runs, totalStaminaCost, remaining int64
	var rewardCultivation, rewardStones, rewardMerit int64
	var sweepTicketsSpent, dungeonDailyRemaining, dungeonDailyMaximum int64
	var levelProgress model.PlayerLevelProgress
	var selectedDungeon model.Dungeon
	var completed, stopped bool
	now := time.Now()
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var jobRow model.PlayerValue
		if findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", player.ID, "afk.job").First(&jobRow).Error; findErr != nil {
			if findErr == gorm.ErrRecordNotFound {
				status = "missing"
				return nil
			}
			return findErr
		}
		if json.Unmarshal([]byte(jobRow.Value), &job) != nil || job.StartedAt.IsZero() {
			status = "corrupt"
			if stop {
				if deleteErr := tx.Delete(&jobRow).Error; deleteErr != nil {
					return deleteErr
				}
				stopped = true
			}
			return nil
		}
		interval = job.Interval
		if interval < 1 {
			interval = defaultInterval
		}
		elapsed := now.Sub(job.StartedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		dueRuns = int64(elapsed / (time.Duration(interval) * time.Minute))
		if job.RequestedRuns > 0 {
			remainingRuns := job.RequestedRuns - job.CompletedRuns
			if remainingRuns <= 0 {
				if deleteErr := tx.Delete(&jobRow).Error; deleteErr != nil {
					return deleteErr
				}
				status = "completed"
				completed = true
				return nil
			}
			if dueRuns > remainingRuns {
				dueRuns = remainingRuns
			}
		}
		if dueRuns < 1 {
			if stop {
				if deleteErr := tx.Delete(&jobRow).Error; deleteErr != nil {
					return deleteErr
				}
				status = "stopped"
				stopped = true
				return nil
			}
			remainingDuration := time.Duration(interval)*time.Minute - elapsed
			waitMinutes = maxInt(int((remainingDuration+time.Minute-1)/time.Minute), 1)
			status = "waiting"
			return nil
		}

		var staminaCost, cultivationPerRun, stonesPerRun, meritPerRun int64
		if job.Type == "monster" || job.Target == "猎妖" || job.Target == "怪物" {
			staminaCost, cultivationPerRun, stonesPerRun, meritPerRun = 4, 8, 0, 2
		} else {
			var dungeon model.Dungeon
			if dungeonErr := tx.Where("name = ? AND enabled = ?", job.Target, true).First(&dungeon).Error; dungeonErr != nil {
				if dungeonErr != gorm.ErrRecordNotFound {
					return dungeonErr
				}
				status = "closed"
				if stop {
					if deleteErr := tx.Delete(&jobRow).Error; deleteErr != nil {
						return deleteErr
					}
					stopped = true
				}
				return nil
			}
			selectedDungeon = dungeon
			var manualClears int64
			if countErr := tx.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND success = ? AND duration_ms > ?", player.ID, dungeon.ID, true, 0).Count(&manualClears).Error; countErr != nil {
				return countErr
			}
			if manualClears == 0 {
				status = "uncleared"
				return nil
			}
			var limitErr error
			dungeonDailyRemaining, dungeonDailyMaximum, limitErr = dungeonRemainingRunsTx(tx, player.ID, dungeon, now.Format("2006-01-02"))
			if limitErr != nil {
				return limitErr
			}
			if dungeonDailyRemaining < 1 {
				status = "daily_limit"
				return nil
			}
			dueRuns = min64(dueRuns, dungeonDailyRemaining)
			var ticket model.Item
			if ticketErr := tx.Where("name = ?", "扫荡券").First(&ticket).Error; ticketErr != nil {
				return ticketErr
			}
			var owned model.PlayerItem
			if ticketErr := tx.Where("player_id = ? AND item_id = ?", player.ID, ticket.ID).First(&owned).Error; ticketErr != nil && ticketErr != gorm.ErrRecordNotFound {
				return ticketErr
			}
			if owned.Quantity < 1 {
				status = "tickets"
				return nil
			}
			dueRuns = min64(dueRuns, owned.Quantity)
			staminaCost = max64(int64(dungeon.StaminaCost), 1)
			cultivationPerRun, stonesPerRun, meritPerRun = max64(dungeon.RecommendedPower/3, 30), max64(dungeon.RecommendedPower/10, 5), 3
		}

		var staminaRow model.PlayerValue
		if staminaErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", player.ID, "stamina.value").First(&staminaRow).Error; staminaErr != nil && staminaErr != gorm.ErrRecordNotFound {
			return staminaErr
		}
		currentStamina := refreshedStamina
		if staminaRow.ID != 0 {
			if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(staminaRow.Value), 10, 64); parseErr == nil {
				currentStamina = parsed
			}
		}
		currentStamina = min64(max64(currentStamina, 0), staminaMaximum)
		runs = min64(dueRuns, currentStamina/staminaCost)
		if runs < 1 {
			status = "stamina"
			remaining = currentStamina
			return nil
		}
		if !validAFKProduct(staminaCost, runs) || !validAFKProduct(cultivationPerRun, runs) || !validAFKProduct(stonesPerRun, runs) || !validAFKProduct(meritPerRun, runs) {
			status = "overflow"
			return nil
		}
		totalStaminaCost = staminaCost * runs
		rewardCultivation = cultivationPerRun * runs
		rewardStones = stonesPerRun * runs
		rewardMerit = meritPerRun * runs
		remaining = currentStamina - totalStaminaCost

		var latest model.Player
		if playerErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "cultivation", "spirit_stones", "merit").First(&latest, player.ID).Error; playerErr != nil {
			return playerErr
		}
		if latest.Cultivation > math.MaxInt64-rewardCultivation || latest.SpiritStones > math.MaxInt64-rewardStones || latest.Merit > math.MaxInt64-rewardMerit {
			status = "overflow"
			return nil
		}
		totalAFKRuns := playerValueIntTx(tx, player.ID, "stats.afk_runs", 0)
		if totalAFKRuns < 0 || totalAFKRuns > math.MaxInt64-runs {
			status = "overflow"
			return nil
		}

		job.StartedAt = job.StartedAt.Add(time.Duration(runs) * time.Duration(interval) * time.Minute)
		job.CompletedRuns += runs
		completed = job.RequestedRuns > 0 && job.CompletedRuns >= job.RequestedRuns
		stopped = stop && runs == dueRuns
		if completed || stopped {
			if deleteErr := tx.Delete(&jobRow).Error; deleteErr != nil {
				return deleteErr
			}
		} else {
			data, marshalErr := json.Marshal(job)
			if marshalErr != nil {
				return marshalErr
			}
			if updateErr := tx.Model(&jobRow).Updates(map[string]any{"value": string(data), "expires_at": nil}).Error; updateErr != nil {
				return updateErr
			}
		}
		if err := upsertPlayerValueTx(tx, player.ID, "stamina.value", strconv.FormatInt(remaining, 10), nil); err != nil {
			return err
		}
		if selectedDungeon.ID != 0 {
			var ticket model.Item
			if err := tx.Where("name = ?", "扫荡券").First(&ticket).Error; err != nil {
				return err
			}
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, ticket.ID, -runs); err != nil {
				return errDungeonInventoryChanged
			}
			sweepTicketsSpent = runs
			dungeonRuns := make([]model.DungeonRun, 0, int(runs))
			for index := int64(0); index < runs; index++ {
				dungeonRuns = append(dungeonRuns, model.DungeonRun{PlayerID: player.ID, DungeonID: selectedDungeon.ID, RunDate: now.Format("2006-01-02"), DurationMS: 0, Score: player.CombatPower, Success: true})
			}
			if err := tx.Create(&dungeonRuns).Error; err != nil {
				return err
			}
			if _, err := addPlayerValueIntTx(tx, player.ID, "stats.dungeons", runs); err != nil {
				return err
			}
			dungeonDailyRemaining -= runs
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"spirit_stones": gorm.Expr("spirit_stones + ?", rewardStones),
			"merit":         gorm.Expr("merit + ?", rewardMerit),
		}).Error; err != nil {
			return err
		}
		levelProgress, err = grantCultivationExperienceTx(tx, player.ID, rewardCultivation)
		if err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "stats.afk_runs", strconv.FormatInt(totalAFKRuns+runs, 10), nil); err != nil {
			return err
		}
		status = "settled"
		return nil
	})
	if errors.Is(err, errDungeonInventoryChanged) {
		return GameResult{Title: "挂机扫荡券数量变化", Content: "结算时扫荡券数量发生变化，本次事务已全部回滚；挂机轮次、体力、次数和奖励均未改变。", Actions: []string{"物品 扫荡券", "领取挂机", "结束挂机"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}

	switch status {
	case "missing":
		return GameResult{Title: "没有挂机", Content: "该任务已被其他结算请求领取。可重新开始挂机。", Actions: []string{"挂机 猎妖", "系统"}}, true, nil
	case "corrupt":
		if stopped {
			return GameResult{Title: "挂机已结束", Content: "旧挂机任务数据不完整，已安全清除；未扣体力，未发放奖励。", Actions: []string{"挂机 猎妖", "系统"}}, true, nil
		}
		return GameResult{Title: "挂机数据损坏", Content: "请发送 `结束挂机` 清理旧任务，再重新开始。", Actions: []string{"结束挂机", "系统"}}, true, nil
	case "completed":
		return GameResult{Title: "挂机队列已完成", Content: fmt.Sprintf("目标：%s\n计划轮数：%d\n全部奖励已经领取。", job.Target, job.RequestedRuns), Actions: []string{"挂机 " + job.Target, "系统"}}, true, nil
	case "stopped":
		return GameResult{Title: "挂机已结束", Content: fmt.Sprintf("目标：%s\n尚未积累完整轮次，本次未扣体力、未发放奖励。", job.Target), Actions: []string{"挂机 " + job.Target, "系统"}}, true, nil
	case "waiting":
		return GameResult{Title: "挂机尚未可领", Content: fmt.Sprintf("目标：%s\n还需%d分钟积累第一个完整轮次。", job.Target, waitMinutes), Actions: []string{"领取挂机", "结束挂机", "系统"}}, true, nil
	case "closed":
		if stopped {
			return GameResult{Title: "挂机已结束", Content: fmt.Sprintf("副本“%s”已关闭，挂机任务已清除；未扣体力、未发放奖励。", job.Target), Actions: []string{"副本", "挂机 猎妖"}}, true, nil
		}
		return GameResult{Title: "副本已关闭", Content: "该挂机目标目前没有开放。可发送 `结束挂机` 安全清理任务。", Actions: []string{"结束挂机", "副本", "系统"}}, true, nil
	case "uncleared":
		return GameResult{Title: "挂机副本未解锁", Content: "该副本缺少手动逐回合通关记录，任务未结算、未扣资源。可结束挂机后先手动通关。", Actions: []string{"结束挂机", "进入 " + job.Target, "副本"}}, true, nil
	case "daily_limit":
		return GameResult{Title: "挂机等待副本次数", Content: fmt.Sprintf("%s今日剩余次数0/%d，成熟轮次仍保留在挂机队列。每日次数刷新或使用合法重置次数后可继续领取。", job.Target, dungeonDailyMaximum), Actions: []string{"副本", "重副", "领取挂机", "结束挂机"}}, true, nil
	case "tickets":
		return GameResult{Title: "挂机等待扫荡券", Content: fmt.Sprintf("%s已经积累%d个成熟轮次，但每轮需要扫荡券×1。任务与轮次均保留，本次没有扣体力或次数。", job.Target, dueRuns), Actions: []string{"物品 扫荡券", "神秘商城", "限时商城", "领取挂机", "结束挂机"}}, true, nil
	case "stamina":
		return GameResult{Title: "挂机等待体力", Content: fmt.Sprintf("已积累%d个可领轮次，当前体力%d/%d不足支付一轮。\n任务和奖励轮次已保留，恢复体力后可继续领取。", dueRuns, remaining, staminaMaximum), Actions: []string{"体力", "领取挂机", "结束挂机", "系统"}}, true, nil
	case "overflow":
		return GameResult{Title: "挂机数值超出安全范围", Content: "本次没有扣除体力、没有改动队列。请联系主人检查玩家余额或副本配置。", Actions: []string{"状态", "货币", "系统"}}, true, nil
	}

	queueText := "持续挂机中"
	if completed {
		queueText = fmt.Sprintf("计划%d轮已全部完成", job.RequestedRuns)
	} else if stopped {
		queueText = "已领取全部成熟轮次并结束挂机"
	} else if job.RequestedRuns > 0 {
		queueText = fmt.Sprintf("队列进度%d/%d轮", job.CompletedRuns, job.RequestedRuns)
	}
	if stop && !stopped && runs < dueRuns {
		queueText = fmt.Sprintf("本次仅领取%d/%d个成熟轮次；体力不足，任务已保留", runs, dueRuns)
	}
	title := "挂机结算"
	if stopped {
		title = "挂机结束"
	}
	actions := []string{"领取挂机", "结束挂机", "系统", "状态"}
	if completed || stopped {
		actions = []string{"挂机 " + job.Target, "系统", "状态"}
	}
	extra := ""
	if sweepTicketsSpent > 0 {
		extra = fmt.Sprintf("\n消耗扫荡券：%d\n今日副本剩余：%d/%d", sweepTicketsSpent, dungeonDailyRemaining, dungeonDailyMaximum)
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	result := GameResult{Title: title, Content: fmt.Sprintf("目标：%s\n本次领取：%d轮\n%s\n修为：+%d\n灵石：+%d\n功德：+%d\n消耗体力：%d\n剩余体力：%d%s", job.Target, runs, queueText, rewardCultivation, rewardStones, rewardMerit, totalStaminaCost, remaining, extra), Actions: actions}
	appendPlayerLevelSettlement(&result, latest, levelProgress)
	return result, true, nil
}

func validAFKProduct(value, count int64) bool {
	return value >= 0 && count >= 0 && (value == 0 || count <= math.MaxInt64/value)
}

func max(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
