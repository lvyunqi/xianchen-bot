package storage

import (
	"strconv"
	"strings"
	"time"

	"xianlv/internal/model"
)

// ensureCriticalRuntimeSchema is intentionally small and runs even when an
// operator copied a database whose schema marker was newer than its actual
// tables. It avoids the expensive full 1000-entry catalogue seed on every
// plugin load while guaranteeing that one-time compensation receipts exist.
func (s *Store) ensureCriticalRuntimeSchema() error {
	return s.DB.AutoMigrate(&model.AccountRewardClaim{})
}

func v222CompensationNotice() model.Notice {
	published := time.Date(2026, time.July, 24, 18, 30, 0, 0, time.Local)
	return model.Notice{
		Code:  "world_notice_v222_compensation_20260724",
		Title: "仙尘 v2.2.2 全服补偿公告",
		Content: "【发放缘由】本次针对角色等级基础属性被旧版渡劫覆盖、装备与榜位称号重复扣除属性，以及插件载入后公告重试造成群聊和设置页面卡顿的问题，发放独立批次“万象归元礼”。属性回正由程序自动完成，补偿不会用异常数值覆盖道籍。\n\n" +
			"【领取对象】2026年7月24日23时59分59秒（北京时间）前已经建立道籍的玩家。符合范围且仍保留原道籍的玩家长期拥有补领资格；本批次与旧补偿相互独立。\n\n" +
			"【领取方式】发送“全服补偿”查看资格和完整清单，再发送“领取全服补偿”。每个平台账号与原角色各限一次；换OpenID、删号重建或重复点击都不会重置。\n\n" +
			"【货币】灵石×88888、银币×3888、功德×88、声望×88。\n\n" +
			"【珍稀与炼器】万象归元纪念令×1、月华问道礼匣×2、灵根精粹×12、龙血芝×8、龙血芝孢子×8、玄铁×188、阵基石×88、星辰砂×66、雷灵晶×36、妖兽内丹×128。\n\n" +
			"【宝石与修行】九霄雷罡石×5、星河道力核×5、混元五炁珠×3、功法残卷×30、双倍修为卡×8、扫荡券×50、传送符×30。\n\n" +
			"【丹药与护道】回元散×99、回灵丹×99、聚灵丹×50、淬脉丹×30、凝元丹×25、破境丹×20、九转还魂丹×3、轮回丹×1、避劫符×8、引劫玉符×5。\n\n" +
			"【灵田】造化仙壤×20、地脉灵肥×50、灵壤肥×99。\n\n" +
			"【领取保障】领取凭证、货币和全部物品在同一数据库事务写入；任何一项缺失或写入失败都会整体回滚，不会出现只领一半。本批次不发仙金，也不直接叠加战力。",
		Type: "公告", Pinned: true, Published: true, PublishedAt: &published,
	}
}

func v222RuntimeNotices() []model.Notice {
	compensation := v222CompensationNotice()
	worldAt := time.Date(2026, time.July, 24, 18, 33, 0, 0, time.Local)
	updateAt := time.Date(2026, time.July, 24, 18, 32, 0, 0, time.Local)
	repairAt := time.Date(2026, time.July, 24, 18, 31, 0, 0, time.Local)
	return []model.Notice{
		compensation,
		{
			Code: "world_notice_v222_player_20260724", Title: "仙尘万象归元新章",
			Content: "仙尘 v2.2.2 已开启。\n━━━━━━━━━━━\n" +
				"角色等级基础成长现统一为每级气血+24、双攻各+7、双防各+5；受旧版影响的道籍会自动回正，已有更高属性不会降低。\n" +
				"装备已经按真实十个槽位结算，同槽替换、穿脱、锻造、套装与榜位尊号不会再把基础属性反复扣到1。\n" +
				"群内指令与设置页已解除失效群公告重试造成的等待；数据列表改为按页读取。\n" +
				"符合范围的旧道籍可领取全新“万象归元礼”，奖励与旧补偿相互独立。\n━━━━━━━━━━━\n" +
				"发送“全服补偿”查看资格与全部奖励；发送“修复公告”查看完整处理内容。",
			Type: "公告", Pinned: true, Published: true, PublishedAt: &worldAt,
		},
		{
			Code: "update_v222_player_20260724", Title: "仙尘 v2.2.2 内容更新",
			Content: "【等级成长】每次角色升级固定获得气血+24、物攻/法强各+7、物防/法防各+5；法力与五维仍按原阶段规则成长。等级与境界继续彼此独立。\n\n" +
				"【属性回正】旧道籍首次载入新版本时会检查等级应得基础属性，只补足缺口、不覆盖更高值；战力在回正后重新计算。\n\n" +
				"【装备体系】法器按本命法器、冠冕、项链、道袍、护腕、腰佩、戒指、护符、灵靴等真实槽位归类；器型与槽位分开显示。锻造、星化、铭刻、宝石和套装均保留。\n\n" +
				"【运行体验】系统参数、功能开关等大量内容改为分页与搜索；版本告示只在当前活跃群被动送达一次，失败后限时退避，不再拖慢每条指令。\n\n" +
				"【万象归元礼】新补偿为独立批次，包含货币、炼器材料、高阶丹药、宝石、灵田物资与纪念令；发送“全服补偿”查看完整清单。",
			Type: "更新", Pinned: true, Published: true, PublishedAt: &updateAt,
		},
		{
			Code: "repair_v222_attributes_runtime_equipment_20260724", Title: "仙尘 v2.2.2 修复说明",
			Content: "【属性归1】修复旧版渡劫升大境时直接用新境界模板覆盖角色总属性的问题。新结算只增加境界差值，并强制保留等级、灵根、传承、装备、称号及其他永久成长。\n\n" +
				"【反复扣除】修复装备穿脱、同槽替换、套装共鸣和榜位称号失效时按旧账本重复扣除的问题。称号属性、当前称号与账本现于同一事务提交；失败会全部回滚。\n\n" +
				"【等级底线】所有装备、称号、灵根替换与渡劫扣减路径都不能低于当前角色等级的基础属性。LV168最低基础为双攻1179、双防840、气血上限4108，之后仍叠加其他来源。\n\n" +
				"【装备槽位】修复批量生成装备全部显示本命法器的问题；葫芦归腰佩、飞舟归灵靴，其余器型按十槽位稳定分配，已有锻造、品质、星阶、灵孔与铭刻不丢失。\n\n" +
				"【载入卡顿】修复每条群消息到来前向多个失效旧群串行重发版本告示的问题；同时将海量系统参数由一次载入改为服务端分页，设置页不再渲染数千行。\n\n" +
				"【数据保护】本次为增量回正，不重置境界、等级、修为、背包、货币、装备、功法、灵兽、灵田、宗门或社交数据。",
			Type: "修复", Pinned: true, Published: true, PublishedAt: &repairAt,
		},
	}
}

func (s *Store) ensureRuntimeHotfixContent() error {
	if err := s.DB.Model(&model.Activity{}).
		Where("code = ?", "xianchen_activity_v221_compensation").
		Updates(map[string]any{
			"name":   "万象归元全服补偿",
			"effect": "向2026年7月24日结束前已经建立道籍的玩家发放一次性万象归元修复补偿。",
		}).Error; err != nil {
		return err
	}
	item := model.Item{
		Code: "v222_runtime_repair_memorial_token", Name: "万象归元纪念令",
		CategoryName: "任务物品", RarityName: "神品",
		Description: "仙尘 v2.2.2 属性、装备与运行优化全服补偿的永久纪念凭证。不增加战力、不可交易、不可出售。",
		EffectType:  "纪念", BaseValue: 0, StackLimit: 1, Stackable: false, Tradable: false,
	}
	if err := s.DB.Where("code = ?", item.Code).Assign(map[string]any{
		"name": item.Name, "category_name": item.CategoryName, "rarity_name": item.RarityName,
		"description": item.Description, "effect_type": item.EffectType, "base_value": int64(0),
		"stack_limit": int64(1), "stackable": false, "tradable": false,
	}).FirstOrCreate(&item).Error; err != nil {
		return err
	}
	for _, notice := range v222RuntimeNotices() {
		if err := s.DB.Where("code = ?", notice.Code).Assign(map[string]any{
			"title": notice.Title, "content": notice.Content, "type": notice.Type,
			"pinned": notice.Pinned, "published": notice.Published, "published_at": notice.PublishedAt,
		}).FirstOrCreate(&notice).Error; err != nil {
			return err
		}
	}
	return s.pruneCatalogContent()
}

// pruneCatalogContent 清理超出目录规模的样板公告与邮件（幂等，每次启动执行）。
// 只删除纯文本内容类目录条目；物品、地图等被玩家数据引用的目录不受影响。
func (s *Store) pruneCatalogContent() error {
	limit := contentSeedLimit()
	var staleNotices []model.Notice
	if err := s.DB.Where("code LIKE ?", "catalog_notice_%").Find(&staleNotices).Error; err != nil {
		return err
	}
	for _, notice := range staleNotices {
		if catalogSequence(notice.Code) > limit {
			if err := s.DB.Delete(&notice).Error; err != nil {
				return err
			}
		}
	}
	var staleMails []model.Mail
	if err := s.DB.Where("code LIKE ?", "catalog_mail_%").Find(&staleMails).Error; err != nil {
		return err
	}
	for _, mail := range staleMails {
		if catalogSequence(mail.Code) > limit {
			if err := s.DB.Delete(&mail).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// catalogSequence 提取 catalog_xxx_<n> 里的序号；无法解析时返回 0（视为保留）。
func catalogSequence(code string) int {
	idx := strings.LastIndex(code, "_")
	if idx < 0 || idx+1 >= len(code) {
		return 0
	}
	n, err := strconv.Atoi(code[idx+1:])
	if err != nil {
		return 0
	}
	return n
}
