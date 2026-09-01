package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestEveryLeaderboardHasThreeDistinctAttributedTitles(t *testing.T) {
	specs := model.RankingTitleCatalog()
	want := len(leaderboardDefinitions) * 3
	if len(specs) != want {
		t.Fatalf("ranking title count=%d want=%d", len(specs), want)
	}
	names := make(map[string]struct{}, len(specs))
	codes := make(map[string]struct{}, len(specs))
	perLeaderboard := make(map[string]int)
	for _, spec := range specs {
		if spec.Name == "" || spec.Code == "" || spec.BonusJSON == "" || spec.BonusJSON == "{}" {
			t.Fatalf("incomplete ranking title: %+v", spec)
		}
		if strings.ContainsAny(spec.Name, "1234567890") || strings.Contains(spec.Name, "第一") || strings.Contains(spec.Name, "第二") || strings.Contains(spec.Name, "第三") || strings.Contains(spec.Name, spec.Leaderboard+"榜") {
			t.Fatalf("ranking title used a numeric/ranking-name placeholder: %+v", spec)
		}
		if _, exists := names[spec.Name]; exists {
			t.Fatalf("duplicate ranking title name: %s", spec.Name)
		}
		if _, exists := codes[spec.Code]; exists {
			t.Fatalf("duplicate ranking title code: %s", spec.Code)
		}
		names[spec.Name] = struct{}{}
		codes[spec.Code] = struct{}{}
		perLeaderboard[spec.Leaderboard]++
	}
	for _, definition := range leaderboardDefinitions {
		if perLeaderboard[definition.Key] != 3 {
			t.Fatalf("leaderboard %s title count=%d", definition.Key, perLeaderboard[definition.Key])
		}
	}
}

func TestRankingTitlesFollowCurrentTopThreeAndRevokeEquippedStats(t *testing.T) {
	game, store := testGame(t)
	champion := registerPlayer(t, game, "ranking-title-champion", "镇岳真君")
	runner := registerPlayer(t, game, "ranking-title-runner", "流霞真人")
	third := registerPlayer(t, game, "ranking-title-third", "听风上人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", champion.ID).Update("combat_power", 30000).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", runner.ID).Update("combat_power", 20000).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", third.ID).Update("combat_power", 10000).Error; err != nil {
		t.Fatal(err)
	}

	ranking, handled, err := game.Execute("group", champion.AccountID, mustParse(t, "排行 战力"))
	if err != nil || !handled || !strings.Contains(ranking.Content, "诸天武圣") || !strings.Contains(ranking.Content, "镇世战仙") || !strings.Contains(ranking.Content, "破军真君") || ranking.BroadcastContent == "" {
		t.Fatalf("ranking title assignment: handled=%v err=%v result=%+v", handled, err, ranking)
	}
	for rank, playerID := range []uint{champion.ID, runner.ID, third.ID} {
		spec, _ := rankingTitleSpec("战力", rank+1)
		var title model.Title
		if err := store.DB.Where("code = ?", spec.Code).First(&title).Error; err != nil {
			t.Fatal(err)
		}
		if !game.playerValueExists(playerID, titleUnlockKey(title)) {
			t.Fatalf("rank %d player did not receive %s", rank+1, title.Name)
		}
	}

	champion, _ = game.players.Get(champion.ID)
	worn, handled, err := game.Execute("group", champion.AccountID, mustParse(t, "佩戴称号 诸天武圣"))
	if err != nil || !handled || !strings.Contains(worn.Content, "属性生效") {
		t.Fatalf("wear ranking title: handled=%v err=%v result=%+v", handled, err, worn)
	}
	wornPlayer, _ := game.players.Get(champion.ID)
	if wornPlayer.Title != "诸天武圣" || wornPlayer.PhysicalAttack <= champion.PhysicalAttack {
		t.Fatalf("ranking title attributes were not applied: before=%+v after=%+v", champion, wornPlayer)
	}

	if err := store.DB.Model(&model.Player{}).Where("id = ?", champion.ID).Update("combat_power", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", runner.ID).Update("combat_power", 900000).Error; err != nil {
		t.Fatal(err)
	}
	ranking, handled, err = game.Execute("group", runner.AccountID, mustParse(t, "排行 战力"))
	if err != nil || !handled {
		t.Fatalf("ranking title rotation: handled=%v err=%v result=%+v", handled, err, ranking)
	}
	championAfter, _ := game.players.Get(champion.ID)
	championSpec, _ := rankingTitleSpec("战力", 1)
	var championTitle model.Title
	if err := store.DB.Where("code = ?", championSpec.Code).First(&championTitle).Error; err != nil {
		t.Fatal(err)
	}
	if championAfter.Title != "" || game.playerValueExists(champion.ID, titleUnlockKey(championTitle)) {
		t.Fatalf("former champion retained title or unlock: %+v", championAfter)
	}
	if !game.playerValueExists(runner.ID, titleUnlockKey(championTitle)) {
		t.Fatal("new champion did not receive champion title")
	}
}
