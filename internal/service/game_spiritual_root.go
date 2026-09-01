package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"xianlv/internal/model"
)

type spiritualRootBonus struct {
	Grade              string
	Cultivation        float64
	Primary            string
	Secondary          string
	CombatDescription  string
	CultivationDisplay string
}

func baseSpiritualRootBonuses(name string, quality int) spiritualRootBonus {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	grade := "驳杂"
	if quality >= 60 {
		grade = "纯净"
	}
	if quality >= 75 {
		grade = "上品"
	}
	if quality >= 90 {
		grade = "极品"
	}
	if quality == 100 {
		grade = "无垢"
	}
	combatRate := 5 + quality*15/100
	cultivation := 1.05 + float64(quality)*.002
	bonus := spiritualRootBonus{Grade: grade, Cultivation: cultivation}
	switch {
	case strings.Contains(name, "混沌"):
		bonus.Cultivation = 1.80
		bonus.Primary, bonus.Secondary = "全战斗属性+30%", "五行伤害减免+20%"
		bonus.CombatDescription = "混沌衍化万法，可自由适配功法属性，是当前灵根体系的最高综合档。"
	case strings.Contains(name, "天灵根"):
		bonus.Cultivation = 1.60
		bonus.Primary, bonus.Secondary = "全战斗属性+20%", "悟道成功率+15%"
		bonus.CombatDescription = "单一属性纯粹无杂，吐纳和破境速度远胜普通灵根。"
	case strings.Contains(name, "雷"):
		bonus.Cultivation += .20
		bonus.Primary, bonus.Secondary = fmt.Sprintf("物攻与法强+%d%%", combatRate+5), "暴击率+8%"
		bonus.CombatDescription = "雷法爆发最高，克制阴魂与邪祟，但持续防护较弱。"
	case strings.Contains(name, "冰"):
		bonus.Cultivation += .15
		bonus.Primary, bonus.Secondary = fmt.Sprintf("法强+%d%%", combatRate), "法抗与控制抵抗+15%"
		bonus.CombatDescription = "擅长冻结、迟滞与护魂，攻守较为均衡。"
	case strings.Contains(name, "风"):
		bonus.Cultivation += .15
		bonus.Primary, bonus.Secondary = fmt.Sprintf("身法+%d%%", combatRate+5), "闪避率+8%"
		bonus.CombatDescription = "出手与移动速度出众，适合先手和持续游斗。"
	case strings.Contains(name, "金"):
		bonus.Primary, bonus.Secondary = fmt.Sprintf("物理攻击+%d%%", combatRate), "暴击率+5%"
		bonus.CombatDescription = "庚金主杀伐，剑诀与近身法器收益最高。"
	case strings.Contains(name, "木"):
		bonus.Primary, bonus.Secondary = fmt.Sprintf("气血上限+%d%%", combatRate+5), "治疗与灵植产量+15%"
		bonus.CombatDescription = "乙木主生机，生存、疗伤和仙府灵田最为擅长。"
	case strings.Contains(name, "水"):
		bonus.Primary, bonus.Secondary = fmt.Sprintf("法力上限+%d%%", combatRate+5), fmt.Sprintf("法术防御+%d%%", combatRate)
		bonus.CombatDescription = "玄水绵长，适合高消耗术法、护魂与持久战。"
	case strings.Contains(name, "火"):
		bonus.Primary, bonus.Secondary = fmt.Sprintf("法术攻击+%d%%", combatRate), "暴击伤害+20%"
		bonus.CombatDescription = "离火主爆发，丹火与攻击术法威力最高。"
	case strings.Contains(name, "土"):
		bonus.Primary, bonus.Secondary = fmt.Sprintf("物理防御+%d%%", combatRate+5), "伤害减免+8%"
		bonus.CombatDescription = "厚土主承载，护体、阵法和正面防守最稳固。"
	default:
		bonus.Primary, bonus.Secondary = fmt.Sprintf("全属性+%d%%", combatRate/2), "无特定克制"
		bonus.CombatDescription = "复合灵根适应性强，可通过进化确定最终道途。"
	}
	bonus.CultivationDisplay = fmt.Sprintf("%.0f%%", (bonus.Cultivation-1)*100)
	return bonus
}

func (g *Game) spiritualRootBonuses(name string, quality int) spiritualRootBonus {
	base := baseSpiritualRootBonuses(name, quality)
	var root model.SpiritualRootTemplate
	if g.store.DB.Where("name = ? AND enabled = ?", name, true).First(&root).Error != nil {
		return base
	}
	qualityFactor := float64(maxInt(quality, 1)) / 100
	base.Grade = root.Grade
	base.Cultivation = 1 + (root.CultivationBonus-1)*qualityFactor
	base.Primary = root.PrimaryBonus
	base.Secondary = root.SecondaryBonus
	base.CombatDescription = root.CombatDescription
	base.CultivationDisplay = fmt.Sprintf("%.2f%%", (base.Cultivation-1)*100)
	return base
}

func (g *Game) randomSpiritualRoot() (model.SpiritualRootTemplate, error) {
	var rows []model.SpiritualRootTemplate
	if err := g.store.DB.Where("enabled = ? AND rarity_weight > ?", true, 0).Find(&rows).Error; err != nil {
		return model.SpiritualRootTemplate{}, err
	}
	if len(rows) == 0 {
		return model.SpiritualRootTemplate{}, fmt.Errorf("灵根图鉴为空")
	}
	totalWeight := 0
	for _, row := range rows {
		totalWeight += maxInt(row.RarityWeight, 1)
	}
	roll := rand.Intn(totalWeight)
	for _, row := range rows {
		roll -= maxInt(row.RarityWeight, 1)
		if roll < 0 {
			return row, nil
		}
	}
	return rows[len(rows)-1], nil
}

func (g *Game) applyInitialSpiritualRootBonus(player *model.Player) {
	rate := int64(5 + player.RootQuality*15/100)
	switch {
	case strings.Contains(player.SpiritualRoot, "金"):
		player.PhysicalAttack += max64(player.PhysicalAttack*rate/100, 1)
		player.CritRate += .05
	case strings.Contains(player.SpiritualRoot, "木"):
		player.MaxHealth += max64(player.MaxHealth*(rate+5)/100, 1)
		player.Health = player.MaxHealth
	case strings.Contains(player.SpiritualRoot, "水"):
		player.MaxMana += max64(player.MaxMana*(rate+5)/100, 1)
		player.Mana = player.MaxMana
		player.MagicDefense += max64(player.MagicDefense*rate/100, 1)
	case strings.Contains(player.SpiritualRoot, "火"):
		player.MagicAttack += max64(player.MagicAttack*rate/100, 1)
		player.CritDamage += .20
	case strings.Contains(player.SpiritualRoot, "土"):
		player.PhysicalDefense += max64(player.PhysicalDefense*(rate+5)/100, 1)
		player.DamageReduction += .08
	}
	var root model.SpiritualRootTemplate
	if g.store.DB.Where("name = ?", player.SpiritualRoot).First(&root).Error == nil {
		var attributes map[string]int64
		if json.Unmarshal([]byte(root.AttributeJSON), &attributes) == nil {
			player.PhysicalAttack += max64(player.PhysicalAttack*attributes["attack_basis_points"]/10000, 1)
			player.MagicAttack += max64(player.MagicAttack*attributes["attack_basis_points"]/12000, 1)
			player.PhysicalDefense += max64(player.PhysicalDefense*attributes["defense_basis_points"]/10000, 1)
			player.MagicDefense += max64(player.MagicDefense*attributes["defense_basis_points"]/11000, 1)
			player.MaxHealth += max64(player.MaxHealth*attributes["health_basis_points"]/10000, 1)
			player.Health = player.MaxHealth
			player.MaxMana += max64(player.MaxMana*attributes["mana_basis_points"]/10000, 1)
			player.Mana = player.MaxMana
			player.Agility += max64(player.Agility*attributes["speed_basis_points"]/10000, 1)
		}
	}
}

func (g *Game) spiritualRootGuide(player *model.Player) string {
	bonus := g.spiritualRootBonuses(player.SpiritualRoot, player.RootQuality)
	var total int64
	_ = g.store.DB.Model(&model.SpiritualRootTemplate{}).Where("enabled = ?", true).Count(&total).Error
	return fmt.Sprintf("灵根：%s · %s纯度%d\n修炼额外加成：+%s\n主加成：%s\n副加成：%s\n属性定位：%s\n━━━━━━━━━━━\n当前灵根图鉴：%d种\n每条灵根均有独立修炼倍率、五维基点、稀有权重与道力指数；注册按权重随机，顶级本源概率最低。", player.SpiritualRoot, bonus.Grade, player.RootQuality, bonus.CultivationDisplay, bonus.Primary, bonus.Secondary, bonus.CombatDescription, total)
}
