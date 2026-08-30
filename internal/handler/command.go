package handler

import "strings"

type CommandSpec struct {
	ID        int    `json:"id"`
	Category  string `json:"category"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Input     string `json:"input"`
	EventOnly bool   `json:"event_only"`
}
type ParsedCommand struct {
	Spec         CommandSpec
	Arguments    []string
	RawArguments string
}

var CommandTable = []CommandSpec{
	{1, "角色", "入道修仙", "入道", "入道 道号 [男/女]", false}, {2, "角色", "道号改名", "改名", "改名 新道号", false}, {3, "角色", "查看状态", "状态", "状态", false}, {4, "角色", "星盘占卜", "占卜", "占卜", false}, {5, "角色", "兵解转世", "转世", "转世", false}, {6, "角色", "查看背包", "背包", "背包", false}, {7, "角色", "查看寿元", "寿元", "寿元", false}, {8, "角色", "修仙日历", "日历", "日历", false}, {9, "角色", "道心检测", "道心", "道心", false}, {10, "角色", "仙缘查询", "仙缘", "仙缘", false}, {11, "角色", "道侣设置", "道侣设置", "道侣设置 @QQ", false}, {12, "角色", "修仙档案", "档案", "档案", false},
	{13, "修炼", "闭关修炼", "修炼", "修炼", false}, {14, "修炼", "快速修行", "速修", "速修", false}, {15, "修炼", "境界突破", "突破", "突破", false}, {16, "修炼", "冥想静坐", "冥想", "冥想", false}, {17, "修炼", "悟道参禅", "悟道", "悟道", false}, {18, "修炼", "诵读经书", "诵经", "诵经", false}, {19, "修炼", "练功打坐", "练功", "练功", false}, {20, "修炼", "精进功法", "精进", "精进", false}, {21, "修炼", "修炼记录", "修记", "修记", false}, {22, "修炼", "结束闭关", "出关", "出关", false},
	{23, "探索", "游历四方", "探索", "探索", false}, {24, "探索", "秘境探险", "秘境", "秘境", false}, {25, "探索", "气运寻宝", "寻宝", "寻宝", false}, {26, "探索", "采集灵草", "采集", "采集", false}, {27, "探索", "访友论道", "访友", "访友 @对方", false}, {28, "探索", "游历名山", "游历", "游历", false}, {29, "探索", "探幽访古", "探幽", "探幽", false}, {30, "探索", "猎杀妖兽", "猎妖", "猎妖", false}, {31, "探索", "参加法会", "法会", "法会", false}, {32, "探索", "遇仙奇缘", "遇仙", "遇仙", false},
	{33, "道侣", "寻觅道侣", "寻缘", "寻缘", false}, {34, "道侣", "发起结缘", "结缘", "结缘 @对方", false}, {35, "道侣", "同意结缘", "应缘", "应缘", false}, {36, "道侣", "论道切磋", "论道", "论道 @对方", false}, {37, "道侣", "双人共修", "双修", "双修", false}, {38, "道侣", "为道侣护法", "护法", "护法 @对方", false}, {39, "道侣", "请求护法", "求护", "求护", false}, {40, "道侣", "合击秘术", "合击", "合击", false}, {41, "道侣", "传功渡力", "传功", "传功 @对方 数量", false}, {42, "道侣", "查看心意", "心意", "心意", false}, {43, "道侣", "赠送物品", "赠礼", "赠礼 @对方 物品 数量", false}, {44, "道侣", "召唤道侣", "召唤", "召唤", false}, {45, "道侣", "心有灵犀", "灵犀", "灵犀", false}, {46, "道侣", "解除道侣", "解缘", "解缘", false},
	{47, "战斗", "野外战斗", "战斗", "战斗", false}, {48, "战斗", "双人合战", "合战", "合战", false}, {49, "战斗", "逃跑脱战", "逃跑", "逃跑", false}, {50, "战斗", "疗伤恢复", "疗伤", "疗伤", false}, {51, "战斗", "丹战辅助", "丹战", "丹战", false}, {52, "战斗", "查看战力", "战力", "战力", false},
	{53, "渡劫", "引动天劫", "引劫", "引劫", false}, {54, "渡劫", "双人渡劫", "共渡", "共渡 @对方", false}, {55, "渡劫", "备劫查看", "备劫", "备劫", false}, {56, "渡劫", "渡劫成功", "自动", "—", true}, {57, "渡劫", "渡劫失败", "自动", "—", true}, {58, "渡劫", "飞升仙界", "飞升", "飞升", false},
	{59, "仙府", "查看仙府", "仙府", "仙府", false}, {60, "仙府", "升级仙府", "升级府", "升级府", false}, {61, "仙府", "种植灵草", "种田", "种田 [种子]", false}, {62, "仙府", "收获灵草", "收获", "收获", false}, {63, "仙府", "炼制丹药", "炼丹", "炼丹 [丹方]", false}, {64, "仙府", "升级丹房", "升丹房", "升丹房", false}, {65, "仙府", "布置阵法", "布阵", "布阵", false}, {66, "仙府", "驯养灵兽", "驯兽", "驯兽", false}, {67, "仙府", "府库查看", "府库", "府库", false}, {68, "仙府", "参观仙府", "参观", "参观 @对方", false},
	{69, "功法", "学习功法", "学功", "学功 功法名", false}, {70, "功法", "查看功法", "功法", "功法", false}, {71, "功法", "切换功法", "换功", "换功 功法名", false}, {72, "功法", "功法突破", "功突", "功突", false}, {73, "功法", "自创功法", "创功", "创功", false}, {74, "功法", "传承功法", "传功", "传功 @对方 功法名", false},
	{75, "灵兽", "捕获灵兽", "捕获", "捕获", false}, {76, "灵兽", "查看灵兽", "灵兽", "灵兽", false}, {77, "灵兽", "灵兽出战", "出战", "出战", false}, {78, "灵兽", "喂养灵兽", "喂养", "喂养 物品", false}, {79, "灵兽", "灵兽进化", "进化", "进化", false}, {80, "灵兽", "放生灵兽", "放生", "放生", false},
	{81, "社交", "全服传音", "传音", "传音 内容", false}, {82, "社交", "私密传音", "密语", "密语 @对方 内容", false}, {83, "社交", "好友列表", "好友", "好友", false}, {84, "社交", "添加好友", "加友", "加友 @对方", false}, {85, "社交", "拜师学艺", "拜师", "拜师 @对方", false}, {86, "社交", "收徒传道", "收徒", "收徒 @对方", false},
	{87, "交易", "批量摆摊", "摆摊", "摆摊 物品*数量 单价", false}, {88, "交易", "购买摊品", "购买", "购买 @摊主 物品*数量", false}, {89, "交易", "查看集市", "集市", "集市", false}, {90, "交易", "发起易物", "易物", "易物 @对方 我的物品*数量 对方物品*数量", false},
	{91, "任务", "每日任务", "日常", "日常 [页码]", false}, {92, "任务", "接受任务", "接任务", "接任务 任务名", false}, {93, "任务", "提交任务", "交任务", "交任务 任务名", false}, {94, "任务", "悬赏榜", "悬赏", "悬赏 [页码]", false}, {95, "任务", "查看成就", "成就", "成就 [页码]", false}, {96, "任务", "成就统计", "成统计", "成统计", false},
	{97, "特殊", "三生石", "三生石", "三生石", false}, {98, "特殊", "修仙日记", "日记", "日记 [内容/页码]", false}, {99, "特殊", "道侣留言", "留言", "留言 [@对方 内容/页码]", false}, {100, "特殊", "功德榜", "功德", "功德", false},
	{101, "宗门", "创建宗门", "创宗", "创宗 宗门名", false}, {102, "宗门", "加入宗门", "入宗", "入宗 宗门名", false}, {103, "宗门", "退出宗门", "退宗", "退宗", false}, {104, "宗门", "宗门信息", "宗门", "宗门", false}, {105, "宗门", "宗门任务", "宗务", "宗务", false}, {106, "宗门", "宗门贡献", "贡献", "贡献", false}, {107, "宗门", "宗门商店", "宗商", "宗商 物品", false}, {108, "宗门", "宗门大战", "宗战", "宗战 @宗门", false},
	{109, "丹药", "丹方查询", "丹方", "丹方", false}, {110, "丹药", "学习丹方", "学丹", "学丹 丹方名", false}, {111, "丹药", "炼制丹药", "炼药", "炼药 丹方名[*数量]", false}, {112, "丹药", "服用丹药", "服药", "服药 丹药名", false}, {113, "丹药", "丹药效果", "药效", "药效 丹药名", false}, {114, "丹药", "丹药批量", "批炼", "批炼 丹方名*数量", false},
	{115, "炼器", "学习器谱", "学器", "学器 器谱名", false}, {116, "炼器", "炼制装备", "炼器", "炼器 器谱名", false}, {117, "炼器", "查看法宝", "法宝", "法宝 [页码]", false}, {118, "炼器", "穿戴法宝", "装备", "装备 法宝名", false}, {119, "炼器", "卸下法宝", "卸宝", "卸宝 法宝名", false}, {120, "炼器", "法宝强化", "强宝", "强宝 法宝名", false},
	{121, "副本", "副本列表", "副本", "副本", false}, {122, "副本", "进入副本", "进入", "进入 副本名", false}, {123, "副本", "组队副本", "组副", "组副 副本名", false}, {124, "副本", "批量扫荡副本", "扫荡", "扫荡 副本名*次数", false}, {125, "副本", "副本排行", "副榜", "副榜", false}, {126, "副本", "副本重置", "重副", "重副", false},
	{127, "竞技", "竞技匹配", "竞技", "竞技", false}, {128, "竞技", "竞技战斗", "决斗", "决斗", false}, {129, "竞技", "竞技排行", "竞榜", "竞榜", false}, {130, "竞技", "竞技商店", "竞商", "竞商 物品", false},
	{131, "奇遇", "奇遇列表", "奇遇", "奇遇", false}, {132, "奇遇", "仙缘抽签", "抽签", "抽签", false}, {133, "奇遇", "天机阁", "天机", "天机", false}, {134, "奇遇", "渡劫预兆", "预兆", "预兆", false}, {135, "奇遇", "灵脉探测", "灵脉", "灵脉", false}, {136, "奇遇", "福地洞天", "福地", "福地", false},
	{137, "生涯", "修仙年表", "年表", "年表", false}, {138, "生涯", "修仙统计", "统计", "统计", false}, {139, "生涯", "修仙目标", "目标", "目标 内容", false}, {140, "生涯", "修仙评价", "评价", "评价", false},
	{141, "阵法", "学习阵图", "学阵", "学阵 阵图名", false}, {142, "阵法", "布置阵法", "布阵", "布阵 阵法名", false}, {143, "阵法", "查看阵法", "阵法", "阵法", false}, {144, "阵法", "阵法升级", "升阵", "升阵 阵法名", false}, {145, "阵法", "阵法组合", "阵合", "阵合 阵法1 阵法2", false}, {146, "阵法", "破阵挑战", "破阵", "破阵 @对方", false},
	{147, "符箓", "学习符箓", "学符", "学符 符箓名", false}, {148, "符箓", "制作符箓", "制符", "制符 符箓名", false}, {149, "符箓", "查看符箓", "符箓", "符箓", false}, {150, "符箓", "使用符箓", "用符", "用符 符箓名", false}, {151, "符箓", "符箓强化", "强符", "强符 符箓名", false}, {152, "符箓", "符箓交易", "符市", "符市", false},
	{153, "傀儡", "炼制傀儡", "炼傀", "炼傀 材料名", false}, {154, "傀儡", "查看傀儡", "傀儡", "傀儡", false}, {155, "傀儡", "傀儡出战", "傀战", "傀战", false}, {156, "傀儡", "傀儡升级", "傀升", "傀升 傀儡名", false}, {157, "傀儡", "傀儡修复", "傀修", "傀修 傀儡名", false}, {158, "傀儡", "傀儡融合", "傀融", "傀融 傀儡1 傀儡2", false},
	{159, "秘境争夺", "秘境探测", "探秘", "探秘", false}, {160, "秘境争夺", "进入秘境", "入秘", "入秘 秘境名", false}, {161, "秘境争夺", "秘境战斗", "秘战", "秘战", false}, {162, "秘境争夺", "秘境占领", "占秘", "占秘 秘境名", false}, {163, "秘境争夺", "秘境防守", "守秘", "守秘", false}, {164, "秘境争夺", "秘境排行", "秘榜", "秘榜", false},
	{165, "传承", "寻找传承", "寻传", "寻传", false}, {166, "传承", "接受传承", "受传", "受传 传承名", false}, {167, "传承", "查看传承", "传承", "传承", false}, {168, "传承", "传承融合", "融传", "融传 传承1 传承2", false}, {169, "传承", "传承他人", "传下", "传下 @对方 传承名", false}, {170, "传承", "传承觉醒", "觉传", "觉传 传承名", false},
	{171, "悟道", "道台悟道", "悟道台", "悟道台", false}, {172, "悟道", "道韵积累", "道韵", "道韵", false}, {173, "悟道", "道法自然", "自然", "自然", false}, {174, "悟道", "道痕参悟", "道痕", "道痕 道痕名", false}, {175, "悟道", "道法施展", "道法", "道法 道法名", false}, {176, "悟道", "道法创造", "创道", "创道", false},
	{177, "仙魔战场", "战场进入", "战场", "战场", false}, {178, "仙魔战场", "阵营选择", "阵营", "阵营 仙/魔", false}, {179, "仙魔战场", "战场战斗", "战战", "战战", false}, {180, "仙魔战场", "战场任务", "战务", "战务", false}, {181, "仙魔战场", "战场排行", "战榜", "战榜", false}, {182, "仙魔战场", "战场商店", "战商", "战商 物品名", false},
	{183, "灵根进化", "灵根检测", "灵检", "灵检", false}, {184, "灵根进化", "灵根淬炼", "灵淬", "灵淬", false}, {185, "灵根进化", "灵根进化", "灵进", "灵进", false}, {186, "灵根进化", "灵根觉醒", "灵觉", "灵觉", false}, {187, "灵根进化", "灵根随机重铸", "灵融", "灵融 [灵根A 灵根B]", false}, {188, "灵根进化", "灵根传承", "灵传", "灵传 @对方", false},
	{189, "渡劫心魔", "心魔检测", "心魔", "心魔", false}, {190, "渡劫心魔", "心魔镇压", "镇魔", "镇魔", false}, {191, "渡劫心魔", "心魔炼化", "炼魔", "炼魔", false}, {192, "渡劫心魔", "心魔试炼", "魔试", "魔试", false}, {193, "渡劫心魔", "渡劫心魔", "劫魔", "劫魔", false}, {194, "渡劫心魔", "心魔封印", "封魔", "封魔", false},
	{195, "合体技", "合体技学习", "合学", "合学 技能名", false}, {196, "合体技", "合体技查看", "合技", "合技", false}, {197, "合体技", "合体技施展", "合施", "合施 技能名", false}, {198, "合体技", "合体技强化", "合强", "合强 技能名", false}, {199, "合体技", "合体技融合", "合融", "合融 技能1 技能2", false}, {200, "合体技", "合体技传承", "合传", "合传 @对方 技能名", false},
	{201, "仙药培育", "仙药种植", "种药", "种药 药种名", false}, {202, "仙药培育", "仙药培育", "育药", "育药 药名", false}, {203, "仙药培育", "仙药采摘", "采药", "采药 药名", false}, {204, "仙药培育", "仙药嫁接", "嫁药", "嫁药 药1 药2", false}, {205, "仙药培育", "仙药催化", "催药", "催药 药名", false}, {206, "仙药培育", "仙药图鉴", "药鉴", "药鉴", false},
	{207, "法宝炼化", "法宝炼化", "炼化", "炼化 法宝名", false}, {208, "法宝炼化", "法宝开光", "开光", "开光 法宝名", false}, {209, "法宝炼化", "法宝蕴养", "蕴养", "蕴养 法宝名", false}, {210, "法宝炼化", "法宝融合", "宝融", "宝融 法宝1 法宝2", false}, {211, "法宝炼化", "法宝认主", "认主", "认主 法宝名", false}, {212, "法宝炼化", "法宝传承", "宝传", "宝传 @对方 法宝名", false},
	{213, "天机推演", "天机推演", "天机", "天机", false}, {214, "天机推演", "天命预测", "天命", "天命", false}, {215, "天机推演", "天劫预警", "劫预", "劫预", false}, {216, "天机推演", "机缘推演", "机缘", "机缘", false}, {217, "天机推演", "气运改命", "改命", "改命", false}, {218, "天机推演", "天机反噬", "反噬", "反噬", false},
	{219, "天地灵脉", "灵脉探测", "脉探", "脉探", false}, {220, "天地灵脉", "灵脉占据", "脉占", "脉占 灵脉名", false}, {221, "天地灵脉", "灵脉争夺", "脉争", "脉争 灵脉名", false}, {222, "天地灵脉", "灵脉修炼", "脉修", "脉修", false}, {223, "天地灵脉", "灵脉转移", "脉转", "脉转 @对方", false}, {224, "天地灵脉", "灵脉融合", "脉合", "脉合 灵脉1 灵脉2", false},
	{225, "宗门战争", "宣战宗门", "宣战", "宣战 宗门名", false}, {226, "宗门战争", "宗门备战", "备战", "备战", false}, {227, "宗门战争", "宗门大战", "宗战", "宗战 宗门名", false}, {228, "宗门战争", "宗门议和", "议和", "议和 宗门名", false}, {229, "宗门战争", "宗门结盟", "结盟", "结盟 宗门名", false}, {230, "宗门战争", "宗门领地", "领地", "领地", false},
	{231, "仙缘奇遇", "仙缘触发", "仙遇", "仙遇", false}, {232, "仙缘奇遇", "仙缘选择", "仙选", "仙选 选项", false}, {233, "仙缘奇遇", "仙缘记录", "仙录", "仙录", false}, {234, "仙缘奇遇", "仙缘加深", "仙深", "仙深", false}, {235, "仙缘奇遇", "仙缘传承", "仙承", "仙承 @对方", false}, {236, "仙缘奇遇", "仙缘觉醒", "仙觉", "仙觉", false},
	{237, "宇宙星河", "星图探索", "星图", "星图", false}, {238, "宇宙星河", "星力吸取", "星力", "星力", false}, {239, "宇宙星河", "星魂觉醒", "星魂", "星魂", false}, {240, "宇宙星河", "星域传送", "星传", "星传 星域名", false},
	{241, "地图", "查看世界地图", "地图", "地图", false}, {242, "地图", "前往地点", "前往", "前往 地点", false},
	{243, "系统", "查看修仙系统", "系统", "系统", false},
	{244, "地图", "区域首领", "首领", "首领", false}, {245, "地图", "讨伐首领", "讨伐", "讨伐", false},
	{246, "挂机", "开始挂机", "挂机", "挂机 猎妖/副本名 [分钟]", false}, {247, "挂机", "挂机结算", "挂机结算", "挂机结算", false},
	{248, "角色", "查看物品", "物品", "物品 名称", false}, {249, "角色", "背包搜索", "背包搜索", "背包搜索 关键词", false},
	{250, "角色", "使用物品", "使用", "使用 物品名", false}, {251, "角色", "物品查询", "查询", "查询 物品ID", false},
	{252, "地图", "查看当前位置", "位置", "位置", false},
	{253, "竞技", "普通攻击", "攻击", "攻击", false}, {254, "竞技", "施展功法", "技能", "技能 [功法名]", false},
	{255, "竞技", "防御姿态", "防御", "防御", false}, {256, "竞技", "投降认输", "投降", "投降", false},
	{257, "系统", "新手引导", "帮助", "帮助", false},
}

var commandIndex = func() map[string][]CommandSpec {
	out := make(map[string][]CommandSpec)
	categories := make(map[string]struct{})
	for _, spec := range CommandTable {
		if !spec.EventOnly {
			out[spec.Command] = append(out[spec.Command], spec)
			categories[spec.Category] = struct{}{}
		}
	}
	for _, spec := range AuxiliaryCommands() {
		if spec.Category != "" && spec.Category != "管理" {
			categories[spec.Category] = struct{}{}
		}
	}
	out["菜单"] = []CommandSpec{{ID: 1000, Category: "系统", Name: "游戏菜单", Command: "菜单", Input: "菜单 [分类]"}}
	out["功能菜单"] = []CommandSpec{{ID: 1000, Category: "系统", Name: "游戏菜单", Command: "功能菜单", Input: "功能菜单"}}
	for _, alias := range []string{"领取挂机", "收取挂机", "收获挂机", "挂机领取", "挂机收取", "挂机收获"} {
		out[alias] = []CommandSpec{{ID: 247, Category: "挂机", Name: "挂机结算", Command: alias, Input: alias}}
	}
	for _, alias := range []string{"结束挂机", "停止挂机", "挂机结束", "挂机停止"} {
		out[alias] = []CommandSpec{{ID: 247, Category: "挂机", Name: "结束挂机", Command: alias, Input: alias}}
	}
	out["挂机状态"] = []CommandSpec{{ID: 246, Category: "挂机", Name: "挂机状态", Command: "挂机状态", Input: "挂机状态"}}
	for category := range categories {
		alias := category + "菜单"
		out[alias] = []CommandSpec{{ID: 1000, Category: category, Name: alias, Command: alias, Input: alias}}
	}
	// 管理入口不计入普通玩法编号，由权限层在业务路由中单独校验。
	for _, spec := range AuxiliaryCommands() {
		out[spec.Command] = []CommandSpec{spec}
	}
	return out
}()

// AuxiliaryCommands are system entries intentionally kept outside the 257
// core gameplay rows so old data imports remain compatible.
func AuxiliaryCommands() []CommandSpec {
	return []CommandSpec{
		{ID: 1001, Category: "管理", Name: "管理系统", Command: "管理", Input: "管理"},
		{ID: 1001, Category: "管理", Name: "神令系统", Command: "神令", Input: "神令"},
		{ID: 1001, Category: "管理", Name: "神令系统", Command: "神令系统", Input: "神令系统"},
		{ID: 1000, Category: "管理", Name: "管理菜单", Command: "管理菜单", Input: "管理菜单"},
		{ID: 1002, Category: "任务", Name: "每日签到", Command: "签到", Input: "签到"},
		{ID: 1003, Category: "任务", Name: "签到记录", Command: "签到记录", Input: "签到记录"},
		{ID: 1007, Category: "交易", Name: "仙门货铺", Command: "商城", Input: "商城 [页码]"},
		{ID: 1007, Category: "交易", Name: "仙门货铺", Command: "货铺", Input: "货铺 [页码]"},
		{ID: 1008, Category: "交易", Name: "买下摊品", Command: "买下", Input: "买下 物品名"},
		{ID: 1009, Category: "交易", Name: "购入商品", Command: "购入", Input: "购入 物品名 [数量]"},
		{ID: 1010, Category: "仙府", Name: "种子商店", Command: "种子商店", Input: "种子商店 [页码]"},
		{ID: 1011, Category: "仙府", Name: "购买种子", Command: "购买种子", Input: "购买种子 种子名 [数量]"},
		{ID: 1012, Category: "特殊", Name: "仙途礼包", Command: "礼包", Input: "礼包 [页码]"},
		{ID: 1013, Category: "特殊", Name: "开启礼包", Command: "开启礼包", Input: "开启礼包 礼包名"},
		{ID: 1014, Category: "装备", Name: "装备系统", Command: "装备系统", Input: "装备系统"},
		{ID: 1014, Category: "装备", Name: "当前装备", Command: "当前装备", Input: "当前装备"},
		{ID: 1015, Category: "装备", Name: "装备背包", Command: "装备背包", Input: "装备背包 [页码]"},
		{ID: 1016, Category: "装备", Name: "穿戴装备", Command: "穿戴", Input: "穿戴 装备名"},
		{ID: 1017, Category: "装备", Name: "卸下装备", Command: "卸下", Input: "卸下 装备名/槽位"},
		{ID: 1018, Category: "装备", Name: "一键卸下", Command: "一键卸下", Input: "一键卸下"},
		{ID: 1019, Category: "装备", Name: "装备锻造", Command: "锻造", Input: "锻造 装备名"},
		{ID: 1020, Category: "装备", Name: "装备篆刻", Command: "篆刻", Input: "篆刻 装备名"},
		{ID: 1021, Category: "仙府", Name: "仙府灵田", Command: "灵田", Input: "灵田 [页码]"},
		{ID: 1021, Category: "仙府", Name: "我的灵田", Command: "我的灵田", Input: "我的灵田 [页码]"},
		{ID: 1022, Category: "仙府", Name: "指定播种", Command: "种植", Input: "种植 种子名 地块"},
		{ID: 1023, Category: "仙府", Name: "一键播种", Command: "一键种植", Input: "一键种植 种子名"},
		{ID: 1024, Category: "仙府", Name: "收取灵植", Command: "收菜", Input: "收菜 [地块]"},
		{ID: 1025, Category: "仙府", Name: "引灵浇水", Command: "浇水", Input: "浇水 地块"},
		{ID: 1026, Category: "仙府", Name: "清除杂草", Command: "除草", Input: "除草 地块"},
		{ID: 1027, Category: "仙府", Name: "驱除灵虫", Command: "除虫", Input: "除虫 地块"},
		{ID: 1028, Category: "仙府", Name: "灵田仓库", Command: "灵田仓库", Input: "灵田仓库 [页码]"},
		{ID: 1029, Category: "仙府", Name: "出售灵植", Command: "出售灵植", Input: "出售灵植 灵植名 [数量]"},
		{ID: 1030, Category: "仙府", Name: "一键出售灵植", Command: "一键出售灵植", Input: "一键出售灵植"},
		{ID: 1031, Category: "仙府", Name: "道友采撷", Command: "采撷", Input: "采撷 道号"},
		{ID: 1031, Category: "仙府", Name: "潜入采灵", Command: "偷菜", Input: "偷菜 道号"},
		{ID: 1031, Category: "仙府", Name: "潜入采灵", Command: "潜入采灵", Input: "潜入采灵 道号"},
		{ID: 1032, Category: "仙府", Name: "土地详情", Command: "土地详情", Input: "土地详情 [地块]"},
		{ID: 1033, Category: "仙府", Name: "灵田说明", Command: "灵田说明", Input: "灵田说明"},
		{ID: 1034, Category: "仙府", Name: "护田记录", Command: "护田记录", Input: "护田记录"},
		{ID: 1035, Category: "仙府", Name: "灵田排行", Command: "灵田榜", Input: "灵田榜"},
		{ID: 1036, Category: "生涯", Name: "诸天排行榜", Command: "排行榜", Input: "排行榜 [类型] [页码]"},
		{ID: 1036, Category: "生涯", Name: "诸天排行榜", Command: "排行", Input: "排行 [类型] [页码]"},
		{ID: 1037, Category: "生涯", Name: "领取排行俸禄", Command: "领取排行奖励", Input: "领取排行奖励 榜单类型"},
		{ID: 1038, Category: "系统", Name: "仙门公告", Command: "公告", Input: "公告 [页码]"},
		{ID: 1038, Category: "系统", Name: "世界公告", Command: "世界公告", Input: "世界公告 [页码]"},
		{ID: 1039, Category: "系统", Name: "版本更新公告", Command: "更新公告", Input: "更新公告 [页码]"},
		{ID: 1040, Category: "系统", Name: "全区通报", Command: "全区通报", Input: "全区通报 [页码]"},
		{ID: 1040, Category: "系统", Name: "全区通报", Command: "通告", Input: "通告 [页码]"},
		{ID: 1041, Category: "探索", Name: "奇遇抉择", Command: "抉择", Input: "抉择 选项名"},
		{ID: 1042, Category: "角色", Name: "货币钱庄", Command: "货币", Input: "货币"},
		{ID: 1042, Category: "角色", Name: "货币钱庄", Command: "钱庄", Input: "钱庄"},
		{ID: 1042, Category: "角色", Name: "银币来源", Command: "银币来源", Input: "银币来源"},
		{ID: 1042, Category: "角色", Name: "赚取银币", Command: "赚银币", Input: "赚银币"},
		{ID: 1043, Category: "交易", Name: "银币商城", Command: "银币商城", Input: "银币商城 [页码]"},
		{ID: 1044, Category: "交易", Name: "仙金商城", Command: "仙金商城", Input: "仙金商城 [页码]"},
		{ID: 1045, Category: "交易", Name: "银币购买", Command: "银币购买", Input: "银币购买 物品名 [数量]"},
		{ID: 1046, Category: "交易", Name: "仙金购买", Command: "仙金购买", Input: "仙金购买 物品名 [数量]"},
		{ID: 1047, Category: "竞技", Name: "竞技档案", Command: "竞技档案", Input: "竞技档案"},
		{ID: 1047, Category: "竞技", Name: "竞技档案", Command: "战绩", Input: "战绩"},
		{ID: 1048, Category: "竞技", Name: "千阶竞技段位", Command: "竞技段位", Input: "竞技段位 [页码]"},
		{ID: 1049, Category: "竞技", Name: "竞技俸禄", Command: "竞技奖励", Input: "竞技奖励"},
		{ID: 1050, Category: "竞技", Name: "竞技规则", Command: "竞技说明", Input: "竞技说明"},
		{ID: 130, Category: "竞技", Name: "竞技商店", Command: "竞技商店", Input: "竞技商店 [物品名]"},
		{ID: 1051, Category: "系统", Name: "专属快捷", Command: "快捷", Input: "快捷"},
		{ID: 1051, Category: "系统", Name: "快捷列表", Command: "快捷列表", Input: "快捷列表"},
		{ID: 1052, Category: "系统", Name: "设置快捷", Command: "设置快捷", Input: "设置快捷 别名=完整指令"},
		{ID: 1053, Category: "系统", Name: "删除快捷", Command: "删除快捷", Input: "删除快捷 别名"},
		{ID: 1054, Category: "角色", Name: "转让道号", Command: "转让道号", Input: "转让道号 对方道号 自己的新道号"},
		{ID: 1055, Category: "角色", Name: "接受道号", Command: "接受道号", Input: "接受道号"},
		{ID: 1056, Category: "天地灵脉", Name: "修仙界灵脉", Command: "灵脉地图", Input: "灵脉地图 [本源] [页码]"},
		{ID: 1056, Category: "天地灵脉", Name: "修仙界灵脉", Command: "修仙界灵脉", Input: "修仙界灵脉 [本源] [页码]"},
		{ID: 1057, Category: "天地灵脉", Name: "灵脉详情", Command: "灵脉详情", Input: "灵脉详情 灵脉名"},
		{ID: 1058, Category: "天地灵脉", Name: "灵脉打坐", Command: "灵脉打坐", Input: "灵脉打坐 灵脉名"},
		{ID: 1059, Category: "天地灵脉", Name: "灵脉出定", Command: "灵脉出定", Input: "灵脉出定"},
		{ID: 1060, Category: "天地灵脉", Name: "采撷灵气", Command: "采灵气", Input: "采灵气 灵脉名"},
		{ID: 1061, Category: "天地灵脉", Name: "灵脉修行榜", Command: "灵脉修行榜", Input: "灵脉修行榜 [页码]"},
		{ID: 1062, Category: "天地灵脉", Name: "探查地脉", Command: "寻脉", Input: "寻脉"},
		{ID: 1063, Category: "灵根进化", Name: "千种灵根图鉴", Command: "灵根图鉴", Input: "灵根图鉴 [页码]"},
		{ID: 1064, Category: "灵根进化", Name: "灵根详情", Command: "灵根详情", Input: "灵根详情 灵根名"},
		{ID: 1065, Category: "地图", Name: "与NPC对话", Command: "对话", Input: "对话 NPC名"},
		{ID: 1066, Category: "地图", Name: "挑战地图妖兽", Command: "挑战", Input: "挑战 妖兽名"},
		{ID: 1067, Category: "系统", Name: "长内容翻页", Command: "翻页", Input: "翻页 页码"},
		{ID: 1068, Category: "氪金", Name: "仙尘独立氪金价格表", Command: "充值菜单", Input: "充值菜单 [页码]"},
		{ID: 1068, Category: "氪金", Name: "仙尘独立氪金价格表", Command: "氪金菜单", Input: "氪金菜单 [页码]"},
		{ID: 1068, Category: "氪金", Name: "仙尘独立氪金价格表", Command: "价格表", Input: "价格表 [页码]"},
		{ID: 1069, Category: "特殊", Name: "仙途定制", Command: "定制菜单", Input: "定制菜单"},
		{ID: 1070, Category: "特殊", Name: "定制灵根", Command: "定制灵根", Input: "定制灵根 灵根名"},
		{ID: 1071, Category: "特殊", Name: "定制称号", Command: "定制称号", Input: "定制称号 新称号"},
		{ID: 1072, Category: "特殊", Name: "定制仙府", Command: "定制仙府", Input: "定制仙府 新名称"},
		{ID: 1073, Category: "特殊", Name: "定制灵兽", Command: "定制灵兽", Input: "定制灵兽 原灵兽名=新名字"},
		{ID: 1074, Category: "特殊", Name: "定制法宝", Command: "定制法宝", Input: "定制法宝 原法宝名=新名字"},
		{ID: 1075, Category: "炼器", Name: "合成系统", Command: "合成菜单", Input: "合成菜单"},
		{ID: 1076, Category: "炼器", Name: "合成图鉴", Command: "合成图鉴", Input: "合成图鉴 [页码]"},
		{ID: 1076, Category: "炼器", Name: "合成列表", Command: "合成列表", Input: "合成列表 [页码]"},
		{ID: 1077, Category: "炼器", Name: "材料合成", Command: "合成", Input: "合成 配方名*数量"},
		{ID: 1078, Category: "炼器", Name: "合成记录", Command: "合成记录", Input: "合成记录"},
		{ID: 1079, Category: "系统", Name: "全部指令大全", Command: "指令大全", Input: "指令大全 [页码]"},
		{ID: 1079, Category: "系统", Name: "完整玩法指南", Command: "怎么玩", Input: "怎么玩"},
		{ID: 1079, Category: "系统", Name: "完整玩法指南", Command: "玩法指南", Input: "玩法指南"},
		{ID: 1080, Category: "仙府", Name: "灵田升阶", Command: "升级灵田", Input: "升级灵田"},
		{ID: 1080, Category: "仙府", Name: "扩建灵田", Command: "扩建灵田", Input: "扩建灵田"},
		{ID: 1081, Category: "角色", Name: "回城复活", Command: "回城复活", Input: "回城复活"},
		{ID: 1081, Category: "角色", Name: "回城复活", Command: "复活", Input: "复活"},
		{ID: 1082, Category: "角色", Name: "申请删除道籍", Command: "申请删号", Input: "申请删号"},
		{ID: 1083, Category: "角色", Name: "确认删除道籍", Command: "确认删号", Input: "确认删号 当前完整道号"},
		{ID: 1084, Category: "角色", Name: "取消删除道籍", Command: "取消删号", Input: "取消删号"},
		{ID: 1085, Category: "炼器", Name: "配方详情", Command: "配方", Input: "配方 配方名"},
		{ID: 1086, Category: "活动", Name: "活动菜单", Command: "活动菜单", Input: "活动菜单"},
		{ID: 1086, Category: "活动", Name: "活动菜单", Command: "活动", Input: "活动"},
		{ID: 1087, Category: "活动", Name: "七日目标", Command: "七日目标", Input: "七日目标 [页码]"},
		{ID: 1087, Category: "活动", Name: "领取七日目标", Command: "领取七日目标", Input: "领取七日目标 目标名"},
		{ID: 1088, Category: "活动", Name: "境界冲刺", Command: "境界冲刺", Input: "境界冲刺 [页码]"},
		{ID: 1088, Category: "活动", Name: "领取境界冲刺", Command: "领取境界冲刺", Input: "领取境界冲刺 里程碑名"},
		{ID: 1089, Category: "活动", Name: "七日福利", Command: "七日福利", Input: "七日福利"},
		{ID: 1089, Category: "活动", Name: "领取七日福利", Command: "领取七日福利", Input: "领取七日福利 [福利名]"},
		{ID: 1090, Category: "活动", Name: "开服密令", Command: "开服密令", Input: "开服密令"},
		{ID: 1091, Category: "活动", Name: "密令兑换", Command: "密令兑换", Input: "密令兑换 密令"},
		{ID: 1092, Category: "活动", Name: "限时福利码", Command: "限时福利码", Input: "限时福利码"},
		{ID: 1093, Category: "活动", Name: "密令任务", Command: "密令任务", Input: "密令任务"},
		{ID: 1094, Category: "活动", Name: "道友召集", Command: "道友召集", Input: "道友召集"},
		{ID: 1095, Category: "活动", Name: "邀请道友", Command: "邀请道友", Input: "邀请道友"},
		{ID: 1095, Category: "活动", Name: "接受邀请", Command: "接受邀请", Input: "接受邀请 邀请码"},
		{ID: 1096, Category: "活动", Name: "结伴奖励", Command: "结伴奖励", Input: "结伴奖励 [奖励名]"},
		{ID: 1097, Category: "活动", Name: "助力修炼", Command: "助力修炼", Input: "助力修炼 道号"},
		{ID: 1098, Category: "活动", Name: "新秀榜", Command: "新秀榜", Input: "新秀榜 [页码]"},
		{ID: 1098, Category: "活动", Name: "领取新秀奖励", Command: "领取新秀奖励", Input: "领取新秀奖励"},
		{ID: 1099, Category: "活动", Name: "庆典专属", Command: "庆典专属", Input: "庆典专属"},
		{ID: 1100, Category: "活动", Name: "天降鸿运", Command: "天降鸿运", Input: "天降鸿运"},
		{ID: 1101, Category: "活动", Name: "限时祈福", Command: "限时祈福", Input: "限时祈福 [问道/护脉/纳福]"},
		{ID: 1102, Category: "活动", Name: "庆典特卖", Command: "庆典特卖", Input: "庆典特卖 [页码]"},
		{ID: 1102, Category: "活动", Name: "庆典购买", Command: "庆典购买", Input: "庆典购买 物品名 [数量]"},
		{ID: 1103, Category: "活动", Name: "活动总览", Command: "活动总览", Input: "活动总览 [页码]"},
		{ID: 1104, Category: "灵根进化", Name: "灵根随机重铸", Command: "灵根合成", Input: "灵根合成 灵根A 灵根B"},
		{ID: 1104, Category: "灵根进化", Name: "灵根随机重铸", Command: "合成灵根", Input: "合成灵根 灵根A 灵根B"},
		{ID: 1105, Category: "灵根进化", Name: "查看合成道种", Command: "灵根道种", Input: "灵根道种"},
		{ID: 1106, Category: "灵根进化", Name: "吸收合成灵根", Command: "吸收灵根", Input: "吸收灵根"},
		{ID: 1107, Category: "灵根进化", Name: "放弃合成灵根", Command: "放弃灵根", Input: "放弃灵根"},
		{ID: 1108, Category: "角色", Name: "角色性别", Command: "性别", Input: "性别 [男/女]"},
		{ID: 1108, Category: "角色", Name: "登记角色性别", Command: "设定性别", Input: "设定性别 男/女"},
		{ID: 1109, Category: "角色", Name: "运气属性", Command: "运气", Input: "运气"},
		{ID: 1110, Category: "系统", Name: "仙盟反馈", Command: "反馈菜单", Input: "反馈菜单"},
		{ID: 1110, Category: "系统", Name: "反馈说明", Command: "反馈说明", Input: "反馈说明"},
		{ID: 1111, Category: "系统", Name: "提交BUG", Command: "提交BUG", Input: "提交BUG 指令、现象与期望结果"},
		{ID: 1111, Category: "系统", Name: "提交BUG", Command: "提交bug", Input: "提交bug 指令、现象与期望结果"},
		{ID: 1111, Category: "系统", Name: "提交BUG", Command: "反馈BUG", Input: "反馈BUG 指令、现象与期望结果"},
		{ID: 1112, Category: "系统", Name: "提交建议", Command: "提交建议", Input: "提交建议 功能、做法与原因"},
		{ID: 1112, Category: "系统", Name: "提交建议", Command: "建议", Input: "建议 功能、做法与原因"},
		{ID: 1113, Category: "系统", Name: "我的反馈", Command: "我的反馈", Input: "我的反馈 [页码]"},
		{ID: 1114, Category: "系统", Name: "仙尘介绍", Command: "仙尘介绍", Input: "仙尘介绍"},
		{ID: 1114, Category: "系统", Name: "游戏介绍", Command: "游戏介绍", Input: "游戏介绍"},
		{ID: 1114, Category: "系统", Name: "仙尘世界观", Command: "世界观", Input: "世界观"},
		{ID: 1114, Category: "系统", Name: "修仙大世界", Command: "大世界", Input: "大世界"},
		{ID: 1115, Category: "角色", Name: "体力周天", Command: "体力", Input: "体力"},
		{ID: 1116, Category: "系统", Name: "独立修复公告", Command: "修复公告", Input: "修复公告 [页码]"},
		{ID: 1117, Category: "社交", Name: "个人通知信箱", Command: "通知", Input: "通知 [页码]"},
		{ID: 1117, Category: "社交", Name: "个人通知信箱", Command: "通知信箱", Input: "通知信箱 [页码]"},
		{ID: 1118, Category: "社交", Name: "未读通知", Command: "通知未读", Input: "通知未读 [页码]"},
		{ID: 1119, Category: "社交", Name: "清理已读通知", Command: "清理已读通知", Input: "清理已读通知"},
		{ID: 1120, Category: "图鉴", Name: "万象图鉴", Command: "图鉴", Input: "图鉴 [类别/页码]"},
		{ID: 1120, Category: "图鉴", Name: "万象图鉴菜单", Command: "图鉴菜单", Input: "图鉴菜单 [页码]"},
		{ID: 1121, Category: "图鉴", Name: "物品图鉴", Command: "物品图鉴", Input: "物品图鉴 [分类] [页码]"},
		{ID: 1121, Category: "装备", Name: "装备图鉴", Command: "装备图鉴", Input: "装备图鉴 [槽位/器型] [页码]"},
		{ID: 1121, Category: "丹药", Name: "丹药图鉴", Command: "丹药图鉴", Input: "丹药图鉴 [页码]"},
		{ID: 1121, Category: "丹药", Name: "丹方图鉴", Command: "丹方图鉴", Input: "丹方图鉴 [页码]"},
		{ID: 1121, Category: "功法", Name: "功法图鉴", Command: "功法图鉴", Input: "功法图鉴 [类型] [页码]"},
		{ID: 1121, Category: "地图", Name: "地图图鉴", Command: "地图图鉴", Input: "地图图鉴 [州域] [页码]"},
		{ID: 1121, Category: "地图", Name: "NPC图鉴", Command: "NPC图鉴", Input: "NPC图鉴 [州域] [页码]"},
		{ID: 1121, Category: "地图", Name: "妖兽图鉴", Command: "妖兽图鉴", Input: "妖兽图鉴 [州域] [页码]"},
		{ID: 1121, Category: "地图", Name: "首领图鉴", Command: "首领图鉴", Input: "首领图鉴 [州域] [页码]"},
		{ID: 1121, Category: "灵兽", Name: "灵兽图鉴", Command: "灵兽图鉴", Input: "灵兽图鉴 [页码]"},
		{ID: 1121, Category: "副本", Name: "副本图鉴", Command: "副本图鉴", Input: "副本图鉴 [难度] [页码]"},
		{ID: 1121, Category: "任务", Name: "任务图鉴", Command: "任务图鉴", Input: "任务图鉴 [类型] [页码]"},
		{ID: 1121, Category: "生涯", Name: "称号图鉴", Command: "称号图鉴", Input: "称号图鉴 [类型] [页码]"},
		{ID: 1121, Category: "仙府", Name: "种子图鉴", Command: "种子图鉴", Input: "种子图鉴 [页码]"},
		{ID: 1121, Category: "交易", Name: "商城图鉴", Command: "商城图鉴", Input: "商城图鉴 [货币] [页码]"},
		{ID: 1121, Category: "活动", Name: "活动图鉴", Command: "活动图鉴", Input: "活动图鉴 [类型] [页码]"},
		{ID: 1121, Category: "奇遇", Name: "事件图鉴", Command: "事件图鉴", Input: "事件图鉴 [类型] [页码]"},
		{ID: 1121, Category: "图鉴", Name: "掉落图鉴", Command: "掉落图鉴", Input: "掉落图鉴 [来源] [页码]"},
		{ID: 1121, Category: "修炼", Name: "境界图鉴", Command: "境界图鉴", Input: "境界图鉴 [页码]"},
		{ID: 1121, Category: "图鉴", Name: "扩展道藏图鉴", Command: "道藏图鉴", Input: "道藏图鉴 类别 [页码]"},
		{ID: 1122, Category: "图鉴", Name: "图鉴详情", Command: "图鉴详情", Input: "图鉴详情 类别 名称"},
		{ID: 1122, Category: "装备", Name: "装备详情", Command: "装备详情", Input: "装备详情 装备名"},
		{ID: 1122, Category: "装备", Name: "器谱详情", Command: "器谱详情", Input: "器谱详情 器谱名"},
		{ID: 1140, Category: "交易", Name: "钱庄存款", Command: "存款", Input: "存款 [银币/灵石] 数量"},
		{ID: 1141, Category: "交易", Name: "钱庄取款", Command: "取款", Input: "取款 [银币/灵石] 数量"},
		{ID: 1142, Category: "交易", Name: "银币借款", Command: "借款", Input: "借款 数量"},
		{ID: 1143, Category: "交易", Name: "银币还款", Command: "还款", Input: "还款 数量"},
		{ID: 1144, Category: "交易", Name: "钱庄账簿", Command: "钱庄账簿", Input: "钱庄账簿 [页码]"},
		{ID: 1145, Category: "交易", Name: "钱庄规则", Command: "钱庄规则", Input: "钱庄规则"},
		{ID: 1150, Category: "装备", Name: "激活器物图鉴", Command: "装备激活", Input: "装备激活 [装备名]"},
		{ID: 1150, Category: "装备", Name: "一键激活器物", Command: "装备一键激活", Input: "装备一键激活"},
		{ID: 1151, Category: "装备", Name: "仙盟器阁", Command: "仙盟器阁", Input: "仙盟器阁 [槽位] [页码]"},
		{ID: 1151, Category: "装备", Name: "仙盟器阁", Command: "仙盟商店", Input: "仙盟商店 [槽位] [页码]"},
		{ID: 1152, Category: "装备", Name: "器阁兑购", Command: "兑购", Input: "兑购 装备名*数量"},
		{ID: 1153, Category: "装备", Name: "法器传承", Command: "法器传承", Input: "法器传承 对方道号 装备名"},
		{ID: 1153, Category: "装备", Name: "法器传承", Command: "传送装备", Input: "传送装备 对方道号 装备名"},
		{ID: 1154, Category: "装备", Name: "套装大全", Command: "套装大全", Input: "套装大全 [页码]"},
		{ID: 1155, Category: "装备", Name: "套装查询", Command: "套装查询", Input: "套装查询 套装名"},
		{ID: 1156, Category: "装备", Name: "当前套装", Command: "当前套装", Input: "当前套装"},
		{ID: 1157, Category: "装备", Name: "法器分解", Command: "装备分解", Input: "装备分解 装备名"},
		{ID: 1158, Category: "装备", Name: "批量法器分解", Command: "装备一键分解", Input: "装备一键分解 品质 确认"},
		{ID: 1159, Category: "装备", Name: "开辟灵孔", Command: "装备开孔", Input: "装备开孔 装备名"},
		{ID: 1160, Category: "装备", Name: "嵌灵宝石", Command: "装备镶嵌", Input: "装备镶嵌 装备名 宝石名"},
		{ID: 1160, Category: "装备", Name: "嵌灵宝石", Command: "嵌灵", Input: "嵌灵 装备名 宝石名"},
		{ID: 1161, Category: "装备", Name: "取下宝石", Command: "装备摘孔", Input: "装备摘孔 装备名 [宝石名]"},
		{ID: 1161, Category: "装备", Name: "取下宝石", Command: "取灵", Input: "取灵 装备名 [宝石名]"},
		{ID: 1162, Category: "装备", Name: "一键取灵", Command: "一键摘孔", Input: "一键摘孔 装备名"},
		{ID: 1163, Category: "装备", Name: "嵌灵宝石图鉴", Command: "宝石查询", Input: "宝石查询 [页码]"},
		{ID: 1163, Category: "装备", Name: "嵌灵宝石图鉴", Command: "宝石图鉴", Input: "宝石图鉴 [页码]"},
		{ID: 1164, Category: "装备", Name: "引星淬器", Command: "装备星化", Input: "装备星化 装备名"},
		{ID: 1165, Category: "装备", Name: "投入玄火炉", Command: "投入熔炉", Input: "投入熔炉 装备名"},
		{ID: 1166, Category: "装备", Name: "取出玄火炉", Command: "熔炉取出", Input: "熔炉取出 装备名"},
		{ID: 1167, Category: "装备", Name: "法器融合", Command: "融合装备", Input: "融合装备 主装备=副装备"},
		{ID: 1168, Category: "装备", Name: "装备完整帮助", Command: "装备帮助", Input: "装备帮助"},
		{ID: 1017, Category: "装备", Name: "卸下装备", Command: "脱下装备", Input: "脱下装备 装备名/槽位"},
		{ID: 120, Category: "装备", Name: "强化装备", Command: "强化装备", Input: "强化装备 装备名"},
		{ID: 1019, Category: "装备", Name: "锻造装备", Command: "锻造装备", Input: "锻造装备 装备名"},
		{ID: 1020, Category: "装备", Name: "灵纹铭刻", Command: "装备铭刻", Input: "装备铭刻 装备名"},
		{ID: 1122, Category: "装备", Name: "查询装备", Command: "查询装备", Input: "查询装备 装备名"},
		{ID: 1169, Category: "交易", Name: "仙盟回收", Command: "出售", Input: "出售 物品名*数量"},
		{ID: 1169, Category: "交易", Name: "仙盟回收", Command: "仙盟回收", Input: "仙盟回收 物品名*数量"},
		{ID: 1170, Category: "交易", Name: "道友转账", Command: "转账", Input: "转账 对方道号 [银币/灵石] 数量"},
		{ID: 1171, Category: "生涯", Name: "我的称号", Command: "我的称号", Input: "我的称号 [页码]"},
		{ID: 1172, Category: "生涯", Name: "解锁称号", Command: "激活称号", Input: "激活称号 称号名"},
		{ID: 1173, Category: "生涯", Name: "佩戴称号", Command: "佩戴称号", Input: "佩戴称号 称号名"},
		{ID: 1174, Category: "生涯", Name: "卸下称号", Command: "卸下称号", Input: "卸下称号"},
		{ID: 1175, Category: "地图", Name: "人物仙商", Command: "NPC商店", Input: "NPC商店 [NPC名] [页码]"},
		{ID: 1175, Category: "地图", Name: "人物仙商", Command: "npc商店", Input: "npc商店 [NPC名] [页码]"},
		{ID: 1176, Category: "地图", Name: "人物赠礼", Command: "NPC赠送", Input: "NPC赠送 [NPC名] 物品名*数量"},
		{ID: 1176, Category: "地图", Name: "人物赠礼", Command: "npc赠送", Input: "npc赠送 [NPC名] 物品名*数量"},
		{ID: 1177, Category: "地图", Name: "人物仙商购买", Command: "NPC购买", Input: "NPC购买 商品名*数量"},
		{ID: 1177, Category: "地图", Name: "人物仙商购买", Command: "npc购买", Input: "npc购买 商品名*数量"},
		{ID: 1178, Category: "地图", Name: "人物关系", Command: "NPC关系", Input: "NPC关系 [NPC名]"},
		{ID: 1179, Category: "仙府", Name: "灵田天象", Command: "灵田天象", Input: "灵田天象"},
		{ID: 1180, Category: "仙府", Name: "护持灵田", Command: "护持灵田", Input: "护持灵田 [地块/全部]"},
		{ID: 1181, Category: "仙府", Name: "灵田灾异录", Command: "灵田灾异录", Input: "灵田灾异录 [页码]"},
		{ID: 1182, Category: "系统", Name: "仙尘运行状态", Command: "运行状态", Input: "运行状态"},
		{ID: 1182, Category: "系统", Name: "仙尘运行状态", Command: "插件状态", Input: "插件状态"},
		{ID: 1183, Category: "交易", Name: "确认易物", Command: "确认易物", Input: "确认易物 申请编号"},
		{ID: 1183, Category: "交易", Name: "确认易物", Command: "同意易物", Input: "同意易物 申请编号"},
		{ID: 1184, Category: "交易", Name: "拒绝或撤回易物", Command: "拒绝易物", Input: "拒绝易物 申请编号"},
		{ID: 1184, Category: "交易", Name: "拒绝或撤回易物", Command: "取消易物", Input: "取消易物 申请编号"},
		{ID: 1185, Category: "交易", Name: "易物申请列表", Command: "易物请求", Input: "易物请求 [页码]"},
		{ID: 1186, Category: "地图", Name: "山河传送阵", Command: "传送阵", Input: "传送阵"},
		{ID: 1187, Category: "地图", Name: "当前界域传送阵图", Command: "传送列表", Input: "传送列表 [界域] [页码]"},
		{ID: 1187, Category: "地图", Name: "十界开放状态", Command: "诸界列表", Input: "诸界列表"},
		{ID: 1187, Category: "地图", Name: "十界开放状态", Command: "世界列表", Input: "世界列表"},
		{ID: 1188, Category: "地图", Name: "界内阵法传送", Command: "传送", Input: "传送 地点"},
		{ID: 1188, Category: "地图", Name: "跨界门接引", Command: "跨界传送", Input: "跨界传送 界域/界门"},
		{ID: 1189, Category: "角色", Name: "道籍迁移", Command: "道籍迁移", Input: "道籍迁移"},
		{ID: 1189, Category: "角色", Name: "生成迁移凭证", Command: "生成迁移码", Input: "生成迁移码"},
		{ID: 1190, Category: "角色", Name: "接管旧道籍", Command: "迁入道籍", Input: "迁入道籍 凭证"},
		{ID: 1191, Category: "系统", Name: "申请群接入", Command: "申请入驻", Input: "申请入驻 群名/用途"},
		{ID: 1191, Category: "系统", Name: "申请群接入", Command: "申请群审核", Input: "申请群审核 群名/用途"},
		{ID: 1191, Category: "系统", Name: "查看群审核", Command: "群审核状态", Input: "群审核状态"},
		{ID: 1192, Category: "仙府", Name: "田垄施灵肥", Command: "施肥", Input: "施肥 地块 [灵肥名]"},
		{ID: 1193, Category: "仙府", Name: "一键施灵肥", Command: "一键施肥", Input: "一键施肥 [灵肥名]"},
		{ID: 1194, Category: "仙府", Name: "灵肥图鉴", Command: "灵肥图鉴", Input: "灵肥图鉴 [页码]"},
		{ID: 1195, Category: "功法", Name: "上传自创功法", Command: "上传功法", Input: "上传功法 功法名"},
		{ID: 1195, Category: "功法", Name: "撤下自创功法", Command: "撤下功法", Input: "撤下功法 功法名"},
		{ID: 1196, Category: "功法", Name: "全服功法分享", Command: "功法分享", Input: "功法分享 [流派] [页码]"},
		{ID: 1197, Category: "功法", Name: "我的自创功法", Command: "我的创功", Input: "我的创功 [页码]"},
		{ID: 1198, Category: "角色", Name: "角色等级", Command: "等级", Input: "等级"},
		{ID: 113, Category: "丹药", Name: "当前药效", Command: "当前药效", Input: "当前药效"},
		{ID: 1199, Category: "仙府", Name: "一键清除杂灵草", Command: "一键除草", Input: "一键除草"},
		{ID: 1200, Category: "仙府", Name: "一键驱除噬灵虫", Command: "一键除虫", Input: "一键除虫"},
		{ID: 1201, Category: "氪金", Name: "累计充值查询", Command: "累充", Input: "累充"},
		{ID: 1201, Category: "氪金", Name: "累计充值查询", Command: "发累充", Input: "发累充"},
		{ID: 1202, Category: "交易", Name: "每日神秘商城", Command: "神秘商城", Input: "神秘商城"},
		{ID: 1203, Category: "交易", Name: "神秘商城购买", Command: "神秘购买", Input: "神秘购买 物品名*数量"},
		{ID: 1204, Category: "交易", Name: "六时辰限时商城", Command: "限时商城", Input: "限时商城"},
		{ID: 1205, Category: "交易", Name: "限时商城购买", Command: "限时购买", Input: "限时购买 物品名*数量"},
		{ID: 1206, Category: "生辰", Name: "生日专属菜单", Command: "生日菜单", Input: "生日菜单"},
		{ID: 1206, Category: "生辰", Name: "生日专属菜单", Command: "生辰菜单", Input: "生辰菜单"},
		{ID: 1206, Category: "生辰", Name: "生日专属菜单", Command: "生日", Input: "生日"},
		{ID: 1207, Category: "角色", Name: "登记生日", Command: "设置生日", Input: "设置生日 月-日"},
		{ID: 1208, Category: "生辰", Name: "生辰签到", Command: "生辰签到", Input: "生辰签到"},
		{ID: 1209, Category: "生辰", Name: "领取仙尘生日礼物", Command: "领取生日礼物", Input: "领取生日礼物"},
		{ID: 1209, Category: "生辰", Name: "领取仙尘生日礼物", Command: "领取生辰礼", Input: "领取生辰礼"},
		{ID: 1210, Category: "生辰", Name: "今日寿星", Command: "今日寿星", Input: "今日寿星"},
		{ID: 1211, Category: "生辰", Name: "道友生日祝福", Command: "生日祝福", Input: "生日祝福 @寿星 祝福语"},
		{ID: 1212, Category: "生辰", Name: "道友生日赠礼", Command: "生日赠礼", Input: "生日赠礼 @寿星 物品名*数量"},
		{ID: 1213, Category: "生辰", Name: "生辰限定抽奖", Command: "生辰抽奖", Input: "生辰抽奖 [次数]"},
		{ID: 1213, Category: "生辰", Name: "生辰限定抽奖", Command: "生日抽奖", Input: "生日抽奖 [次数]"},
		{ID: 1214, Category: "生辰", Name: "生辰福签兑换", Command: "生辰兑换", Input: "生辰兑换 [物品名*数量]"},
		{ID: 1215, Category: "生辰", Name: "生日专属任务", Command: "生日任务", Input: "生日任务"},
		{ID: 1216, Category: "生辰", Name: "领取生日任务", Command: "领取生日任务", Input: "领取生日任务 任务名"},
		{ID: 1217, Category: "生辰", Name: "寿星福缘榜", Command: "寿星榜", Input: "寿星榜 [页码]"},
		{ID: 1218, Category: "活动", Name: "全服补偿", Command: "全服补偿", Input: "全服补偿"},
		{ID: 1218, Category: "活动", Name: "全服补偿公告", Command: "补偿公告", Input: "补偿公告"},
		{ID: 1218, Category: "活动", Name: "领取全服补偿", Command: "领取全服补偿", Input: "领取全服补偿"},
	}
}

// ParseCommand only accepts a bare command as the first token. Prefix forms such
// as #状态 and /状态 are intentionally not normalized or accepted.
func ParseCommand(message string) (ParsedCommand, bool) {
	message = strings.TrimSpace(strings.ReplaceAll(message, "　", " "))
	if message == "" {
		return ParsedCommand{}, false
	}

	parts := strings.Fields(message)
	specs := commandIndex[parts[0]]
	if len(specs) == 0 && strings.HasSuffix(parts[0], "图鉴") && parts[0] != "图鉴" {
		specs = []CommandSpec{{
			ID: 1121, Category: "图鉴", Name: "分类道藏图鉴",
			Command: parts[0], Input: parts[0] + " [筛选] [页码]",
		}}
	}
	if len(specs) == 0 {
		return ParsedCommand{}, false
	}

	selected := specs[0]
	if parts[0] == "传功" && len(parts) >= 3 && !isInteger(parts[len(parts)-1]) {
		for _, spec := range specs {
			if spec.ID == 74 {
				selected = spec
				break
			}
		}
	}
	if parts[0] == "布阵" && len(parts) >= 2 {
		selected = selectCommandID(specs, 142, selected)
	}
	if parts[0] == "天机" {
		selected = selectCommandID(specs, 213, selected)
	}
	if parts[0] == "宗战" && len(parts) >= 2 {
		selected = selectCommandID(specs, 227, selected)
	}

	raw := ""
	if selected.ID == 1000 && selected.Command != "菜单" && selected.Command != "功能菜单" {
		raw = selected.Category
		if len(parts) > 1 {
			raw += " " + strings.Join(parts[1:], " ")
		}
	} else if len(parts) > 1 {
		raw = strings.Join(parts[1:], " ")
	}
	return ParsedCommand{Spec: selected, Arguments: parts[1:], RawArguments: raw}, true
}

func selectCommandID(specs []CommandSpec, id int, fallback CommandSpec) CommandSpec {
	for _, spec := range specs {
		if spec.ID == id {
			return spec
		}
	}
	return fallback
}

func isInteger(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
