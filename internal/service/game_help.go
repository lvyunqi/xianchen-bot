package service

import (
	"fmt"
	"strconv"
	"strings"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

var helpCategoryOrder = []string{
	"角色", "修炼", "探索", "地图", "挂机", "道侣", "战斗", "渡劫", "仙府", "功法",
	"灵兽", "装备", "社交", "交易", "任务", "活动", "特殊", "宗门", "丹药", "炼器", "副本", "竞技",
	"奇遇", "生涯", "阵法", "符箓", "傀儡", "秘境争夺", "传承", "悟道", "仙魔战场", "灵根进化",
	"渡劫心魔", "合体技", "仙药培育", "法宝炼化", "天机推演", "天地灵脉", "宗门战争", "仙缘奇遇", "宇宙星河", "图鉴", "系统",
}

func (g *Game) helpGuide(player *model.Player, raw string) (GameResult, bool, error) {
	query, page := parseHelpRequest(raw)
	if query == "" || query == "怎么玩" || query == "玩法指南" {
		return g.helpOverview(player), true, nil
	}
	if query == "全部" || query == "指令大全" || query == "所有" {
		return g.helpCommandCatalog(player, "", page), true, nil
	}
	query = strings.TrimSuffix(query, "菜单")
	for _, category := range helpCategoryOrder {
		if query == category {
			return g.helpCommandCatalog(player, category, page), true, nil
		}
	}
	if query == "管理" {
		if _, _, authorized := g.gmAuthority(player.AccountID); authorized {
			return g.helpCommandCatalog(player, "管理", page), true, nil
		}
		return GameResult{Title: "管理帮助不可用", Content: "管理指令仅主人和已授权管理员可查看。普通玩家可发送 `指令大全` 查看全部玩家功能。", Actions: []string{"指令大全", "系统菜单", "功能菜单"}}, true, nil
	}
	return GameResult{Title: "帮助分类不存在", Content: "没有找到“" + query + "”分类。可发送 `帮助` 查看完整玩法路线，或发送 `指令大全` 分页查看全部命令。", Actions: []string{"帮助", "指令大全", "功能菜单"}}, true, nil
}

func (g *Game) helpOverview(player *model.Player) GameResult {
	categories := append([]string(nil), helpCategoryOrder...)
	if _, _, authorized := g.gmAuthority(player.AccountID); authorized {
		categories = append(categories, "管理")
	}
	categoryMenus := make([]string, 0, len(categories))
	for _, category := range categories {
		categoryMenus = append(categoryMenus, category+"菜单")
	}
	lines := []string{
		fmt.Sprintf("道友：%s · %s%d层 · 当前位于%s", player.DaoName, player.RealmName, player.RealmLevel, displayOr(player.Location, "未知之地")),
		"所有指令均不需要前缀，消息中的蓝字可以直接点击。",
		"━━━━━━━━━━━",
		"【从零开始】",
		"一、发送“入道 道号 男/女”建立全服唯一道籍，随后发送“开启礼包 青云入道礼匣”；旧角色发送“性别 男/女”补录。",
		"二、发送“状态”“灵检”“背包”“位置”，认识属性、灵根、资源与当前地图。",
		"运气初始10、上限50；发送“运气”查看它对奇遇、寻宝、捕获、炼丹、合成与遇仙的实时概率加成。",
		"三、发送“签到”“活动菜单”“日常”“公告”和“通知”，领取每日资源、查看个人未读并选择当前段位能完成的任务。",
		"四、在地图蓝字中对话、接任务、采集、挑战妖兽；到访带阵地点会刻录阵纹，发送“传送阵”“传送列表”“诸界列表”查看界内与跨界通路。",
		"五、发送“修炼”开始真实计时，达到最低时间后发送“出关”；修为、道心、气血、法力和前置丹药齐备后再突破。",
		"六、每个大境必须从一层逐层修至十层，十层圆满后先备劫，再消耗引劫玉符挑战三道天劫。",
		"━━━━━━━━━━━",
		"【日常成长循环】",
		"签到 → 活动菜单 → 日常/悬赏 → 位置 → 对话/接任务 → 采集/挑战 → 交任务 → 修炼/出关 → 合成破境材料 → 突破。",
		"活动中的七日目标、境界冲刺、密令、邀请、新秀榜、鸿运、祈福与特卖均有独立状态和真实奖励，发送“活动总览”查看剩余时间。",
		"体力基础上限100、每分钟自动恢复10点；每提升一个大境界，上限增加100、每分钟恢复增加10，恢复速度不设上限。在线离线都会按时间恢复，无需打坐；发送“体力”查看详情。",
		"体力不足时可闭关、经营灵田、整理装备、炼丹炼器、修习功法或参与社交玩法。",
		"十座正式界域各有一千处地图。步行必须遵循相邻路线；已刻录阵点可消耗传送符界内挪移，跨界需从界门出发，未解锁世界会显示所需境界且不会扣道具。",
		"━━━━━━━━━━━",
		"【战斗与副本】",
		"地图妖兽、区域首领、PVP和副本均使用回合制。进入战斗后不要重复发送挑战，按回合选择攻击、技能 功法名、防御或投降。",
		"自创功法支持剑道、术法、炼体、神魂、遁法与均衡六种真实流派；创成后默认私藏，发送“上传功法 功法名”才会供全服学习，可用“我的创功”和“功法分享”管理。悟性、道心、已创数量与材料决定难度，运气会提高推演成功率。",
		"灵兽不能连续凭空捕获：先在当前地图探索到“灵兽现踪”，再于十分钟内用灵兽口粮尝试一次；喂养会增加忠诚与灵悟经验，经验进度条满后按该灵兽独立成长值升级；长期不喂养会触发焦躁、拒战、反噬和叛变。",
		"挂机格式：挂机 猎妖*次数，或挂机 副本名*次数；发送“领取挂机”或“挂机结算”领取已完成轮次，发送“结束挂机”领取后停止。单独“收获”只用于仙府灵田。",
		"━━━━━━━━━━━",
		"【物品、制作与经济】",
		"查看物品：物品 名称；搜索背包：背包搜索 关键词；批量使用：使用 物品名*数量。",
		"炼丹、炼器、合成、装备、灵田、仙药与法宝均先查看图鉴或菜单，再按材料和前置条件操作。",
		"灵根合成：发送“灵根合成 灵根A 灵根B”，两种父系必须不同；结果先成为待定道种，再发送“吸收灵根”或“放弃灵根”。",
		"货铺、种子商店、银币商城、仙金商城与竞技商店全部常设不限购，购买量只受实际余额和安全数值范围约束。",
		"玩家易物：发送“易物 @对方 我的物品*数量 对方物品*数量”只会建立申请；对方必须发送“确认易物 编号”才会原子成交，也可拒绝或由发起人撤回。",
		"━━━━━━━━━━━",
		"【通用输入规则】",
		"指定目标：使用道号或点击消息中的目标蓝字；不要输入内部数据库ID。",
		"批量数量：使用“名称*数量”，数量没有游戏上限，但必须拥有足够资源且不能超过系统安全数值范围。",
		"长列表翻页：在原命令后加页码，例如“指令大全 2”“灵根图鉴 3”。",
		"自定义短令：设置快捷 别名=完整指令；发送“快捷列表”查看、执行或删除本人快捷。",
		"个人信箱：发送“通知 [页码]”查看留言、密语、仙缘请求、易物申请、道号传承和问剑结果；发送“通知未读”只看未读。",
		"问题与建议：发送“反馈菜单”；有效且非重复的BUG或可行建议会进入仙盟审核台并立即发放提交奖励。已确认修复只在“修复公告”中单独发布。",
		"━━━━━━━━━━━",
		fmt.Sprintf("【全部系统入口 · 共%d类】", len(categories)),
	}
	markdownLines := append([]string(nil), lines...)
	lines = append(lines, pairPlainLines(categoryMenus)...)
	markdownMenus := make([]string, 0, len(categoryMenus))
	for _, menu := range categoryMenus {
		markdownMenus = append(markdownMenus, markdownInlineCommand(menu))
	}
	markdownLines = append(markdownLines, pairPlainLines(markdownMenus)...)
	footer := []string{
		"━━━━━━━━━━━",
		"查某一类：帮助 分类，例如“帮助 炼器”。",
		"查全部指令：指令大全 [页码]。每条都会列出功能名称和准确输入格式。",
	}
	lines = append(lines, footer...)
	markdownLines = append(markdownLines, footer...)
	return GameResult{Title: "仙尘完整玩法指南", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: []string{"仙尘介绍", "游戏介绍", "大世界", "体力", "指令大全", "帮助 角色", "帮助 修炼", "帮助 地图", "帮助 战斗", "帮助 副本", "帮助 炼器", "帮助 活动", "帮助 系统", "活动菜单", "快捷列表", "反馈菜单", "修复公告", "功能菜单"}}
}

func (g *Game) helpCommandCatalog(player *model.Player, category string, page int) GameResult {
	const pageSize = 10
	specs := g.playerHelpSpecs(player, category)
	pages := maxInt((len(specs)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(specs))
	title := "全部指令大全"
	if category != "" {
		title = category + "操作指南"
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d项功能", page, pages, len(specs)), "所有指令均不带前缀；参数之间使用空格，数量可使用“*数量”。", "━━━━━━━━━━━"}
	markdownLines := append([]string(nil), lines...)
	actions := []string{"帮助", "功能菜单"}
	for index := start; index < end; index++ {
		spec := specs[index]
		lines = append(lines, fmt.Sprintf("%d. 【%s】\n发送：%s\n用途：%s", index+1, spec.Name, spec.Input, helpUsage(spec)))
		markdownLines = append(markdownLines, fmt.Sprintf("%d. 【%s】\n发送：%s\n用途：%s", index+1, spec.Name, markdownInlineCommand(spec.Input, spec.Command), helpUsage(spec)))
		if index+1 < end {
			lines = append(lines, "━━━━━━━")
			markdownLines = append(markdownLines, "━━━━━━━")
		}
		actions = append(actions, spec.Command)
	}
	if len(specs) == 0 {
		lines = append(lines, "该分类暂未注册玩家指令。")
		markdownLines = append(markdownLines, "该分类暂未注册玩家指令。")
	}
	pageCommand := "指令大全"
	if category != "" {
		pageCommand = "帮助 " + category
		actions = append(actions, category+"菜单", "指令大全")
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", pageCommand, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", pageCommand, page+1))
	}
	return GameResult{Title: title, Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: actions}
}

func (g *Game) playerHelpSpecs(player *model.Player, category string) []handler.CommandSpec {
	authorized := false
	if player != nil {
		_, _, authorized = g.gmAuthority(player.AccountID)
	}
	all := append([]handler.CommandSpec(nil), handler.CommandTable...)
	all = append(all, handler.AuxiliaryCommands()...)
	seen := make(map[string]struct{})
	result := make([]handler.CommandSpec, 0, len(all))
	for _, spec := range all {
		if spec.EventOnly || spec.Input == "" || spec.Input == "—" || spec.Category == "管理" && !authorized {
			continue
		}
		if category != "" && spec.Category != category {
			continue
		}
		key := spec.Category + "\x00" + spec.Command + "\x00" + spec.Input
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, spec)
	}
	return result
}

func parseHelpRequest(raw string) (string, int) {
	parts := strings.Fields(strings.TrimSpace(raw))
	page := 1
	if len(parts) > 0 {
		if value, err := strconv.Atoi(parts[len(parts)-1]); err == nil && value > 0 {
			page = value
			parts = parts[:len(parts)-1]
		}
	}
	query := strings.Join(parts, " ")
	if query == "" && strings.TrimSpace(raw) != "" {
		query = "全部"
	}
	return query, page
}

func helpUsage(spec handler.CommandSpec) string {
	if strings.TrimSpace(spec.Name) != "" {
		return spec.Name + "；参数缺失时会返回可点击操作引导。"
	}
	return "执行对应玩法；参数缺失时会返回可点击操作引导。"
}
