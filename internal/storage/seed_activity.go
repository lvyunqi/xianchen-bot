package storage

import (
	"time"

	"xianlv/internal/model"
)

// seedActivityContent provides the operational rows consumed by the player
// activity center. Existing rows are deliberately preserved so operators can
// change activity windows and descriptions without a later migration undoing
// those choices.
func (s *Store) seedActivityContent() error {
	now := time.Now()
	startsAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	standardEnd := startsAt.AddDate(0, 2, 0)
	longEnd := startsAt.AddDate(1, 0, 0)
	saleEnd := startsAt.AddDate(0, 1, 0)
	activities := []model.Activity{
		{Code: "xianchen_activity_v221_compensation", Name: "万象归元全服补偿", Type: "版本补偿", StartsAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.Local), EndsAt: time.Date(2035, 12, 31, 23, 59, 59, 0, time.Local), Effect: "向2026年7月24日结束前已经建立道籍的玩家发放一次性修复补偿。", EffectJSON: `{"eligibility_cutoff":"2026-07-24T23:59:59+08:00","claim":"once_per_account","paid_currency":false}`, Status: "进行中"},
		{Code: "xianchen_activity_seven_goals", Name: "七曜问道目标", Type: "七日成长", StartsAt: startsAt, EndsAt: standardEnd, Effect: "入道前七日逐日解锁目标，未领取奖励保留至第十四日。", EffectJSON: `{"unlock":"registration_day","claim_deadline_days":14}`, Status: "进行中"},
		{Code: "xianchen_activity_realm_sprint", Name: "万境登仙冲刺", Type: "境界冲刺", StartsAt: startsAt, EndsAt: longEnd, Effect: "从炼气初阶到诸天最高境按真实境界顺序设置里程碑。", EffectJSON: `{"order":"realm_ascending","claim":"once_per_milestone"}`, Status: "进行中"},
		{Code: "xianchen_activity_seven_benefits", Name: "青云七曜福缘", Type: "七日福利", StartsAt: startsAt, EndsAt: standardEnd, Effect: "按入道天数解锁七份修行物资，可在补领期内逐份领取。", EffectJSON: `{"unlock":"registration_day","claim_deadline_days":14}`, Status: "进行中"},
		{Code: "xianchen_activity_opening_codes", Name: "太虚开服密令", Type: "开服密令", StartsAt: startsAt, EndsAt: longEnd, Effect: "公开福利码与修行密令均为每名道友限兑一次。", EffectJSON: `{"redemption":"atomic_once_per_player"}`, Status: "进行中"},
		{Code: "xianchen_activity_code_quests", Name: "天机密令悬卷", Type: "密令任务", StartsAt: startsAt, EndsAt: standardEnd, Effect: "完成探索、镇妖与签到目标后揭示隐藏密令。", EffectJSON: `{"reveal":"objective_progress"}`, Status: "进行中"},
		{Code: "xianchen_activity_invitation", Name: "四海道友召集令", Type: "道友召集", StartsAt: startsAt, EndsAt: standardEnd, Effect: "新道友绑定邀请后双方得礼，邀请人数达到里程碑可再领奖。", EffectJSON: `{"invitee_age_days":7,"binding":"once"}`, Status: "进行中"},
		{Code: "xianchen_activity_rookie_rank", Name: "青云新秀问道榜", Type: "新秀榜", StartsAt: startsAt, EndsAt: standardEnd, Effect: "仅统计入道未满七日的修士，每日前十可领新秀俸禄。", EffectJSON: `{"rookie_days":7,"reward_ranks":10,"cycle":"daily"}`, Status: "进行中"},
		{Code: "xianchen_activity_fortune", Name: "诸天鸿运降临", Type: "天降鸿运", StartsAt: startsAt, EndsAt: standardEnd, Effect: "每名道友每日承接一次天降福缘，稀有鸿运会全区通报。", EffectJSON: `{"attempts_per_day":1}`, Status: "进行中"},
		{Code: "xianchen_activity_prayer", Name: "太一祈福法会", Type: "限时祈福", StartsAt: startsAt, EndsAt: standardEnd, Effect: "每日可在问道、护脉与纳福三签中择一祈愿。", EffectJSON: `{"attempts_per_day":1,"cost_spirit_stones":20}`, Status: "进行中"},
		{Code: "xianchen_activity_festival_sale", Name: "万宝庆典特卖", Type: "庆典特卖", StartsAt: startsAt, EndsAt: saleEnd, Effect: "活动期间以银币购入常用丹药、突破材料与炼器资源。", EffectJSON: `{"currency":"银币","purchase_limit":0}`, Status: "进行中"},
	}
	for _, row := range activities {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	codeExpiry := standardEnd
	codes := []model.RedemptionCode{
		{Code: "SHANHEWENDAO", RewardJSON: `[{"item":"灵果","count":3},{"currency":"灵石","count":188}]`, MaxUses: 100000, ExpiresAt: &codeExpiry, Status: "有效"},
		{Code: "ZHENYAOTIANXIA", RewardJSON: `[{"item":"妖兽内丹","count":5},{"currency":"银币","count":188}]`, MaxUses: 100000, ExpiresAt: &codeExpiry, Status: "有效"},
		{Code: "QIRIXIUXING", RewardJSON: `[{"item":"功法残卷","count":2},{"item":"仙露","count":3}]`, MaxUses: 100000, ExpiresAt: &codeExpiry, Status: "有效"},
	}
	for _, row := range codes {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	saleGoods := []struct {
		Code  string
		Name  string
		Price int64
	}{
		{"event_sale_spirit_fruit", "灵果", 28},
		{"event_sale_immortal_dew", "仙露", 18},
		{"event_sale_meridian_pill", "淬脉丹", 66},
		{"event_sale_origin_pill", "凝元丹", 188},
		{"event_sale_realm_pill", "破境丹", 588},
		{"event_sale_skill_scroll", "功法残卷", 128},
		{"event_sale_spirit_iron", "玄铁", 12},
		{"event_sale_star_sand", "星辰砂", 48},
		{"event_sale_formation_stone", "阵基石", 48},
		{"event_sale_tribulation_charm", "避劫符", 688},
		{"event_sale_summon_talisman", "引劫玉符", 1888},
	}
	for index, good := range saleGoods {
		var item model.Item
		if err := s.DB.Where("name = ?", good.Name).First(&item).Error; err != nil {
			return err
		}
		row := model.ShopEntry{
			Code: good.Code, ItemID: item.ID, ItemName: item.Name, Currency: "银币", Price: good.Price,
			PurchaseLimit: 0, RefreshCycle: "永不", Sort: 30000 + index, Enabled: true,
		}
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
