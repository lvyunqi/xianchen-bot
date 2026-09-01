package storage

import (
	"time"

	"xianlv/internal/model"
)

func (s *Store) seedFullContent() error {
	items := []model.Item{
		{Code: "gift_starter_qingyun", Name: "青云入道礼匣", CategoryName: "礼包", RarityName: "灵品", Description: "青云接引殿赠予新入道修士的启程礼，内含修炼、疗伤、灵田、功法和两件入门装备。", EffectType: "礼包", EffectFunc: "open_gift_pack", EffectParams: `{"items":{"灵果":5,"仙露":5,"灵茶":3,"功法残卷":1,"凝露草籽":3,"灵兽口粮":5},"artifacts":["青竹练气剑","云纹护身道袍"],"spirit_stones":500,"cultivation":100,"merit":5}`, BaseValue: 1000, StackLimit: 10, Stackable: true, Tradable: false},
		{Code: "gift_paid_moonlight", Name: "月华问道礼匣", CategoryName: "礼包", RarityName: "仙品", Description: "仙金商会为长期修行准备的确认式礼包，开启后获得护劫、炼器、灵田与修炼资源；购买不会自动开启。", EffectType: "付费礼包", EffectFunc: "open_gift_pack", EffectParams: `{"items":{"灵果":20,"仙露":20,"灵茶":10,"功法残卷":3,"玄铁":20,"星辰砂":8,"避劫符":2,"龙血芝孢子":2},"spirit_stones":3000,"cultivation":500,"merit":20}`, BaseValue: 12000, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "paid_tribulation_lamp", Name: "紫府护劫天灯", CategoryName: "材料", RarityName: "仙品", Description: "使用后30分钟内下一次引劫获得12%成功率加成；无论成败，引劫后灯火都会熄灭。", EffectType: "付费护劫", EffectFunc: "tribulation_guard", EffectParams: `{"rate":0.12,"minutes":30}`, EffectValue: 12, BaseValue: 4800, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "paid_custom_root", Name: "太初灵根定制玉牒", CategoryName: "材料", RarityName: "神品", Description: "在一千种已启用灵根图鉴中指定一种本源；保留当前纯度，不允许填写图鉴外数值。发送“定制灵根 灵根名”使用。", EffectType: "灵根定制", EffectFunc: "customize_root", BaseValue: 30000, StackLimit: 10, Stackable: true, Tradable: false},
		{Code: "paid_custom_title", Name: "九霄尊号玉册", CategoryName: "材料", RarityName: "仙品", Description: "定制专属称号并首次获得攻击+20、防御+12、气血+120、法力+60；以后改名不重复叠加。发送“定制称号 新称号”使用。", EffectType: "称号定制", EffectFunc: "customize_title", BaseValue: 12000, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "paid_custom_artifact", Name: "万象器灵铭契", CategoryName: "材料", RarityName: "神品", Description: "为自己的法宝定名并首次觉醒1星，完整保留品质、等级、锻造和灵纹；以后改名不重复加星。发送“定制法宝 原名=新名”使用。", EffectType: "法宝定制", EffectFunc: "customize_artifact", BaseValue: 18000, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "paid_custom_mansion", Name: "洞天幻境地契", CategoryName: "材料", RarityName: "神品", Description: "为已建立的仙府定名，并首次获得繁荣+200、阵法/兽室/仓库各+1级；以后改名不重复增加。发送“定制仙府 新名称”使用。", EffectType: "仙府定制", EffectFunc: "customize_mansion", BaseValue: 18000, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "paid_custom_pet", Name: "山海灵兽血契", CategoryName: "材料", RarityName: "神品", Description: "为自己的灵兽定名并首次获得当前攻击、防御、体魄各10%的血契成长；进化保留且以后改名不重复叠加。发送“定制灵兽 原名=新名”使用。", EffectType: "灵兽定制", EffectFunc: "customize_pet", BaseValue: 18000, StackLimit: 20, Stackable: true, Tradable: false},
		{Code: "birthday_fortune_token", Name: "岁序福签", CategoryName: "生辰", RarityName: "仙品", Description: "仙尘生辰庆典的独立凭证，只能在本人生日当天参与生辰抽奖与专属兑换；祝福道友所得福签会保留至自己的生日。", EffectType: "生辰凭证", BaseValue: 0, StackLimit: 999999, Stackable: true, Tradable: false},
		{Code: "birthday_longevity_peach", Name: "长生蟠桃", CategoryName: "生辰", RarityName: "神品", Description: "生辰星辉温养的限定仙果，服用后获得888修为并同步角色经验。", EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: 888, BaseValue: 1888, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "birthday_wish_lantern", Name: "生辰许愿灯", CategoryName: "生辰", RarityName: "神品", Description: "点亮后两小时内闭关修炼收益提高25%，同类增益按丹药规则结算。", EffectType: "修炼", EffectFunc: "temporary_buff", EffectParams: `{"cultivation_multiplier":1.25,"minutes":120}`, BaseValue: 2888, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "birthday_blessing_pack", Name: "万福同心礼匣", CategoryName: "礼包", RarityName: "神品", Description: "寿星限定礼匣，汇聚群中道友祝愿；开启后获得生辰道具、修行材料、银币、灵石与功德。", EffectType: "生辰礼包", EffectFunc: "open_gift_pack", EffectParams: `{"items":{"长生蟠桃":1,"阵基石":2,"灵果":5,"功法残卷":1},"silver_coins":188,"spirit_stones":1888,"merit":8}`, BaseValue: 5000, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "v221_repair_memorial_token", Name: "同尘重铸纪念令", CategoryName: "任务物品", RarityName: "神品", Description: "仙尘 v2.2.1 全服修复补偿的永久纪念凭证。此令不增加战力、不可交易、不可出售，每个符合范围的账号只能领取一枚。", EffectType: "纪念", BaseValue: 0, StackLimit: 1, Stackable: false, Tradable: false},
		{Code: "v222_runtime_repair_memorial_token", Name: "万象归元纪念令", CategoryName: "任务物品", RarityName: "神品", Description: "仙尘 v2.2.2 属性、装备与运行优化全服补偿的永久纪念凭证。不增加战力、不可交易、不可出售。", EffectType: "纪念", BaseValue: 0, StackLimit: 1, Stackable: false, Tradable: false},
		{Code: "breakthrough_meridian_pill", Name: "淬脉丹", CategoryName: "丹药", RarityName: "凡品", Description: "冲击本境二至四层时稳固经脉的前置丹药，突破尝试无论成败都会消耗。", EffectType: "突破前置", EffectFunc: "breakthrough_material", BaseValue: 180, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "breakthrough_origin_pill", Name: "凝元丹", CategoryName: "丹药", RarityName: "灵品", Description: "冲击本境五至七层时凝聚真元的前置丹药，突破尝试无论成败都会消耗。", EffectType: "突破前置", EffectFunc: "breakthrough_material", BaseValue: 650, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "breakthrough_realm_pill", Name: "破境丹", CategoryName: "丹药", RarityName: "仙品", Description: "冲击本境八至十层时撼动境界壁垒的前置丹药，突破尝试无论成败都会消耗。", EffectType: "突破前置", EffectFunc: "breakthrough_material", BaseValue: 2200, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "tribulation_summon_talisman", Name: "引劫玉符", CategoryName: "材料", RarityName: "仙品", Description: "十层圆满后引动三重天劫的必要玉符，每次引劫消耗一枚。", EffectType: "渡劫前置", EffectFunc: "breakthrough_material", BaseValue: 5000, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "item_foundation_pill", Name: "筑基丹", CategoryName: "丹药", RarityName: "灵品", Description: "辅助炼气修士筑基。", EffectType: "突破", EffectFunc: "tribulation_bonus", EffectParams: `{"rate":0.10,"minutes":30}`, BaseValue: 500, StackLimit: 99, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 650},
		{Code: "item_mind_pill", Name: "清心丹", CategoryName: "丹药", RarityName: "灵品", Description: "平复心魔，临时提高道心。", EffectType: "道心", EffectFunc: "temporary_buff", EffectParams: `{"dao_heart":10,"minutes":60}`, BaseValue: 420, StackLimit: 99, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 520},
		{Code: "item_golden_pill", Name: "金元丹", CategoryName: "丹药", RarityName: "仙品", Description: "金丹修士精进修为的珍贵丹药。", EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: 150, BaseValue: 1500, StackLimit: 50, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 1800},
		{Code: "item_rebirth_pill", Name: "轮回丹", CategoryName: "丹药", RarityName: "神品", Description: "兵解转世时护持真灵。", EffectType: "转世", EffectFunc: "rebirth_guard", EffectParams: `{"bonus_percent":5}`, BaseValue: 12000, StackLimit: 10, Stackable: true},
		{Code: "item_revive_pill", Name: "九转还魂丹", CategoryName: "丹药", RarityName: "神品", Description: "濒死时恢复全部气血。", EffectType: "复活", EffectFunc: "revive", EffectValue: 100, BaseValue: 10000, StackLimit: 10, Stackable: true, Tradable: true},
		{Code: "item_double_card", Name: "双倍修为卡", CategoryName: "丹药", RarityName: "仙品", Description: "一小时内闭关收益翻倍。", EffectType: "修炼", EffectFunc: "temporary_buff", EffectParams: `{"cultivation_multiplier":2,"minutes":60}`, BaseValue: 2000, StackLimit: 20, Stackable: true, Tradable: true},
		{Code: "item_recovery_powder", Name: "回元散", CategoryName: "丹药", RarityName: "灵品", Description: "以凝露草和灵茶调和经脉，每份恢复45%最大气血，可在战斗外或回合中服用。", EffectType: "治疗比例", EffectFunc: "heal_hp", EffectParams: `{"max_health_percent":45}`, EffectValue: 45, BaseValue: 160, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_mana_recovery_pill", Name: "回灵丹", CategoryName: "丹药", RarityName: "灵品", Description: "以灵茶引导清灵药气归入丹田，每颗恢复40%最大法力，可在回合战斗中服用。", EffectType: "法力恢复比例", EffectFunc: "restore_mana", EffectParams: `{"max_mana_percent":40}`, EffectValue: 40, BaseValue: 180, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 120},
		{Code: "item_spirit_gathering_pill", Name: "聚灵丹", CategoryName: "丹药", RarityName: "灵品", Description: "赤焰草炼开灵果药性，服下一颗获得120点修为。", EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: 120, BaseValue: 260, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_spirit_grass", Name: "凝露草", CategoryName: "灵草", RarityName: "凡品", Description: "晨露滋养的基础灵草。", EffectType: "材料", BaseValue: 20, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_fire_grass", Name: "赤焰草", CategoryName: "灵草", RarityName: "灵品", Description: "火云洞与灵田可得的火行药材，是聚灵丹、凝元丹、金元丹及离火装备的核心药引。", EffectType: "炼丹材料", BaseValue: 80, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_moon_flower", Name: "月华花", CategoryName: "灵草", RarityName: "仙品", Description: "只在月华最盛时开放。", EffectType: "材料", BaseValue: 350, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_dragon_blood", Name: "龙血芝", CategoryName: "灵草", RarityName: "神品", Description: "沾染真龙精血的万年灵芝。", EffectType: "材料", BaseValue: 3000, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "item_spirit_iron", Name: "玄铁", CategoryName: "材料", RarityName: "凡品", Description: "炼制法宝的常用矿材。", EffectType: "炼器", BaseValue: 60, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_star_sand", Name: "星辰砂", CategoryName: "材料", RarityName: "灵品", Description: "坠星中提炼的银色细砂。", EffectType: "炼器", BaseValue: 260, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_thunder_crystal", Name: "雷灵晶", CategoryName: "材料", RarityName: "仙品", Description: "九霄雷域孕育的雷道晶核，可炼引劫玉符、避劫符、渡厄装备与高阶法宝。", EffectType: "雷炼材料", BaseValue: 900, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "item_beast_core", Name: "妖兽内丹", CategoryName: "材料", RarityName: "灵品", Description: "妖兽修为凝聚之物。", EffectType: "材料", BaseValue: 180, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_root_essence", Name: "灵根精粹", CategoryName: "材料", RarityName: "仙品", Description: "承载一缕灵根道纹的无属性精粹；两份精粹配合阵基石，可将两种不同灵根道纹合成为随机新灵根。", EffectType: "灵根合成", BaseValue: 1800, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "item_pet_food", Name: "灵兽口粮", CategoryName: "材料", RarityName: "凡品", Description: "灵兽喜爱的灵谷口粮。", EffectType: "灵兽", EffectFunc: "pet_loyalty", EffectValue: 10, BaseValue: 35, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 45},
		{Code: "item_formation_stone", Name: "阵基石", CategoryName: "材料", RarityName: "灵品", Description: "四象定位灵材，可布阵、护府、篆刻装备，并用于引劫玉符、避劫符与传送符。", EffectType: "阵法材料", BaseValue: 220, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "farm_fertilizer_spirit_soil", Name: "灵壤肥", CategoryName: "灵肥", RarityName: "凡品", Description: "以凝露草、灵果残渣与无主灵壤沤炼而成。每轮可为一垄灵植施用一次，使成熟提前10分钟、预计收成增加1株，并赋予10点抗灾。", EffectType: "灵田施肥", EffectFunc: "fertilize_crop", EffectParams: `{"advance_minutes":10,"yield_bonus":1,"disaster_resistance":10,"minimum_farm_level":1}`, BaseValue: 60, StackLimit: 999999, Stackable: true, Tradable: true},
		{Code: "farm_fertilizer_leyline", Name: "地脉灵肥", CategoryName: "灵肥", RarityName: "灵品", Description: "将妖丹灵机引入地脉沃壤炼成，需聚灵药畦以上田阶承载。每轮使剩余生长时间缩短25%、预计收成增加2株，并赋予25点抗灾。", EffectType: "灵田施肥", EffectFunc: "fertilize_crop", EffectParams: `{"advance_percent":25,"yield_bonus":2,"disaster_resistance":25,"minimum_farm_level":2}`, BaseValue: 280, StackLimit: 999999, Stackable: true, Tradable: true},
		{Code: "farm_fertilizer_creation", Name: "造化仙壤", CategoryName: "灵肥", RarityName: "仙品", Description: "月华、龙血芝药性与星髓灵砂共同温养的高阶仙壤，需五行药境以上田阶承载。每轮使剩余生长时间缩短40%、预计收成增加4株，并赋予45点抗灾。", EffectType: "灵田施肥", EffectFunc: "fertilize_crop", EffectParams: `{"advance_percent":40,"yield_bonus":4,"disaster_resistance":45,"minimum_farm_level":5}`, BaseValue: 1400, StackLimit: 999999, Stackable: true, Tradable: true},
		{Code: "item_tribulation_charm", Name: "避劫符", CategoryName: "材料", RarityName: "仙品", Description: "渡劫时抵消部分雷劫威能。", EffectType: "渡劫", EffectFunc: "tribulation_bonus", EffectParams: `{"rate":0.15}`, BaseValue: 2500, StackLimit: 20, Stackable: true, Tradable: true},
		{Code: "item_teleport_charm", Name: "传送符", CategoryName: "材料", RarityName: "灵品", Description: "在已记录地点间传送。", EffectType: "传送", EffectFunc: "teleport", BaseValue: 500, StackLimit: 50, Stackable: true, Tradable: true},
		{Code: "item_arena_token", Name: "竞技令", CategoryName: "材料", RarityName: "灵品", Description: "竞技场兑换凭证。", EffectType: "竞技", BaseValue: 100, StackLimit: 999, Stackable: true},
		{Code: "item_sect_token", Name: "宗门令", CategoryName: "材料", RarityName: "灵品", Description: "记录宗门贡献的令牌。", EffectType: "宗门", BaseValue: 100, StackLimit: 999, Stackable: true},
		{Code: "gem_breaking_gold", Name: "庚金破军石", CategoryName: "嵌灵宝石", RarityName: "灵品", Description: "嵌入灵孔后增强攻伐。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"attack":8}`, BaseValue: 360, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 480},
		{Code: "gem_black_tortoise", Name: "玄武镇岳石", CategoryName: "嵌灵宝石", RarityName: "灵品", Description: "嵌入灵孔后增强防御。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"defense":6}`, BaseValue: 360, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 480},
		{Code: "gem_evergreen", Name: "青木长生玉", CategoryName: "嵌灵宝石", RarityName: "灵品", Description: "嵌入灵孔后扩充气血。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"health":45}`, BaseValue: 420, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 560},
		{Code: "gem_moon_mana", Name: "太阴纳灵珠", CategoryName: "嵌灵宝石", RarityName: "灵品", Description: "嵌入灵孔后扩充法力。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"mana":35}`, BaseValue: 420, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 560},
		{Code: "gem_wind_step", Name: "流风神行晶", CategoryName: "嵌灵宝石", RarityName: "仙品", Description: "嵌入灵孔后提升身法。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"speed":7}`, BaseValue: 800, StackLimit: 999, Stackable: true, Tradable: true, StoreEnabled: true, StorePrice: 980},
		{Code: "gem_fire_soul", Name: "离火焚魂晶", CategoryName: "嵌灵宝石", RarityName: "仙品", Description: "兼顾攻击与道力的火行晶石。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"attack":6,"power":6}`, BaseValue: 920, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "gem_water_guard", Name: "玄水护魂玉", CategoryName: "嵌灵宝石", RarityName: "仙品", Description: "兼顾法力与防御的水行宝玉。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"mana":25,"defense":4}`, BaseValue: 920, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "gem_thunder", Name: "九霄雷罡石", CategoryName: "嵌灵宝石", RarityName: "神品", Description: "雷罡淬成的高阶攻速宝石。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"attack":12,"speed":5}`, BaseValue: 1800, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "gem_star", Name: "星河道力核", CategoryName: "嵌灵宝石", RarityName: "神品", Description: "星河凝成的纯粹道力核心。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"power":18}`, BaseValue: 2100, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "gem_hunyuan", Name: "混元五炁珠", CategoryName: "嵌灵宝石", RarityName: "神品", Description: "五炁相生，少量提升全部基础战斗属性。", EffectType: "装备嵌灵", EffectFunc: "equipment_gem", EffectParams: `{"attack":5,"defense":5,"health":25,"mana":20,"speed":3}`, BaseValue: 2600, StackLimit: 999, Stackable: true, Tradable: true},
	}
	for _, row := range items {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}
	customDescriptions := map[string]string{
		"paid_custom_title":    "定制专属称号并首次获得攻击+20、防御+12、气血+120、法力+60；以后改名不重复叠加。发送“定制称号 新称号”使用。",
		"paid_custom_artifact": "为自己的法宝定名并首次觉醒1星，完整保留品质、等级、锻造和灵纹；以后改名不重复加星。发送“定制法宝 原名=新名”使用。",
		"paid_custom_mansion":  "为已建立的仙府定名，并首次获得繁荣+200、阵法/兽室/仓库各+1级；以后改名不重复增加。发送“定制仙府 新名称”使用。",
		"paid_custom_pet":      "为自己的灵兽定名并首次获得当前攻击、防御、体魄各10%的血契成长；进化保留且以后改名不重复叠加。发送“定制灵兽 原名=新名”使用。",
	}
	for code, description := range customDescriptions {
		if err := s.DB.Model(&model.Item{}).Where("code = ?", code).Update("description", description).Error; err != nil {
			return err
		}
	}
	if err := s.seedSynthesisRecipes(); err != nil {
		return err
	}

	events := []model.Event{
		{Name: "山间灵泉", Type: "机缘", Description: "古松下涌出一眼灵泉，灵气清冽。", Probability: .10, RewardJSON: `{"cultivation":60}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "古修洞府", Type: "奇遇", Description: "断崖后藏着一座尘封洞府。", Probability: .04, RewardJSON: `{"cultivation":120,"items":{"功法残卷":1}}`, ConditionJSON: `{"min_realm":"筑基"}`, Enabled: true},
		{Name: "妖兽伏击", Type: "劫难", Description: "林中腥风骤起，妖兽从暗处扑来。", Probability: .12, RewardJSON: `{"on_win":{"cultivation":80,"items":{"妖兽内丹":1}},"on_lose":{"health_percent":-20}}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "仙人指路", Type: "奇遇", Description: "白衣道人点出灵脉所在，转身不见。", Probability: .01, RewardJSON: `{"immortal_affinity":20,"root_quality":1}`, ConditionJSON: `{"luck":30}`, Enabled: true},
		{Name: "坠星余烬", Type: "机缘", Description: "夜空流星坠落，山谷中星辉未散。", Probability: .05, RewardJSON: `{"items":{"星辰砂":2}}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "心魔幻境", Type: "劫难", Description: "旧日执念化作幻象，侵入识海。", Probability: .06, RewardJSON: `{"success":{"dao_heart":3},"failure":{"dao_heart":-5}}`, ConditionJSON: `{"min_realm":"金丹"}`, Enabled: true},
		{Name: "灵草山谷", Type: "机缘", Description: "雾气散开，谷中遍布凝露草。", Probability: .11, RewardJSON: `{"items":{"凝露草":3}}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "雷雨悟道", Type: "机缘", Description: "春雷滚过群峰，一缕道韵落入心间。", Probability: .07, RewardJSON: `{"perception":1,"cultivation":30}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "残碑剑意", Type: "奇遇", Description: "半截古碑上残留着冲霄剑意。", Probability: .04, RewardJSON: `{"physical_attack":1,"perception":1}`, ConditionJSON: `{"min_perception":15}`, Enabled: true},
		{Name: "商旅遇袭", Type: "机缘", Description: "你救下一队被妖兽围困的商旅。", Probability: .08, RewardJSON: `{"merit":5,"spirit_stones":80}`, ConditionJSON: `{}`, Enabled: true},
		{Name: "宗门遗迹", Type: "奇遇", Description: "荒山深处浮现一座破败山门。", Probability: .02, RewardJSON: `{"items":{"功法残卷":1,"阵基石":1}}`, ConditionJSON: `{"min_realm":"筑基"}`, Enabled: true},
		{Name: "天外传音", Type: "奇遇", Description: "识海中响起一段陌生经文，似来自天外。", Probability: .01, RewardJSON: `{"cultivation":300,"immortal_affinity":10}`, ConditionJSON: `{"min_realm":"元婴"}`, Enabled: true},
	}
	for _, row := range events {
		if err := s.DB.Where("name = ?", row.Name).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	tasks := []model.TaskTemplate{
		{Name: "每日吐纳", Type: "日常", Description: "完成一次闭关并成功出关。", PrerequisiteJSON: `{"minimum_realm_sequence":1,"minimum_realm_level":1}`, ObjectiveJSON: `{"type":"cultivation","count":1}`, RewardJSON: `{"cultivation":80,"items":{"灵果":1}}`, Weight: 100, Daily: true, Enabled: true},
		{Name: "山中采药", Type: "日常", Description: "采集灵草2次。", PrerequisiteJSON: `{"minimum_perception":10}`, ObjectiveJSON: `{"type":"collect","count":2}`, RewardJSON: `{"cultivation":70,"items":{"灵茶":2}}`, Weight: 90, Daily: true, Enabled: true},
		{Name: "清剿妖兽", Type: "日常", Description: "击败妖兽3只。", PrerequisiteJSON: `{"minimum_combat_power":80}`, ObjectiveJSON: `{"type":"hunt","count":3}`, RewardJSON: `{"cultivation":120,"merit":5}`, Weight: 90, Daily: true, Enabled: true},
		{Name: "秘境初探", Type: "日常", Description: "通关任意副本1次。", PrerequisiteJSON: `{"minimum_realm_sequence":1,"minimum_realm_level":3,"minimum_combat_power":120}`, ObjectiveJSON: `{"type":"dungeon","count":1}`, RewardJSON: `{"cultivation":150,"items":{"扫荡券":1}}`, Weight: 80, Daily: true, Enabled: true},
		{Name: "仙侣同修", Type: "日常", Description: "与仙侣完成一次双修。", PrerequisiteJSON: `{"couple_required":true,"minimum_immortal_affinity":10}`, ObjectiveJSON: `{"type":"dual_cultivation","count":1}`, RewardJSON: `{"cultivation":100,"affinity":10}`, Weight: 60, Daily: true, Enabled: true},
		{Name: "宗门巡山", Type: "宗门", Description: "完成宗门巡查。", PrerequisiteJSON: `{"sect_required":true}`, ObjectiveJSON: `{"type":"sect_patrol","count":1}`, RewardJSON: `{"contribution":20,"sect_funds":100}`, Weight: 100, Enabled: true},
		{Name: "修复阵基", Type: "宗门", Description: "上交阵基石2枚。", PrerequisiteJSON: `{"sect_required":true,"item":"阵基石","item_count":2}`, ObjectiveJSON: `{"type":"submit_item","item":"阵基石","count":2}`, RewardJSON: `{"contribution":50}`, Weight: 70, Enabled: true},
		{Name: "猎杀赤焰妖狼", Type: "悬赏", Description: "击败盘踞火云岭的赤焰妖狼。", PrerequisiteJSON: `{"minimum_realm_sequence":2,"minimum_realm_level":1,"minimum_combat_power":260}`, ObjectiveJSON: `{"type":"boss","target":"赤焰妖狼","count":1}`, RewardJSON: `{"cultivation":300,"spirit_stones":200}`, Weight: 80, Enabled: true},
		{Name: "百战修士", Type: "成就", Description: "累计赢得100场战斗。", PrerequisiteJSON: `{"minimum_realm_sequence":2,"minimum_realm_level":1,"minimum_combat_power":300}`, ObjectiveJSON: `{"type":"battle_wins","count":100}`, RewardJSON: `{"title":"百战勇士"}`, Enabled: true},
		{Name: "情比金坚", Type: "成就", Description: "仙侣道缘达到500。", PrerequisiteJSON: `{"couple_required":true,"minimum_immortal_affinity":100}`, ObjectiveJSON: `{"type":"affinity","count":500}`, RewardJSON: `{"title":"情比金坚"}`, Enabled: true},
		{Name: "万兽之友", Type: "成就", Description: "累计捕获5只灵兽。", PrerequisiteJSON: `{"minimum_realm_sequence":2,"minimum_spirit":20,"minimum_perception":15}`, ObjectiveJSON: `{"type":"pet_capture","count":5}`, RewardJSON: `{"title":"万兽之友"}`, Enabled: true},
		{Name: "飞升之路", Type: "成就", Description: "踏过千境万层，完成最终大道飞升。", PrerequisiteJSON: `{"minimum_realm_sequence":1000,"minimum_realm_level":10,"minimum_dao_heart":90,"minimum_merit":1000}`, ObjectiveJSON: `{"type":"realm","target":"飞升"}`, RewardJSON: `{"title":"飞升仙人","items":{"轮回丹":1}}`, Enabled: true},
	}
	for _, row := range tasks {
		if err := s.DB.Where("name = ?", row.Name).FirstOrCreate(&row).Error; err != nil {
			return err
		}
		if row.PrerequisiteJSON != "" {
			if err := s.DB.Model(&model.TaskTemplate{}).Where("name = ? AND (prerequisite_json = '' OR prerequisite_json = '{}' OR prerequisite_json IS NULL)", row.Name).Update("prerequisite_json", row.PrerequisiteJSON).Error; err != nil {
				return err
			}
		}
	}

	skills := []model.Skill{
		{Name: "青木长生诀", Type: "辅助", Rarity: "灵品", RealmRequired: "炼气", Description: "引草木生机滋养肉身。", EffectJSON: `{"health_per_level":12}`, UpgradeJSON: `{"mastery_per_level":100}`},
		{Name: "庚金剑典", Type: "攻击", Rarity: "仙品", RealmRequired: "筑基", Description: "庚金化剑，锋锐无匹。", EffectJSON: `{"attack_per_level":8,"crit_rate_per_level":0.01}`, UpgradeJSON: `{"mastery_per_level":130}`},
		{Name: "厚土玄功", Type: "防御", Rarity: "灵品", RealmRequired: "筑基", Description: "身如山岳，气沉丹田。", EffectJSON: `{"defense_per_level":7,"health_per_level":8}`, UpgradeJSON: `{"mastery_per_level":120}`},
		{Name: "风雷遁法", Type: "辅助", Rarity: "仙品", RealmRequired: "金丹", Description: "借风雷之势纵横天地。", EffectJSON: `{"speed_per_level":5,"dodge_per_level":0.01}`, UpgradeJSON: `{"mastery_per_level":150}`},
		{Name: "太阴炼神篇", Type: "均衡", Rarity: "仙品", RealmRequired: "元婴", Description: "以太阴月华淬炼神识。", EffectJSON: `{"spirit_per_level":6,"mana_per_level":15}`, UpgradeJSON: `{"mastery_per_level":180}`},
		{Name: "九霄御雷真诀", Type: "攻击", Rarity: "神品", RealmRequired: "化神", Description: "号令九霄神雷，代天行罚。", EffectJSON: `{"magic_attack_per_level":12}`, UpgradeJSON: `{"mastery_per_level":220}`},
		{Name: "无相护体经", Type: "防御", Rarity: "神品", RealmRequired: "合体", Description: "法相无形，万法难侵。", EffectJSON: `{"defense_per_level":12}`, UpgradeJSON: `{"mastery_per_level":250}`},
		{Name: "大衍天机术", Type: "辅助", Rarity: "神品", RealmRequired: "渡劫", Description: "推演天机，趋吉避凶。", EffectJSON: `{"luck_per_level":3}`, UpgradeJSON: `{"mastery_per_level":300}`},
	}
	for _, row := range skills {
		if err := s.DB.Where("name = ?", row.Name).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	pets := []model.PetTemplate{
		{Code: "pet_wolf", Name: "啸月狼", InitialPower: 18, GrowthPerLevel: 4, LoyaltyDecay: 2, EvolutionCondition: `{"loyalty":80,"level":8}`, EvolutionTarget: "银月狼王", Enabled: true},
		{Code: "pet_tiger", Name: "赤纹虎", InitialPower: 22, GrowthPerLevel: 5, LoyaltyDecay: 3, EvolutionCondition: `{"loyalty":85,"level":10}`, EvolutionTarget: "焚天虎王", Enabled: true},
		{Code: "pet_deer", Name: "踏云鹿", InitialPower: 14, GrowthPerLevel: 4, LoyaltyDecay: 1, EvolutionCondition: `{"loyalty":80,"level":6}`, EvolutionTarget: "九色仙鹿", Enabled: true},
		{Code: "pet_serpent", Name: "碧鳞蛇", InitialPower: 16, GrowthPerLevel: 5, LoyaltyDecay: 2, EvolutionCondition: `{"loyalty":80,"level":8}`, EvolutionTarget: "碧海蛟龙", Enabled: true},
		{Code: "pet_ape", Name: "搬山猿", InitialPower: 25, GrowthPerLevel: 4, LoyaltyDecay: 3, EvolutionCondition: `{"loyalty":90,"level":12}`, EvolutionTarget: "撼岳神猿", Enabled: true},
		{Code: "pet_butterfly", Name: "幻梦蝶", InitialPower: 12, GrowthPerLevel: 6, LoyaltyDecay: 1, EvolutionCondition: `{"loyalty":80,"level":7}`, EvolutionTarget: "轮回梦蝶", Enabled: true},
		{Code: "pet_lion", Name: "雷角狮", InitialPower: 30, GrowthPerLevel: 6, LoyaltyDecay: 3, EvolutionCondition: `{"loyalty":90,"level":15}`, EvolutionTarget: "九霄雷麟", Enabled: true},
	}
	for _, row := range pets {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}

	dungeons := []model.Dungeon{
		{Code: "dungeon_bamboo", Name: "青竹幻境", Difficulty: "普通", RecommendedPower: 80, StaminaCost: 3, RewardPoolJSON: `{"cultivation":100,"items":{"凝露草":2}}`, DailyLimit: 20, Enabled: true},
		{Code: "dungeon_fire", Name: "火云洞", Difficulty: "困难", RecommendedPower: 260, StaminaCost: 6, RewardPoolJSON: `{"cultivation":260,"items":{"赤焰草":2,"玄铁":1}}`, DailyLimit: 12, Enabled: true},
		{Code: "dungeon_moon", Name: "月华宫", Difficulty: "困难", RecommendedPower: 420, StaminaCost: 6, RewardPoolJSON: `{"cultivation":400,"items":{"月华花":1}}`, DailyLimit: 12, Enabled: true},
		{Code: "dungeon_beast", Name: "万兽山脉", Difficulty: "噩梦", RecommendedPower: 650, StaminaCost: 9, RewardPoolJSON: `{"cultivation":650,"items":{"妖兽内丹":3}}`, DailyLimit: 8, Enabled: true},
		{Code: "dungeon_void", Name: "太虚裂隙", Difficulty: "地狱", RecommendedPower: 1000, StaminaCost: 12, RewardPoolJSON: `{"cultivation":1000,"items":{"星辰砂":3}}`, DailyLimit: 5, Enabled: true},
	}
	for _, row := range dungeons {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}

	recipes := []model.AlchemyRecipe{
		{Code: "recipe_foundation", Name: "筑基丹方", MaterialsJSON: `{"凝露草":3,"灵茶":1}`, OutputName: "筑基丹", SuccessRate: .70, Description: "炼制筑基丹。", Enabled: true},
		{Code: "recipe_mind", Name: "清心丹方", MaterialsJSON: `{"灵茶":2,"月华花":1}`, OutputName: "清心丹", SuccessRate: .75, Description: "炼制清心丹。", Enabled: true},
		{Code: "recipe_golden", Name: "金元丹方", MaterialsJSON: `{"赤焰草":3,"妖兽内丹":2}`, OutputName: "金元丹", SuccessRate: .55, Description: "炼制金元丹。", Enabled: true},
		{Code: "recipe_revive", Name: "九转还魂丹方", MaterialsJSON: `{"龙血芝":1,"月华花":3,"妖兽内丹":5}`, OutputName: "九转还魂丹", SuccessRate: .25, Description: "炼制九转还魂丹。", Enabled: true},
		{Code: "recipe_rebirth", Name: "轮回丹方", MaterialsJSON: `{"龙血芝":2,"雷灵晶":3}`, OutputName: "轮回丹", SuccessRate: .18, Description: "炼制轮回丹。", Enabled: true},
		{Code: "recipe_tribulation", Name: "避劫符配方", MaterialsJSON: `{"雷灵晶":2,"阵基石":2}`, OutputName: "避劫符", SuccessRate: .45, Description: "绘制避劫符。", Enabled: true},
	}
	for _, row := range recipes {
		var item model.Item
		if s.DB.Where("name = ?", row.OutputName).First(&item).Error == nil {
			row.OutputItemID = item.ID
		}
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}

	artifacts := []model.ArtifactTemplate{
		{Code: "equipment_bamboo_sword", Name: "青竹练气剑", Type: "本命法器", MaterialsJSON: `{"玄铁":1,"凝露草":2}`, AttributeJSON: `{"attack":6,"mana":5}`, MaxLevel: 20, Enabled: true},
		{Code: "equipment_cloud_robe", Name: "云纹护身道袍", Type: "道袍", MaterialsJSON: `{"玄铁":1,"凝露草":3}`, AttributeJSON: `{"defense":4,"health":25}`, MaxLevel: 20, Enabled: true},
		{Code: "equipment_stargazer_crown", Name: "观星紫金冠", Type: "冠冕", MaterialsJSON: `{"星辰砂":3,"玄铁":2}`, AttributeJSON: `{"mana":30,"defense":5}`, MaxLevel: 30, Enabled: true},
		{Code: "equipment_dragon_bracer", Name: "伏龙镇脉腕", Type: "护腕", MaterialsJSON: `{"妖兽内丹":4,"玄铁":3}`, AttributeJSON: `{"attack":10,"health":20}`, MaxLevel: 30, Enabled: true},
		{Code: "equipment_jade_belt", Name: "青霄藏灵佩", Type: "腰佩", MaterialsJSON: `{"月华花":2,"星辰砂":2}`, AttributeJSON: `{"mana":35,"defense":4}`, MaxLevel: 30, Enabled: true},
		{Code: "equipment_wind_boots", Name: "踏云逐风履", Type: "灵靴", MaterialsJSON: `{"星辰砂":3,"赤焰草":1}`, AttributeJSON: `{"speed":12,"health":15}`, MaxLevel: 30, Enabled: true},
		{Code: "equipment_fire_ring", Name: "离火照夜戒", Type: "戒指", MaterialsJSON: `{"赤焰草":4,"雷灵晶":1}`, AttributeJSON: `{"attack":12,"mana":10}`, MaxLevel: 35, Enabled: true},
		{Code: "equipment_moon_necklace", Name: "太阴护魂链", Type: "项链", MaterialsJSON: `{"月华花":4,"妖兽内丹":2}`, AttributeJSON: `{"defense":8,"mana":25}`, MaxLevel: 35, Enabled: true},
		{Code: "equipment_tribulation_talisman", Name: "九雷渡厄符", Type: "护符", MaterialsJSON: `{"雷灵晶":3,"阵基石":2}`, AttributeJSON: `{"health":50,"defense":10}`, MaxLevel: 40, Enabled: true},
		{Code: "equipment_sixfold_array", Name: "六合镇岳阵盘", Type: "阵盘", MaterialsJSON: `{"阵基石":6,"玄铁":4}`, AttributeJSON: `{"power":18,"defense":12}`, MaxLevel: 40, Enabled: true},
		{Code: "birthday_longevity_pendant", Name: "岁序长生佩", Type: "腰佩", Slot: "腰佩", Archetype: "寿曜灵佩", Positioning: "生辰限定·护体纳灵", SetName: "岁序长生", MaterialsJSON: `{}`, AttributeJSON: `{"defense":10,"health":188,"mana":88}`, MinimumRealmSequence: 1, MinimumRealmLevel: 1, Description: "只在仙尘生辰抽奖与专属兑换中出现的限定法器。寿曜护体、福炁纳灵，属性受品质、星阶与强化规则正常成长。", SourceJSON: `{"birthday_lottery":true,"birthday_exchange":true}`, MaxLevel: 40, Enabled: true},
		{Code: "artifact_fan", Name: "流云扇", Type: "辅助", MaterialsJSON: `{"玄铁":2,"星辰砂":1}`, AttributeJSON: `{"speed":5,"mana":20}`, MaxLevel: 20, Enabled: true},
		{Code: "artifact_mirror", Name: "照心镜", Type: "辅助", MaterialsJSON: `{"玄铁":3,"月华花":1}`, AttributeJSON: `{"spirit":8,"perception":3}`, MaxLevel: 20, Enabled: true},
		{Code: "artifact_fire_sword", Name: "赤霄剑", Type: "攻击", MaterialsJSON: `{"玄铁":5,"赤焰草":3}`, AttributeJSON: `{"attack":15,"crit_rate":0.03}`, MaxLevel: 30, Enabled: true},
		{Code: "artifact_moon_robe", Name: "月华仙衣", Type: "防御", MaterialsJSON: `{"月华花":3,"星辰砂":3}`, AttributeJSON: `{"defense":12,"dodge":0.03}`, MaxLevel: 30, Enabled: true},
		{Code: "artifact_thunder_seal", Name: "九霄雷印", Type: "攻击", MaterialsJSON: `{"雷灵晶":5,"星辰砂":5}`, AttributeJSON: `{"magic_attack":25}`, MaxLevel: 40, Enabled: true},
		{Code: "artifact_void_bell", Name: "太虚钟", Type: "防御", MaterialsJSON: `{"雷灵晶":3,"阵基石":8}`, AttributeJSON: `{"defense":25}`, MaxLevel: 40, Enabled: true},
		{Code: "artifact_reincarnation", Name: "轮回盘", Type: "辅助", MaterialsJSON: `{"龙血芝":2,"雷灵晶":5}`, AttributeJSON: `{"luck":10}`, MaxLevel: 50, Enabled: true},
		{Code: "artifact_beast_pagoda", Name: "御兽塔", Type: "辅助", MaterialsJSON: `{"妖兽内丹":10,"阵基石":5}`, AttributeJSON: `{"pet_power_percent":0.15}`, MaxLevel: 30, Enabled: true},
	}
	for _, row := range artifacts {
		if err := s.firstOrCreateCodeName(&row, row.Code, row.Name); err != nil {
			return err
		}
	}

	start, end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), time.Date(2035, 12, 31, 23, 59, 59, 0, time.Local)
	activities := []model.Activity{
		{Code: "activity_weekend", Name: "周末灵潮", Type: "修炼", StartsAt: start, EndsAt: end, Effect: "每周六日修炼收益提高50%", EffectJSON: `{"weekdays":[6,0],"cultivation_multiplier":1.5}`, Status: "进行中"},
		{Code: "activity_beast", Name: "万兽试炼", Type: "战斗", StartsAt: start, EndsAt: end, Effect: "猎妖掉落提高30%", EffectJSON: `{"hunt_drop_multiplier":1.3}`, Status: "进行中"},
		{Code: "activity_tribulation", Name: "天劫赐福", Type: "渡劫", StartsAt: start, EndsAt: end, Effect: "每月初一渡劫成功率提高10%", EffectJSON: `{"tribulation_bonus":0.1}`, Status: "进行中"},
	}
	for _, row := range activities {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	mails := []model.Mail{
		{Code: "mail_welcome", Title: "欢迎踏入仙途", Content: "天地广阔，大道万千。愿你守住道心，与有缘人共证长生。", Sender: "仙尘", RewardJSON: `[{"item":"灵果","count":5},{"item":"仙露","count":3},{"currency":"灵石","count":200}]`, TargetType: "全部"},
		{Code: "mail_guide", Title: "新手修行指引", Content: "发送状态查看属性，发送修炼开始闭关，达到最低时间后发送出关结算。", Sender: "引路道人", RewardJSON: `[{"item":"功法残卷","count":1},{"item":"灵茶","count":3}]`, TargetType: "全部"},
	}
	for _, row := range mails {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	if err := s.seedShopAndOperations(); err != nil {
		return err
	}
	return s.seedDropPools()
}

func (s *Store) seedShopAndOperations() error {
	seeds := []model.Item{
		{Code: "seed_dew_grass", Name: "凝露草籽", CategoryName: "种子", RarityName: "凡品", Description: "晨露浸润的草籽，适合初阶灵田，成熟后收获凝露草。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"凝露草","grow_minutes":30,"yield":3}`, BaseValue: 15, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "seed_fire_grass", Name: "赤焰草籽", CategoryName: "种子", RarityName: "灵品", Description: "封存火行灵性的赤红草籽，成熟后收获赤焰草。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"赤焰草","grow_minutes":90,"yield":3}`, BaseValue: 60, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "seed_moon_flower", Name: "月华花种", CategoryName: "种子", RarityName: "仙品", Description: "需以月华滋养的银色花种，成熟后收获月华花。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"月华花","grow_minutes":240,"yield":2}`, BaseValue: 180, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "seed_dragon_blood", Name: "龙血芝孢子", CategoryName: "种子", RarityName: "神品", Description: "沾染真龙精血的灵芝孢子，生长缓慢但价值极高。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"龙血芝","grow_minutes":720,"yield":2}`, BaseValue: 900, StackLimit: 99, Stackable: true, Tradable: true},
		{Code: "seed_spirit_tea", Name: "云雾茶籽", CategoryName: "种子", RarityName: "凡品", Description: "云雾山茶结出的灵籽，成熟后可采得灵茶。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"灵茶","grow_minutes":45,"yield":4}`, BaseValue: 25, StackLimit: 999, Stackable: true, Tradable: true},
		{Code: "seed_spirit_fruit", Name: "聚灵果核", CategoryName: "种子", RarityName: "灵品", Description: "蕴藏聚灵果树生机的果核，成熟后结出灵果。", EffectType: "种植", EffectFunc: "plant_seed", EffectParams: `{"crop":"灵果","grow_minutes":120,"yield":3}`, BaseValue: 50, StackLimit: 999, Stackable: true, Tradable: true},
	}
	for _, seed := range seeds {
		if err := s.DB.Where("code = ?", seed.Code).FirstOrCreate(&seed).Error; err != nil {
			return err
		}
	}
	shop := []struct {
		Code, Name, Currency, Cycle string
		Price, Limit                int64
	}{
		{"shop_fruit", "灵果", "灵石", "每日", 50, 20}, {"shop_dew", "仙露", "灵石", "每日", 30, 20}, {"shop_mana_pill", "回灵丹", "灵石", "每日", 120, 20},
		{"shop_tea", "灵茶", "灵石", "每日", 80, 10}, {"shop_food", "灵兽口粮", "灵石", "每日", 45, 20},
		{"shop_material", "仙府材料", "灵石", "每周", 120, 30}, {"shop_iron", "玄铁", "灵石", "每日", 75, 50},
		{"shop_scroll", "功法残卷", "灵石", "每周", 500, 5}, {"shop_ticket", "扫荡券", "灵石", "每日", 250, 5},
		{"shop_fertilizer_spirit_soil", "灵壤肥", "灵石", "永不", 80, 0}, {"shop_fertilizer_leyline", "地脉灵肥", "灵石", "永不", 360, 0},
		{"shop_fertilizer_creation", "造化仙壤", "灵石", "永不", 1800, 0},
	}
	for index, entry := range shop {
		var item model.Item
		if s.DB.Where("name = ?", entry.Name).First(&item).Error != nil {
			continue
		}
		row := model.ShopEntry{Code: entry.Code, ItemID: item.ID, ItemName: item.Name, Currency: entry.Currency, Price: entry.Price, PurchaseLimit: 0, RefreshCycle: "永不", Sort: index + 1, Enabled: true}
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	paidShop := []struct {
		Code, Name, Cycle string
		Price, Limit      int64
	}{
		{"jade_gift_moonlight", "月华问道礼匣", "永不", 12000, 0},
		{"jade_tribulation_lamp", "紫府护劫天灯", "永不", 25600, 0},
		{"jade_custom_title", "九霄尊号玉册", "永不", 80000, 0},
		{"jade_custom_artifact", "万象器灵铭契", "永不", 60000, 0},
		{"jade_custom_mansion", "洞天幻境地契", "永不", 100000, 0},
		{"jade_custom_pet", "山海灵兽血契", "永不", 100000, 0},
		{"jade_custom_root", "太初灵根定制玉牒", "永不", 50000, 0},
	}
	for index, entry := range paidShop {
		var item model.Item
		if err := s.DB.Where("name = ?", entry.Name).First(&item).Error; err != nil {
			return err
		}
		row := model.ShopEntry{Code: entry.Code, ItemID: item.ID, ItemName: item.Name, Currency: "仙金", Price: entry.Price, PurchaseLimit: 0, RefreshCycle: "永不", Sort: 20000 + index, Enabled: true}
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
		legacyPrices := map[string][]int64{
			"jade_gift_moonlight": {60}, "jade_tribulation_lamp": {128}, "jade_custom_title": {680},
			"jade_custom_artifact": {1280}, "jade_custom_mansion": {1280}, "jade_custom_pet": {1280}, "jade_custom_root": {1980},
		}
		if prices := legacyPrices[row.Code]; len(prices) > 0 {
			if err := s.DB.Model(&model.ShopEntry{}).Where("code = ? AND price IN ?", row.Code, prices).Updates(map[string]any{"price": row.Price, "purchase_limit": int64(0), "refresh_cycle": "永不"}).Error; err != nil {
				return err
			}
		}
	}
	seedPrices := map[string]int64{"凝露草籽": 20, "赤焰草籽": 85, "月华花种": 260, "龙血芝孢子": 1500, "云雾茶籽": 35, "聚灵果核": 70}
	seedSort := 10000
	for name, price := range seedPrices {
		var item model.Item
		if err := s.DB.Where("name = ?", name).First(&item).Error; err != nil {
			return err
		}
		row := model.ShopEntry{Code: "seed_shop_" + item.Code, ItemID: item.ID, ItemName: item.Name, Currency: "灵石", Price: price, PurchaseLimit: 0, RefreshCycle: "永不", Sort: seedSort, Enabled: true}
		seedSort++
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	expires := time.Date(2030, 12, 31, 23, 59, 59, 0, time.Local)
	cdks := []model.RedemptionCode{
		{Code: "XIANLV666", RewardJSON: `[{"item":"灵果","count":10},{"currency":"灵石","count":500}]`, MaxUses: 100000, ExpiresAt: &expires, Status: "有效"},
		{Code: "DAOYOU2026", RewardJSON: `[{"item":"功法残卷","count":2},{"item":"灵茶","count":5}]`, MaxUses: 100000, ExpiresAt: &expires, Status: "有效"},
		{Code: "QINGYUAN", RewardJSON: `[{"item":"仙露","count":5},{"item":"灵兽口粮","count":10}]`, MaxUses: 100000, ExpiresAt: &expires, Status: "有效"},
	}
	for _, row := range cdks {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	published := time.Date(2026, 7, 21, 10, 0, 0, 0, time.Local)
	statusPublished := time.Date(2026, 7, 22, 15, 0, 0, 0, time.Local)
	luckPublished := time.Date(2026, 7, 22, 22, 59, 0, 0, time.Local)
	repairPublished := time.Date(2026, 7, 22, 23, 10, 0, 0, time.Local)
	latestRepairPublished := time.Date(2026, 7, 22, 23, 58, 0, 0, time.Local)
	balanceRepairPublished := time.Date(2026, 7, 23, 18, 10, 0, 0, time.Local)
	skillVitalsRepairPublished := time.Date(2026, 7, 23, 20, 40, 0, 0, time.Local)
	afkClaimRepairPublished := time.Date(2026, 7, 23, 21, 25, 0, 0, time.Local)
	notices := []model.Notice{
		{Code: "repair_afk_claim_commands_20260723", Title: "挂机领取与结束指令修复", Content: "【指令修复】修复玩家发送“领取挂机”或“结束挂机”时机器人完全不响应的问题。现在可使用“领取挂机”、“挂机结算”、“挂机收获”、“收获挂机”等常用写法领取已完成轮次；“结束挂机”会先领取成熟轮次再停止任务。\n\n【真实结算】猎妖挂机会增加修为与功德，副本挂机会增加修为、灵石与功德；同时扣除实际结算轮次所需体力。奖励、体力、队列进度和挂机统计改为同一事务写入，任何一步失败都会整体回滚，不会出现扣了体力却没奖励或重复领取。\n\n【体力保护】结束时若体力不足领完所有成熟轮次，系统只领取当前可支付部分，并保留余下任务，不会吞掉待领奖励。单独“收获”仍专属仙府灵田，避免两个系统再次冲突。\n\n【数据保护】本次为增量逻辑修复，保留玩家当前挂机任务、已经过时间、队列进度、体力、修为、货币和其他道籍数据。", Type: "修复", Pinned: true, Published: true, PublishedAt: &afkClaimRepairPublished},
		{Code: "repair_skill_vital_recovery_20260723", Title: "主修功法气血与法力恢复修复", Content: "【修复·功法气血】修复主修功法增加气血上限后，状态图显示上限已经提高，但疗伤仍按角色基础上限截断，导致功法增加的那段气血始终无法恢复的问题。仙露、运功疗伤、回元类丹药和复生效果现在统一读取当前主修功法后的真实气血上限，恢复结果会明确显示当前值与有效上限。\n\n【修复·功法法力】主修功法增加的法力上限同步接入回灵丹与其他回蓝物品，不再出现状态显示更高上限、实际法力只能回到基础值的情况。\n\n【战斗一致性】状态、状态图和战斗属性统一把数据库中的当前气血、法力视为真实当前值，功法只增加有效上限，不会在每次进入战斗时凭空重复补满。切换到较低气血或法力功法时，超出新上限的当前值会安全截断，并同步正在进行的战局，不能利用来回切换重复回血。\n\n【数据保护】本次为增量逻辑修复，不重置玩家境界、修为、已有功法、功法等级、熟练度、背包、装备、灵兽、货币或社交数据。升级后直接发送“疗伤”或使用对应丹药，即可恢复主修功法提供的额外气血与法力。", Type: "修复", Pinned: true, Published: true, PublishedAt: &skillVitalsRepairPublished},
		{Code: "repair_title_pet_skill_balance_20260723", Title: "称号实效、灵兽进化与自创功法修复", Content: "【修复·称号真实生效】称号玉册中的炼丹成功率、修炼收益与渡劫成功率不再只是展示文字。只有当前佩戴称号生效；炼药、出关与备劫结果会分别列出称号加成。丹道圣手的炼丹成功率+15%现已进入实际成丹判定，旧称号字段同样兼容。\n\n【修复·灵兽重复进化】灵兽进化改为读取万兽谱中该物种的真实忠诚和等级门槛，并在同一事务中锁定条件。一只灵兽最多完成一次进化，重复发送不会再次叠加攻击、防御、体魄或战力。\n\n【修复·异常灵兽数据】旧版漏洞造成进化次数超过1的灵兽会在升级时按原物种模板、当前等级和合法进化状态重算。尚未达到模板等级门槛的异常进化会恢复为未进化；已经达到门槛的只保留一次进化。灵兽归属、等级、经验、忠诚和出战关系不会删除。\n\n【修复·自创功法数值溢出】创功强度改为悟性、神识与创作次数的递减成长并设置安全上限。旧版异常功法只裁剪超限道效，不删除名称、作者、学习者、功法等级、熟练度或公开状态。\n\n【修复·战斗排队连点】同一战局的胜负与奖励改为原子领取，重复提交不能再次结算。普通猎妖胜利后有8秒收势期，胜利页不再直接提供重开同一妖兽的按钮；排队到达的旧挑战不会扣体力或创建新战局。修炼中发送攻击会明确提示先出关。\n\n【修复·排行榜战力】升级完成后，受上述数据影响的角色会自动重算综合战力并清理迁移标记；诸天战力榜与灵兽榜随后使用修复后的实时数值。\n\n本次采用增量迁移，不覆盖玩家数据库，不清空道籍、背包、货币、装备、功法、灵兽或社交数据。", Type: "修复", Pinned: true, Published: true, PublishedAt: &balanceRepairPublished},
		{Code: "world_notice_complete_repairs_20260722", Title: "仙尘已修复问题总公告", Content: "本公告只记录已经写入程序并通过全量回归测试的修复，不包含尚未落地的计划项。\n\n【指令与回复】\n1. 修复地图NPC对话、地图妖兽挑战、位置蓝字、接任务蓝字点击后无回复的问题；指令中出现换行或多余空白时仍按完整参数解析。\n2. 修复副本指令返回null、缺少配置时静默、数据尚未载入时没有提示的问题；现在会返回明确原因、正确格式和下一步蓝字。\n3. 修复新功能可执行却无法从菜单找到的问题；签到、活动、商城、礼包、装备、合成、排行、快捷、删号、灵脉和灵根图鉴均进入对应子菜单与完整帮助。\n4. 修复菜单、公告、成就、排行、副本、丹方、法宝等长内容一次刷满的问题；长列表按页展示，地图与角色状态保持单面板。\n\n【地图、任务与探索】\n1. 修复地图任务蓝字发送任务名、接取逻辑却只认数字编号的问题；接任务与交任务统一使用任务名。\n2. 修复任务奖励显示cultivation、merit等内部字段的问题；统一显示修为、功德、声望、灵石和真实物品名。\n3. 修复任务不按当前境界筛选、前置过高或没有前置的问题；日常与悬赏从当前可完成阶段向上排列，并显示境界、层数、战力、神识、地点及材料条件。\n4. 修复阵亡后仍能探索、采集或挑战的问题；元神离体时必须先回城复生。探索结果现在读取当前州域、地点、NPC、妖兽、采集物和当地图片，不再只发一句随机修为。\n5. 修复突破、合成与疗伤材料只写名称却找不到出处的问题；物品查询会列出地图采集、妖兽掉落、副本、灵田、商店和相关配方。\n\n【回合制战斗】\n1. 修复地图战斗、PVP和副本自动一口气打完的问题；进入战局后由玩家逐回合选择普通攻击、功法技能、防御、服药或投降。\n2. 修复疗伤后角色状态已回血、进入战斗却仍读取旧血量的问题；气血和法力在角色、战局与丹药结算之间双向同步。\n3. 修复死亡后仍可行动、复活惩罚过重的问题；复生扣除当前修为1%，且不超过本层突破需求25%，并恢复基础气血与法力。\n4. 修复灵兽捕获、灵兽空间、状态与排行榜显示三套战力的问题；统一使用同一公式，只有出战灵兽计入角色总战力。\n5. 修复装备、灵根、灵兽、境界和永久属性成长后总战力不刷新的问题；每次指令完成后统一重算派生战力。\n\n【背包、丹药与制作】\n1. 修复乾坤袋不显示货币、物品没有蓝字、无法直接查看和使用的问题；现在显示灵石、银币、仙金、竞技币，并提供物品详情、使用、药效和来源蓝字。\n2. 修复丹药只显示名称、不说明吃了加什么的问题；丹方、背包、物品详情和炼药结算均显示单颗药效与本炉总药力。\n3. 修复聚灵丹投入材料后产出灵果的问题；现在正确产出聚灵丹并增加120修为，配方已补入赤焰草。\n4. 修复只有回血药、没有回蓝药的问题；回元散恢复45%最大气血，回灵丹恢复40%最大法力，两者均支持战斗回合使用。\n5. 修复赤焰草、阵基石、雷灵晶缺少用途和来源的问题；三类材料已接入丹药、阵法、渡劫、装备与合成链。\n6. 修复合成列表只给编号、批量合成无真实成功判定的问题；配方按名称操作，每份独立判定并显示材料来源、前置、产物及成功失败数量。\n\n【修炼、突破与副本】\n1. 修复境界可以越层直升和所有境界要求相同的问题；每个大境必须从一层逐层修至十层，十层圆满后才能进入下一境。\n2. 修复突破只扣少量修为、没有状态和道具前置的问题；突破会检查修为、道心、气血、法力和对应破境丹药，并进行真实成功/失败判定。\n3. 修复渡劫只有一句成功结果的问题；现有备劫清单、引劫玉符、三道劫关、逐关概率、失败反噬和仙侣共渡。\n4. 修复低收益副本次数太少、体力不够使用的问题；普通、困难、噩梦、地狱分别调整为每日20、12、8、5次，体力消耗分别为3、6、9、12。\n5. 修复挂机和批量物品不能指定数量的问题；支持“目标*次数”和“物品名*数量”，不设玩法数量上限，但受实际资源、耗时和安全整数范围约束。\n\n【交易、账号与管理】\n1. 修复购买玩家摊品必须输入长ID、看不到摊主的问题；集市只显示全服唯一道号，可按物品名买下最低有效报价。\n2. 修复商城、种子商店、银币商城、仙金商城和竞技商店入口缺失或限购的问题；商店按名称购买并支持批量，常设商品不限购。\n3. 修复删号后道号不释放、违规道号审核过松的问题；确认删号会永久清理关联动态并释放唯一道号，审核会拦截辱骂、广告、控制符、乱码与不可读生僻字堆砌。\n4. 修复快捷指令只能设置、不能查看的问题；快捷列表仅本人可见，可点击执行和逐条删除，且不能覆盖系统或管理指令。\n\n【灵根、运气与仙侣】\n1. 修复灵根检测显示第七阶段、觉醒却读取第一阶段的问题；检测、进化、觉醒和前置统一读取同一份玩家进度。\n2. 新增并修复灵根合成完整链路：两种不同父系随机凝成第三种非父系灵根；相同父系不扣材料，已有待定道种不会被覆盖，可放弃或确认吸收，吸收后保留境界、功法、装备和永久成长并重算战力。\n3. 新增运气10至50体系；旧版被寻宝或抽签扣低的角色恢复至10，超过50和不可达前置压至50。仙缘奇遇可概率永久加1，运气真实提高奇遇、寻宝、遇仙、捕获、炼丹、普通合成和稀有灵根合成概率。\n4. 修复角色没有性别、仙侣和双修资料不完整的问题；入道或“性别 男/女”可登记，状态、档案、寻缘、结缘、心意和双修统一显示，性别组合不限制结缘。\n5. 修复传功1点时七成整数截断为0的问题；正数小额传功至少实得1点，结果明确显示传出、实得和损耗。\n\n以上条目均已进入本版本；后续每次确认修复的新问题仍会继续写入独立修复公告。", Type: "更新", Pinned: true, Published: true, PublishedAt: &repairPublished},
		{Code: "world_notice_transfer_feedback_pet_20260722", Title: "灵根传承、灵兽生态与反馈台修复公告", Content: "【修复·状态图】玩家头像现已严格裁入底图头像框内圈，道号牌与称号沿同一人物中心线排布；头像不会再向右下漂移，也不会覆盖金色框体。\n\n【修复·灵根传承】“灵传 @对方”不再依赖灵根进化配置。系统直接读取传承者当前真实灵根、纯度与本源道阶，消耗灵根精粹×1和灵石×300，为目标生成待确认传承道种；传承者不会失去原灵根。目标已有道种时拒绝覆盖且零扣费，吸收与放弃均有独立文案、属性重算与全区通报。\n\n【新增·仙盟反馈台】发送“反馈菜单”可提交BUG或玩法建议。系统自动检查复现信息、异常现象、期望结果、建议做法、重复内容、数值公平与权限风险；有效内容进入内容与玩家反馈审核队列并按每日次数发放银币、灵石奖励。“我的反馈 页码”可查看初审、人工状态和奖励记录。\n\n【修复·灵兽捕获】捕获不再凭空连续随机。玩家必须先在当前地图探索到十分钟灵兽遭遇，消耗灵兽口粮×1和体力5尝试一次；成功、失败或离开地图都会清除该遭遇。系统新增灵兽空间容量与御兽印冷却，所有扣除和结果在同一事务结算。\n\n【新增·灵兽照料事件】忠诚会按未照料天数真实衰减，并依次触发思食、焦躁、拒绝出战、反噬与叛变。低忠诚灵兽会移出战位并同步扣除角色战力，忠诚归零会离去并自动全区通报；喂养灵兽口粮可恢复忠诚并重置照料周期。\n\n【新增·仙尘介绍】系统菜单新增“仙尘介绍”“游戏介绍”“世界观”，完整说明千境十层、山河地图、回合斗法、灵根万法、灵兽因果、仙侣宗门、经济货币和新手五步路线。", Type: "更新", Pinned: true, Published: true, PublishedAt: &latestRepairPublished},
		{Code: "world_notice_luck_gender", Title: "天缘气数与仙侣道籍更新", Content: "【新增】所有新角色固定拥有10点运气，上限50；成功承接仙缘奇遇时可概率永久增加1点。运气会真实提高奇遇触发与抉择、寻宝、遇仙、灵兽捕获、炼丹、材料合成及稀有灵根合成概率，所有结算均显示基础、运气加成与最终概率。\n【新增】入道支持填写男修或女修，旧角色可发送“性别 男/女”补录；状态、修仙档案、寻缘、结缘、心意与双修统一显示角色性别，性别组合不限制结缘资格。\n【优化】运气不再被寻宝或抽签消耗；旧玩家低于10点的历史气运恢复至10，超过50及所有不可达前置统一压至50。\n【修复】传功1点时对方不再因整数截断实得0；丹药在乾坤袋中同时显示使用蓝字、真实药效与用途。", Type: "更新", Pinned: true, Published: true, PublishedAt: &luckPublished},
		{Code: "world_notice_status_gifts", Title: "仙尘属性图与千礼道藏更新", Content: "【新增】状态指令可切换单张属性图模式，自动读取发令道友头像，并完整展示境界、层数、修为、战力、气血、法力、攻防、身法、道心、神识、仙侣、宗门、灵兽、货币与位置。\n【新增】千类修仙礼包已录入天机阁，每类礼包均有独立名称、专属丹药、独有器胚、奖励数值和银币或仙金获取途径。\n【优化】乾坤袋与背包搜索中的物品名称改为可点击蓝字，礼包可直接开启，帮助与全部指令按分类完整列出。\n【修复】属性成长后战力会按实时数值统一校准；境界升降、等级变化与受伤后的气血会立即反映在状态图；灵根传承缺少开放道统时会自动恢复基础道藏。", Type: "更新", Pinned: true, Published: true, PublishedAt: &statusPublished},
		{Code: "notice_welcome", Title: "欢迎来到仙尘", Content: "所有游戏指令均无需前缀。首次进入请发送：入道 道号。", Type: "系统", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "notice_cultivation", Title: "闭关修炼规则", Content: "发送修炼开始计时，达到最低闭关时间后发送出关结算。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "notice_message", Title: "消息双通道已启用", Content: "群聊与好友私信优先使用QQ开放平台原生Markdown；发送失败时自动回退Bee普通消息，避免指令静默。", Type: "更新", Published: true, PublishedAt: &published},
		{Code: "notice_update_rankings", Title: "诸天万榜与全区通报更新", Content: "【新增】综合、境界、战力、修为、财富、功德、声望、道心、仙缘、灵根、灵兽、装备、宗门、道缘、副本、竞技、灵田、首领、成就、活跃、炼丹、锻造、渡劫共二十三类排行榜；各榜前十每日可领取灵石与功德俸禄；重大天赐、劫罚、仙缘、破境、首领和排行事件会自动全区通报。\n【优化】挂机支持“目标*次数”排队，物品支持“物品名*数量”批量使用，均无游戏数量上限但受实际资源与时间约束。\n【修复】修复玩法结果参数错误、任务种子编译错误、加载未完成时静默无提示的问题。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_beginner", Title: "青云接引殿开坛", Content: "新道友发送“入道 道号”建立全服唯一道籍，随后开启青云入道礼匣。发送“帮助”会按当前阶段给出下一步，不需要记忆指令编号。", Type: "系统", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_roots", Title: "千种灵根图鉴开放", Content: "修仙界已记录一千种独立灵根。每种灵根拥有不同修炼倍率、五维基点、稀有权重和道相加成，注册按真实权重随机抽取。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_leylines", Title: "地脉潮汐贯通诸界", Content: "一千条修仙界灵脉已经苏醒，仅分布在部分地图。抵达后发送“寻脉”，满足境界、层数、战力、神识、灵根和材料前置即可入脉打坐，收益约为普通修炼二至十二倍。", Type: "活动", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_arena", Title: "问剑竞技赛季开启", Content: "竞技改为逐回合选择攻击、功法、防御或投降。段位积分只用于排位，竞技币独立用于商店；每日可发送“竞技奖励”领取段位俸禄。", Type: "活动", Published: true, PublishedAt: &published},
		{Code: "world_notice_silver", Title: "银币钱庄开始流通", Content: "银币可通过每日签到、竞技俸禄和运营活动免费获得，用于银币商城；银币与灵石、仙金、竞技币互不混扣。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_jade", Title: "仙金商会入驻", Content: "仙金属于充值货币，仅由主人或具备高阶权限的管理员通过统一充值神令入账。仙金商城独立定价，所有变动写入审计。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_customization", Title: "太初万象定制开放", Content: "仙金商城现已上架月华问道礼匣、紫府护劫天灯及五类定制玉牒。灵根可从千种图鉴指定，称号、仙府、灵兽与法宝可安全定名；所有自定义文本先经过敏感词审核，凭证仅在定制成功后扣除。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_checkin", Title: "七日道印签到", Content: "每日发送“签到”可领取银币与当天物品，连续七日循环。签到银币与每日灵物以仙盟当期签到道印为准。", Type: "活动", Published: true, PublishedAt: &published},
		{Code: "world_notice_boss", Title: "镇域妖王悬赏", Content: "各地图镇域首领拥有独立战力、狂暴逻辑、刷新时间和奖励。成功讨伐会触发官机全区通报并进入首领排行榜。", Type: "活动", Published: true, PublishedAt: &published},
		{Code: "world_notice_tribulation", Title: "九霄劫云示警", Content: "每个大境界必须逐层修满十层才可引劫。天劫包含雷劫、业火、心魔、因果、岁月、虚空与混沌等劫程，失败会受到真实修为和气血惩罚。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_couple", Title: "三生石重开", Content: "寻缘、结缘、双修、护法、合击与合体技均需要真实仙侣关系和道缘前置。结缘与道号传承等重大因果会自动全区通报。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_farm", Title: "仙府灵田春令", Content: "种子商店、指定种植、一键播种、浇水、除草、除虫、成熟收菜、灵植出售、道友采撷与护田灵兽已经贯通。木系灵根享有独立产量收益。", Type: "活动", Published: true, PublishedAt: &published},
		{Code: "world_notice_maps", Title: "诸界地图完成勘定", Content: "地图地点拥有独立NPC、任务、妖兽、首领、采集物、传送条件、相邻路线和图片。移动必须遵循通路，不可跨过未解锁区域。", Type: "更新", Published: true, PublishedAt: &published},
		{Code: "world_notice_rank", Title: "诸天万榜每日结算", Content: "二十三类排行榜均隐藏账号ID，只显示道号。各榜前十每日可领取一次灵石与功德俸禄，重复领取会被事务锁阻止。", Type: "活动", Published: true, PublishedAt: &published},
		{Code: "world_notice_shortcut", Title: "专属快捷令开放", Content: "发送“设置快捷 别名=完整指令”可建立个人专属短指令；快捷仅本人可用，不能覆盖系统指令，也不能映射到管理神令。", Type: "更新", Published: true, PublishedAt: &published},
		{Code: "world_notice_shortcut_list", Title: "专属快捷令簿开放", Content: "功能菜单已增加系统入口。发送“快捷列表”或“快捷”可查看本人全部专属指令，列表按别名排序，并提供立即执行与逐条删除蓝字；任何玩家都无法查看他人的快捷令。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_unlimited_shops", Title: "仙盟诸市取消限购", Content: "仙门货铺、种子商店、银币商城、仙金商城与竞技商店现已全部改为常设不限购。玩家可按实际余额购买任意正整数数量，系统仅在总价超过安全数值范围时要求拆分操作。", Type: "更新", Pinned: true, Published: true, PublishedAt: &published},
		{Code: "world_notice_review", Title: "仙盟内容审核令", Content: "道号、传音、密语、日记和留言启用自动敏感词审核。词库覆盖广告、诈骗、赌博、低俗、违禁、隐私与辱骂变体，命中内容会阻止发送并记录。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_market", Title: "云海集市交易公约", Content: "玩家集市只显示摊主道号，不公开QQ号或内部ID。发送“买下 物品名”会原子选择最低有效报价，避免重复成交。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_gifts", Title: "仙途礼包规则", Content: "礼包先进入乾坤袋，再由玩家确认开启。仙品、神品礼包与月卡激活可触发荣耀通报，普通新手礼包不会刷屏。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_equipment", Title: "百炼神兵谱更新", Content: "装备拥有槽位、品质、等级、锻造和灵纹。仙品或神品法宝出炉、装备升品会自动全区通报；同槽替换会先移除旧加成。", Type: "更新", Published: true, PublishedAt: &published},
		{Code: "world_notice_maintenance", Title: "仙尘维护约定", Content: "玩法数据保存后立即生效。结构升级会保留玩家道籍、授权、主人和管理员设置；若初始化尚未完成，官机会明确回复“数据正在加载中”。", Type: "系统", Published: true, PublishedAt: &published},
		{Code: "world_notice_feedback", Title: "天机阁异常收录", Content: "遇到无回复、奖励异常、地图断路或数据保存问题时，请主人保留时间、群号、道号、指令和运行日志，以便准确追踪修复。", Type: "系统", Published: true, PublishedAt: &published},
	}
	for _, row := range notices {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	// 已确认修复使用独立类型，避免混入世界公告和版本更新公告。
	if err := s.DB.Model(&model.Notice{}).Where("code IN ?", []string{
		"world_notice_complete_repairs_20260722",
		"world_notice_transfer_feedback_pet_20260722",
	}).Update("type", "修复").Error; err != nil {
		return err
	}
	currentRepairPublished := time.Date(2026, 7, 23, 2, 10, 0, 0, time.Local)
	currentRepair := model.Notice{
		Code:  "world_notice_stamina_leyline_interaction_repairs_20260723",
		Title: "体力、灵脉与交互修复公告",
		Content: "【体力】体力上限改为炼气期基础100，每提升一个大境界永久增加100；换日恢复、自然恢复、扣除、系统、状态文字、状态图和副本说明统一读取同一动态上限。新增“体力”查询，列明境界加成、恢复速度与常见消耗。\n\n" +
			"【灵脉】灵脉打坐会真实校验最低大境界与层数，并同时显示当前境界、所需境界、战力、神识、本源和护脉材料。修复“灵根详情 庚金本源”被误判未收录的问题；灵根图鉴和灵脉地图均可按本源筛选，前置失败会给出兼容灵根、当前灵根适配灵脉及材料来源入口。\n\n" +
			"【反馈】修复自然语言灵脉BUG只有25分的问题；打坐、采灵气、不契合、无法和修复等表达可以被正确识别。近似重复的已受理反馈不再重复发奖，已经拒绝的记录允许补充后重新提交。\n\n" +
			"【日记与留言】单独发送“日记”改为查看并分页，带正文才写入；单独发送“留言”查看收到的留言，换行输入的目标和正文可正常发送。\n\n" +
			"【通知】新增个人通知信箱、未读筛选和已读清理；留言、密语、仙缘请求、护法、道号传承、问剑结果与个人事件统一汇集，查看普通通知不会误处理待确认请求。\n\n" +
			"【竞技】问剑段位扩展为一千个独立修仙段位，每段拥有唯一名称、晋阶积分、竞技币俸禄、银币俸禄和道意说明；段位图鉴按页查看。\n\n" +
			"【公告与介绍】修复公告成为独立子菜单；世界公告、更新公告、修复公告和全区通报互不串流。仙尘介绍、游戏介绍、世界观与大世界已拆成不同正文。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &currentRepairPublished,
	}
	if err := s.DB.Where("code = ?", currentRepair.Code).FirstOrCreate(&currentRepair).Error; err != nil {
		return err
	}
	staminaRecoveryPublished := time.Date(2026, 7, 23, 8, 45, 0, 0, time.Local)
	staminaRecoveryRepair := model.Notice{
		Code:    "repair_stamina_recovery_scaling_20260723",
		Title:   "体力周天自动恢复修复公告",
		Content: "【修复】体力上限随大境成长后，自然恢复速度不再固定为每分钟1点。炼气期基础改为每分钟自动恢复10点，每提升一个大境界再增加10点恢复速度，且不设置恢复速度上限。\n\n【结算】体力无需打坐恢复；角色在线或离线时都会按真实经过时间补回，恢复至当前体力上限后停止。由于上限每个大境增加100、恢复每个大境增加10，各境界从零回满均约需10分钟。\n\n【数据】本次只调整恢复公式和系统参数，不会重置玩家已有体力、境界、背包、货币或其他角色数据。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &staminaRecoveryPublished,
	}
	if err := s.DB.Where("code = ?", staminaRecoveryRepair.Code).FirstOrCreate(&staminaRecoveryRepair).Error; err != nil {
		return err
	}
	mapTaskRepairPublished := time.Date(2026, 7, 23, 8, 50, 0, 0, time.Local)
	mapTaskRepair := model.Notice{
		Code:    "repair_map_resource_and_task_progress_20260723",
		Title:   "地图采集与任务状态修复公告",
		Content: "【地图采集】修复地图显示多株灵植、采集一株后却整片立即冷却的问题。地图资源数量现在代表真实可采株数，每次采集一株并显示剩余数量；全部采尽后才进入地图配置的刷新倒计时，物品入包、库存扣减和刷新时间在同一事务完成。\n\n【任务进度】修复新接任务继承接取前历史击杀、探索或采集次数的问题。需要行动的任务会保存接取时统计基线，只计算接取后的真实进度，显示值不会超过目标数量。\n\n【任务状态】修复同日已完成任务再次点击时错误回复“任务已接取”，随后交付又称没有进行中任务的问题。进行中的任务会提示继续完成，已完成任务会明确显示今日已经领取，且不会重复发奖。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &mapTaskRepairPublished,
	}
	if err := s.DB.Where("code = ?", mapTaskRepair.Code).FirstOrCreate(&mapTaskRepair).Error; err != nil {
		return err
	}
	skillPetRepairPublished := time.Date(2026, 7, 23, 9, 0, 0, 0, time.Local)
	skillPetRepair := model.Notice{
		Code:    "repair_skill_creation_and_pet_experience_20260723",
		Title:   "创功与灵兽灵悟修复公告",
		Content: "【创功】修复不填写名称时反复使用“道号+心经”、再次创功触发功法名唯一约束并只返回天机紊乱的问题。无参数创功会从修仙功名谱中选择下一个全服唯一且不带数字的名称；手动重名、名称违规、悟性不足和推演失败均返回明确原因，功法与玩家归属在同一事务写入。\n\n【灵兽经验】灵兽经验不再只是累积数字。灵兽空间、捕获、出战、喂养与进化结果会显示十格灵悟进度条、百分比和当前经验；一级需要100灵悟，之后每级需求增加50。\n\n【真实升级】灵悟满值会真实提升等级并扣除本级经验，允许一次获得的经验连续跨级。每次升级按照该灵兽模板独立的成长值增加攻击、防御与体魄；出战灵兽升级后会立即同步角色总战力。旧灵兽已经积累的经验会在查看灵兽空间时自动结算，不会丢失。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &skillPetRepairPublished,
	}
	if err := s.DB.Where("code = ?", skillPetRepair.Code).FirstOrCreate(&skillPetRepair).Error; err != nil {
		return err
	}
	skillEffectSharingPublished := time.Date(2026, 7, 23, 14, 20, 0, 0, time.Local)
	skillEffectSharingRepair := model.Notice{
		Code:  "repair_created_skill_effect_and_sharing_20260723",
		Title: "自创功法实效与分享修复公告",
		Content: "【真实生效】修复自创功法只显示独立道效、角色面板与技能伤害却没有变化的问题。当前主修功法的物攻、法强、双防、气血、法力、身法、暴击、闪避和减伤会进入状态、状态图、总战力、地图战斗、PVP与技能伤害的统一计算；切换功法会显示新旧道效和战力贡献差值。\n\n" +
			"【六种流派】自创功法不再全部是同一种均衡模板。剑道侧重物攻、身法与暴击；术法侧重法强、法力与暴击；炼体侧重双防、气血与减伤；神魂侧重攻法、法力与护神；遁法侧重身法、闪避与迅捷攻击；均衡兼顾攻守。可发送“创功 流派 功法名”主动选择。\n\n" +
			"【创作难度与运气】首部功法开始需要悟性、道心、修为、功法残卷和灵茶；已经成功创作的数量越多，下一部所需悟性、修为和材料越高。失败只消耗部分推演资源，不生成残缺功法。运气10至50会明确转化为创功成功率加成，并小幅提高成篇后的道效强度。\n\n" +
			"【作者自主分享】创作成功后默认私藏，不会自动出现在普通功法图鉴、万象查询或其他玩家的学习入口。原作者可发送“上传功法 功法名”公开到功法分享阁，也可发送“撤下功法 功法名”停止后续传播；已经学会的玩家保留原有功法与等级。\n\n" +
			"【旧数据与保护】旧版自创功法按最早掌握记录补认原作者并保持私藏，等待作者自行决定是否公开。本次升级只新增功法发布记录并增量迁移，不重置玩家境界、修为、背包、装备、功法等级或其他道籍数据。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &skillEffectSharingPublished,
	}
	if err := s.DB.Where("code = ?", skillEffectSharingRepair.Code).FirstOrCreate(&skillEffectSharingRepair).Error; err != nil {
		return err
	}
	farmFertilizerPublished := time.Date(2026, 7, 23, 15, 45, 0, 0, time.Local)
	farmFertilizerRepair := model.Notice{
		Code:    "repair_farm_fertilizer_null_state_20260723",
		Title:   "灵田施肥状态一致性修复公告",
		Content: "【修复现象】修复“我的灵田”显示作物待施灵肥，一键施肥却回复没有可施肥田垄的问题。该问题来自旧田垄新增施肥字段后保留的空值：页面读取时将空值显示为未施肥，执行筛选却只识别明确的未施肥状态。\n\n【统一判定】单块施肥、一键施肥、事务二次校验和灵田展示现在统一把旧空值视为未施肥。页面显示“待施灵肥”且作物仍在生长时，对应田垄一定会进入施肥候选；已经施肥或已经成熟的田垄仍不会重复消耗灵肥。\n\n【旧数据迁移】升级时会把旧田垄的空施肥状态增量归一为未施肥，不修改作物名称、数量、成熟时间、浇水、除草、除虫、守护或灾害状态，也不会重置仙府、背包和玩家其他数据。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &farmFertilizerPublished,
	}
	if err := s.DB.Where("code = ?", farmFertilizerRepair.Code).FirstOrCreate(&farmFertilizerRepair).Error; err != nil {
		return err
	}
	npcTeleportPublished := time.Date(2026, 7, 23, 15, 55, 0, 0, time.Local)
	npcTeleportRepair := model.Notice{
		Code:    "repair_npc_shop_and_teleport_list_routes_20260723",
		Title:   "人物仙商与传送阵图修复公告",
		Content: "【人物仙商】修复发送“NPC商店”直接返回天机紊乱的问题。人物商店、赠送、购买和关系四组指令已经接入真实业务路由；裸发NPC商店会列出当前位置人物，选择后显示其独立货物、好感门槛与灵石价格。购买会在同一事务扣除灵石并发放物品，赠礼会扣除真实背包物品并增加人物好感。\n\n【人物关系】与当地NPC首次对话会建立人物记录并增加初见好感，对话页可直接进入商店、赠礼和关系页面。高好感逐步解锁后续货物；不在当前位置的人物不能隔空交易。\n\n【传送列表】修复“传送列表”和“诸界列表”显示完全相同内容的问题。裸发“传送列表”现在默认打开当前界域的阵点列表，显示已刻录、未刻录、境界锁定和可接引界门；“诸界列表”与“世界列表”只负责显示十界开放状态。\n\n【漏接路由】同时补齐灵田天象、护持灵田和灵田灾异录的业务入口，不再因菜单已有但服务未路由而返回天机紊乱。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &npcTeleportPublished,
	}
	if err := s.DB.Where("code = ?", npcTeleportRepair.Code).FirstOrCreate(&npcTeleportRepair).Error; err != nil {
		return err
	}
	playerLevelPublished := time.Date(2026, 7, 23, 16, 20, 0, 0, time.Local)
	playerLevelRepair := model.Notice{
		Code:  "repair_player_level_and_silver_job_20260723",
		Title: "角色等级与银币差事修复公告",
		Content: "【角色等级】修复玩家无论获得多少修为，角色等级和经验始终停在一级的问题。任务、战斗、挂机、修炼、丹药、差事、双修、论道、助力及天赐修为等系统修行奖励，会按修为净增数额同步获得等量角色经验；经验满足需求后可连续跨级并保留结余。玩家间传功只转移已有修为，不重复生成经验。\n\n" +
			"【升级成长】角色升级会真实增加气血、法力、物攻、法强、双防与阶段性五维，总战力在结算后立即重算。状态文字、状态图、系统总览和独立“等级”页面会显示角色等级、当前经验与下一等级进度；角色等级与突破境界仍是两条独立成长线。\n\n" +
			"【旧玩家补偿】升级时会将旧道籍当前已有修为一次性折算为角色经验，并按相同规则补发对应等级和属性成长。迁移只更新等级、经验及其应得成长，不清空或覆盖境界、修为、背包、货币、装备、功法、任务、地图、灵田或社交数据。\n\n" +
			"【银币差事】“银币来源”继续保留所有免费渠道说明；“赚银币”不再重复打开说明页，而是承接一趟真实仙盟差事。差事消耗少量体力、十分钟刷新，银币与少量修为随角色等级、当前大境和层数缓慢成长；体力不足、仍在冷却或数值达到安全上限时均不扣体力、不提前写入冷却。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &playerLevelPublished,
	}
	if err := s.DB.Where("code = ?", playerLevelRepair.Code).FirstOrCreate(&playerLevelRepair).Error; err != nil {
		return err
	}
	runtimeBarterRepairPublished := time.Date(2026, 7, 23, 10, 30, 0, 0, time.Local)
	runtimeBarterRepair := model.Notice{
		Code:    "repair_runtime_status_and_barter_consent_20260723",
		Title:   "运行状态与易物确认修复公告",
		Content: "【新增·运行状态】系统菜单新增“运行状态”和“插件状态”。页面会显示真实运行结果、插件载入、指令系统、框架名称、插件名称、插件版本、数据库迁移版本、启动时间、持续运行时间、数据库连接池、消息模式、状态图模式及核心数据载入数量；数据库或数据表检查失败时会明确标为异常，不再写死正常。\n\n【版本】仙尘插件版本由1.0.0升级为2.0.0。插件运行页、DLL元数据、数据包名称和安装说明统一读取同一版本源，后续升级不会再出现多个页面版本号不一致。\n\n【修复·玩家易物】“易物”不再由发起方单方面直接交换。发起时只建立十分钟待确认申请，不扣除双方物品；收件人可在通知信箱或“易物请求”中确认、拒绝，发起人可撤回。\n\n【交易安全】确认成交时会重新校验双方库存、数量、绑定状态和交易资格，再在同一数据库事务内一次性交换；任一步失败会整体回滚。重复确认、过期申请和已经处理的申请不会再次扣物或重复成交。\n\n【数据保护】本次升级仅新增易物申请记录表和运行状态入口，不重置玩家境界、背包、装备、货币、任务、仙侣、宗门或其他既有数据。",
		Type:    "修复", Pinned: true, Published: true, PublishedAt: &runtimeBarterRepairPublished,
	}
	if err := s.DB.Where("code = ?", runtimeBarterRepair.Code).FirstOrCreate(&runtimeBarterRepair).Error; err != nil {
		return err
	}
	leylineSilverRepairPublished := time.Date(2026, 7, 23, 11, 40, 0, 0, time.Local)
	leylineSilverRepair := model.Notice{
		Code:  "repair_leyline_silver_notice_alchemy_20260723",
		Title: "灵脉本源、银币与批量炼丹修复公告",
		Content: "【灵脉本源】修复千种灵根与千条世界灵脉使用两套本源表的问题。风灵、雷灵、冰魄、时空与轮回均有真实契合灵脉；十类本源在炼气期起始灵脉枢纽各有一条低阶灵脉，后续随地图和境界逐级开放。旧版系统灵脉会原地迁移，玩家道籍、已探明记录、物品与进度不重置。\n\n" +
			"【寻脉路线】“灵脉地图 本源”显示全界结果、最低境界、战力、距离和下一站；本地没有灵脉时发送“寻脉”不会扣法力，而会列出最近三条契合灵脉及逐站路线。远方灵脉不再提供无法直接跨越地图的错误前往指令。\n\n" +
			"【银币来源】普通任务、日常、悬赏、地图任务、主线、支线与宗门委托新增随前置境界和目标数量成长的真实银币奖励。接取页面、任务图鉴、完成结算与角色余额读取同一数值；新增“银币来源”查看签到、当前任务、竞技俸禄、活动、邀请、助力和反馈奖励。\n\n" +
			"【批量炼丹】“炼药 丹方名*数量”与“批炼 丹方名*数量”统一支持任意正整数数量，不设一百炉上限。每份丹方独立判定，材料总量先做安全校验，再在同一事务扣除并发放成丹；例如可直接发送“炼药 回灵丹*99”。\n\n" +
			"【公告分页】世界公告、更新公告、修复公告和全区通报各自使用稳定页码。单篇长公告会拆成连续正文页，发送原公告指令加页码继续查看，不占用其他功能的临时翻页记录。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &leylineSilverRepairPublished,
	}
	if err := s.DB.Where("code = ?", leylineSilverRepair.Code).FirstOrCreate(&leylineSilverRepair).Error; err != nil {
		return err
	}
	worldTeleportRepairPublished := time.Date(2026, 7, 23, 12, 20, 0, 0, time.Local)
	worldTeleportRepair := model.Notice{
		Code:  "repair_ten_world_maps_and_teleport_20260723",
		Title: "十界万图与传送阵修复公告",
		Content: "【十界扩容】东洲、南疆、西漠、北原、中天域、沧海、幽冥界、九霄天、太虚境与星河界均扩充为一千处系统地图，合计一万处。新增地点拥有独立名称、NPC、地图任务、采集物、妖兽、首领、境界层数、战力、奖励与界内相邻路线。\n\n" +
			"【路线修复】修复旧地图把十界地点交叉串成一条长路、只能逐站步行的问题。系统默认地图改为各界独立路网；仍由运营手动配置过的路线保持原样，不覆盖自定义数据。普通“前往”仍只走相邻路线，结果不再误写成跨界传送。\n\n" +
			"【真实传送】新增“传送阵”“传送列表 界域 页码”“传送 地点”和“跨界传送 界域”。亲自抵达带阵地点或查看当地地图会永久刻录阵纹；界内挪移消耗传送符一张，跨界接引消耗三张，物品扣除、位置变化与阵纹记录在同一事务结算。\n\n" +
			"【世界开放】新增“诸界列表”和“世界列表”，一次展示十界地图数量、个人已刻录阵点、当前所在世界、入口界门及开启境界。境界未达的世界会明确标为未解锁并显示前置，不会扣除传送符；达到前置后可从当前跨界门直接接引至新世界入口。\n\n" +
			"【数据保护】扩容采用增量写入，保留玩家道籍、当前位置、境界、背包、货币、装备、任务、灵脉发现记录及运营修改。旧版系统默认路线仅在确认未被修改时迁移，十座入口界门会补齐以保证跨界功能可用。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &worldTeleportRepairPublished,
	}
	if err := s.DB.Where("code = ?", worldTeleportRepair.Code).FirstOrCreate(&worldTeleportRepair).Error; err != nil {
		return err
	}
	spiritualRootReforgePublished := time.Date(2026, 7, 23, 21, 30, 0, 0, time.Local)
	spiritualRootReforgeRepair := model.Notice{
		Code:  "repair_spiritual_root_reforge_and_formation_sale_20260723",
		Title: "灵根重铸说明与阵基石特卖公告",
		Content: "【灵根随机重铸】原“灵根融合/灵根合成”菜单统一改名为“灵根随机重铸”。裸发“灵融”、重铸成功页和待吸收道种页都会明确说明：该玩法随机生成一枚候选灵根，属性不会与当前灵根叠加；只有确认吸收后才会替换当前灵根。\n\n" +
			"【材料确认】随机重铸消耗灵根精粹×2、阵基石×1与灵石×500。生成候选道种后选择放弃，当前灵根保持不变，但已经投入的材料不会返还；确认前不会覆盖当前灵根。\n\n" +
			"【庆典特卖】万宝庆典特卖新增阵基石，活动价48银币一枚，不设活动购买数量上限，可发送“庆典特卖 2”查看或发送“庆典购买 阵基石 数量”批量购买。购买仍受银币余额与安全整数范围约束。\n\n" +
			"【数据保护】本次只增量补充菜单说明、商品和公告，不重置玩家灵根、待定道种、背包、货币、装备、境界或其他道籍数据。",
		Type: "修复", Pinned: true, Published: true, PublishedAt: &spiritualRootReforgePublished,
	}
	if err := s.DB.Where("code = ?", spiritualRootReforgeRepair.Code).FirstOrCreate(&spiritualRootReforgeRepair).Error; err != nil {
		return err
	}
	v221WorldPublished := time.Date(2026, 7, 24, 8, 30, 0, 0, time.Local)
	v221UpdatePublished := time.Date(2026, 7, 24, 8, 25, 0, 0, time.Local)
	v221RepairPublished := time.Date(2026, 7, 24, 8, 20, 0, 0, time.Local)
	v221CompensationPublished := time.Date(2026, 7, 24, 10, 30, 0, 0, time.Local)
	v221Notices := []model.Notice{
		{
			Code:  "world_notice_v221_compensation_20260724",
			Title: "仙尘 v2.2.1 全服补偿公告",
			Content: "【发放缘由】针对本次版本维护期间出现的角色属性异常，以及装备归槽、锻造修复给道友带来的等待，仙尘发放“同尘重铸礼”。补偿只增发物资，不会把异常属性写成新的养成基线，也不替代原道籍属性回正。\n\n" +
				"【领取对象】2026年7月24日23时59分59秒（北京时间）前已经建立道籍的玩家。此后新建立的道籍未经历本次异常，不在本批次范围。符合范围且仍保留原道籍的玩家长期拥有补领资格。\n\n" +
				"【领取方式】发送“全服补偿”查看资格和完整清单；确认后发送“领取全服补偿”。每个平台账号仅限一次；领取成功后，删号、换OpenID或重复点击不会重置次数。\n\n" +
				"【货币与珍稀】灵石×18888、银币×888、功德×18、同尘重铸纪念令×1、灵根精粹×4、龙血芝×2。\n\n" +
				"【炼器与修行】玄铁×50、阵基石×20、星辰砂×12、雷灵晶×6、功法残卷×8、双倍修为卡×2、扫荡券×10。\n\n" +
				"【丹药与护道】回元散×20、回灵丹×20、聚灵丹×10、避劫符×2、引劫玉符×1、造化仙壤×4。\n\n" +
				"【领取保障】领取凭证、货币与全部物品在同一事务中写入；任何一项发放失败都会整体撤回，不会出现领了一半或重复入账。本批次不发仙金，也不直接增加角色基础属性。",
			Type: "公告", Pinned: true, Published: true, PublishedAt: &v221CompensationPublished,
		},
		{
			Code:  "world_notice_v221_player_20260724",
			Title: "仙尘新章告示",
			Content: "仙尘新章现已开启。\n━━━━━━━━━━━\n" +
				"功能菜单已补齐“生辰档案”“氪金菜单”“世界公告”“更新公告”“修复公告”五个入口，打开菜单就能直接找到。\n" +
				"“生日”平日可查看登记月日与下次生辰倒计时；只有寿星生日当天，生辰签到、年度寿礼、专属任务、限定抽奖、福签兑换等庆典玩法才会开启。\n" +
				"世界公告、更新公告与修复公告各自独立翻页；功能菜单下方会展示最新的玩家告示，不再被旧消息长期占住。\n━━━━━━━━━━━\n" +
				"法器现按器谱记载的真实槽位穿戴：葫芦归腰佩、飞舟归灵靴，其余器型同样不会再全部挤入本命法器。锻造会显示真实前后重数，满重或未成功时不扣玄铁。\n━━━━━━━━━━━\n" +
				"发送“功能菜单”查看全部入口，发送“更新公告”或“修复公告”了解本次变化。",
			Type: "公告", Pinned: true, Published: true, PublishedAt: &v221WorldPublished,
		},
		{
			Code:  "update_v221_player_menu_notices_20260724",
			Title: "仙尘 v2.2.1 内容更新",
			Content: "【全局菜单】氪金系统拥有独立分类与独立入口，不再藏在交易或其他分类中；价格表与累计充值查询可从“氪金菜单”直接进入。\n\n" +
				"【生辰档案】功能菜单永久显示“生辰档案”。未登记时会引导发送“设置生日 月-日”；已经登记且尚未到生日时，会显示登记日期、下次开启日期和准确倒计时。\n\n" +
				"【生辰庆典】只有寿星生日当天才显示生日专属菜单，并开放生辰签到、仙尘寿礼、生日任务、限定抽奖、福签兑换、寿星榜、道友祝福与赠礼；平日查看档案不会提前开放或消耗次数。\n\n" +
				"【三卷告示】功能菜单新增世界公告、更新公告、修复公告三个固定入口。三类内容各自分页，玩家可直接查看当前告示、本次新增内容以及已经完成的修复。\n\n" +
				"【世界消息】功能菜单下方优先展示发布时间最新、适合玩家阅读的世界告示，旧置顶内容不会继续遮住新消息。\n\n" +
				"【装备归槽】炼器、礼包、活动与器阁获得的装备统一读取器谱真实槽位。葫芦归腰佩、飞舟归灵靴，剑枪、衣索、镜琴、钟塔、扇珠、幡鼎与印伞均按各自器谱归位；已有装备保留品质、强化、锻造、铭刻、星阶、灵孔与宝石。\n\n" +
				"【锻造结算】玄火锻造会显示真实的旧重数与新重数，材料扣除和装备提升同时完成；材料不足、状态变化或达到三十重上限时不扣玄铁。",
			Type: "更新", Pinned: true, Published: true, PublishedAt: &v221UpdatePublished,
		},
		{
			Code:  "repair_v221_world_menu_visibility_20260724",
			Title: "菜单入口与世界消息修复公告",
			Content: "【菜单缺项】修复生辰内容已经开放却只能从角色菜单深页寻找的问题；“生辰档案”现在常驻功能菜单，登记、日期和倒计时随时可查。\n\n" +
				"【氪金入口】修复氪金价格表已有指令却没有独立全局入口的问题；氪金分类、氪金菜单与累充查询现在形成完整入口，不与交易菜单混放。\n\n" +
				"【公告入口】修复新增和修复内容只能自行猜指令查找的问题；世界公告、更新公告与修复公告已经在功能菜单明确列出，并保持频道互不串流。\n\n" +
				"【世界消息】修复旧置顶告示长期占据菜单底部的问题；现在按真实发布时间从新到旧选择，并跳过不适合直接展示给玩家的文字。\n\n" +
				"【生日边界】修复生日入口与专属玩法显示范围不清的问题；档案全年可见，签到、礼物、任务、抽奖和兑换仍严格限制在本人生日当天。\n\n" +
				"【器型归槽】修复炼器等旧入口忽略器谱槽位、把大量陌生器型默认放入本命法器的问题；现有系统器谱与玩家装备会增量归位，同槽冲突只卸下较弱装备并完整留在背包，不清除任何养成。\n\n" +
				"【锻造显示与扣费】修复锻造完成后结果把旧重数显示成新重数、出现“9 → 9”的问题；锻造的玄铁扣除、等级提升、品质变化和穿戴属性同步现为同一次结算，任一步失败都会全部撤回。三十重满级时明确提示且零扣费。\n\n" +
				"以上条目均已完成并可由对应指令直接核验，不包含尚未开放的计划内容。",
			Type: "修复", Pinned: true, Published: true, PublishedAt: &v221RepairPublished,
		},
	}
	for _, row := range v221Notices {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	if err := s.DB.Model(&model.Notice{}).Where("code = ? AND content = ?", "notice_message", "群聊与好友私信统一使用 Bee 普通文本消息，保证基础收发稳定。").Updates(map[string]any{"title": "消息双通道已启用", "content": "群聊与好友私信优先使用QQ开放平台原生Markdown；发送失败时自动回退Bee普通消息，避免指令静默。"}).Error; err != nil {
		return err
	}
	return nil
}

func (s *Store) seedDropPools() error {
	type poolSeed struct {
		Name, SourceType, SourceName string
		Entries                      map[string][3]int64
	}
	pools := []poolSeed{
		{Name: "野外探索掉落", SourceType: "探索", SourceName: "探索", Entries: map[string][3]int64{"凝露草": {40, 1, 3}, "灵果": {25, 1, 2}, "玄铁": {20, 1, 2}, "灵茶": {15, 1, 2}}},
		{Name: "普通妖兽掉落", SourceType: "战斗", SourceName: "猎妖", Entries: map[string][3]int64{"妖兽内丹": {35, 1, 2}, "灵兽口粮": {30, 1, 3}, "玄铁": {25, 1, 2}, "仙露": {10, 1, 1}}},
		{Name: "幽冥秘境掉落", SourceType: "副本", SourceName: "幽冥秘境", Entries: map[string][3]int64{"仙露": {35, 1, 3}, "功法残卷": {20, 1, 1}, "星辰砂": {15, 1, 2}, "扫荡券": {30, 1, 1}}},
		{Name: "剑冢遗址掉落", SourceType: "副本", SourceName: "剑冢遗址", Entries: map[string][3]int64{"玄铁": {35, 2, 5}, "功法残卷": {25, 1, 2}, "星辰砂": {40, 1, 3}}},
		{Name: "九霄雷域掉落", SourceType: "副本", SourceName: "九霄雷域", Entries: map[string][3]int64{"雷灵晶": {35, 1, 3}, "避劫符": {15, 1, 1}, "九转还魂丹": {5, 1, 1}, "星辰砂": {45, 2, 5}}},
	}
	for _, seed := range pools {
		pool := model.DropPool{Name: seed.Name, SourceType: seed.SourceType, SourceName: seed.SourceName, Enabled: true}
		if err := s.DB.Where("name = ?", pool.Name).FirstOrCreate(&pool).Error; err != nil {
			return err
		}
		for itemName, values := range seed.Entries {
			var item model.Item
			if s.DB.Where("name = ?", itemName).First(&item).Error != nil {
				continue
			}
			entry := model.DropEntry{DropPoolID: pool.ID, ItemID: item.ID, ItemName: item.Name, Weight: int(values[0]), Minimum: values[1], Maximum: values[2]}
			if err := s.DB.Where("drop_pool_id = ? AND item_id = ?", pool.ID, item.ID).FirstOrCreate(&entry).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
