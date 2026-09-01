package service

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

type feedbackAssessment struct {
	Feasible bool
	Score    int
	Reason   string
}

type feedbackResolutionPlan struct {
	Diagnosis        string
	ResolutionType   string
	Resolution       string
	GameplayCategory string
}

func (g *Game) executeFeedback(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 1110:
		return g.feedbackMenu(player), true, nil
	case 1111:
		return g.submitPlayerFeedback(player, "BUG反馈", command.RawArguments)
	case 1112:
		return g.submitPlayerFeedback(player, "玩法建议", command.RawArguments)
	case 1113:
		return g.playerFeedbackList(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) feedbackMenu(player *model.Player) GameResult {
	limit := max64(g.settingInt("feedback.daily_reward_limit", 3), 0)
	return GameResult{Title: "📨 仙盟反馈台", Content: fmt.Sprintf("道友：%s\n━━━━━━━━━━━\n【提交BUG】\n格式：提交BUG 指令：状态；现象：发送后没有回复；期望：正常显示角色属性\n有效BUG会检查复现信息、异常现象与期望结果，通过天机初审后进入内容审核队列。\n\n【提交建议】\n格式：提交建议 功能：宗门仓库；做法：成员捐献材料并按贡献兑换；原因：方便宗门协作\n建议会自动检查描述完整度、实现边界、重复内容、数值安全与权限风险。\n\n【奖励规则】\nBUG初审通过：银币%d · 灵石%d\n建议初审可行：银币%d · 灵石%d\n每日最多领取%d次提交奖励；超过后有效反馈仍会正常进入审核队列。重复、灌水、违规或要求无限资源/绕过权限的内容不发奖励。\n━━━━━━━━━━━\n发送“我的反馈”可查看状态、初审结论与奖励记录；已经验证完成的问题只会写入独立“修复公告”。", player.DaoName, g.settingInt("feedback.bug_silver_reward", 120), g.settingInt("feedback.bug_stone_reward", 80), g.settingInt("feedback.suggestion_silver_reward", 80), g.settingInt("feedback.suggestion_stone_reward", 50), limit), Actions: []string{"提交BUG ", "提交建议 ", "我的反馈", "修复公告", "系统菜单"}}
}

func (g *Game) submitPlayerFeedback(player *model.Player, feedbackType, raw string) (GameResult, bool, error) {
	content := strings.TrimSpace(raw)
	format := "提交建议 功能、做法与原因"
	if feedbackType == "BUG反馈" {
		format = "提交BUG 指令、异常现象与期望结果"
	}
	if content == "" {
		return GameResult{Title: "反馈内容不能为空", Content: "请写清楚可核对的信息。\n格式：`" + format + "`", Actions: []string{"反馈说明", "我的反馈"}}, true, nil
	}
	if len([]rune(content)) > 1000 {
		return GameResult{Title: "反馈内容过长", Content: "单条反馈最多1000字。请保留指令、复现步骤、实际结果、期望结果或建议理由后重新提交。", Actions: []string{"反馈说明"}}, true, nil
	}
	normalized := normalizeModerationText(content)
	if normalized == "" || distinctFeedbackRunes(normalized) < 6 {
		return GameResult{Title: "反馈信息不足", Content: "内容过短或重复字符过多，无法判断是否可处理。本次未进入审核队列，也没有发放奖励。", Actions: []string{format, "反馈说明"}}, true, nil
	}
	if duplicate, err := g.duplicatePlayerFeedback(player.ID, feedbackType, normalized); err != nil {
		return GameResult{}, true, err
	} else if duplicate.ID != 0 {
		return GameResult{Title: "请勿重复提交", Content: fmt.Sprintf("相同内容已经提交。\n原反馈：#%d · %s\n初审说明：%s\n━━━━━━━━━━━\n重复提交不会再次发放奖励；可在问题出现新现象时补充不同的复现信息后重新提交。", duplicate.ID, duplicate.Status, displayOr(duplicate.Reason, "等待审核")), Actions: []string{"我的反馈", "反馈说明"}}, true, nil
	}
	var submissionsToday int64
	startOfDay := time.Now().Format("2006-01-02") + " 00:00:00"
	if err := g.store.DB.Model(&model.ContentReview{}).Where("player_id = ? AND type IN ? AND created_at >= ?", player.ID, []string{"BUG反馈", "玩法建议"}, startOfDay).Count(&submissionsToday).Error; err != nil {
		return GameResult{}, true, err
	}
	if submissionsToday >= 10 {
		return GameResult{Title: "今日反馈次数已满", Content: "每名道友每日最多提交10条反馈，防止重复内容淹没审核队列。请明日再提交，或合并为一条完整说明。", Actions: []string{"我的反馈", "反馈说明"}}, true, nil
	}
	if word, _, matched, err := g.matchSensitiveWord(content); err != nil {
		return GameResult{}, true, err
	} else if matched {
		resolvedAt := time.Now()
		row := model.ContentReview{
			Type: feedbackType, PlayerID: player.ID, PlayerName: player.DaoName, Content: content,
			Status: "已拒绝", Reason: "自动审核命中禁用词：" + word,
			Diagnosis: "内容安全检查命中禁用词，未进入技术诊断。", ResolutionType: "自动驳回",
			Resolution: "未执行任何源码、配置、货币或玩家数据修改。", ResolvedAt: &resolvedAt,
		}
		if err := g.store.DB.Create(&row).Error; err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "反馈审核未通过", Content: fmt.Sprintf("反馈编号：#%d\n自动结论：内容命中仙盟禁用词\n本次未发放奖励。请移除违规内容后重新描述实际问题。", row.ID), Actions: []string{"反馈说明", "我的反馈"}}, true, nil
	}

	assessment := assessPlayerFeedback(feedbackType, content)
	resolutionPlan := diagnosePlayerFeedback(feedbackType, content)
	row := model.ContentReview{
		Type: feedbackType, PlayerID: player.ID, PlayerName: player.DaoName, Content: content,
		Status: "已拒绝", Reason: assessment.Reason, Diagnosis: resolutionPlan.Diagnosis,
		ResolutionType: resolutionPlan.ResolutionType, Resolution: resolutionPlan.Resolution,
	}
	if !assessment.Feasible {
		resolvedAt := time.Now()
		row.Diagnosis = "自动初审无法获得足够的复现或设计信息。"
		row.ResolutionType = "退回补充"
		row.Resolution = "未执行任何源码、配置、货币或玩家数据修改；补齐信息后可重新提交。"
		row.ResolvedAt = &resolvedAt
		if err := g.store.DB.Create(&row).Error; err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "天机初审需要补充", Content: fmt.Sprintf("反馈编号：#%d\n类型：%s\n可行度：%d/100\n结论：%s\n━━━━━━━━━━━\n该内容没有进入待处理队列，也未发放奖励。补齐信息后可重新提交不同的完整描述。", row.ID, feedbackType, assessment.Score, assessment.Reason), Actions: []string{format, "反馈说明", "我的反馈"}}, true, nil
	}

	silverReward := int64(g.settingInt("feedback.suggestion_silver_reward", 80))
	stoneReward := int64(g.settingInt("feedback.suggestion_stone_reward", 50))
	if feedbackType == "BUG反馈" {
		silverReward = int64(g.settingInt("feedback.bug_silver_reward", 120))
		stoneReward = int64(g.settingInt("feedback.bug_stone_reward", 80))
	}
	if silverReward < 0 {
		silverReward = 0
	}
	if stoneReward < 0 {
		stoneReward = 0
	}
	rewarded := false
	rewardLimit := max64(g.settingInt("feedback.daily_reward_limit", 3), 0)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var rewardedToday int64
		if err := tx.Model(&model.ContentReview{}).Where("player_id = ? AND type IN ? AND created_at >= ? AND reason LIKE ?", player.ID, []string{"BUG反馈", "玩法建议"}, startOfDay, "%已发放提交奖励%").Count(&rewardedToday).Error; err != nil {
			return err
		}
		row.Status = "待审核"
		if resolutionPlan.GameplayCategory != "" {
			row.Status = "处理中"
		}
		if rewardLimit > 0 && rewardedToday < rewardLimit {
			row.Reason = fmt.Sprintf("天机初审可处理（%d分）：%s；已发放提交奖励", assessment.Score, assessment.Reason)
			rewarded = true
		} else {
			row.Reason = fmt.Sprintf("天机初审可处理（%d分）：%s；今日提交奖励已达上限", assessment.Score, assessment.Reason)
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if !rewarded {
			return nil
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"silver_coins":  gorm.Expr("silver_coins + ?", silverReward),
			"spirit_stones": gorm.Expr("spirit_stones + ?", stoneReward),
		}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if resolutionPlan.GameplayCategory != "" {
		before, after, repairErr := g.store.ReconcileGameplayCategory(resolutionPlan.GameplayCategory)
		updates := map[string]any{}
		switch {
		case repairErr != nil:
			row.Status = "待审核"
			row.ResolutionType = "人工排查"
			row.Resolution = "标准配置自动核对未完成，已转入人工排查；未改动玩家货币、背包或角色进度。"
			updates["status"] = row.Status
			updates["resolution_type"] = row.ResolutionType
			updates["resolution"] = row.Resolution
		case after > before:
			completedAt := time.Now()
			row.Status = "已修复"
			row.Diagnosis = fmt.Sprintf("%s；核验前%d条，核验后%d条。", resolutionPlan.Diagnosis, before, after)
			row.Resolution = fmt.Sprintf("已在事务中补回“%s”缺失的%d条标准配置，原有运营配置和玩家数据均保留。", resolutionPlan.GameplayCategory, after-before)
			row.ReviewedAt = &completedAt
			row.ResolvedAt = &completedAt
			updates["status"] = row.Status
			updates["diagnosis"] = row.Diagnosis
			updates["resolution"] = row.Resolution
			updates["reviewed_at"] = &completedAt
			updates["resolved_at"] = &completedAt
		default:
			row.Status = "待审核"
			row.ResolutionType = "执行链排查"
			row.Diagnosis = fmt.Sprintf("%s；配置表已核验为%d条，未发现条目缺失。", resolutionPlan.Diagnosis, after)
			row.Resolution = "标准数据完整，已转入指令路由、前置条件与结算链排查；未改动玩家数据。"
			updates["status"] = row.Status
			updates["diagnosis"] = row.Diagnosis
			updates["resolution_type"] = row.ResolutionType
			updates["resolution"] = row.Resolution
		}
		if err := g.store.DB.Model(&model.ContentReview{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return GameResult{}, true, err
		}
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.feedback_submitted", 1)
	rewardText := "今日提交奖励已达上限，本条仍已正常进入审核队列。"
	if rewarded {
		rewardText = fmt.Sprintf("提交奖励：银币+%d · 灵石+%d", silverReward, stoneReward)
	}
	return GameResult{Title: "反馈提交成功", Content: fmt.Sprintf("反馈编号：#%d\n类型：%s\n自动初审：可处理\n可行度：%d/100\n结论：%s\n━━━━━━━━━━━\n处理状态：%s\n诊断：%s\n处理方式：%s\n处理结果：%s\n%s\n后续状态可发送“我的反馈”查看；确认修复后会进入独立修复公告。", row.ID, feedbackType, assessment.Score, assessment.Reason, row.Status, displayOr(row.Diagnosis, "等待诊断"), displayOr(row.ResolutionType, "等待分派"), displayOr(row.Resolution, "等待处理"), rewardText), Actions: []string{"我的反馈", "反馈菜单", "货币", "修复公告"}}, true, nil
}

func (g *Game) duplicatePlayerFeedback(playerID uint, feedbackType, normalized string) (model.ContentReview, error) {
	var rows []model.ContentReview
	cutoff := time.Now().AddDate(0, 0, -30)
	if err := g.store.DB.Where("player_id = ? AND type = ? AND status <> ? AND created_at >= ?", playerID, feedbackType, "已拒绝", cutoff).Order("id DESC").Limit(200).Find(&rows).Error; err != nil {
		return model.ContentReview{}, err
	}
	for _, row := range rows {
		if feedbackTextSimilar(normalizeModerationText(row.Content), normalized) {
			return row, nil
		}
	}
	return model.ContentReview{}, nil
}

func assessPlayerFeedback(feedbackType, content string) feedbackAssessment {
	normalized := normalizeModerationText(content)
	length := len([]rune(content))
	score := 10
	reasons := make([]string, 0, 5)
	if feedbackType == "BUG反馈" {
		if length < 12 {
			return feedbackAssessment{Score: 15, Reason: "描述太短，请补充触发指令、实际现象和期望结果"}
		}
		if containsFeedbackTerm(normalized, "发送", "点击", "使用", "进入", "挑战", "购买", "炼药", "突破", "战斗", "然后", "之后", "灵脉打坐", "打坐", "采灵气", "采集", "灵脉", "状态", "背包", "地图", "副本", "灵传") {
			score += 25
			reasons = append(reasons, "包含复现操作")
		}
		if containsFeedbackTerm(normalized, "无回复", "没有回复", "报错", "错误", "异常", "不显示", "没显示", "不增加", "没增加", "不一致", "卡住", "失效", "扣错", "奖励错误", "无法", "不能", "不契合", "失败", "需要", "当前", "缺少", "不生效") {
			score += 30
			reasons = append(reasons, "包含异常现象")
		}
		if containsFeedbackTerm(normalized, "期望", "应该", "正常", "正确", "希望", "修复", "应当", "可以", "能够", "不该") {
			score += 20
			reasons = append(reasons, "包含期望结果")
		}
		if length >= 30 {
			score += 15
			reasons = append(reasons, "描述较完整")
		}
	} else {
		if containsFeedbackTerm(normalized, "无限仙金", "无限银币", "无限灵石", "免费充值", "百分百掉落", "100爆率", "无敌", "绕过权限", "绕过审核", "删除所有玩家", "开外挂") {
			return feedbackAssessment{Score: 0, Reason: "建议破坏货币、权限、安全或公平规则，自动判定不可行"}
		}
		if length < 12 {
			return feedbackAssessment{Score: 15, Reason: "描述太短，请补充要改的功能、具体做法和理由"}
		}
		if containsFeedbackTerm(normalized, "建议", "希望", "增加", "新增", "优化", "支持", "可以", "改成") {
			score += 25
			reasons = append(reasons, "目标明确")
		}
		if containsFeedbackTerm(normalized, "功能", "系统", "菜单", "地图", "任务", "战斗", "灵根", "宗门", "仙侣", "灵兽", "丹药", "装备", "副本", "活动", "商城", "背包") {
			score += 25
			reasons = append(reasons, "作用对象明确")
		}
		if containsFeedbackTerm(normalized, "做法", "通过", "发送", "点击", "显示", "设置", "按照", "允许", "限制", "分页", "奖励") {
			score += 20
			reasons = append(reasons, "包含实现方式")
		}
		if containsFeedbackTerm(normalized, "因为", "原因", "方便", "避免", "这样", "让玩家", "提升", "减少") {
			score += 10
			reasons = append(reasons, "包含收益或原因")
		}
		if length >= 30 {
			score += 10
			reasons = append(reasons, "描述较完整")
		}
	}
	if score > 100 {
		score = 100
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "缺少可核对的具体信息")
	}
	return feedbackAssessment{Feasible: score >= 55, Score: score, Reason: strings.Join(reasons, "、")}
}

func diagnosePlayerFeedback(feedbackType, content string) feedbackResolutionPlan {
	if feedbackType == "玩法建议" {
		return feedbackResolutionPlan{
			Diagnosis:      "建议已通过完整度与安全边界初审，等待运营评估玩法收益、数值影响和开发成本。",
			ResolutionType: "玩法评审",
			Resolution:     "已进入玩法评审；系统不会依据玩家文字自动修改经济、权限、源码或发放物品。",
		}
	}
	if category, ok := missingGameplayCategory(content); ok {
		return feedbackResolutionPlan{
			Diagnosis:        fmt.Sprintf("检测到“%s”标准配置可能缺失，允许执行受控数据核对。", category),
			ResolutionType:   "自动数据修复",
			Resolution:       "正在事务化核对标准配置编码；只补缺失内置数据，不覆盖运营自定义内容。",
			GameplayCategory: category,
		}
	}
	return feedbackResolutionPlan{
		Diagnosis:      "自动初诊已完成，未命中允许自动处理的标准配置缺失类型。",
		ResolutionType: "人工排查",
		Resolution:     "等待管理员复现并定位指令、状态或结算链；系统未自动改动源码、货币或玩家数据。",
	}
}

func missingGameplayCategory(content string) (string, bool) {
	normalized := normalizeModerationText(content)
	if !containsFeedbackTerm(normalized,
		"无数据", "没数据", "没有数据", "没有内容", "没内容", "内容为空", "列表为空", "配置为空",
		"配置不存在", "找不到配置", "无可用配置", "全部都没有", "全部没有", "都没有", "也没有", "也没", "没东西", "不存在",
	) {
		return "", false
	}
	categories := []struct {
		Label string
		Terms []string
	}{
		{"宇宙星河", []string{"宇宙星河", "星图", "星河"}},
		{"道侣合体技", []string{"道侣合体技", "合体技", "合技"}},
		{"争夺秘境", []string{"秘境争夺", "争夺秘境", "探秘", "秘境"}},
		{"上古传承", []string{"上古传承", "传承"}},
		{"大道真法", []string{"大道真法", "悟道", "真法"}},
		{"仙魔战场", []string{"仙魔战场", "战场"}},
		{"灵根进化", []string{"灵根进化", "灵根觉醒"}},
		{"渡劫心魔", []string{"渡劫心魔", "心魔"}},
		{"九天仙药", []string{"九天仙药", "仙药培育", "仙药"}},
		{"法宝炼化", []string{"法宝炼化", "炼器", "炼化"}},
		{"天机推演", []string{"天机推演", "推演"}},
		{"天地灵脉", []string{"天地灵脉", "灵脉"}},
		{"宗门战争", []string{"宗门战争", "宗战"}},
		{"仙缘奇遇", []string{"仙缘奇遇", "仙遇", "仙录"}},
		{"阵法", []string{"阵法", "阵图"}},
		{"符箓", []string{"符箓", "符咒"}},
		{"傀儡", []string{"傀儡", "炼傀"}},
	}
	for _, category := range categories {
		if containsFeedbackTerm(normalized, category.Terms...) {
			return category.Label, true
		}
	}
	return "", false
}

func containsFeedbackTerm(normalized string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(normalized, normalizeModerationText(term)) {
			return true
		}
	}
	return false
}

func feedbackTextSimilar(first, second string) bool {
	if first == second {
		return true
	}
	firstRunes, secondRunes := []rune(first), []rune(second)
	if len(firstRunes) < 8 || len(secondRunes) < 8 {
		return false
	}
	shorter, longer := first, second
	shorterLength, longerLength := len(firstRunes), len(secondRunes)
	if shorterLength > longerLength {
		shorter, longer = longer, shorter
		shorterLength, longerLength = longerLength, shorterLength
	}
	if strings.Contains(longer, shorter) && float64(shorterLength)/float64(longerLength) >= .75 {
		return true
	}
	firstPairs := feedbackRunePairs(firstRunes)
	secondPairs := feedbackRunePairs(secondRunes)
	shared := 0
	for pair := range firstPairs {
		if _, ok := secondPairs[pair]; ok {
			shared++
		}
	}
	return float64(2*shared)/float64(len(firstPairs)+len(secondPairs)) >= .70
}

func feedbackRunePairs(value []rune) map[string]struct{} {
	pairs := make(map[string]struct{}, len(value)-1)
	for index := 0; index+1 < len(value); index++ {
		pairs[string(value[index:index+2])] = struct{}{}
	}
	return pairs
}

func distinctFeedbackRunes(value string) int {
	distinct := make(map[rune]struct{})
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			distinct[r] = struct{}{}
		}
	}
	return len(distinct)
}

func (g *Game) playerFeedbackList(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.ContentReview{}).Where("player_id = ? AND type IN ?", player.ID, []string{"BUG反馈", "玩法建议"})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.ContentReview
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "我的反馈", Content: "尚未提交BUG或玩法建议。完整且非重复的有效反馈通过初审后会获得提交奖励。", Actions: []string{"反馈菜单", "提交BUG ", "提交建议 "}}, true, nil
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d条", page, pages, total), "━━━━━━━━━━━"}
	for _, row := range rows {
		completed := ""
		if row.ResolvedAt != nil {
			completed = "\n完成：" + row.ResolvedAt.Format("01-02 15:04")
		}
		lines = append(lines, fmt.Sprintf("#%d【%s】· %s · %s\n内容：%s\n初审：%s\n诊断：%s\n处理：%s\n结果：%s%s", row.ID, row.Type, row.Status, row.CreatedAt.Format("01-02 15:04"), feedbackPreview(row.Content, 72), displayOr(row.Reason, "等待人工审核"), displayOr(row.Diagnosis, "等待诊断"), displayOr(row.ResolutionType, "等待分派"), displayOr(row.Resolution, "等待处理"), completed), "━━━━━━━")
	}
	actions := []string{"反馈菜单", "提交BUG ", "提交建议 "}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("我的反馈 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("我的反馈 %d", page+1))
	}
	return GameResult{Title: "我的反馈", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func feedbackPreview(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maximum {
		return string(runes)
	}
	return string(runes[:maximum]) + "…"
}
