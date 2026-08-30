package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func currentBirthdayArgument(offsetDays int) string {
	return time.Now().AddDate(0, 0, offsetDays).Format("01-02")
}

func TestBirthdayMenuOnlyAppearsOnBirthdayAndAnnualRewardsAreAtomic(t *testing.T) {
	game, store := testGame(t)
	offDay := registerPlayer(t, game, "birthday-off-day", "候星真人")
	registered, handled, err := game.Execute("group", offDay.AccountID, mustParse(t, "设置生日 "+currentBirthdayArgument(1)))
	if err != nil || !handled || !strings.Contains(registered.Title, "登记完成") {
		t.Fatalf("off-day registration: handled=%v err=%v result=%+v", handled, err, registered)
	}
	menu, handled, err := game.Execute("group", offDay.AccountID, mustParse(t, "生日菜单"))
	if err != nil || !handled || !strings.Contains(menu.Title, "尚未开启") || containsAction(menu.Actions, "生辰签到") || strings.Contains(menu.Content, "限定抽奖") {
		t.Fatalf("off-day menu leaked birthday functions: handled=%v err=%v result=%+v", handled, err, menu)
	}
	mainMenu, _, err := game.Execute("group", offDay.AccountID, mustParse(t, "菜单"))
	if err != nil || !strings.Contains(mainMenu.MarkdownContent, "生辰档案") || strings.Contains(mainMenu.MarkdownContent, "生日专属菜单") {
		t.Fatalf("off-day main menu birthday entry mismatch: err=%v result=%+v", err, mainMenu)
	}
	for _, entry := range []string{"氪金菜单", "世界公告", "更新公告", "修复公告"} {
		if !strings.Contains(mainMenu.MarkdownContent, entry) {
			t.Fatalf("global menu missing %s: %s", entry, mainMenu.MarkdownContent)
		}
	}
	profile, handled, err := game.Execute("group", offDay.AccountID, mustParse(t, "生日"))
	if err != nil || !handled || !strings.Contains(profile.Title, "尚未开启") || !strings.Contains(profile.Content, "倒计时") || containsAction(profile.Actions, "生辰签到") || containsAction(profile.Actions, "生辰抽奖") {
		t.Fatalf("off-day birthday profile leaked exclusive functions: handled=%v err=%v result=%+v", handled, err, profile)
	}
	locked, _, err := game.Execute("group", offDay.AccountID, mustParse(t, "设置生日 "+currentBirthdayArgument(2)))
	if err != nil || !strings.Contains(locked.Title, "已经登记") {
		t.Fatalf("birthday was not permanently locked: err=%v result=%+v", err, locked)
	}

	birthday := registerPlayer(t, game, "birthday-today", "长明寿星")
	registered, handled, err = game.Execute("group", birthday.AccountID, mustParse(t, "设置生日 "+currentBirthdayArgument(0)))
	if err != nil || !handled || !containsAction(registered.Actions, "生日菜单") {
		t.Fatalf("birthday registration: handled=%v err=%v result=%+v", handled, err, registered)
	}
	menu, handled, err = game.Execute("group", birthday.AccountID, mustParse(t, "生日菜单"))
	if err != nil || !handled || !strings.Contains(menu.Title, "专属菜单") || !containsAction(menu.Actions, "生辰签到") || !containsAction(menu.Actions, "生辰抽奖") || !containsAction(menu.Actions, "生辰兑换") {
		t.Fatalf("birthday menu: handled=%v err=%v result=%+v", handled, err, menu)
	}
	mainMenu, _, err = game.Execute("group", birthday.AccountID, mustParse(t, "菜单"))
	if err != nil || !strings.Contains(mainMenu.MarkdownContent, "生辰档案") || !strings.Contains(mainMenu.MarkdownContent, "生日专属菜单") {
		t.Fatalf("birthday main menu has no conditional entry: err=%v result=%+v", err, mainMenu)
	}

	greeting, greeted, err := game.BirthdayAmbientGreeting("birthday-group-a", birthday.AccountID)
	if err != nil || !greeted || !strings.Contains(greeting.Content, "生辰吉乐") || !strings.Contains(greeting.Content, birthday.DaoName) {
		t.Fatalf("ambient greeting: greeted=%v err=%v result=%+v", greeted, err, greeting)
	}
	if _, greeted, err = game.BirthdayAmbientGreeting("birthday-group-a", birthday.AccountID); err != nil || greeted {
		t.Fatalf("same-group greeting repeated: greeted=%v err=%v", greeted, err)
	}
	if _, greeted, err = game.BirthdayAmbientGreeting("birthday-group-b", birthday.AccountID); err != nil || !greeted {
		t.Fatalf("new-group greeting missing: greeted=%v err=%v", greeted, err)
	}

	before, _ := game.players.Get(birthday.ID)
	checkin, handled, err := game.Execute("group", birthday.AccountID, mustParse(t, "生辰签到"))
	if err != nil || !handled || !strings.Contains(checkin.Title, "完成") || game.birthdayTicketQuantity(birthday.ID) != 8 {
		t.Fatalf("birthday checkin: handled=%v err=%v tickets=%d result=%+v", handled, err, game.birthdayTicketQuantity(birthday.ID), checkin)
	}
	afterCheckin, _ := game.players.Get(birthday.ID)
	if afterCheckin.SilverCoins-before.SilverCoins != 188 || afterCheckin.Cultivation-before.Cultivation != 188 || afterCheckin.Experience <= before.Experience {
		t.Fatalf("birthday checkin settlement: before=%+v after=%+v", before, afterCheckin)
	}
	duplicateCheckin, _, err := game.Execute("group", birthday.AccountID, mustParse(t, "生辰签到"))
	if err != nil || !strings.Contains(duplicateCheckin.Title, "已经签到") || game.birthdayTicketQuantity(birthday.ID) != 8 {
		t.Fatalf("duplicate birthday checkin: err=%v result=%+v", err, duplicateCheckin)
	}

	beforeGift, _ := game.players.Get(birthday.ID)
	gift, handled, err := game.Execute("group", birthday.AccountID, mustParse(t, "领取生日礼物"))
	if err != nil || !handled || !strings.Contains(gift.Title, "生日礼物") || gift.BroadcastContent == "" || game.birthdayTicketQuantity(birthday.ID) != 38 {
		t.Fatalf("birthday main gift: handled=%v err=%v tickets=%d result=%+v", handled, err, game.birthdayTicketQuantity(birthday.ID), gift)
	}
	afterGift, _ := game.players.Get(birthday.ID)
	if afterGift.SilverCoins-beforeGift.SilverCoins != 888 || afterGift.SpiritStones-beforeGift.SpiritStones != 8888 || afterGift.Merit-beforeGift.Merit != 88 || afterGift.Cultivation-beforeGift.Cultivation != 888 {
		t.Fatalf("birthday gift currencies: before=%+v after=%+v", beforeGift, afterGift)
	}
	var title model.Title
	if err := store.DB.Where("code = ?", birthdayTitleCode).First(&title).Error; err != nil || !game.playerValueExists(birthday.ID, titleUnlockKey(title)) {
		t.Fatalf("birthday title was not unlocked: title=%+v err=%v", title, err)
	}
	beforeDuplicate, _ := game.players.Get(birthday.ID)
	duplicateGift, _, err := game.Execute("group", birthday.AccountID, mustParse(t, "领取生日礼物"))
	afterDuplicate, _ := game.players.Get(birthday.ID)
	if err != nil || !strings.Contains(duplicateGift.Title, "已领取") || beforeDuplicate.SilverCoins != afterDuplicate.SilverCoins || game.birthdayTicketQuantity(birthday.ID) != 38 {
		t.Fatalf("duplicate birthday gift changed state: err=%v result=%+v", err, duplicateGift)
	}

	for _, taskName := range []string{"星灯初明", "生辰留印", "天赐长生"} {
		claimed, handled, err := game.Execute("group", birthday.AccountID, mustParse(t, "领取生日任务 "+taskName))
		if err != nil || !handled || !strings.Contains(claimed.Title, "任务完成") {
			t.Fatalf("claim birthday task %s: handled=%v err=%v result=%+v", taskName, handled, err, claimed)
		}
	}

	if err := game.adjustNamedItem(birthday.ID, birthdayTicketName, 100); err != nil {
		t.Fatal(err)
	}
	beforeExchangeTickets := game.birthdayTicketQuantity(birthday.ID)
	exchanged, handled, err := game.Execute("group", birthday.AccountID, mustParse(t, "生辰兑换 "+birthdayArtifactName))
	if err != nil || !handled || !strings.Contains(exchanged.Title, "兑换完成") || game.birthdayTicketQuantity(birthday.ID) != beforeExchangeTickets-60 {
		t.Fatalf("birthday artifact exchange: handled=%v err=%v result=%+v", handled, err, exchanged)
	}
	var template model.ArtifactTemplate
	var artifactCount int64
	if err := store.DB.Where("code = ?", birthdayArtifactCode).First(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND template_id = ?", birthday.ID, template.ID).Count(&artifactCount).Error; err != nil || artifactCount != 1 {
		t.Fatalf("birthday artifact count=%d err=%v", artifactCount, err)
	}
	beforeUniqueReject := game.birthdayTicketQuantity(birthday.ID)
	uniqueReject, _, err := game.Execute("group", birthday.AccountID, mustParse(t, "生辰兑换 "+birthdayArtifactName))
	if err != nil || !strings.Contains(uniqueReject.Title, "已拥有") || game.birthdayTicketQuantity(birthday.ID) != beforeUniqueReject {
		t.Fatalf("duplicate unique artifact exchange: err=%v result=%+v", err, uniqueReject)
	}

	beforeLottery := game.birthdayTicketQuantity(birthday.ID)
	lottery, handled, err := game.Execute("group", birthday.AccountID, mustParse(t, "生辰抽奖 2"))
	if err != nil || !handled || !strings.Contains(lottery.Title, "限定抽奖") || game.birthdayTicketQuantity(birthday.ID) < beforeLottery-6 {
		t.Fatalf("birthday lottery: handled=%v err=%v before=%d after=%d result=%+v", handled, err, beforeLottery, game.birthdayTicketQuantity(birthday.ID), lottery)
	}
}

func TestBirthdayBlessingsPresentsTasksAndRankingUseRealState(t *testing.T) {
	game, store := testGame(t)
	target := registerPlayer(t, game, "birthday-social-target", "福曜寿星")
	if _, _, err := game.Execute("group", target.AccountID, mustParse(t, "设置生日 "+currentBirthdayArgument(0))); err != nil {
		t.Fatal(err)
	}
	donors := make([]model.Player, 0, 3)
	for index := 1; index <= 3; index++ {
		donors = append(donors, registerPlayer(t, game, fmt.Sprintf("birthday-donor-%d", index), fmt.Sprintf("贺岁道友%d", index)))
	}
	for index, donor := range donors {
		blessing, handled, err := game.Execute("group", donor.AccountID, mustParse(t, "生日祝福 @"+target.DaoName+fmt.Sprintf(" 愿你第%d岁道途长明", index+1)))
		if err != nil || !handled || !strings.Contains(blessing.Title, "同贺") {
			t.Fatalf("birthday blessing %d: handled=%v err=%v result=%+v", index, handled, err, blessing)
		}
	}
	if got := game.playerValueInt(target.ID, birthdayReceivedBlessingKey(time.Now().Year()), 0); got != 3 {
		t.Fatalf("received blessing count=%d want=3", got)
	}
	beforeDuplicateScore := game.playerValueInt(target.ID, birthdayLifetimeScoreKey, 0)
	duplicate, _, err := game.Execute("group", donors[0].AccountID, mustParse(t, "生日祝福 @"+target.DaoName+" 再贺一次"))
	if err != nil || !strings.Contains(duplicate.Title, "已经祝福") || game.playerValueInt(target.ID, birthdayLifetimeScoreKey, 0) != beforeDuplicateScore {
		t.Fatalf("duplicate blessing changed score: err=%v result=%+v", err, duplicate)
	}

	var fruit model.Item
	if err := store.DB.Where("name = ?", "灵果").First(&fruit).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(donors[0].ID, fruit.ID, 5); err != nil {
		t.Fatal(err)
	}
	targetFruitBefore := game.itemQuantity(target.ID, fruit.ID)
	present, handled, err := game.Execute("group", donors[0].AccountID, mustParse(t, "生日赠礼 @"+target.DaoName+" 灵果*2"))
	if err != nil || !handled || !strings.Contains(present.Title, "赠礼送达") || game.itemQuantity(target.ID, fruit.ID) != targetFruitBefore+2 || game.itemQuantity(donors[0].ID, fruit.ID) != 3 {
		t.Fatalf("birthday present: handled=%v err=%v result=%+v", handled, err, present)
	}
	if game.playerValueInt(target.ID, birthdayReceivedGiftKey(time.Now().Year()), 0) != 1 || game.playerValueInt(target.ID, birthdayLifetimeScoreKey, 0) != 50 {
		t.Fatalf("birthday social counters: blessings=%d gifts=%d score=%d", game.playerValueInt(target.ID, birthdayReceivedBlessingKey(time.Now().Year()), 0), game.playerValueInt(target.ID, birthdayReceivedGiftKey(time.Now().Year()), 0), game.playerValueInt(target.ID, birthdayLifetimeScoreKey, 0))
	}

	tasks, handled, err := game.Execute("group", target.AccountID, mustParse(t, "生日任务"))
	if err != nil || !handled || !containsAction(tasks.Actions, "领取生日任务 万友同贺") || !containsAction(tasks.Actions, "领取生日任务 礼承四海") {
		t.Fatalf("social birthday tasks: handled=%v err=%v result=%+v", handled, err, tasks)
	}
	for _, taskName := range []string{"万友同贺", "礼承四海"} {
		claimed, _, err := game.Execute("group", target.AccountID, mustParse(t, "领取生日任务 "+taskName))
		if err != nil || !strings.Contains(claimed.Title, "任务完成") {
			t.Fatalf("claim social task %s: err=%v result=%+v", taskName, err, claimed)
		}
	}

	ranking, handled, err := game.Execute("group", donors[0].AccountID, mustParse(t, "寿星榜"))
	if err != nil || !handled || !strings.Contains(ranking.Title, "寿星福缘榜") || !strings.Contains(ranking.Content, target.DaoName) || !strings.Contains(ranking.Content, "福曜天官") {
		t.Fatalf("birthday ranking: handled=%v err=%v result=%+v", handled, err, ranking)
	}
	var championTitle model.Title
	if err := store.DB.Where("code = ?", "rank_birthday_crown").First(&championTitle).Error; err != nil || !game.playerValueExists(target.ID, titleUnlockKey(championTitle)) {
		t.Fatalf("birthday champion title missing: title=%+v err=%v", championTitle, err)
	}

	nonBirthday := registerPlayer(t, game, "birthday-non-target", "明日寿星")
	if _, _, err := game.Execute("group", nonBirthday.AccountID, mustParse(t, "设置生日 "+currentBirthdayArgument(1))); err != nil {
		t.Fatal(err)
	}
	beforeDonorFruit := game.itemQuantity(donors[0].ID, fruit.ID)
	rejected, _, err := game.Execute("group", donors[0].AccountID, mustParse(t, "生日赠礼 @"+nonBirthday.DaoName+" 灵果*1"))
	if err != nil || !strings.Contains(rejected.Title, "未开启") || game.itemQuantity(donors[0].ID, fruit.ID) != beforeDonorFruit {
		t.Fatalf("off-day target accepted present: err=%v result=%+v", err, rejected)
	}
}
