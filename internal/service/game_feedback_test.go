package service

import (
	"fmt"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestPlayerFeedbackAssessmentRewardsDeduplicationAndPagination(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "feedback-player", "问策真人")
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}

	bugText := "指令：发送状态；异常现象：连续发送三次都没有回复；期望结果：正常显示角色全部属性"
	bug, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+bugText))
	if err != nil || !handled || !strings.Contains(bug.Title, "提交成功") || !strings.Contains(bug.Content, "银币+120") || !strings.Contains(bug.Content, "灵石+80") {
		t.Fatalf("valid bug feedback: handled=%v err=%v result=%+v", handled, err, bug)
	}
	var bugReview model.ContentReview
	if err := store.DB.Where("player_id = ? AND type = ?", player.ID, "BUG反馈").Order("id DESC").First(&bugReview).Error; err != nil || bugReview.Status != "待审核" {
		t.Fatalf("bug review row=%+v err=%v", bugReview, err)
	}
	afterBug, _ := game.players.Get(player.ID)
	if afterBug.SilverCoins-before.SilverCoins != 120 || afterBug.SpiritStones-before.SpiritStones != 80 {
		t.Fatalf("bug reward silver=%d stone=%d", afterBug.SilverCoins-before.SilverCoins, afterBug.SpiritStones-before.SpiritStones)
	}

	duplicate, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+bugText))
	if err != nil || !handled || !strings.Contains(duplicate.Title, "重复提交") {
		t.Fatalf("duplicate feedback: handled=%v err=%v result=%+v", handled, err, duplicate)
	}
	afterDuplicate, _ := game.players.Get(player.ID)
	if afterDuplicate.SilverCoins != afterBug.SilverCoins || afterDuplicate.SpiritStones != afterBug.SpiritStones {
		t.Fatal("duplicate feedback issued another reward")
	}

	suggestionText := "建议增加宗门仓库功能，做法是成员发送捐献物品并按照贡献兑换，因为这样方便宗门协作并减少交易步骤"
	suggestion, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交建议 "+suggestionText))
	if err != nil || !handled || !strings.Contains(suggestion.Title, "提交成功") || !strings.Contains(suggestion.Content, "银币+80") || !strings.Contains(suggestion.Content, "灵石+50") {
		t.Fatalf("valid suggestion: handled=%v err=%v result=%+v", handled, err, suggestion)
	}
	var suggestionReview model.ContentReview
	if err := store.DB.Where("player_id = ? AND type = ?", player.ID, "玩法建议").Order("id DESC").First(&suggestionReview).Error; err != nil {
		t.Fatal(err)
	}
	if suggestionReview.Status != "待审核" || suggestionReview.ResolutionType != "玩法评审" || suggestionReview.ResolvedAt != nil || !strings.Contains(suggestionReview.Resolution, "不会") {
		t.Fatalf("suggestion bypassed review boundary: %+v", suggestionReview)
	}

	rejected, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交建议 建议增加无限仙金功能，允许所有玩家免费充值并绕过权限审核"))
	if err != nil || !handled || !strings.Contains(rejected.Content, "不可行") {
		t.Fatalf("unsafe suggestion: handled=%v err=%v result=%+v", handled, err, rejected)
	}
	afterRejected, _ := game.players.Get(player.ID)
	if afterRejected.SilverCoins-afterBug.SilverCoins != 80 || afterRejected.SpiritStones-afterBug.SpiritStones != 50 {
		t.Fatalf("unexpected rewards after suggestion/rejection: silver=%d stone=%d", afterRejected.SilverCoins-afterBug.SilverCoins, afterRejected.SpiritStones-afterBug.SpiritStones)
	}

	for index := 0; index < 7; index++ {
		row := model.ContentReview{Type: "BUG反馈", PlayerID: player.ID, PlayerName: player.DaoName, Content: fmt.Sprintf("分页测试反馈%d：发送位置后显示异常，期望正常显示地图", index), Status: "待审核", Reason: "测试记录"}
		if err := store.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, handled, err := game.Execute("group", player.AccountID, mustParse(t, "我的反馈 2"))
	if err != nil || !handled || !strings.Contains(page.Content, "第2/") || !containsAction(page.Actions, "我的反馈 1") {
		t.Fatalf("feedback pagination: handled=%v err=%v result=%+v", handled, err, page)
	}
}

func TestMissingGameplayFeedbackReconcilesOnlyCanonicalData(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "star-feedback-player", "观星真人")

	if err := store.DB.Where("code = ?", "star_realm_2").Delete(&model.StarRealmConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := store.DB.Model(&model.StarRealmConfig{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("test seed count before repair=%d, want 2", before)
	}

	content := "指令：发送宇宙星河；异常现象：星图列表无数据并且无法显示；期望结果：恢复完整星图配置并正常查看"
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+content))
	if err != nil || !handled || !strings.Contains(result.Title, "提交成功") || !strings.Contains(result.Content, "处理状态：已修复") || !strings.Contains(result.Content, "补回") {
		t.Fatalf("automatic gameplay repair: handled=%v err=%v result=%+v", handled, err, result)
	}

	var review model.ContentReview
	if err := store.DB.Where("player_id = ? AND type = ?", player.ID, "BUG反馈").Order("id DESC").First(&review).Error; err != nil {
		t.Fatal(err)
	}
	if review.Status != "已修复" || review.ResolutionType != "自动数据修复" || review.ResolvedAt == nil || !strings.Contains(review.Diagnosis, "核验前2条") || !strings.Contains(review.Resolution, "缺失的1条") {
		t.Fatalf("repair audit trail incomplete: %+v", review)
	}

	var rows []model.StarRealmConfig
	if err := store.DB.Order("code").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("star realm rows after repair=%d, want 3", len(rows))
	}
	codes, names := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if strings.TrimSpace(row.Code) == "" || strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Description) == "" || strings.TrimSpace(row.EffectParams) == "" || strings.TrimSpace(row.CostMaterials) == "" || strings.TrimSpace(row.Prerequisite) == "" {
			t.Fatalf("reconciled row has empty gameplay fields: %+v", row)
		}
		if codes[row.Code] || names[row.Name] {
			t.Fatalf("reconciled row is not unique: %+v", row)
		}
		codes[row.Code], names[row.Name] = true, true
	}

	list, handled, err := game.Execute("group", player.AccountID, mustParse(t, "我的反馈"))
	if err != nil || !handled || !strings.Contains(list.Content, "自动数据修复") || !strings.Contains(list.Content, "完成：") {
		t.Fatalf("player feedback list missing closure: handled=%v err=%v result=%+v", handled, err, list)
	}
}

func TestNaturalLeylineBugFeedbackPassesAndFuzzyDuplicateDoesNotReward(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "natural-feedback-player", "听泉散人")
	content := "灵脉打坐 青丘庚金龙脊灵脉，灵根不契合：需要庚金本源，当前无极时空道莲灵根，无法打坐、采集灵气，希望修复并显示适配灵脉"
	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+content))
	if err != nil || !handled || !strings.Contains(first.Title, "提交成功") || !strings.Contains(first.Content, "可行度：") || strings.Contains(first.Content, "翻页") || hasGlobalPagination(first) {
		t.Fatalf("natural bug feedback: handled=%v err=%v result=%+v", handled, err, first)
	}
	afterFirst, _ := game.players.Get(player.ID)
	nearDuplicate := "灵脉打坐青丘庚金龙脊灵脉：当前无极时空道莲灵根与庚金本源不契合，无法打坐采集灵气，希望尽快修复并显示适配灵脉"
	duplicate, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+nearDuplicate))
	if err != nil || !handled || !strings.Contains(duplicate.Title, "重复提交") {
		t.Fatalf("fuzzy duplicate: handled=%v err=%v result=%+v", handled, err, duplicate)
	}
	afterDuplicate, _ := game.players.Get(player.ID)
	if afterDuplicate.SilverCoins != afterFirst.SilverCoins || afterDuplicate.SpiritStones != afterFirst.SpiritStones {
		t.Fatal("fuzzy duplicate issued another reward")
	}

	rejectedContent := "灵脉打坐赤霄离火凤巢灵脉，当前玄水灵根无法进入，希望修复并显示适配的离火灵根"
	rejectedRow := model.ContentReview{Type: "BUG反馈", PlayerID: player.ID, PlayerName: player.DaoName, Content: rejectedContent, Status: "已拒绝", Reason: "信息不足"}
	if err := store.DB.Create(&rejectedRow).Error; err != nil {
		t.Fatal(err)
	}
	resubmitted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "提交BUG "+rejectedContent))
	if err != nil || !handled || !strings.Contains(resubmitted.Title, "提交成功") {
		t.Fatalf("rejected feedback could not be resubmitted: handled=%v err=%v result=%+v", handled, err, resubmitted)
	}
}
