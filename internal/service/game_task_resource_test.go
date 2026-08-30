package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestMapResourceQuantityAllowsEachPlantBeforeCooldown(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "map-resource-stock-player", "采露散人")
	var location model.WorldLocation
	if err := store.DB.Where("name = ?", player.Location).First(&location).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&location).Updates(map[string]any{
		"resource_name": "凝露草", "resource_quantity": 3, "resource_cooldown_min": 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var item model.Item
	if err := store.DB.Where("name = ?", "凝露草").First(&item).Error; err != nil {
		t.Fatal(err)
	}
	before := game.itemQuantity(player.ID, item.ID)
	for index, wantRemaining := range []int{2, 1, 0} {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "采集 凝露草"))
		if err != nil || !handled || !strings.Contains(result.Title, "区域采集") || !strings.Contains(result.Content, "本次获得：凝露草×1") || !strings.Contains(result.Content, fmt.Sprintf("剩余可采：%d株", wantRemaining)) {
			t.Fatalf("gather %d: handled=%v err=%v result=%+v", index+1, handled, err, result)
		}
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "采集 凝露草"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "灵植尚未再生") || !strings.Contains(blocked.Content, "剩余0株") {
		t.Fatalf("depleted gather: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	if got := game.itemQuantity(player.ID, item.ID); got != before+3 {
		t.Fatalf("gathered quantity=%d want=%d", got, before+3)
	}
	mapResult, handled, err := game.Execute("group", player.AccountID, mustParse(t, "位置"))
	if err != nil || !handled || !strings.Contains(mapResult.Content, "凝露草 x0") || !strings.Contains(mapResult.Content, "后刷新") {
		t.Fatalf("depleted map stock: handled=%v err=%v result=%+v", handled, err, mapResult)
	}

	past := time.Now().Add(-time.Minute)
	if err := game.setPlayerValue(player.ID, fmt.Sprintf("map.resource.refresh.%d", location.ID), past.Format(time.RFC3339Nano), &past); err != nil {
		t.Fatal(err)
	}
	mapResult, handled, err = game.Execute("group", player.AccountID, mustParse(t, "位置"))
	if err != nil || !handled || !strings.Contains(mapResult.Content, "凝露草 x3") {
		t.Fatalf("refreshed map stock: handled=%v err=%v result=%+v", handled, err, mapResult)
	}
	refreshed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "采集 凝露草"))
	if err != nil || !handled || !strings.Contains(refreshed.Content, "剩余可采：2株") {
		t.Fatalf("gather after refresh: handled=%v err=%v result=%+v", handled, err, refreshed)
	}
}

func TestTaskProgressStartsAtAcceptanceAndCompletedTaskCannotBeReaccepted(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "task-progress-baseline-player", "靖妖行者")
	task := model.TaskTemplate{
		Name: "平息回风谷的妖患", Type: "地图", Description: "击败接取任务后出现的当地妖灵。",
		PrerequisiteJSON: `{}`, ObjectiveJSON: `{"type":"hunt","count":1}`,
		RewardJSON: `{"cultivation":62,"merit":3,"reputation":4}`, Enabled: true,
	}
	if err := store.DB.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stats.wins", 11); err != nil {
		t.Fatal(err)
	}
	accepted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "接任务 "+task.Name))
	if err != nil || !handled || !strings.Contains(accepted.Title, "任务已接取") || !strings.Contains(accepted.Content, "当前进度：0/1") || strings.Contains(accepted.Content, "11/1") {
		t.Fatalf("accept task: handled=%v err=%v result=%+v", handled, err, accepted)
	}
	var row model.PlayerTask
	if err := store.DB.Where("player_id = ? AND task_template_id = ?", player.ID, task.ID).First(&row).Error; err != nil || row.Status != "进行中" {
		t.Fatalf("active task row=%+v err=%v", row, err)
	}
	notReady, handled, err := game.Execute("group", player.AccountID, mustParse(t, "交任务 "+task.Name))
	if err != nil || !handled || !strings.Contains(notReady.Title, "任务未完成") || !strings.Contains(notReady.Content, "当前进度：0/1") {
		t.Fatalf("submit before new kill: handled=%v err=%v result=%+v", handled, err, notReady)
	}
	if _, err := game.addPlayerValueInt(player.ID, "stats.wins", 1); err != nil {
		t.Fatal(err)
	}
	beforeCompletion, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	completed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "交任务 "+task.Name))
	if err != nil || !handled || !strings.Contains(completed.Title, "任务完成") || !strings.Contains(completed.Content, "目标进度：1/1") || !strings.Contains(completed.Content, "银币") {
		t.Fatalf("submit completed task: handled=%v err=%v result=%+v", handled, err, completed)
	}
	afterCompletion, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := afterCompletion.SilverCoins-beforeCompletion.SilverCoins, model.TaskSilverReward(task); got != want || want <= 0 {
		t.Fatalf("task silver delta=%d want=%d", got, want)
	}
	sources, handled, err := game.Execute("group", player.AccountID, mustParse(t, "银币来源"))
	if err != nil || !handled || !strings.Contains(sources.Content, "每日签到") || !strings.Contains(sources.Content, "日常/悬赏/地图任务") || !containsAction(sources.Actions, "日常") {
		t.Fatalf("silver income guide: handled=%v err=%v result=%+v", handled, err, sources)
	}
	reaccepted, handled, err := game.Execute("group", player.AccountID, mustParse(t, "接任务 "+task.Name))
	if err != nil || !handled || !strings.Contains(reaccepted.Title, "今日任务已完成") || strings.Contains(reaccepted.Title, "任务已接取") {
		t.Fatalf("reaccept completed task: handled=%v err=%v result=%+v", handled, err, reaccepted)
	}
	var taskRows int64
	if err := store.DB.Model(&model.PlayerTask{}).Where("player_id = ? AND task_template_id = ?", player.ID, task.ID).Count(&taskRows).Error; err != nil || taskRows != 1 {
		t.Fatalf("task row count=%d err=%v", taskRows, err)
	}
}

func TestCreateSkillSkipsExistingDefaultNameAndNeverLeaksConstraintError(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "create-skill-duplicate-player", "悟法真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("perception", 100).Error; err != nil {
		t.Fatal(err)
	}
	player.Perception = 100
	existing := model.Skill{Name: player.DaoName + "心经", Type: "均衡", Rarity: "自创", Description: "既有自创功法。", EffectJSON: `{}`, UpgradeJSON: `{}`}
	if err := store.DB.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&model.PlayerSkill{PlayerID: player.ID, SkillID: existing.ID, Level: 1}).Error; err != nil {
		t.Fatal(err)
	}
	candidate, err := game.nextCreatedSkillName(&player)
	if err != nil || candidate == existing.Name || strings.ContainsAny(candidate, "0123456789") {
		t.Fatalf("next skill name=%q err=%v", candidate, err)
	}
	duplicate, handled, err := game.Execute("group", player.AccountID, mustParse(t, "创功 "+existing.Name))
	if err != nil || !handled || !strings.Contains(duplicate.Title, "功法已经掌握") {
		t.Fatalf("explicit duplicate skill: handled=%v err=%v result=%+v", handled, err, duplicate)
	}
	for attempt := 0; attempt < 12; attempt++ {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "创功"))
		if err != nil || !handled || strings.Contains(result.Title, "天机紊乱") {
			t.Fatalf("automatic create attempt %d: handled=%v err=%v result=%+v", attempt+1, handled, err, result)
		}
	}
}

func TestPetExperienceBarAndRealLevelUp(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "pet-experience-player", "御灵真人")
	var template model.PetTemplate
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&template).Error; err != nil {
		t.Fatal(err)
	}
	var food model.Item
	if err := store.DB.Where("effect_func = ?", "pet_loyalty").Order("id").First(&food).Error; err != nil {
		t.Fatal(err)
	}
	gain := max64(int64(food.EffectValue), 10)
	pet := model.Pet{
		PlayerID: player.ID, Name: "试炼云纹灵鹿", Species: template.Name, Rarity: "凡品", Level: 1,
		Experience: petExperienceRequired(1) - gain, Attack: template.InitialPower, Defense: max64(template.InitialPower/2, 1),
		Health: template.InitialPower * 10, Loyalty: 60, Active: true, SkillJSON: `[]`,
	}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("active_pet_id", pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, food.ID, 1); err != nil {
		t.Fatal(err)
	}
	beforeAttack, beforeDefense, beforeHealth := pet.Attack, pet.Defense, pet.Health
	fed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "喂养 "+food.Name))
	if err != nil || !handled || !strings.Contains(fed.Content, "等级：1 → 2") || !strings.Contains(fed.Content, "灵悟 [░░░░░░░░░░] 0% · 0/150") {
		t.Fatalf("feed level up: handled=%v err=%v result=%+v", handled, err, fed)
	}
	if err := store.DB.First(&pet, pet.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pet.Level != 2 || pet.Experience != 0 || pet.Attack <= beforeAttack || pet.Defense <= beforeDefense || pet.Health <= beforeHealth {
		t.Fatalf("leveled pet=%+v before attack=%d defense=%d health=%d", pet, beforeAttack, beforeDefense, beforeHealth)
	}
	space, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵兽"))
	if err != nil || !handled || !strings.Contains(space.Content, "灵悟 [░░░░░░░░░░] 0% · 0/150") {
		t.Fatalf("pet space progress: handled=%v err=%v result=%+v", handled, err, space)
	}
	partial := model.Pet{Level: 1, Experience: 50}
	if progress := petExperienceProgress(partial); !strings.Contains(progress, "[█████░░░░░] 50% · 50/100") {
		t.Fatalf("half progress bar=%q", progress)
	}
}
