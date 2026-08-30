package service

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestAnnouncementsAreSeparatedAndIntroductionsAreDistinct(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "notice-intro-player", "问界书生")
	published := time.Now().Add(time.Hour)
	longRepair := model.Notice{Code: "repair_pagination_test", Title: "长文分页验证", Content: strings.Repeat("本段记录已经验证完成的修复内容。\n", 30), Type: "修复", Pinned: true, Published: true, PublishedAt: &published}
	if err := store.DB.Create(&longRepair).Error; err != nil {
		t.Fatal(err)
	}
	plainNotice := model.Notice{Code: "plain_notice_channel_test", Title: "纯公告频道验证", Content: "这是一则普通运营告示，仅用于公布仙盟活动安排。", Type: "公告", Pinned: true, Published: true, PublishedAt: &published}
	if err := store.DB.Create(&plainNotice).Error; err != nil {
		t.Fatal(err)
	}
	var legacyTypes int64
	if err := store.DB.Model(&model.Notice{}).Where("type IN ?", []string{"系统", "活动"}).Count(&legacyTypes).Error; err != nil || legacyTypes != 0 {
		t.Fatalf("legacy normal notice types were not migrated: count=%d err=%v", legacyTypes, err)
	}
	repairs, handled, err := game.Execute("group", player.AccountID, mustParse(t, "修复公告"))
	if err != nil || !handled || !strings.Contains(repairs.Title, "修复公告") || !strings.Contains(repairs.Content, "长文分页验证") || !strings.Contains(repairs.Content, "正文分段：1/") || !containsAction(repairs.Actions, "修复公告 2") || hasGlobalPagination(repairs) {
		t.Fatalf("repair notices: handled=%v err=%v result=%+v", handled, err, repairs)
	}
	repairSecond, handled, err := game.Execute("group", player.AccountID, mustParse(t, "修复公告 2"))
	if err != nil || !handled || !strings.Contains(repairSecond.Content, "长文分页验证") || !strings.Contains(repairSecond.Content, "正文分段：2/") || hasGlobalPagination(repairSecond) {
		t.Fatalf("repair notice second page: handled=%v err=%v result=%+v", handled, err, repairSecond)
	}
	updates, handled, err := game.Execute("group", player.AccountID, mustParse(t, "更新公告"))
	if err != nil || !handled || strings.Contains(updates.Content, "体力、灵脉与交互修复公告") || strings.Contains(updates.Content, "仙尘已修复问题总公告") {
		t.Fatalf("repair notice leaked into updates: handled=%v err=%v result=%+v", handled, err, updates)
	}
	world, handled, err := game.Execute("group", player.AccountID, mustParse(t, "世界公告"))
	if err != nil || !handled || !strings.Contains(world.Content, "纯公告频道验证") || strings.Contains(world.Content, "体力、灵脉与交互修复公告") || strings.Contains(world.Content, "版本更新") {
		t.Fatalf("repair/update leaked into world notices: handled=%v err=%v result=%+v", handled, err, world)
	}
	if containsAction(world.Actions, "更新公告") || containsAction(world.Actions, "修复公告") {
		t.Fatalf("world notices referenced update/repair actions: %+v", world.Actions)
	}
	if !containsAction(world.Actions, "世界公告") {
		t.Fatalf("world notice action missing: %+v", world.Actions)
	}
	if containsAction(updates.Actions, "世界公告") || containsAction(updates.Actions, "修复公告") {
		t.Fatalf("update notices referenced other notice channels: %+v", updates.Actions)
	}
	if containsAction(repairs.Actions, "世界公告") || containsAction(repairs.Actions, "更新公告") {
		t.Fatalf("repair notices referenced other notice channels: %+v", repairs.Actions)
	}

	intro, _, err := game.Execute("group", player.AccountID, mustParse(t, "仙尘介绍"))
	if err != nil {
		t.Fatal(err)
	}
	gameIntro, _, err := game.Execute("group", player.AccountID, mustParse(t, "游戏介绍"))
	if err != nil {
		t.Fatal(err)
	}
	worldIntro, _, err := game.Execute("group", player.AccountID, mustParse(t, "大世界"))
	if err != nil {
		t.Fatal(err)
	}
	if intro.Content == gameIntro.Content || intro.Content == worldIntro.Content || gameIntro.Content == worldIntro.Content {
		t.Fatal("introduction commands still return identical content")
	}
	if !strings.Contains(intro.Content, "核心规模") || !strings.Contains(gameIntro.Content, "每日循环") || !strings.Contains(worldIntro.Content, "山河秩序") {
		t.Fatalf("introduction routing mismatch: intro=%s game=%s world=%s", intro.Title, gameIntro.Title, worldIntro.Title)
	}
}

func TestV221PlayerNoticesAndWorldMessageSafety(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "notice-safety-player", "听诏散人")

	expected := map[string]string{
		"world_notice_v221_player_20260724":          "公告",
		"update_v221_player_menu_notices_20260724":   "更新",
		"repair_v221_world_menu_visibility_20260724": "修复",
	}
	for code, noticeType := range expected {
		var row model.Notice
		if err := store.DB.Where("code = ?", code).First(&row).Error; err != nil {
			t.Fatalf("seeded notice %s missing: %v", code, err)
		}
		if row.Type != noticeType || !row.Published || row.PublishedAt == nil || len([]rune(strings.TrimSpace(row.Content))) < 100 {
			t.Fatalf("seeded notice %s incomplete: %+v", code, row)
		}
		if noticeType == "更新" || noticeType == "修复" {
			for _, want := range []string{"装备", "槽位", "锻造", "玄铁"} {
				if !strings.Contains(row.Content, want) {
					t.Fatalf("seeded %s notice does not contain equipment settlement detail %q: %s", noticeType, want, row.Content)
				}
			}
		}
		if noticeType == "公告" {
			for _, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
				if strings.Contains(row.Title+row.Content, forbidden) {
					t.Fatalf("world notice contains forbidden player-facing word %q: %+v", forbidden, row)
				}
			}
		}
	}

	safePublished := time.Now().AddDate(5, 0, 0)
	safe := model.Notice{Code: "world_notice_runtime_safe_test", Title: "诸界清音", Content: "新一轮仙门庆典已经开启，道友可发送活动总览查看今日安排。", Type: "公告", Published: true, PublishedAt: &safePublished}
	if err := store.DB.Create(&safe).Error; err != nil {
		t.Fatal(err)
	}
	for index, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
		published := safePublished.AddDate(index+1, 0, 0)
		row := model.Notice{
			Code:        "world_notice_runtime_unsafe_" + strconv.Itoa(index),
			Title:       "不应展示的告示",
			Content:     "这条消息含有" + forbidden + "字样。",
			Type:        "公告",
			Pinned:      true,
			Published:   true,
			PublishedAt: &published,
		}
		if err := store.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	latest := game.latestWorldNotice()
	if latest != safe.Content {
		t.Fatalf("latest world message did not skip unsafe newer notices: %q", latest)
	}
	menu, handled, err := game.Execute("group", player.AccountID, mustParse(t, "功能菜单"))
	if err != nil || !handled || !strings.Contains(menu.Content, safe.Content) {
		t.Fatalf("global menu did not render safe latest world message: handled=%v err=%v result=%+v", handled, err, menu)
	}
	for _, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
		if strings.Contains(menu.Content, forbidden) {
			t.Fatalf("global menu world message contains forbidden word %q: %s", forbidden, menu.Content)
		}
	}
}

func TestArenaTierCatalogIsDataBackedAndPaged(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "arena-tier-player", "试剑真人")
	var existing int64
	if err := store.DB.Model(&model.ArenaTier{}).Where("enabled = ?", true).Count(&existing).Error; err != nil || existing == 0 {
		t.Fatalf("seeded arena tiers=%d err=%v", existing, err)
	}
	testNames := []string{"春岚", "夏雨", "秋鸿", "冬雪", "晨曦", "暮霞", "流云", "飞星", "照月", "听雷", "观海", "问山", "临渊", "踏歌", "长风"}
	for sequence := int(existing) + 1; sequence <= 15; sequence++ {
		row := model.ArenaTier{Code: "test_arena_tier_" + strconv.Itoa(sequence), Name: "试炼" + testNames[sequence-1] + "境", Sequence: sequence, MinimumRating: 1000 + int64(sequence-1)*20, DailyCoin: int64(20 + sequence*2), DailySilver: int64(80 + sequence*7), Description: "测试分页段位道意。", Enabled: true}
		if err := store.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first, handled, err := game.Execute("group", player.AccountID, mustParse(t, "竞技段位 1"))
	if err != nil || !handled || !strings.Contains(first.Title, "千阶问剑段位") || !strings.Contains(first.Content, "第1/2页") || !containsAction(first.Actions, "竞技段位 2") || hasGlobalPagination(first) {
		t.Fatalf("arena tier first page: handled=%v err=%v result=%+v", handled, err, first)
	}
	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "竞技段位 2"))
	if err != nil || !handled || !strings.Contains(second.Content, "第2/2页") {
		t.Fatalf("arena tier second page: handled=%v err=%v result=%+v", handled, err, second)
	}
}
