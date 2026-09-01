package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type configuredEventChoice struct {
	Name        string         `json:"name"`
	SuccessRate float64        `json:"success_rate"`
	Reward      map[string]any `json:"reward"`
	Failure     map[string]any `json:"failure"`
}

type configuredEventRewards struct {
	Choices []configuredEventChoice `json:"choices"`
}

type pendingConfiguredEvent struct {
	EventID   uint      `json:"event_id"`
	StartedAt time.Time `json:"started_at"`
}

func (g *Game) triggerConfiguredEvent(player *model.Player, remainingStamina int64) (GameResult, bool, error) {
	var events []model.Event
	if err := g.store.DB.Where("enabled = ?", true).Find(&events).Error; err != nil {
		return GameResult{}, false, err
	}
	eligible := make([]model.Event, 0, len(events))
	for _, event := range events {
		allowed, err := g.eventConditionsMet(player, event.ConditionJSON)
		if err != nil {
			continue
		}
		if allowed {
			eligible = append(eligible, event)
		}
	}
	if len(eligible) == 0 {
		return GameResult{}, false, nil
	}
	event := eligible[rand.Intn(len(eligible))]
	baseProbability := event.Probability
	if baseProbability <= 0 {
		baseProbability = .05
	}
	if baseProbability > 1 {
		baseProbability = 1
	}
	probability := baseProbability
	triggerBonus := float64(0)
	if eventReceivesLuckBonus(event.Type) {
		probability, triggerBonus = probabilityWithLuck(probability, player.Luck, luckEventTriggerBonusCap)
	}
	if rand.Float64() > probability {
		return GameResult{}, false, nil
	}
	var rewards configuredEventRewards
	if json.Unmarshal([]byte(event.RewardJSON), &rewards) == nil && len(rewards.Choices) > 0 {
		expires := time.Now().Add(10 * time.Minute)
		pending := pendingConfiguredEvent{EventID: event.ID, StartedAt: time.Now()}
		encoded, _ := json.Marshal(pending)
		if err := g.setPlayerValue(player.ID, "event.pending", string(encoded), &expires); err != nil {
			return GameResult{}, false, err
		}
		lines := []string{event.Description, "", "【可选分支】"}
		actions := make([]string, 0, len(rewards.Choices)+2)
		for _, choice := range rewards.Choices {
			rateText := "因果必然"
			if choice.SuccessRate > 0 && choice.SuccessRate < 1 {
				actualRate, luckBonus := probabilityWithLuck(choice.SuccessRate, player.Luck, luckEventChoiceBonusCap)
				rateText = fmt.Sprintf("基础%.0f%% · 运气+%.1f%% · 实际%.1f%%", choice.SuccessRate*100, luckBonus*100, actualRate*100)
			}
			lines = append(lines, fmt.Sprintf("- %s · %s\n  成功：%s\n  失败：%s", choice.Name, rateText, eventRewardText(choice.Reward), eventRewardText(choice.Failure)))
			actions = append(actions, "抉择 "+choice.Name)
		}
		lines = append(lines, "", fmt.Sprintf("触发率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%", baseProbability*100, triggerBonus*100, probability*100), fmt.Sprintf("剩余体力：%d · 十分钟内必须作出抉择", remainingStamina))
		return GameResult{Title: event.Name, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	var direct map[string]any
	if err := json.Unmarshal([]byte(event.RewardJSON), &direct); err != nil {
		return GameResult{Title: event.Name, Content: event.Description + "\n机缘奖励道纹无法解析，本次不会发放或扣除资源，请主人检查事件配置。", Actions: []string{"探索"}}, true, nil
	}
	if nested, ok := direct["on_win"].(map[string]any); ok {
		direct = nested
	} else if nested, ok := direct["success"].(map[string]any); ok {
		direct = nested
	}
	if err := g.applyConfiguredEventReward(player, direct); err != nil {
		return GameResult{}, false, err
	}
	luckLine := ""
	if isLuckGrowthEncounter(event.Type) {
		var luckErr error
		luckLine, luckErr = g.tryGrowLuckFromEncounter(player)
		if luckErr != nil {
			return GameResult{}, false, luckErr
		}
	}
	if luckLine != "" {
		luckLine = "\n" + luckLine
	}
	return GameResult{Title: event.Name, Content: fmt.Sprintf("%s\n━━━━━━━━━━━\n触发率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n机缘所得：%s%s\n剩余体力：%d", event.Description, baseProbability*100, triggerBonus*100, probability*100, eventRewardText(direct), luckLine, remainingStamina), Actions: []string{"探索", "仙缘", "状态"}}, true, nil
}

func (g *Game) resolveEventChoice(player *model.Player, raw string) (GameResult, bool, error) {
	choiceName := strings.TrimSpace(raw)
	if choiceName == "" {
		return GameResult{Title: "奇遇抉择", Content: "请输入：`抉择 选项名`。必须先在探索中触发带分支的奇遇。", Actions: []string{"探索"}}, true, nil
	}
	value, err := g.playerValue(player.ID, "event.pending")
	if err != nil {
		return GameResult{Title: "没有待决奇遇", Content: "当前没有等待选择的奇遇，或抉择时间已经超过十分钟。", Actions: []string{"探索"}}, true, nil
	}
	var pending pendingConfiguredEvent
	if json.Unmarshal([]byte(value), &pending) != nil || pending.EventID == 0 {
		return GameResult{Title: "奇遇记录损坏", Content: "待决奇遇无法读取，已清除，请重新探索。", Actions: []string{"探索"}}, true, nil
	}
	var event model.Event
	if g.store.DB.First(&event, pending.EventID).Error != nil || !event.Enabled {
		return GameResult{Title: "奇遇已经消散", Content: "这段机缘已经关闭，请重新探索寻找其他缘法。", Actions: []string{"探索"}}, true, nil
	}
	var rewards configuredEventRewards
	if json.Unmarshal([]byte(event.RewardJSON), &rewards) != nil {
		return GameResult{Title: "奇遇配置错误", Content: "事件分支不是有效 JSON。"}, true, nil
	}
	var selected *configuredEventChoice
	for index := range rewards.Choices {
		if rewards.Choices[index].Name == choiceName {
			selected = &rewards.Choices[index]
			break
		}
	}
	if selected == nil {
		actions := make([]string, 0, len(rewards.Choices))
		for _, choice := range rewards.Choices {
			actions = append(actions, "抉择 "+choice.Name)
		}
		return GameResult{Title: "选项不存在", Content: "“" + choiceName + "”不是当前奇遇可选分支，请点击下方选择。", Actions: actions}, true, nil
	}
	rate := selected.SuccessRate
	if rate <= 0 {
		rate = 1
	}
	baseRate := rate
	rate, luckBonus := probabilityWithLuck(rate, player.Luck, luckEventChoiceBonusCap)
	succeeded := rand.Float64() <= rate
	outcome := selected.Reward
	resultTitle := event.Name + "·因果已定"
	resultText := "你选择了“" + selected.Name + "”，顺利承接这段因果。"
	if !succeeded {
		outcome = selected.Failure
		if len(outcome) == 0 {
			outcome = map[string]any{"health_percent": -10}
		}
		resultTitle = event.Name + "·因果反噬"
		resultText = "你选择了“" + selected.Name + "”，却未能驾驭其中变数。"
	}
	if err := g.applyConfiguredEventReward(player, outcome); err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Where("player_id = ? AND key = ?", player.ID, "event.pending").Delete(&model.PlayerValue{}).Error
	luckLine := ""
	if succeeded && isLuckGrowthEncounter(event.Type) {
		luckLine, err = g.tryGrowLuckFromEncounter(player)
		if err != nil {
			return GameResult{}, true, err
		}
	}
	if luckLine != "" {
		luckLine = "\n" + luckLine
	}
	result := GameResult{Title: resultTitle, Content: fmt.Sprintf("%s\n成功率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n结算：%s%s", resultText, baseRate*100, luckBonus*100, rate*100, eventRewardText(outcome), luckLine), Actions: []string{"探索", "仙缘", "状态", "全区通报"}}
	if succeeded && (rewardNumber(outcome, "immortal_affinity") > 0 || rewardNumber(outcome, "merit") >= 10) {
		broadcast := fmt.Sprintf("【仙缘奇遇】%s在%s中选择“%s”，承下一段不凡因果：%s。", player.DaoName, event.Name, selected.Name, eventRewardText(outcome))
		_ = g.publishWorldBroadcast("仙缘", player.DaoName+"触发"+event.Name, broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func (g *Game) eventConditionsMet(player *model.Player, raw string) (bool, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return true, nil
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return false, err
	}
	realmSequence, err := g.playerRealmSequence(player)
	if err != nil {
		return false, err
	}
	checks := []struct {
		key     string
		current int64
	}{
		{"minimum_level", int64(player.Level)}, {"minimum_luck", player.Luck},
		{"luck", player.Luck}, {"min_perception", player.Perception},
	}
	for _, check := range checks {
		if required := eventNumber(values, check.key); required > 0 && check.current < required {
			return false, nil
		}
	}
	requiredSequence := int(eventNumber(values, "minimum_realm_sequence"))
	if requiredSequence > 0 && realmSequence < requiredSequence {
		return false, nil
	}
	requiredLayer := int(eventNumber(values, "minimum_realm_level"))
	if requiredLayer > 0 && (requiredSequence == 0 || realmSequence == requiredSequence) && player.RealmLevel < requiredLayer {
		return false, nil
	}
	if requiredRealm, _ := values["min_realm"].(string); requiredRealm != "" {
		var realm model.Realm
		if g.store.DB.Where("name = ?", requiredRealm).First(&realm).Error == nil && realmSequence < realm.Sequence {
			return false, nil
		}
	}
	if location, _ := values["location"].(string); location != "" && player.Location != location {
		return false, nil
	}
	if region, _ := values["region"].(string); region != "" {
		var location model.WorldLocation
		if g.store.DB.Where("name = ?", player.Location).First(&location).Error != nil || location.Region != region {
			return false, nil
		}
	}
	return true, nil
}

func (g *Game) applyConfiguredEventReward(player *model.Player, reward map[string]any) error {
	effective := g.playerWithActiveSkillStats(player)
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		updates := make(map[string]any)
		for key, column := range map[string]string{"cultivation": "cultivation", "spirit_stones": "spirit_stones", "merit": "merit", "reputation": "reputation", "immortal_affinity": "immortal_affinity", "dao_heart": "dao_heart", "perception": "perception", "root_quality": "root_quality", "physical_attack": "physical_attack"} {
			if value := rewardNumber(reward, key); value != 0 {
				updates[column] = gorm.Expr(column+" + ?", value)
			}
		}
		if delta := rewardNumber(reward, "luck"); delta != 0 {
			var current int64
			if err := tx.Model(&model.Player{}).Select("luck").Where("id = ?", player.ID).Scan(&current).Error; err != nil {
				return err
			}
			updates["luck"] = normalizedPlayerLuck(current + delta)
		}
		if percent := rewardNumber(reward, "health_percent"); percent != 0 {
			health := effective.Health + effective.MaxHealth*percent/100
			updates["health"] = min64(max64(health, 1), effective.MaxHealth)
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
		items, _ := reward["items"].(map[string]any)
		repo := storage.NewPlayerRepository(tx)
		for itemName, rawQuantity := range items {
			quantity := int64FromAny(rawQuantity)
			if quantity <= 0 {
				continue
			}
			var item model.Item
			if err := tx.Where("name = ?", itemName).First(&item).Error; err != nil {
				return err
			}
			if err := repo.AdjustItem(player.ID, item.ID, quantity); err != nil {
				return err
			}
		}
		return nil
	})
}

func eventNumber(values map[string]any, key string) int64  { return int64FromAny(values[key]) }
func rewardNumber(values map[string]any, key string) int64 { return int64FromAny(values[key]) }

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

func eventRewardText(reward map[string]any) string {
	if len(reward) == 0 {
		return "无额外所得"
	}
	labels := map[string]string{"cultivation": "修为", "spirit_stones": "灵石", "merit": "功德", "reputation": "声望", "immortal_affinity": "仙缘", "dao_heart": "道心", "perception": "悟性", "root_quality": "灵根纯度", "physical_attack": "攻击", "health_percent": "气血比例", "luck": "运气"}
	order := []string{"cultivation", "spirit_stones", "merit", "reputation", "immortal_affinity", "dao_heart", "perception", "root_quality", "physical_attack", "health_percent", "luck"}
	lines := make([]string, 0, len(reward))
	for _, key := range order {
		if value := rewardNumber(reward, key); value != 0 {
			lines = append(lines, fmt.Sprintf("%s%+d", labels[key], value))
		}
	}
	if items, ok := reward["items"].(map[string]any); ok {
		for name, value := range items {
			lines = append(lines, fmt.Sprintf("%s×%d", name, int64FromAny(value)))
		}
	}
	if len(lines) == 0 {
		return displayConfigText(mustJSON(reward))
	}
	return strings.Join(lines, "、")
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
