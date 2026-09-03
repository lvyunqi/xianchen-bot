package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeCouple(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 33:
		return g.findPartners(player)
	case 34:
		return g.requestBond(player, command.RawArguments)
	case 35:
		return g.acceptBond(player)
	case 36:
		return g.discussDao(player, command.RawArguments)
	case 37:
		return g.dualCultivation(player)
	case 38:
		return g.guardPartner(player, command.RawArguments)
	case 39:
		return g.requestGuard(player)
	case 40:
		return g.jointAttack(player)
	case 41:
		return g.transferCultivation(player, command)
	case 42:
		return g.coupleAffinity(player)
	case 43:
		return g.giftPartner(player, command.Arguments)
	case 44:
		return g.summonPartner(player)
	case 45:
		return g.partnerStatus(player)
	case 46:
		return g.requestDissolve(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) activeCouple(player *model.Player) (model.Couple, model.Player, error) {
	if player.CoupleID == 0 {
		return model.Couple{}, model.Player{}, gorm.ErrRecordNotFound
	}
	var couple model.Couple
	if err := g.store.DB.Where("id = ? AND status = ?", player.CoupleID, model.CoupleStatusActive).First(&couple).Error; err != nil {
		return model.Couple{}, model.Player{}, err
	}
	partnerID := couple.PlayerAID
	if partnerID == player.ID {
		partnerID = couple.PlayerBID
	}
	partner, err := g.players.Get(partnerID)
	return couple, partner, err
}

func (g *Game) findPartners(player *model.Player) (GameResult, bool, error) {
	if player.CoupleID != 0 {
		return GameResult{Title: "已有仙侣", Content: "你已有仙侣，可发送 `心意` 查看道缘。", Actions: []string{"心意", "双修"}}, true, nil
	}
	if !hasPlayerGender(player) {
		return GameResult{Title: "寻缘道籍不全", Content: "你的道籍尚未登记性别。请先选择男修或女修，寻缘名单和仙侣资料才会显示完整角色信息。\n性别不会限制你可选择的结缘对象。", Actions: []string{"性别 男", "性别 女", "性别"}}, true, nil
	}
	var players []model.Player
	err := g.store.DB.Where("id <> ? AND couple_id = 0 AND banned = ? AND gender IN ?", player.ID, false, []string{"男修", "女修"}).
		Order(fmt.Sprintf("CASE WHEN realm_id = %d THEN 0 ELSE 1 END, ABS(realm_id - %d), updated_at DESC", player.RealmID, player.RealmID)).
		Limit(6).Find(&players).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(players) == 0 {
		return GameResult{Title: "寻缘", Content: "红尘寂静，暂未寻到可结缘的修士。"}, true, nil
	}
	lines := make([]string, 0, len(players))
	actions := make([]string, 0, len(players))
	for _, candidate := range players {
		lines = append(lines, fmt.Sprintf("- %s · %s · %s · 战力%d", candidate.DaoName, displayPlayerGender(candidate.Gender), candidate.RealmName, candidate.CombatPower))
		actions = append(actions, "结缘 "+candidate.AccountID)
	}
	return GameResult{Title: "红线寻缘", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) requestBond(player *model.Player, argument string) (GameResult, bool, error) {
	if player.CoupleID != 0 {
		return GameResult{Title: "结缘失败", Content: "你已有仙侣。"}, true, nil
	}
	if !hasPlayerGender(player) {
		return GameResult{Title: "结缘道籍不全", Content: "你尚未登记性别，无法生成完整仙侣道契。请先补录；性别不会限制结缘对象。", Actions: []string{"性别 男", "性别 女", "道侣菜单"}}, true, nil
	}
	target, err := g.findPlayer(argument)
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "结缘", Content: "请输入：`结缘 @对方`"}, true, nil
	}
	if target.CoupleID != 0 {
		return GameResult{Title: "红线无缘", Content: target.DaoName + "已有仙侣。"}, true, nil
	}
	if !hasPlayerGender(&target) {
		return GameResult{Title: "对方道籍不全", Content: target.DaoName + "尚未登记性别，暂不能生成完整仙侣道契。请对方先发送“性别 男/女”。", Actions: []string{"寻缘", "道侣菜单"}}, true, nil
	}
	request := model.SocialMessage{SenderID: player.ID, ReceiverID: target.ID, Type: "couple_request", Content: "请求结缘", Read: false}
	if err := g.social.Create(&request); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "结缘请求已送达", Content: fmt.Sprintf("你向%s递出同心玉。\n对方发送 `应缘` 后即可结为仙侣。", target.DaoName)}, true, nil
}

func (g *Game) acceptBond(player *model.Player) (GameResult, bool, error) {
	if player.CoupleID != 0 {
		return GameResult{Title: "应缘失败", Content: "你已有仙侣。"}, true, nil
	}
	if !hasPlayerGender(player) {
		return GameResult{Title: "应缘道籍不全", Content: "你尚未登记性别，无法签下完整仙侣道契。请先补录后再次应缘。", Actions: []string{"性别 男", "性别 女", "应缘"}}, true, nil
	}
	var request model.SocialMessage
	if err := g.store.DB.Where("receiver_id = ? AND type = ? AND read = ?", player.ID, "couple_request", false).Order("id DESC").First(&request).Error; err != nil {
		return GameResult{Title: "无人求缘", Content: "当前没有待处理的结缘请求。"}, true, nil
	}
	sender, err := g.players.Get(request.SenderID)
	if err != nil || sender.CoupleID != 0 {
		_ = g.store.DB.Model(&request).Update("read", true).Error
		return GameResult{Title: "红线已散", Content: "这份结缘请求已经失效。"}, true, nil
	}
	if !hasPlayerGender(&sender) {
		return GameResult{Title: "求缘者道籍不全", Content: sender.DaoName + "尚未登记性别，这份道契暂不能生效。请对方补录后重新结缘。", Actions: []string{"道侣菜单"}}, true, nil
	}
	repo := storage.NewCoupleRepository(g.store.DB)
	couple, err := repo.ForceBond(sender.ID, player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Model(&request).Update("read", true).Error
	broadcast := fmt.Sprintf("【三生仙缘】%s与%s于三生石前结为仙侣，同心玉合二为一，自此共问长生。", sender.DaoName, player.DaoName)
	_ = g.publishWorldBroadcast("仙缘", sender.DaoName+"与"+player.DaoName+"结为仙侣", broadcast)
	return GameResult{Title: "结缘成功", Content: fmt.Sprintf("同心玉合二为一。\n%s（%s）与%s（%s）结为仙侣。\n初始道缘：%d", sender.DaoName, displayPlayerGender(sender.Gender), player.DaoName, displayPlayerGender(player.Gender), couple.Affinity), Actions: []string{"心意", "双修"}, BroadcastContent: broadcast}, true, nil
}

func (g *Game) discussDao(player *model.Player, argument string) (GameResult, bool, error) {
	target, err := g.findPlayer(argument)
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "论道", Content: "请输入：`论道 @对方`"}, true, nil
	}
	remaining, ok, err := g.cooldown(player.ID, "discuss_dao."+strconv.FormatUint(uint64(target.ID), 10), 30*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "论道未歇", Content: "还需" + formatDuration(remaining) + "方可再次论道。"}, true, nil
	}
	reward := int64(10)
	if player.RealmID < target.RealmID {
		reward = 15
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Updates(map[string]any{"cultivation": gorm.Expr("cultivation + ?", reward), "perception": gorm.Expr("perception + 1")}).Error; err != nil {
			return err
		}
		if _, err := grantCultivationExperienceTx(tx, target.ID, 10); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", target.ID).Update("perception", gorm.Expr("perception + 1")).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if refreshed, loadErr := g.players.Get(target.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&refreshed)
	}
	return GameResult{Title: "论道切磋", Content: fmt.Sprintf("你与%s互证道法。\n你的修为：+%d\n双方悟性：+1", target.DaoName, reward)}, true, nil
}

func (g *Game) dualCultivation(player *model.Player) (GameResult, bool, error) {
	couple, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "双修失败", Content: "你尚未结缘。", Actions: []string{"寻缘"}}, true, nil
	}
	if !hasPlayerGender(player) || !hasPlayerGender(&partner) {
		missing := player.DaoName
		if hasPlayerGender(player) {
			missing = partner.DaoName
		}
		return GameResult{Title: "双修道籍不全", Content: fmt.Sprintf("%s尚未登记性别，仙侣周天无法生成完整角色叙事。\n请缺失资料的一方先发送“性别 男/女”；性别组合不会影响双修资格或收益。", missing), Actions: []string{"性别", "心意", "道侣菜单"}}, true, nil
	}
	remaining, ok, err := g.cooldown(player.ID, "dual_cultivation", 60*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "灵息未稳", Content: "还需" + formatDuration(remaining) + "才能再次双修。"}, true, nil
	}
	base := g.settingInt("cultivation.base_reward", 20)
	reward := int64(float64(base) * 1.5 * (1 + float64(couple.BondLevel)/20))
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Update("cultivation", gorm.Expr("cultivation + ?", reward)).Error; err != nil {
			return err
		}
		if _, err := grantCultivationExperienceTx(tx, partner.ID, reward); err != nil {
			return err
		}
		return tx.Model(&couple).Updates(map[string]any{"affinity": gorm.Expr("affinity + 10"), "interaction_count": gorm.Expr("interaction_count + 1"), "last_interaction_at": time.Now()}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if refreshed, loadErr := g.players.Get(partner.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&refreshed)
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.dual_cultivation", 1)
	return GameResult{Title: "仙侣双修", Content: fmt.Sprintf("%s（%s）与%s（%s）同坐聚灵阵眼，两道灵息依次贯通十二重周天。\n━━━━━━━━━━━\n双方修为：+%d\n道缘：+10\n同心等级：%d\n下次双修：一小时后", player.DaoName, displayPlayerGender(player.Gender), partner.DaoName, displayPlayerGender(partner.Gender), reward, couple.BondLevel), Actions: []string{"心意", "状态", "道侣菜单"}}, true, nil
}

func (g *Game) guardPartner(player *model.Player, argument string) (GameResult, bool, error) {
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "护法失败", Content: "你尚未结缘。"}, true, nil
	}
	if strings.TrimSpace(argument) != "" {
		target, findErr := g.findPlayer(argument)
		if findErr != nil || target.ID != partner.ID {
			return GameResult{Title: "护法失败", Content: "只能为自己的仙侣护法。"}, true, nil
		}
	}
	expires := time.Now().Add(30 * time.Minute)
	if err := g.setPlayerValue(partner.ID, "buff.guard", "2.0", &expires); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "护法", Content: fmt.Sprintf("你为%s镇守心神。\n对方30分钟内修炼速度×2。", partner.DaoName)}, true, nil
}

func (g *Game) requestGuard(player *model.Player) (GameResult, bool, error) {
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "求护失败", Content: "你尚未结缘。"}, true, nil
	}
	message := model.SocialMessage{SenderID: player.ID, ReceiverID: partner.ID, Type: "guard_request", Content: player.DaoName + "请求你为其护法"}
	if err := g.social.Create(&message); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "护法请求已送达", Content: "已通知" + partner.DaoName + "。"}, true, nil
}

func (g *Game) jointAttack(player *model.Player) (GameResult, bool, error) {
	couple, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "合击失败", Content: "你尚未结缘。"}, true, nil
	}
	multiplier := 1.2 + float64(couple.Affinity)/2000
	if multiplier > 2.5 {
		multiplier = 2.5
	}
	damage := int64(float64(player.PhysicalAttack+partner.PhysicalAttack) * multiplier)
	_ = g.store.DB.Model(&couple).Update("joint_battle_count", gorm.Expr("joint_battle_count + 1")).Error
	return GameResult{Title: "仙侣合击", Content: fmt.Sprintf("默契系数：%.2f\n合击伤害：**%d**", multiplier, damage), Actions: []string{"合战", "心意"}}, true, nil
}

func (g *Game) transferCultivation(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	if len(command.Arguments) < 2 {
		return GameResult{Title: "传功", Content: "请输入：`传功 @对方 数量`"}, true, nil
	}
	target, err := g.findPlayer(command.Arguments[0])
	amount := parsePositiveInt(command.Arguments[len(command.Arguments)-1], 0)
	if err != nil || amount <= 0 || target.ID == player.ID {
		return GameResult{Title: "传功", Content: "目标或数量不正确。"}, true, nil
	}
	if player.Cultivation < amount {
		return GameResult{Title: "传功失败", Content: "你的修为不足。"}, true, nil
	}
	received := (amount/10)*7 + (amount%10)*7/10
	if received < 1 {
		received = 1
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Update("cultivation", gorm.Expr("cultivation - ?", amount)).Error; err != nil {
			return err
		}
		return tx.Model(&target).Update("cultivation", gorm.Expr("cultivation + ?", received)).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "传功渡力", Content: fmt.Sprintf("你向%s传出%d修为。\n对方实得：%d\n传输损耗：%d\n━━━━━━━━━━━\n规则：正常传功按七成转化；正数小额传功至少实得1点，不会再因整数截断归零。", target.DaoName, amount, received, amount-received)}, true, nil
}

func (g *Game) coupleAffinity(player *model.Player) (GameResult, bool, error) {
	couple, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "道缘", Content: "你尚未结缘。", Actions: []string{"寻缘"}}, true, nil
	}
	return GameResult{Title: "仙侣心意", Content: fmt.Sprintf("道号：%s · %s\n仙侣：%s · %s\n道缘深度：%d\n同心等级：%d\n互动次数：%d\n合战次数：%d", player.DaoName, displayPlayerGender(player.Gender), partner.DaoName, displayPlayerGender(partner.Gender), couple.Affinity, couple.BondLevel, couple.InteractionCount, couple.JointBattleCount), Actions: []string{"性别", "双修", "赠礼", "灵犀"}}, true, nil
}

func (g *Game) giftPartner(player *model.Player, args []string) (GameResult, bool, error) {
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "赠礼失败", Content: "你尚未结缘。"}, true, nil
	}
	if len(args) < 2 {
		return GameResult{Title: "赠礼", Content: "请输入：`赠礼 物品 数量`，也兼容 `赠礼 @仙侣 物品 数量`。"}, true, nil
	}
	itemIndex := 0
	if len(args) >= 3 {
		itemIndex = 1
	}
	quantity := parsePositiveInt(args[len(args)-1], 0)
	item, itemErr := g.itemByName(args[itemIndex])
	if itemErr != nil || quantity <= 0 || g.itemQuantity(player.ID, item.ID) < quantity {
		return GameResult{Title: "赠礼失败", Content: "物品或数量不正确。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		repo := storage.NewPlayerRepository(tx)
		if err := repo.AdjustItem(player.ID, item.ID, -quantity); err != nil {
			return err
		}
		if err := repo.AdjustItem(partner.ID, item.ID, quantity); err != nil {
			return err
		}
		return tx.Model(&model.Couple{}).Where("id = ?", player.CoupleID).Updates(map[string]any{"affinity": gorm.Expr("affinity + ?", quantity*5), "gift_count": gorm.Expr("gift_count + ?", quantity)}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙侣赠礼", Content: fmt.Sprintf("赠予%s：%s×%d\n道缘：+%d", partner.DaoName, item.Name, quantity, quantity*5), Actions: []string{"心意"}}, true, nil
}

func (g *Game) summonPartner(player *model.Player) (GameResult, bool, error) {
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "召唤失败", Content: "你尚未结缘。"}, true, nil
	}
	if player.Mana < 10 {
		return GameResult{Title: "法力不足", Content: "召唤仙侣需要10点法力。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Update("mana", gorm.Expr("mana - 10")).Error; err != nil {
			return err
		}
		return tx.Model(&partner).Update("location", player.Location).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "同心召唤", Content: fmt.Sprintf("同心印亮起，%s被传送至%s。\n法力：-10", partner.DaoName, player.Location)}, true, nil
}

func (g *Game) partnerStatus(player *model.Player) (GameResult, bool, error) {
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "心有灵犀", Content: "你尚未结缘。"}, true, nil
	}
	online := "离线"
	if partner.Online {
		online = "在线"
	}
	return GameResult{Title: "心有灵犀", Content: fmt.Sprintf("仙侣：%s\n状态：%s\n境界：%s\n位置：%s\n当前活动：%s", partner.DaoName, online, partner.RealmName, partner.Location, partner.State)}, true, nil
}

func (g *Game) requestDissolve(player *model.Player) (GameResult, bool, error) {
	couple, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "解缘", Content: "你尚未结缘。"}, true, nil
	}
	var reverse model.SocialMessage
	err = g.store.DB.Where("sender_id = ? AND receiver_id = ? AND type = ? AND read = ?", partner.ID, player.ID, "dissolve_request", false).Order("id DESC").First(&reverse).Error
	if err == nil {
		if err := storage.NewCoupleRepository(g.store.DB).ForceDissolve(couple.ID); err != nil {
			return GameResult{}, true, err
		}
		_ = g.store.DB.Model(&reverse).Update("read", true).Error
		return GameResult{Title: "缘尽", Content: fmt.Sprintf("双方已确认，%s与%s的仙侣关系解除。", player.DaoName, partner.DaoName)}, true, nil
	}
	request := model.SocialMessage{SenderID: player.ID, ReceiverID: partner.ID, Type: "dissolve_request", Content: "请求解缘"}
	if err := g.social.Create(&request); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "解缘确认", Content: fmt.Sprintf("解缘请求已送达%s。\n对方发送 `解缘` 后关系才会解除。", partner.DaoName)}, true, nil
}

var _ = errors.Is
