package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/appinfo"
	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) systemOverview(player *model.Player) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	stamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	staminaMaximum, err := g.staminaMaximum(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	staminaRecovery, err := g.staminaRecoveryPerMinute(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	var current model.Realm
	if err := g.store.DB.Where("id = ? OR name = ?", player.RealmID, player.RealmName).Order("sequence").First(&current).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}
	var next model.Realm
	nextText := "已至当前境界尽头"
	if err := g.store.DB.Where("sequence > ?", current.Sequence).Order("sequence").First(&next).Error; err == nil {
		nextText = fmt.Sprintf("%s（需要修为%d）", next.Name, next.RequiredCultivation)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}
	categorySet := make(map[string]struct{})
	allCommands := append(append([]handler.CommandSpec(nil), handler.CommandTable...), handler.AuxiliaryCommands()...)
	for _, spec := range allCommands {
		if spec.Category != "系统" && !spec.EventOnly {
			categorySet[spec.Category] = struct{}{}
		}
	}
	progress := 0
	if player.CultivationRequired > 0 {
		progress = int(player.Cultivation * 100 / player.CultivationRequired)
		if progress > 100 {
			progress = 100
		}
	}
	lines := []string{
		"道号：" + player.DaoName,
		fmt.Sprintf("境界：%s · %d层", player.RealmName, player.RealmLevel),
		fmt.Sprintf("角色等级：LV%d · %s", maxInt(player.Level, 1), playerExperienceProgress(*player)),
		fmt.Sprintf("修为：%d/%d · %d%%", player.Cultivation, player.CultivationRequired, progress),
		fmt.Sprintf("灵根：%s · 纯度%d · 进化阶段%d", player.SpiritualRoot, player.RootQuality, spiritualRootStage(g.spiritualRootEvolutionValue(player.ID, "evolve"))),
		fmt.Sprintf("气血：%d/%d · 法力：%d/%d", effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana),
		fmt.Sprintf("灵石：%d · 银币：%d · 仙金：%d · 竞技币：%d", player.SpiritStones, player.SilverCoins, player.ImmortalJade, player.ArenaCoins),
		fmt.Sprintf("体力：%d/%d · 每个大境上限+%d · 每分钟自动恢复%d", stamina, staminaMaximum, g.settingInt("player.stamina_growth_per_realm", 100), staminaRecovery),
		"所在：" + displayOr(player.Location, "未知") + " · 已开放系统：" + fmt.Sprintf("%d类", len(categorySet)),
		"下一境界：" + nextText,
	}
	return GameResult{Title: "修仙系统总览", Content: strings.Join(lines, "\n"), Actions: []string{"状态", "等级", "体力", "灵检", "地图", "修炼", "快捷列表", "更新公告", "修复公告", "菜单"}}, true, nil
}

func (g *Game) runtimeOverview() (GameResult, bool, error) {
	sqlDB, err := g.store.DB.DB()
	if err != nil {
		return GameResult{}, true, err
	}
	now := time.Now()
	overallState := "正常"
	databaseState := "正常"
	if err := sqlDB.Ping(); err != nil {
		databaseState = "异常：" + err.Error()
		overallState = "异常"
	}
	stats := sqlDB.Stats()
	var schemaVersion string
	if err := g.store.DB.Model(&model.SystemSetting{}).Select("value").Where("key = ?", "system.schema_version").Scan(&schemaVersion).Error; err != nil {
		schemaVersion = "读取异常"
		overallState = "异常"
	}
	type countSpec struct {
		label string
		model any
		where string
		args  []any
	}
	counts := []countSpec{
		{"道籍", &model.Player{}, "deleted_at IS NULL", nil}, {"境界", &model.Realm{}, "", nil},
		{"灵根", &model.SpiritualRootTemplate{}, "enabled = ?", []any{true}}, {"灵脉", &model.WorldLeyline{}, "enabled = ?", []any{true}},
		{"地图", &model.WorldLocation{}, "enabled = ?", []any{true}}, {"物品", &model.Item{}, "", nil},
		{"器谱", &model.ArtifactTemplate{}, "enabled = ?", []any{true}}, {"灵兽", &model.PetTemplate{}, "enabled = ?", []any{true}},
		{"副本", &model.Dungeon{}, "enabled = ?", []any{true}}, {"任务", &model.TaskTemplate{}, "enabled = ?", []any{true}},
	}
	countText := make([]string, 0, len(counts))
	for _, spec := range counts {
		var count int64
		query := g.store.DB.Model(spec.model)
		if spec.where != "" {
			query = query.Where(spec.where, spec.args...)
		}
		if err := query.Count(&count).Error; err != nil {
			countText = append(countText, spec.label+"载入异常")
			overallState = "异常"
			continue
		}
		countText = append(countText, fmt.Sprintf("%s%d", spec.label, count))
	}
	categorySet := make(map[string]struct{})
	commandSet := make(map[string]struct{})
	allCommands := append(append([]handler.CommandSpec(nil), handler.CommandTable...), handler.AuxiliaryCommands()...)
	for _, spec := range allCommands {
		if spec.EventOnly {
			continue
		}
		categorySet[spec.Category] = struct{}{}
		commandSet[spec.Command] = struct{}{}
	}
	messageMode := g.settingText("message.mode", "native_markdown")
	if messageMode == "native_markdown" {
		messageMode = "QQ原生Markdown（失败自动回退文字）"
	}
	statusMode := "完整文字模式"
	if g.settingBool("display.status_image_mode", true) {
		statusMode = "单张属性图模式"
	}
	started := g.startedAt
	if started.IsZero() {
		started = now
	}
	content := fmt.Sprintf("运行状态：%s\n插件载入：正常\n指令系统：正常 · %d条指令 · %d类菜单\n━━━━━━━━━━━\n框架名称：%s\n插件名称：%s\n插件版本：v%s\n数据库版本：%s\n━━━━━━━━━━━\n启动时间：%s\n持续运行：%s\n当前时间：%s\n━━━━━━━━━━━\n数据库连接：%s\n数据存储：%s\n连接数：打开%d · 使用中%d · 空闲%d\n消息模式：%s\n状态显示：%s\n━━━━━━━━━━━\n数据载入：%s\n━━━━━━━━━━━\n版本号、数据库迁移号和数据包名称由同一版本源维护；升级只执行增量迁移，不覆盖现有玩家数据库。", overallState, len(commandSet), len(categorySet), appinfo.FrameworkName, appinfo.PluginName, appinfo.Version, displayOr(schemaVersion, "未读取"), started.Format("2006-01-02 15:04:05"), readableRuntimeDuration(now.Sub(started)), now.Format("2006-01-02 15:04:05"), databaseState, g.store.DatabaseMode(), stats.OpenConnections, stats.InUse, stats.Idle, messageMode, statusMode, strings.Join(countText, " · "))
	return GameResult{Title: "⚙️ 仙尘运行状态", Content: content, Actions: []string{"系统", "更新公告", "修复公告", "图鉴菜单", "功能菜单"}}, true, nil
}

func readableRuntimeDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟%d秒", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d分钟%d秒", minutes, seconds)
}

func (g *Game) staminaOverview(player *model.Player) (GameResult, bool, error) {
	current, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	maximum, err := g.staminaMaximum(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if sequence < 1 {
		sequence = 1
	}
	base := max64(g.settingInt("player.daily_stamina", 100), 0)
	growth := max64(g.settingInt("player.stamina_growth_per_realm", 100), 0)
	baseRecovery := max64(g.settingInt("player.stamina_recovery_per_minute", 10), 0)
	recoveryGrowth := max64(g.settingInt("player.stamina_recovery_growth_per_realm", 10), 0)
	recovery := g.staminaRecoveryForSequence(sequence)
	fullRecovery := int64(0)
	if recovery > 0 {
		fullRecovery = (maximum + recovery - 1) / recovery
	}
	content := fmt.Sprintf("当前体力：%d/%d\n当前境界：第%d大境 · %s%d层\n━━━━━━━━━━━\n基础上限：%d\n境界加成：+%d（已跨越%d个大境）\n成长规则：每提升一个大境界，体力上限永久+%d\n恢复成长：基础每分钟+%d，每提升一个大境再+%d，恢复速度不设上限\n当前恢复：每分钟自动+%d，约%d分钟可从零回满\n恢复方式：无需打坐，在线或离线都会按经过时间自动结算\n每日换日：恢复至当前境界对应的完整上限\n━━━━━━━━━━━\n常见消耗：野外探索2 · 灵兽捕获5 · 普通副本3 · 困难副本6 · 噩梦副本9 · 地狱副本12\n小境层数不会提高上限或恢复速度；进入下一个大境后两者才同步增加。", current, maximum, sequence, player.RealmName, player.RealmLevel, base, maximum-base, sequence-1, growth, baseRecovery, recoveryGrowth, recovery, fullRecovery)
	return GameResult{Title: "🧘 体力周天", Content: content, Actions: []string{"状态", "系统", "副本", "探索", "挂机"}}, true, nil
}
