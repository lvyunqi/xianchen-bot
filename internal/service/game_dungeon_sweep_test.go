package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func prepareManualDungeonClear(t *testing.T, game *Game, dungeon model.Dungeon, player model.Player) model.Item {
	t.Helper()
	manual := model.DungeonRun{
		PlayerID: player.ID, DungeonID: dungeon.ID,
		RunDate:    time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		DurationMS: 1500, Score: 100, Success: true,
	}
	if err := game.store.DB.Create(&manual).Error; err != nil {
		t.Fatal(err)
	}
	var ticket model.Item
	if err := game.store.DB.Where("name = ?", "扫荡券").First(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	return ticket
}

func fillDungeonRuns(t *testing.T, game *Game, playerID, dungeonID uint, count int) {
	t.Helper()
	if count <= 0 {
		return
	}
	runs := make([]model.DungeonRun, 0, count)
	for index := 0; index < count; index++ {
		runs = append(runs, model.DungeonRun{PlayerID: playerID, DungeonID: dungeonID, RunDate: time.Now().Format("2006-01-02"), DurationMS: 0, Score: 1, Success: true})
	}
	if err := game.store.DB.Create(&runs).Error; err != nil {
		t.Fatal(err)
	}
}

func TestBatchDungeonSweepCannotExceedOriginalDailyLimit(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "batch-sweep-player", "破界行者")
	var dungeon model.Dungeon
	if err := store.DB.Where("enabled = ? AND daily_limit > ?", true, 1).Order("id").First(&dungeon).Error; err != nil {
		t.Fatal(err)
	}
	ticket := prepareManualDungeonClear(t, game, dungeon, player)
	if err := game.players.AdjustItem(player.ID, ticket.ID, 10); err != nil {
		t.Fatal(err)
	}
	fillDungeonRuns(t, game, player.ID, dungeon.ID, dungeon.DailyLimit-1)
	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 100); err != nil {
		t.Fatal(err)
	}

	beforeTicket := game.itemQuantity(player.ID, ticket.ID)
	beforeStamina, _ := game.currentStamina(player.ID)
	beforePlayer, _ := game.players.Get(player.ID)
	rejected, handled, err := game.Execute("group", player.AccountID, mustParse(t, "扫荡 "+dungeon.Name+"*2"))
	if err != nil || !handled || !strings.Contains(rejected.Title, "次数不足") || !strings.Contains(rejected.Content, "没有扣除") {
		t.Fatalf("over-limit batch sweep: handled=%v err=%v result=%+v", handled, err, rejected)
	}
	afterReject, _ := game.players.Get(player.ID)
	afterRejectStamina, _ := game.currentStamina(player.ID)
	if game.itemQuantity(player.ID, ticket.ID) != beforeTicket || afterRejectStamina != beforeStamina || afterReject.Cultivation != beforePlayer.Cultivation {
		t.Fatal("rejected batch sweep changed ticket, stamina, or cultivation")
	}

	success, handled, err := game.Execute("group", player.AccountID, mustParse(t, "扫荡 "+dungeon.Name+"*1"))
	if err != nil || !handled || !strings.Contains(success.Title, "批量扫荡") || !strings.Contains(success.Content, "今日剩余次数：0/") || game.itemQuantity(player.ID, ticket.ID) != beforeTicket-1 {
		t.Fatalf("final legal sweep: handled=%v err=%v result=%+v", handled, err, success)
	}
	var todayRuns int64
	if err := store.DB.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND run_date = ?", player.ID, dungeon.ID, time.Now().Format("2006-01-02")).Count(&todayRuns).Error; err != nil || todayRuns != int64(dungeon.DailyLimit) {
		t.Fatalf("today runs=%d want=%d err=%v", todayRuns, dungeon.DailyLimit, err)
	}
	again, _, err := game.Execute("group", player.AccountID, mustParse(t, "扫荡 "+dungeon.Name))
	if err != nil || !strings.Contains(again.Title, "次数不足") || game.itemQuantity(player.ID, ticket.ID) != beforeTicket-1 {
		t.Fatalf("post-limit sweep: err=%v result=%+v", err, again)
	}
}

func TestDungeonAFKConsumesTicketsAndStopsAtDailyLimit(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "daily-limit-afk-player", "守限真人")
	var dungeon model.Dungeon
	if err := store.DB.Where("enabled = ? AND daily_limit > ?", true, 1).Order("id").First(&dungeon).Error; err != nil {
		t.Fatal(err)
	}
	ticket := prepareManualDungeonClear(t, game, dungeon, player)
	if err := game.players.AdjustItem(player.ID, ticket.ID, 5); err != nil {
		t.Fatal(err)
	}
	fillDungeonRuns(t, game, player.ID, dungeon.ID, dungeon.DailyLimit-1)
	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 100); err != nil {
		t.Fatal(err)
	}
	started, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 "+dungeon.Name+"*5"))
	if err != nil || !handled || !strings.Contains(started.Content, "每轮消耗1张扫荡券") {
		t.Fatalf("start limited dungeon AFK: handled=%v err=%v result=%+v", handled, err, started)
	}
	value, err := game.playerValue(player.ID, "afk.job")
	if err != nil {
		t.Fatal(err)
	}
	var job afkJob
	if err := json.Unmarshal([]byte(value), &job); err != nil {
		t.Fatal(err)
	}
	job.StartedAt = time.Now().Add(-51 * time.Minute)
	data, _ := json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		t.Fatal(err)
	}

	claimed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取挂机"))
	if err != nil || !handled || !strings.Contains(claimed.Content, "本次领取：1轮") || !strings.Contains(claimed.Content, "今日副本剩余：0/") || game.itemQuantity(player.ID, ticket.ID) != 4 {
		t.Fatalf("daily-capped AFK claim: handled=%v err=%v result=%+v", handled, err, claimed)
	}
	var todayRuns int64
	if err := store.DB.Model(&model.DungeonRun{}).Where("player_id = ? AND dungeon_id = ? AND run_date = ?", player.ID, dungeon.ID, time.Now().Format("2006-01-02")).Count(&todayRuns).Error; err != nil || todayRuns != int64(dungeon.DailyLimit) {
		t.Fatalf("AFK today runs=%d want=%d err=%v", todayRuns, dungeon.DailyLimit, err)
	}
	waiting, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取挂机"))
	if err != nil || !handled || !strings.Contains(waiting.Title, "等待副本次数") || game.itemQuantity(player.ID, ticket.ID) != 4 {
		t.Fatalf("AFK exceeded daily limit: handled=%v err=%v result=%+v", handled, err, waiting)
	}
	if _, err := game.playerValue(player.ID, "afk.job"); err != nil {
		t.Fatal("daily-limited AFK job should remain queued")
	}
}

func TestDungeonSweepAndAFKRequireManualClear(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "uncleared-dungeon-player", "初探散人")
	var dungeon model.Dungeon
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&dungeon).Error; err != nil {
		t.Fatal(err)
	}
	sweep, handled, err := game.Execute("group", player.AccountID, mustParse(t, "扫荡 "+dungeon.Name))
	if err != nil || !handled || !strings.Contains(sweep.Title, "未解锁") {
		t.Fatalf("uncleared direct sweep: handled=%v err=%v result=%+v", handled, err, sweep)
	}
	afk, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 "+dungeon.Name))
	if err != nil || !handled || !strings.Contains(afk.Title, "未解锁") {
		t.Fatalf("uncleared AFK: handled=%v err=%v result=%+v", handled, err, afk)
	}
}
