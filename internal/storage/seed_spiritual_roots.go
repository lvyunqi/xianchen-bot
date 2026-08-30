package storage

import (
	"fmt"

	"xianlv/internal/model"
)

func (s *Store) seedSpiritualRootCatalog() error {
	limit := contentSeedLimit()
	origins := []string{"凡尘", "玄黄", "太阴", "太阳", "紫府", "九霄", "太古", "鸿蒙", "混沌", "无极"}
	elements := []string{"庚金", "乙木", "玄水", "离火", "厚土", "风灵", "雷灵", "冰魄", "时空", "轮回"}
	forms := []string{"剑魄", "道莲", "龙脉", "凤髓", "星核", "月轮", "天门", "神树", "仙骨", "界心"}
	grades := []string{"凡品", "良品", "上品", "极品", "地灵", "天灵", "仙灵", "神灵", "混沌", "超脱"}
	for index := 1; index <= limit; index++ {
		i := index - 1
		originIndex := (i / 100) % len(origins)
		elementIndex := (i / 10) % len(elements)
		formIndex := i % len(forms)
		name := origins[originIndex] + elements[elementIndex] + forms[formIndex] + "灵根"
		baseQuality := 45 + originIndex*5 + formIndex/2
		if baseQuality > 99 {
			baseQuality = 99
		}
		cultivation := 1.05 + float64(index)*0.00073
		attackBP := 180 + index
		defenseBP := 140 + index*2
		healthBP := 220 + index*3
		manaBP := 160 + index*4
		speedBP := 90 + index*5
		primary := fmt.Sprintf("%s亲和%d.%02d%% · 主属性道基%d", elements[elementIndex], attackBP/100, attackBP%100, 20+index*7)
		secondary := fmt.Sprintf("%s道相减伤%d.%02d%% · 悟道修正%d", forms[formIndex], defenseBP/100, defenseBP%100, 10+index*3)
		description := fmt.Sprintf("%s本源孕育的%s%s灵根，以%s为主脉、%s为道相。其修炼倍率、五维道基和稀有权重均为独立配置。", origins[originIndex], elements[elementIndex], forms[formIndex], elements[elementIndex], forms[formIndex])
		weight := 100 - originIndex*9 - elementIndex/3
		if weight < 1 {
			weight = 1
		}
		row := model.SpiritualRootTemplate{
			Code: fmt.Sprintf("root_catalog_%d", index), Name: name, Element: elements[elementIndex], Grade: grades[originIndex],
			BaseQuality: baseQuality, CultivationBonus: cultivation, PrimaryBonus: primary, SecondaryBonus: secondary,
			CombatDescription: description, Description: description,
			AttributeJSON: fmt.Sprintf(`{"attack_basis_points":%d,"defense_basis_points":%d,"health_basis_points":%d,"mana_basis_points":%d,"speed_basis_points":%d,"unique_power_index":%d}`, attackBP, defenseBP, healthBP, manaBP, speedBP, 10000+index*137),
			RarityWeight:  weight, Enabled: true,
		}
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
