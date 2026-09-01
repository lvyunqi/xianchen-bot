package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

const databaseSchemaVersion = "2026.07.24.258.02"

type Store struct {
	DB  *gorm.DB
	cfg config.Config
}

func (s *Store) DatabaseMode() string {
	if s != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return "PostgreSQL云数据"
	}
	return "SQLite本地数据"
}

func Open(cfg config.Config) (*Store, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(cfg.Database.Driver) {
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.Database.DSN)
	default:
		path := cfg.Database.DSN
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		dialector = sqliteDialector(path)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConnections)
	store := &Store{DB: db, cfg: cfg}
	if err := store.ensureCriticalRuntimeSchema(); err != nil {
		return nil, fmt.Errorf("ensure critical runtime schema: %w", err)
	}
	if store.databaseReady() {
		if err := store.ensureRuntimeHotfixContent(); err != nil {
			return nil, fmt.Errorf("ensure runtime hotfix content: %w", err)
		}
		return store, nil
	}
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	if err := store.Seed(); err != nil {
		return nil, err
	}
	marker := model.SystemSetting{Key: "system.schema_version", Value: databaseSchemaVersion, ValueType: "string", Description: "数据库结构版本，请勿手动修改"}
	if err := store.DB.Where("key = ?", marker.Key).Assign(map[string]any{"value": marker.Value, "value_type": marker.ValueType, "description": marker.Description}).FirstOrCreate(&marker).Error; err != nil {
		return nil, err
	}
	if err := store.ensureRuntimeHotfixContent(); err != nil {
		return nil, fmt.Errorf("ensure runtime hotfix content: %w", err)
	}
	return store, nil
}

func (s *Store) databaseReady() bool {
	var value string
	err := s.DB.Table("system_settings").Select("value").Where("key = ?", "system.schema_version").Scan(&value).Error
	return err == nil && value == databaseSchemaVersion
}

func (s *Store) Migrate() error {
	return s.DB.AutoMigrate(
		&model.Realm{}, &model.SystemSetting{}, &model.Player{}, &model.PlayerItem{},
		&model.PlayerValue{}, &model.PlayerExtendedProgress{}, &model.AccountMigrationCode{}, &model.GroupAccessRequest{},
		&model.SpiritualRootTemplate{},
		&model.WorldLeyline{},
		&model.Couple{}, &model.ItemCategory{}, &model.Rarity{}, &model.Item{},
		&model.DropPool{}, &model.DropEntry{}, &model.Event{}, &model.Broadcast{},
		&model.TaskTemplate{}, &model.PlayerTask{}, &model.Skill{}, &model.PlayerSkill{}, &model.SkillPublication{},
		&model.Pet{}, &model.PetTemplate{}, &model.Mansion{}, &model.MansionCrop{}, &model.TradeListing{},
		&model.TradeRecord{}, &model.BarterRequest{}, &model.Friendship{}, &model.SocialMessage{}, &model.Mentorship{},
		&model.GameLog{}, &model.OperationLog{}, &model.GMAccount{}, &model.RankEntry{},
		&model.Dungeon{}, &model.Title{}, &model.Activity{}, &model.Mail{}, &model.CheckinReward{},
		&model.ShopEntry{}, &model.RedemptionCode{}, &model.Notice{}, &model.Sect{}, &model.SectMember{},
		&model.AlchemyRecipe{}, &model.ArtifactTemplate{}, &model.PlayerArtifact{},
		&model.BankAccount{}, &model.BankTransaction{},
		&model.ReferralCode{}, &model.ReferralBinding{}, &model.ReferralClaim{},
		&model.AccountRewardClaim{},
		&model.SynthesisRecipe{},
		&model.DungeonRun{}, &model.ArenaRecord{}, &model.ArenaTier{},
		&model.AdminMenu{}, &model.AdminMenuPermission{}, &model.AdminMenuLog{},
		&model.ContentReview{}, &model.SensitiveWord{}, &model.SlowQueryLog{}, &model.ManagerAccount{},
		&model.FormationConfig{}, &model.TalismanConfig{}, &model.PuppetConfig{}, &model.SecretRealmConflictConfig{},
		&model.InheritanceConfig{}, &model.DaoInsightConfig{}, &model.ImmortalDemonBattlefieldConfig{}, &model.SpiritualRootEvolutionConfig{},
		&model.InnerDemonConfig{}, &model.CoupleCombinationSkillConfig{}, &model.ImmortalHerbConfig{}, &model.ArtifactRefinementConfig{},
		&model.DestinyDeductionConfig{}, &model.LeylineConfig{}, &model.SectWarConfig{}, &model.ImmortalEncounterConfig{}, &model.StarRealmConfig{},
		&model.WorldLocation{},
	)
}

func (s *Store) Seed() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		seedStore := &Store{DB: tx, cfg: s.cfg}
		return seedStore.seed()
	})
}

func (s *Store) seed() error {
	settings := []model.SystemSetting{
		{Key: "cultivation.base_reward", Value: "5", ValueType: "int", Description: "每分钟闭关获得修为，资质和灵根会放大收益"},
		{Key: "cultivation.aptitude_factor", Value: "0.05", ValueType: "float", Description: "每点资质增加的修炼速度"},
		{Key: "cultivation.root_bonus", Value: "1.3", ValueType: "float", Description: "灵根修炼倍率"},
		{Key: "cultivation.couple_bonus", Value: "1.3", ValueType: "float", Description: "仙侣修炼倍率"},
		{Key: "cultivation.minimum_minutes", Value: "5", ValueType: "int", Description: "闭关最短结算分钟"},
		{Key: "cultivation.maximum_minutes", Value: "20160", ValueType: "int", Description: "单次闭关最大结算分钟，默认14天"},
		{Key: "battle.normal_hunt_cooldown_seconds", Value: "8", ValueType: "int", Description: "普通猎妖胜利后的再次挑战间隔秒数，防止按钮连点和排队消息重复开战"},
		{Key: "checkin.silver_reward", Value: "120", ValueType: "int", Description: "每日签到基础银币奖励"},
		{Key: "silver_job.cooldown_minutes", Value: "10", ValueType: "int", Description: "仙盟银币差事的刷新间隔分钟"},
		{Key: "silver_job.stamina_cost", Value: "4", ValueType: "int", Description: "每次仙盟银币差事消耗的体力"},
		{Key: "bank.loan_days", Value: "7", ValueType: "int", Description: "银币借款基础期限天数"},
		{Key: "bank.loan_interest_basis_points", Value: "500", ValueType: "int", Description: "每笔银币借款的基础利息万分比，默认5%"},
		{Key: "bank.overdue_daily_basis_points", Value: "100", ValueType: "int", Description: "逾期银币每日追加利息万分比，默认1%"},
		{Key: "trade.barter_expiry_minutes", Value: "10", ValueType: "int", Description: "玩家易物申请等待对方确认的有效分钟数"},
		{Key: "tribulation.base_rate", Value: "0.70", ValueType: "float", Description: "单人渡劫基础成功率"},
		{Key: "tribulation.couple_bonus", Value: "0.30", ValueType: "float", Description: "双人渡劫成功率加成"},
		{Key: "task.daily_count", Value: "3", ValueType: "int", Description: "每日任务数量"},
		{Key: "player.max_rebirth", Value: "3", ValueType: "int", Description: "最大转世次数"},
		{Key: "inventory.capacity", Value: "50", ValueType: "int", Description: "背包最大物品种类"},
		{Key: "mansion.material_factor", Value: "1.0", ValueType: "float", Description: "仙府材料消耗倍率"},
		{Key: "arena.match_range", Value: "0.20", ValueType: "float", Description: "竞技匹配战力浮动"},
		{Key: "broadcast.cooldown_seconds", Value: "60", ValueType: "int", Description: "全服传音冷却秒数"},
		{Key: "pet.loyalty_decay", Value: "2", ValueType: "int", Description: "灵兽每日忠诚衰减"},
		{Key: "pet.base_capacity", Value: "5", ValueType: "int", Description: "炼气期基础灵兽空间容量"},
		{Key: "pet.capacity_per_realm", Value: "1", ValueType: "int", Description: "每提升一个大境增加的灵兽空间"},
		{Key: "pet.encounter_minutes", Value: "10", ValueType: "int", Description: "探索发现灵兽后可尝试捕获的分钟数"},
		{Key: "pet.capture_cooldown_seconds", Value: "60", ValueType: "int", Description: "每次真实捕获结算后的御兽印冷却秒数"},
		{Key: "dungeon.stamina_cost", Value: "10", ValueType: "int", Description: "副本默认体力消耗"},
		{Key: "player.daily_stamina", Value: "100", ValueType: "int", Description: "炼气期基础体力上限"},
		{Key: "player.stamina_growth_per_realm", Value: "100", ValueType: "int", Description: "每提升一个大境界增加的体力上限"},
		{Key: "player.stamina_recovery_per_minute", Value: "10", ValueType: "int", Description: "炼气期体力每分钟基础自动恢复点数"},
		{Key: "player.stamina_recovery_growth_per_realm", Value: "10", ValueType: "int", Description: "每提升一个大境界增加的每分钟体力恢复点数，不设速度上限"},
		{Key: "afk.interval_minutes", Value: "10", ValueType: "int", Description: "挂机每轮结算间隔分钟"},
		{Key: "afk.max_minutes", Value: "1440", ValueType: "int", Description: "挂机单次最长时长，默认24小时"},
		{Key: "spiritual_root.evolve_cooldown_minutes", Value: "10", ValueType: "int", Description: "灵根进化成功后的本源沉淀时间"},
		{Key: "spiritual_root.awaken_cooldown_hours", Value: "24", ValueType: "int", Description: "灵根觉醒后的道基沉淀时间"},
		{Key: "luck.encounter_growth_rate", Value: "0.10", ValueType: "float", Description: "成功承接仙缘奇遇时永久增加1点运气的基础概率；当前运气还会额外提高此概率"},
		{Key: "feedback.bug_silver_reward", Value: "120", ValueType: "int", Description: "自动初审通过的有效BUG提交奖励银币"},
		{Key: "feedback.bug_stone_reward", Value: "80", ValueType: "int", Description: "自动初审通过的有效BUG提交奖励灵石"},
		{Key: "feedback.suggestion_silver_reward", Value: "80", ValueType: "int", Description: "自动初审通过的可行建议提交奖励银币"},
		{Key: "feedback.suggestion_stone_reward", Value: "50", ValueType: "int", Description: "自动初审通过的可行建议提交奖励灵石"},
		{Key: "feedback.daily_reward_limit", Value: "3", ValueType: "int", Description: "每名玩家每日可领取的有效反馈提交奖励次数，超出后仍正常入库"},
		{Key: "message.mode", Value: "native_markdown", ValueType: "string", Description: "消息模式：native_markdown或text"},
		{Key: "message.native_fallback", Value: "true", ValueType: "bool", Description: "原生Markdown失败时自动回退普通文本"},
		{Key: "recharge.price_table", Value: `[{"price":6,"jade":60,"bonus":0},{"price":30,"jade":300,"bonus":30},{"price":68,"jade":680,"bonus":100},{"price":128,"jade":1280,"bonus":300},{"price":328,"jade":3280,"bonus":1100},{"price":648,"jade":6480,"bonus":2800}]`, ValueType: "json", Description: "玩家充值菜单仙金价格表"},
		{Key: "recharge.instructions", Value: "请联系主人确认收款方式；付款时提供全服唯一道号，到账后由主人使用统一充值神令入账。", ValueType: "string", Description: "玩家充值指引"},
		{Key: "recharge.spirit_stones_per_yuan", Value: "2000000", ValueType: "int", Description: "氪金菜单每元对应灵石及累充折算比例"},
		{Key: "recharge.jade_per_yuan", Value: "2000", ValueType: "int", Description: "氪金菜单每元对应仙金及累充折算比例"},
		{Key: "menu.cover_url", Value: "", ValueType: "string", Description: "功能菜单顶部图片URL，可在插件设置上传后填写"},
		{Key: "owner.user_id", Value: "", ValueType: "string", Description: "主人QQ开放平台用户ID，仅主人可执行上传图片等管理命令"},
		{Key: "image.status_url", Value: "", ValueType: "string", Description: "状态面板图片URL"},
		{Key: "image.battle_url", Value: "", ValueType: "string", Description: "战斗面板图片URL"},
		{Key: "image.logo_url", Value: "", ValueType: "string", Description: "插件Logo图片URL"},
		{Key: "image.avatar_template", Value: "", ValueType: "string", Description: "玩家头像URL模板，使用{user_id}替换QQ开放平台用户ID"},
		{Key: "image.inventory_url", Value: "", ValueType: "string", Description: "背包面板图片URL"},
		{Key: "image.status_font_name", Value: "Microsoft YaHei UI", ValueType: "string", Description: "状态属性图使用的Windows中文字体名称"},
		{Key: "display.status_image_mode", Value: "true", ValueType: "bool", Description: "开启后状态指令只发送带玩家头像的动态属性图；关闭后发送完整文字属性"},
	}
	for _, setting := range settings {
		if err := s.DB.Where("key = ?", setting.Key).FirstOrCreate(&setting).Error; err != nil {
			return err
		}
	}
	// 480分钟是早期测试值；只迁移未被运营人员改过的默认值，保留自定义设置。
	if err := s.DB.Model(&model.SystemSetting{}).Where("key = ? AND value = ?", "cultivation.maximum_minutes", "480").Updates(map[string]any{"value": "20160", "description": "单次闭关最大结算分钟，默认14天"}).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.SystemSetting{}).Where("key = ? AND value = ?", "cultivation.base_reward", "20").Updates(map[string]any{"value": "5", "description": "每分钟闭关获得修为，资质和灵根会放大收益"}).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.SystemSetting{}).Where("key = ? AND description = ?", "player.daily_stamina", "每日体力上限").Update("description", "炼气期基础体力上限").Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.SystemSetting{}).Where("key = ? AND value = ? AND description IN ?", "player.stamina_recovery_per_minute", "1", []string{"体力每分钟自然恢复点数", "炼气期体力每分钟基础自动恢复点数"}).Updates(map[string]any{"value": "10", "description": "炼气期体力每分钟基础自动恢复点数"}).Error; err != nil {
		return err
	}
	if err := s.DB.Where("key LIKE ?", "markdown.%").Delete(&model.SystemSetting{}).Error; err != nil {
		return err
	}
	realms := realmCatalog()[:9]
	for _, realm := range realms {
		if err := s.DB.Where("name = ?", realm.Name).FirstOrCreate(&realm).Error; err != nil {
			return err
		}
	}
	categories := []model.ItemCategory{{Name: "丹药", Sort: 10}, {Name: "材料", Sort: 20}, {Name: "灵草", Sort: 30}, {Name: "功法", Sort: 40}, {Name: "残卷", Sort: 45}, {Name: "种子", Sort: 50}, {Name: "礼包", Sort: 60}, {Name: "装备", Sort: 70}, {Name: "法宝", Sort: 75}, {Name: "嵌灵宝石", Sort: 80}, {Name: "灵肥", Sort: 90}, {Name: "生辰", Sort: 100}, {Name: "任务物品", Sort: 110}}
	for _, category := range categories {
		if err := s.DB.Where("name = ?", category.Name).FirstOrCreate(&category).Error; err != nil {
			return err
		}
	}
	rarities := []model.Rarity{{Name: "凡品", Level: 1, ValueMultiplier: 1, DropWeight: 60, Color: "#9ca3af"}, {Name: "灵品", Level: 2, ValueMultiplier: 2, DropWeight: 28, Color: "#22c55e"}, {Name: "仙品", Level: 3, ValueMultiplier: 6, DropWeight: 10, Color: "#38bdf8"}, {Name: "神品", Level: 4, ValueMultiplier: 20, DropWeight: 2, Color: "#f59e0b"}}
	for _, rarity := range rarities {
		if err := s.DB.Where("name = ?", rarity.Name).FirstOrCreate(&rarity).Error; err != nil {
			return err
		}
	}
	items := []model.Item{
		{Code: "item_spirit_fruit", Name: "灵果", CategoryName: "丹药", RarityName: "凡品", Description: "蕴含少量灵气，可用于快速修行。", EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: 10, BaseValue: 50, StackLimit: 99, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 50},
		{Code: "item_immortal_dew", Name: "仙露", CategoryName: "丹药", RarityName: "凡品", Description: "凝露草淬出的温和疗伤灵液，每份恢复35%最大气血。", EffectType: "治疗比例", EffectFunc: "heal_hp", EffectParams: `{"max_health_percent":35}`, EffectValue: 35, BaseValue: 30, StackLimit: 99, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 30},
		{Code: "item_recovery_powder", Name: "回元散", CategoryName: "丹药", RarityName: "灵品", Description: "以凝露草和灵茶调和经脉，每份恢复45%最大气血，可在战斗外或回合中服用。", EffectType: "治疗比例", EffectFunc: "heal_hp", EffectParams: `{"max_health_percent":45}`, EffectValue: 45, BaseValue: 160, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_mana_recovery_pill", Name: "回灵丹", CategoryName: "丹药", RarityName: "灵品", Description: "以灵茶引导清灵药气归入丹田，每颗恢复40%最大法力，可在回合战斗中服用。", EffectType: "法力恢复比例", EffectFunc: "restore_mana", EffectParams: `{"max_mana_percent":40}`, EffectValue: 40, BaseValue: 180, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 120},
		{Code: "item_spirit_gathering_pill", Name: "聚灵丹", CategoryName: "丹药", RarityName: "灵品", Description: "赤焰草炼开灵果药性，服下一颗获得120点修为。", EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: 120, BaseValue: 260, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_spirit_tea", Name: "灵茶", CategoryName: "丹药", RarityName: "灵品", Description: "悟道、诵经所需。", EffectType: "悟性", EffectFunc: "add_perception", EffectValue: 1, BaseValue: 80, StackLimit: 99, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 80},
		{Code: "item_mansion_wood", Name: "仙府材料", CategoryName: "材料", RarityName: "凡品", Description: "升级仙府所需的基础材料。", EffectType: "材料", BaseValue: 100, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_root_essence", Name: "灵根精粹", CategoryName: "材料", RarityName: "仙品", Description: "承载一缕灵根道纹的无属性精粹；两份精粹配合阵基石，可将两种不同灵根道纹合成为随机新灵根。", EffectType: "灵根合成", BaseValue: 1800, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_skill_scroll", Name: "功法残卷", CategoryName: "残卷", RarityName: "灵品", Description: "学习功法与自创功法推演所需，可从秘境、剑冢、活动和商店获得。", EffectType: "材料", BaseValue: 300, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "item_sweep_ticket", Name: "扫荡券", CategoryName: "材料", RarityName: "灵品", Description: "快速扫荡已通关副本。", EffectType: "副本", BaseValue: 200, StackLimit: 99, Stackable: true, Tradable: true},
	}
	for _, item := range items {
		if err := s.firstOrCreateCodeName(&item, item.Code, item.Name); err != nil {
			return err
		}
	}
	if err := s.DB.Model(&model.Item{}).Where("code = ?", "item_skill_scroll").Update("description", "学习功法与自创功法推演所需，可从秘境、剑冢、活动和商店获得。").Error; err != nil {
		return err
	}
	// 修复旧版本疗伤物品只有文案、没有明确比例参数的问题。
	if err := s.DB.Model(&model.Item{}).Where("code = ?", "item_immortal_dew").Updates(map[string]any{
		"description": "凝露草淬出的温和疗伤灵液，每份恢复35%最大气血。", "effect_type": "治疗比例",
		"effect_func": "heal_hp", "effect_params": `{"max_health_percent":35}`, "effect_value": 35,
	}).Error; err != nil {
		return err
	}
	checkins := []model.CheckinReward{{Day: 1, ItemName: "灵果", Quantity: 3}, {Day: 2, ItemName: "仙露", Quantity: 2}, {Day: 3, ItemName: "灵茶", Quantity: 5}, {Day: 4, ItemName: "仙府材料", Quantity: 3}, {Day: 5, ItemName: "功法残卷", Quantity: 1}, {Day: 6, ItemName: "灵果", Quantity: 5}, {Day: 7, ItemName: "随机宝物", Quantity: 1, SpecialReward: "双倍修为卡x1"}}
	for _, row := range checkins {
		if err := s.DB.Where("day = ?", row.Day).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	titles := []model.Title{
		{Code: "title_begin", Name: "初入仙途", Condition: "入道即获得", Type: "基础", Enabled: true},
		{Code: "title_foundation", Name: "筑基真人", Condition: "达到筑基", AttributeBonus: `{"health":50}`, Type: "境界", Enabled: true},
		{Code: "title_golden", Name: "金丹宗师", Condition: "达到金丹", AttributeBonus: `{"attack":10}`, Type: "境界", Enabled: true},
		{Code: "title_nascent", Name: "元婴老祖", Condition: "达到元婴", AttributeBonus: `{"defense":5}`, Type: "境界", Enabled: true},
		{Code: "title_couple", Name: "道侣同心", Condition: "结为仙侣", AttributeBonus: `{"affinity":20}`, Type: "道侣", Enabled: true},
		{Code: "title_deep_bond", Name: "情比金坚", Condition: "道缘深度达到500", AttributeBonus: `{"joint_attack_percent":10}`, Type: "道侣", Enabled: true},
		{Code: "title_lucky", Name: "天眷之人", Condition: "运气达到50", AttributeBonus: `{"explore_percent":10}`, Type: "隐藏", Enabled: true},
		{Code: "title_tribulation", Name: "渡劫仙尊", Condition: "渡劫成功", AttributeBonus: `{"all_percent":10}`, Type: "渡劫", Enabled: true},
		{Code: "title_warrior", Name: "百战勇士", Condition: "战斗100次", AttributeBonus: `{"attack":5}`, Type: "战斗", Enabled: true},
		{Code: "title_alchemy", Name: "炼丹大师", Condition: "丹道达到50", AttributeBonus: `{"pill_percent":20}`, Type: "特殊", Enabled: true},
		{Code: "title_beasts", Name: "万兽之友", Condition: "捕获灵兽5只", AttributeBonus: `{"pet_loyalty":10}`, Type: "灵兽", Enabled: true},
		{Code: "title_ascend", Name: "飞升仙人", Condition: "飞升成功", AttributeBonus: `{"all_percent":20}`, Type: "飞升", Enabled: true},
		{Code: "title_birthday_long_life", Name: "岁序长生", Condition: "领取仙尘生辰礼", AttributeBonus: `{"attack":18,"defense":12,"health":188,"mana":88}`, Type: "生辰", Enabled: true},
	}
	for _, row := range titles {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	rankingSeats := []string{"冠席", "亚席", "季席"}
	for _, spec := range model.RankingTitleCatalog() {
		row := model.Title{
			Code: spec.Code, Name: spec.Name,
			Condition:      "当前执掌" + spec.Leaderboard + "榜" + rankingSeats[spec.Rank-1],
			AttributeBonus: spec.BonusJSON, Type: "排行", Enabled: true,
		}
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
		if err := s.DB.Model(&model.Title{}).Where("code = ?", row.Code).Updates(map[string]any{
			"name": row.Name, "condition": row.Condition, "attribute_bonus": row.AttributeBonus, "type": row.Type, "enabled": true,
		}).Error; err != nil {
			return err
		}
	}
	skills := []model.Skill{
		{Name: "火云诀", Type: "攻击", Rarity: "灵品", RealmRequired: "炼气", Description: "引火云之气淬炼攻伐。", EffectJSON: `{"attack_per_level":5}`, UpgradeJSON: `{"mastery_per_level":100}`},
		{Name: "玄水经", Type: "防御", Rarity: "灵品", RealmRequired: "炼气", Description: "玄水周流，护持经脉。", EffectJSON: `{"defense_per_level":4}`, UpgradeJSON: `{"mastery_per_level":100}`},
		{Name: "太虚引", Type: "均衡", Rarity: "仙品", RealmRequired: "筑基", Description: "接引太虚灵息，攻守兼备。", EffectJSON: `{"attack_per_level":3,"defense_per_level":3}`, UpgradeJSON: `{"mastery_per_level":120}`},
	}
	for _, row := range skills {
		if err := s.DB.Where("name = ?", row.Name).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	petTemplates := []model.PetTemplate{
		{Code: "pet_fox", Name: "赤焰狐", InitialPower: 12, GrowthPerLevel: 3, LoyaltyDecay: 2, EvolutionCondition: `{"loyalty":80,"level":5}`, EvolutionTarget: "九尾炎狐", Enabled: true},
		{Code: "pet_crane", Name: "青羽鹤", InitialPower: 10, GrowthPerLevel: 4, LoyaltyDecay: 1, EvolutionCondition: `{"loyalty":80,"level":5}`, EvolutionTarget: "云霄仙鹤", Enabled: true},
		{Code: "pet_turtle", Name: "玄甲龟", InitialPower: 15, GrowthPerLevel: 2, LoyaltyDecay: 1, EvolutionCondition: `{"loyalty":80,"level":5}`, EvolutionTarget: "玄武遗种", Enabled: true},
	}
	for _, row := range petTemplates {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	tasks := []model.TaskTemplate{
		{Name: "山野巡查", Type: "日常", Description: "完成一次探索或战斗。", PrerequisiteJSON: `{"minimum_realm_sequence":1,"minimum_realm_level":1}`, ObjectiveJSON: `{"type":"action","count":1}`, RewardJSON: `{"cultivation":100,"reputation":5}`, Weight: 100, Daily: true, Enabled: true},
		{Name: "猎妖悬赏", Type: "悬赏", Description: "猎杀妖兽，平息山野祸患。", PrerequisiteJSON: `{"minimum_realm_sequence":1,"minimum_realm_level":1,"minimum_combat_power":80}`, ObjectiveJSON: `{"type":"hunt","count":1}`, RewardJSON: `{"cultivation":150,"merit":5}`, Weight: 80, Enabled: true},
		{Name: "采药济世", Type: "日常", Description: "前往山野采集灵草。", PrerequisiteJSON: `{"minimum_realm_sequence":1,"minimum_realm_level":1,"minimum_perception":10}`, ObjectiveJSON: `{"type":"collect","count":1}`, RewardJSON: `{"cultivation":80}`, Weight: 70, Daily: true, Enabled: true},
	}
	for _, row := range tasks {
		if err := s.DB.Where("name = ?", row.Name).FirstOrCreate(&row).Error; err != nil {
			return err
		}
		if err := s.DB.Model(&model.TaskTemplate{}).Where("name = ? AND (prerequisite_json = '' OR prerequisite_json = '{}' OR prerequisite_json IS NULL)", row.Name).Update("prerequisite_json", row.PrerequisiteJSON).Error; err != nil {
			return err
		}
	}
	dungeons := []model.Dungeon{
		{Code: "dungeon_ghost", Name: "幽冥秘境", Difficulty: "普通", RecommendedPower: 150, StaminaCost: 3, RewardPoolJSON: `{"cultivation":180,"items":["仙露"]}`, DailyLimit: 20, Enabled: true},
		{Code: "dungeon_sword", Name: "剑冢遗址", Difficulty: "困难", RecommendedPower: 350, StaminaCost: 6, RewardPoolJSON: `{"cultivation":350,"items":["功法残卷"]}`, DailyLimit: 12, Enabled: true},
		{Code: "dungeon_sky", Name: "九霄雷域", Difficulty: "噩梦", RecommendedPower: 700, StaminaCost: 9, RewardPoolJSON: `{"cultivation":700,"items":["仙府材料"]}`, DailyLimit: 8, Enabled: true},
	}
	for _, row := range dungeons {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	recipes := []model.AlchemyRecipe{
		{Code: "recipe_recovery", Name: "回元散", MaterialsJSON: `{"凝露草":2,"灵茶":1}`, OutputName: "回元散", SuccessRate: .75, Description: "凝露草修复经脉、灵茶调和药性；成丹后每份恢复45%最大气血。", Enabled: true},
		{Code: "recipe_mana", Name: "回灵丹", MaterialsJSON: `{"凝露草":1,"灵果":1,"灵茶":2}`, OutputName: "回灵丹", SuccessRate: .78, Description: "灵茶清理识海浊气、灵果补充丹田灵机；成丹后每颗恢复40%最大法力。", Enabled: true},
		{Code: "recipe_spirit", Name: "聚灵丹", MaterialsJSON: `{"灵果":2,"灵茶":1,"赤焰草":1}`, OutputName: "聚灵丹", SuccessRate: .60, Description: "以赤焰草为药引炼开灵果精华；成丹后每颗增加120点修为。", Enabled: true},
	}
	for _, row := range recipes {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	// firstOrCreate preserves operator-managed rows, so these two historically
	// broken recipes need an explicit one-time canonical migration on upgrades.
	for _, row := range recipes {
		var output model.Item
		if err := s.DB.Where("name = ?", row.OutputName).First(&output).Error; err != nil {
			return err
		}
		if err := s.DB.Model(&model.AlchemyRecipe{}).Where("code = ?", row.Code).Updates(map[string]any{
			"materials_json": row.MaterialsJSON, "output_item_id": output.ID, "output_name": row.OutputName,
			"success_rate": row.SuccessRate, "description": row.Description, "enabled": true,
		}).Error; err != nil {
			return err
		}
	}
	artifacts := []model.ArtifactTemplate{
		{Code: "artifact_sword", Name: "青冥剑", Type: "攻击", MaterialsJSON: `{"仙府材料":3}`, AttributeJSON: `{"attack":8}`, MaxLevel: 20, Enabled: true},
		{Code: "artifact_bell", Name: "玄元钟", Type: "防御", MaterialsJSON: `{"仙府材料":3}`, AttributeJSON: `{"defense":6}`, MaxLevel: 20, Enabled: true},
	}
	for _, row := range artifacts {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	if err := s.seedFullContent(); err != nil {
		return err
	}
	if err := s.migrateCoreItemGuidance(); err != nil {
		return err
	}
	if err := s.seedActivityContent(); err != nil {
		return err
	}
	if err := s.migrateLegacyMedicineCatalog(); err != nil {
		return err
	}
	if err := s.migrateLegacyTaskCatalog(); err != nil {
		return err
	}
	if err := s.migrateGeneratedRealms(); err != nil {
		return err
	}
	if err := s.seedRealmCatalog(); err != nil {
		return err
	}
	if err := s.seedSpiritualRootCatalog(); err != nil {
		return err
	}
	if err := s.seedWorldLeylineCatalog(); err != nil {
		return err
	}
	if err := s.seedArenaTierCatalog(); err != nil {
		return err
	}
	if err := s.syncPlayerRealmRequirements(); err != nil {
		return err
	}
	if err := s.seedMenus(); err != nil {
		return err
	}
	if err := s.seedHundredContent(); err != nil {
		return err
	}
	if err := s.normalizeNoticeCategories(); err != nil {
		return err
	}
	if err := s.normalizeTaskSilverRewards(); err != nil {
		return err
	}
	if err := s.normalizeDungeonAccess(); err != nil {
		return err
	}
	if err := s.normalizeUnlimitedShops(); err != nil {
		return err
	}
	if err := s.migrateGeneratedStaticCatalog(); err != nil {
		return err
	}
	if err := s.normalizeGeneratedArtifactCatalog(); err != nil {
		return err
	}
	if err := s.migrateAccountScopedReferrals(); err != nil {
		return err
	}
	if err := s.seedGiftPackCatalog(); err != nil {
		return err
	}
	if err := s.migrateLegacyCurrencies(); err != nil {
		return err
	}
	if err := s.seedExtendedGameplay(); err != nil {
		return err
	}
	if err := s.normalizeLuckSystem(); err != nil {
		return err
	}
	if err := s.normalizePlayerLevels(); err != nil {
		return err
	}
	if err := s.normalizeFarmFertilizerState(); err != nil {
		return err
	}
	if err := s.migrateCreatedSkillPublications(); err != nil {
		return err
	}
	if err := s.normalizePetEvolutionBalance(); err != nil {
		return err
	}
	if err := s.normalizeCreatedSkillBalance(); err != nil {
		return err
	}
	return s.migratePlayerFacingTerminology()
}

func (s *Store) normalizePlayerLevels() error {
	if err := s.DB.Model(&model.Player{}).Where("level < ?", 1).UpdateColumn("level", 1).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.Player{}).Where("experience < ?", 0).UpdateColumn("experience", 0).Error; err != nil {
		return err
	}
	var legacy []model.Player
	if err := s.DB.Where("level = ? AND experience = ? AND cultivation > ?", 1, 0, 0).Find(&legacy).Error; err != nil {
		return err
	}
	for _, player := range legacy {
		model.ApplyPlayerExperience(&player, player.Cultivation)
		if err := s.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"level": player.Level, "experience": player.Experience,
			"health": player.Health, "max_health": player.MaxHealth,
			"mana": player.Mana, "max_mana": player.MaxMana,
			"physical_attack": player.PhysicalAttack, "magic_attack": player.MagicAttack,
			"physical_defense": player.PhysicalDefense, "magic_defense": player.MagicDefense,
			"agility": player.Agility, "strength": player.Strength, "constitution": player.Constitution,
			"spirit": player.Spirit, "perception": player.Perception, "willpower": player.Willpower,
		}).Error; err != nil {
			return err
		}
		marker := model.PlayerValue{PlayerID: player.ID, Key: "migration.player_level_power_sync", Value: "true"}
		if err := s.DB.Where("player_id = ? AND key = ?", player.ID, marker.Key).Assign(map[string]any{"value": marker.Value, "expires_at": nil}).FirstOrCreate(&marker).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) normalizeFarmFertilizerState() error {
	return s.DB.Model(&model.MansionCrop{}).Where("fertilized IS NULL").UpdateColumn("fertilized", false).Error
}

func (s *Store) migrateCreatedSkillPublications() error {
	var skills []model.Skill
	if err := s.DB.Where("rarity = ?", "自创").Find(&skills).Error; err != nil {
		return err
	}
	for _, skill := range skills {
		var count int64
		if err := s.DB.Model(&model.SkillPublication{}).Where("skill_id = ?", skill.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		var owner struct {
			PlayerID uint
			DaoName  string
		}
		err := s.DB.Table("player_skills").
			Select("player_skills.player_id, players.dao_name").
			Joins("JOIN players ON players.id = player_skills.player_id").
			Where("player_skills.skill_id = ?", skill.ID).
			Order("player_skills.created_at, player_skills.id").
			Limit(1).
			Scan(&owner).Error
		if err != nil {
			return err
		}
		if owner.PlayerID == 0 {
			continue
		}
		publication := model.SkillPublication{
			SkillID:         skill.ID,
			CreatorPlayerID: owner.PlayerID,
			CreatorName:     owner.DaoName,
			Published:       false,
		}
		if err := s.DB.Create(&publication).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) normalizeLuckSystem() error {
	// Old releases randomized starting luck up to 20 and consumed it during
	// treasure hunts. Restore the promised baseline and enforce the new cap.
	if err := s.DB.Model(&model.Player{}).Where("luck < ?", 10).Update("luck", 10).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.Player{}).Where("luck > ?", 50).Update("luck", 50).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.Player{}).Where("gender = '' OR gender IS NULL").Update("gender", "未定").Error; err != nil {
		return err
	}
	if err := s.DB.Model(&model.Title{}).Where("code = ?", "title_lucky").Update("condition", "运气达到50").Error; err != nil {
		return err
	}

	documents := []struct {
		table  string
		column string
	}{
		{"events", "condition_json"},
		{"task_templates", "prerequisite_json"},
		{"formation_configs", "prerequisite"},
		{"talisman_configs", "prerequisite"},
		{"puppet_configs", "prerequisite"},
		{"secret_realm_conflict_configs", "prerequisite"},
		{"inheritance_configs", "prerequisite"},
		{"dao_insight_configs", "prerequisite"},
		{"immortal_demon_battlefield_configs", "prerequisite"},
		{"spiritual_root_evolution_configs", "prerequisite"},
		{"inner_demon_configs", "prerequisite"},
		{"couple_combination_skill_configs", "prerequisite"},
		{"immortal_herb_configs", "prerequisite"},
		{"artifact_refinement_configs", "prerequisite"},
		{"destiny_deduction_configs", "prerequisite"},
		{"leyline_configs", "prerequisite"},
		{"sect_war_configs", "prerequisite"},
		{"immortal_encounter_configs", "prerequisite"},
		{"star_realm_configs", "prerequisite"},
	}
	type documentRow struct {
		ID       uint
		Document string `gorm:"column:document"`
	}
	for _, source := range documents {
		var rows []documentRow
		selectClause := "id, " + source.column + " AS document"
		if err := s.DB.Table(source.table).Select(selectClause).Where(source.column+" LIKE ?", "%luck%").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			normalized, changed := normalizeLuckDocument(row.Document)
			if !changed {
				continue
			}
			if err := s.DB.Table(source.table).Where("id = ?", row.ID).Update(source.column, normalized).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeLuckDocument(raw string) (string, bool) {
	values := make(map[string]any)
	if json.Unmarshal([]byte(raw), &values) != nil {
		return raw, false
	}
	changed := false
	for _, key := range []string{"minimum_luck", "luck"} {
		value, exists := values[key]
		if !exists {
			continue
		}
		number, ok := value.(float64)
		if !ok {
			continue
		}
		if number > 50 {
			values[key] = float64(50)
			changed = true
		} else if number < 0 {
			values[key] = float64(0)
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return raw, false
	}
	return string(encoded), true
}

func (s *Store) migrateCoreItemGuidance() error {
	updates := map[string]map[string]any{
		"item_fire_grass": {
			"description": "火云洞与灵田可得的火行药材，是聚灵丹、凝元丹、金元丹及离火装备的核心药引。",
			"effect_type": "炼丹材料",
		},
		"item_thunder_crystal": {
			"description": "九霄雷域孕育的雷道晶核，可炼引劫玉符、避劫符、渡厄装备与高阶法宝。",
			"effect_type": "雷炼材料",
		},
		"item_formation_stone": {
			"description": "四象定位灵材，可布阵、护府、篆刻装备，并用于引劫玉符、避劫符与传送符。",
			"effect_type": "阵法材料",
		},
	}
	for code, fields := range updates {
		if err := s.DB.Model(&model.Item{}).Where("code = ?", code).Updates(fields).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migratePlayerFacingTerminology() error {
	fields := []struct{ table, column string }{
		{"notices", "content"}, {"world_locations", "description"}, {"task_templates", "description"}, {"items", "description"},
		{"formation_configs", "description"}, {"talisman_configs", "description"}, {"puppet_configs", "description"},
		{"secret_realm_conflict_configs", "description"}, {"inheritance_configs", "description"}, {"dao_insight_configs", "description"},
		{"immortal_demon_battlefield_configs", "description"}, {"spiritual_root_evolution_configs", "description"}, {"inner_demon_configs", "description"},
		{"couple_combination_skill_configs", "description"}, {"immortal_herb_configs", "description"}, {"artifact_refinement_configs", "description"},
		{"destiny_deduction_configs", "description"}, {"leyline_configs", "description"}, {"sect_war_configs", "description"},
		{"immortal_encounter_configs", "description"}, {"star_realm_configs", "description"},
	}
	for _, field := range fields {
		expression := "REPLACE(" + field.column + ", ?, ?)"
		if err := s.DB.Table(field.table).Where(field.column+" LIKE ?", "%后台%").Update(field.column, gorm.Expr(expression, "后台", "数据管理")).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) normalizeUnlimitedShops() error {
	return s.DB.Model(&model.ShopEntry{}).Where("purchase_limit <> ? OR refresh_cycle <> ?", 0, "永不").Updates(map[string]any{"purchase_limit": 0, "refresh_cycle": "永不"}).Error
}

func (s *Store) normalizeDungeonAccess() error {
	profiles := map[string]struct {
		Stamina int
		Limit   int
	}{
		"普通": {Stamina: 3, Limit: 20},
		"困难": {Stamina: 6, Limit: 12},
		"噩梦": {Stamina: 9, Limit: 8},
		"地狱": {Stamina: 12, Limit: 5},
	}
	for difficulty, profile := range profiles {
		if err := s.DB.Model(&model.Dungeon{}).Where("difficulty = ?", difficulty).Updates(map[string]any{
			"stamina_cost": profile.Stamina, "daily_limit": profile.Limit,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) normalizeTaskSilverRewards() error {
	var tasks []model.TaskTemplate
	if err := s.DB.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		silver := model.TaskSilverReward(task)
		if silver <= 0 {
			continue
		}
		var rewards map[string]any
		if err := json.Unmarshal([]byte(task.RewardJSON), &rewards); err != nil {
			return fmt.Errorf("任务%s奖励配置无法解析: %w", task.Name, err)
		}
		if rewards == nil {
			rewards = map[string]any{}
		}
		explicit := int64FromSeedValue(rewards["silver_coins"])
		if explicit <= 0 {
			explicit = int64FromSeedValue(rewards["silver"])
		}
		if explicit <= 0 {
			explicit = int64FromSeedValue(rewards["银币"])
		}
		if explicit > 0 {
			continue
		}
		rewards["silver_coins"] = silver
		encoded, err := json.Marshal(rewards)
		if err != nil {
			return err
		}
		if err := s.DB.Model(&model.TaskTemplate{}).Where("id = ?", task.ID).Update("reward_json", string(encoded)).Error; err != nil {
			return err
		}
	}
	return nil
}

func int64FromSeedValue(value any) int64 {
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

func (s *Store) migrateLegacyCurrencies() error {
	var values []model.PlayerValue
	if err := s.DB.Where("key = ?", "currency.jade").Find(&values).Error; err != nil {
		return err
	}
	for _, value := range values {
		amount, err := strconv.ParseInt(strings.TrimSpace(value.Value), 10, 64)
		if err != nil || amount <= 0 {
			continue
		}
		if err := s.DB.Model(&model.Player{}).Where("id = ? AND immortal_jade = 0", value.PlayerID).Update("immortal_jade", amount).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateGeneratedRealms() error {
	var generated []model.Realm
	if err := s.DB.Where("sequence > ? AND (name LIKE ? OR description LIKE ?)", 9, "天阶道境·%", "%完整境界配置%").Find(&generated).Error; err != nil {
		return err
	}
	if len(generated) == 0 {
		return nil
	}
	catalog := realmCatalog()
	for _, realm := range generated {
		if realm.Sequence < 1 || realm.Sequence > len(catalog) {
			continue
		}
		target := catalog[realm.Sequence-1]
		if err := s.DB.Model(&realm).Updates(map[string]any{
			"name": target.Name, "required_cultivation": target.RequiredCultivation,
			"attribute_multiplier": target.AttributeMultiplier, "base_health": target.BaseHealth,
			"base_mana": target.BaseMana, "base_attack": target.BaseAttack, "base_defense": target.BaseDefense,
			"base_speed": target.BaseSpeed, "base_dodge": target.BaseDodge, "base_lifespan": target.BaseLifespan,
			"tribulation_base_rate": target.TribulationBaseRate, "description": target.Description,
		}).Error; err != nil {
			return err
		}
		if err := s.DB.Model(&model.Player{}).Where("realm_id = ?", realm.ID).Update("realm_name", target.Name).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) syncPlayerRealmRequirements() error {
	var realms []model.Realm
	if err := s.DB.Order("sequence").Find(&realms).Error; err != nil {
		return err
	}
	for index, realm := range realms {
		next := model.Realm{}
		if index+1 < len(realms) {
			next = realms[index+1]
		}
		cost := realm.RequiredCultivation / 10
		if next.ID != 0 {
			cost = (next.RequiredCultivation - realm.RequiredCultivation + 9) / 10
		}
		if cost < 1 {
			cost = 1
		}
		if err := s.DB.Model(&model.Player{}).Where("realm_id = ?", realm.ID).Updates(map[string]any{
			"realm_name": realm.Name, "cultivation_required": cost,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	db, err := s.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (s *Store) RuntimeCacheDirectory() string {
	if driver := strings.ToLower(strings.TrimSpace(s.cfg.Database.Driver)); driver != "postgres" && driver != "postgresql" {
		directory := filepath.Dir(s.cfg.Database.DSN)
		if absolute, err := filepath.Abs(directory); err == nil {
			return filepath.Join(absolute, "cache")
		}
	}
	return filepath.Join(os.TempDir(), "xianchen-cache")
}

func (s *Store) Transaction(fn func(*gorm.DB) error) error { return s.DB.Transaction(fn) }
