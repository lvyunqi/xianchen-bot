package storage

import (
	"fmt"

	"xianlv/internal/model"
)

func (s *Store) seedArenaTierCatalog() error {
	for _, row := range arenaTierCatalog(contentSeedLimit()) {
		if err := s.DB.Where("code = ?", row.Code).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func arenaTierCatalog(limit int) []model.ArenaTier {
	if limit < 1 {
		return nil
	}
	if limit > 1000 {
		limit = 1000
	}
	prefixes := []string{"青霄", "赤霄", "玄冥", "紫府", "太虚", "鸿蒙", "混沌", "九曜", "星河", "万象"}
	paths := []string{"问剑", "藏锋", "破军", "镇岳", "凌云", "摘星", "斩月", "御雷", "焚天", "归墟"}
	titles := []string{"道徒", "行者", "剑师", "宗匠", "真君", "天骄", "道尊", "圣主", "帝君", "剑仙"}
	descriptions := []string{
		"初识剑势，以守正心法磨炼每一次出手。", "藏锋养意，在胜负之间稳住自身道心。",
		"破阵争先，以连贯招式夺取问剑先机。", "镇守四方，以沉稳防御化去凌厉攻势。",
		"凌云而战，以身法和神识寻找破绽。", "引星入剑，以精准判断掌握回合节奏。",
		"斩月明心，以攻守转换应对不同道法。", "御雷淬锋，以果断出手压制敌方法力。",
		"焚天证道，以完整战局检验道基深浅。", "归墟返真，以万法归一争夺诸天魁首。",
	}
	rows := make([]model.ArenaTier, 0, limit)
	for index := 0; index < limit; index++ {
		prefixIndex := index / 100
		pathIndex := index / 10 % 10
		titleIndex := index % 10
		minimum := int64(0)
		if index > 0 {
			minimum = 1000 + int64(index)*20
		}
		name := prefixes[prefixIndex] + paths[pathIndex] + titles[titleIndex]
		rows = append(rows, model.ArenaTier{
			Code: fmt.Sprintf("arena_tier_%04d", index+1), Name: name, Sequence: index + 1,
			MinimumRating: minimum, DailyCoin: int64(20 + index*2), DailySilver: int64(80 + index*7),
			Description: descriptions[pathIndex] + "段位道号为“" + name + "”，俸禄随问剑次序独立增长。",
			Enabled:     true,
		})
	}
	return rows
}
