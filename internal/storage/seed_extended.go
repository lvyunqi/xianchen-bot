package storage

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

type gameplaySeedSpec struct {
	Prefix string
	Label  string
	Create func(model.GameplayConfigBase) any
}

func (s *Store) seedExtendedGameplay() error {
	for _, spec := range gameplaySeedSpecs() {
		if err := s.seedGameplaySpec(spec); err != nil {
			return err
		}
	}
	return nil
}

func gameplaySeedSpecs() []gameplaySeedSpec {
	return []gameplaySeedSpec{
		{"formation", "阵法", func(v model.GameplayConfigBase) any { row := model.FormationConfig(v); return &row }},
		{"talisman", "符箓", func(v model.GameplayConfigBase) any { row := model.TalismanConfig(v); return &row }},
		{"puppet", "傀儡", func(v model.GameplayConfigBase) any { row := model.PuppetConfig(v); return &row }},
		{"secret_conflict", "争夺秘境", func(v model.GameplayConfigBase) any { row := model.SecretRealmConflictConfig(v); return &row }},
		{"inheritance", "上古传承", func(v model.GameplayConfigBase) any { row := model.InheritanceConfig(v); return &row }},
		{"dao_insight", "大道真法", func(v model.GameplayConfigBase) any { row := model.DaoInsightConfig(v); return &row }},
		{"battlefield", "仙魔战场", func(v model.GameplayConfigBase) any { row := model.ImmortalDemonBattlefieldConfig(v); return &row }},
		{"root_evolution", "灵根进化", func(v model.GameplayConfigBase) any { row := model.SpiritualRootEvolutionConfig(v); return &row }},
		{"inner_demon", "渡劫心魔", func(v model.GameplayConfigBase) any { row := model.InnerDemonConfig(v); return &row }},
		{"couple_skill", "道侣合体技", func(v model.GameplayConfigBase) any { row := model.CoupleCombinationSkillConfig(v); return &row }},
		{"immortal_herb", "九天仙药", func(v model.GameplayConfigBase) any { row := model.ImmortalHerbConfig(v); return &row }},
		{"artifact_refine", "法宝炼化", func(v model.GameplayConfigBase) any { row := model.ArtifactRefinementConfig(v); return &row }},
		{"destiny", "天机推演", func(v model.GameplayConfigBase) any { row := model.DestinyDeductionConfig(v); return &row }},
		{"leyline", "天地灵脉", func(v model.GameplayConfigBase) any { row := model.LeylineConfig(v); return &row }},
		{"sect_war", "宗门战争", func(v model.GameplayConfigBase) any { row := model.SectWarConfig(v); return &row }},
		{"immortal_encounter", "仙缘奇遇", func(v model.GameplayConfigBase) any { row := model.ImmortalEncounterConfig(v); return &row }},
		{"star_realm", "宇宙星河", func(v model.GameplayConfigBase) any { row := model.StarRealmConfig(v); return &row }},
	}
}

func (s *Store) seedGameplaySpec(spec gameplaySeedSpec) error {
	limit := contentSeedLimit()
	probe := spec.Create(model.GameplayConfigBase{})
	for i := 1; i <= limit; i++ {
		base := extendedSeedBase(spec, i)
		if err := s.DB.Model(probe).Where("code = ? AND (description LIKE ? OR name LIKE ?)", base.Code, "%唯一配置%", spec.Label+"·%").Updates(map[string]any{
			"name": base.Name, "type": base.Type, "level": base.Level, "description": base.Description,
			"effect_params": base.EffectParams, "cost_materials": base.CostMaterials,
			"prerequisite": base.Prerequisite, "sort_order": base.SortOrder,
		}).Error; err != nil {
			return err
		}
		// 只升级仍保持旧默认值的内置配置，运营人员改过的前置条件不覆盖。
		if err := s.DB.Model(probe).Where("code = ? AND prerequisite = ?", base.Code, legacyExtendedPrerequisite(i)).Update("prerequisite", base.Prerequisite).Error; err != nil {
			return err
		}
	}
	// Count alone cannot prove that every built-in row exists because operators
	// may add custom rows. Reconcile each canonical code and preserve custom rows.
	for i := 1; i <= limit; i++ {
		base := extendedSeedBase(spec, i)
		row := spec.Create(base)
		if err := s.DB.Where("code = ?", base.Code).FirstOrCreate(row).Error; err != nil {
			return err
		}
	}
	var enabled int64
	if err := s.DB.Model(probe).Where("status = ?", "启用").Count(&enabled).Error; err != nil {
		return err
	}
	if enabled == 0 {
		var first model.GameplayConfigBase
		if err := s.DB.Table(gameplayConfigTable(spec.Label)).Order("sort_order, id").First(&first).Error; err != nil {
			return err
		}
		if err := s.DB.Table(gameplayConfigTable(spec.Label)).Where("id = ?", first.ID).Update("status", "启用").Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReconcileGameplayCategory(label string) (before, after int64, err error) {
	for _, spec := range gameplaySeedSpecs() {
		if spec.Label != label {
			continue
		}
		probe := spec.Create(model.GameplayConfigBase{})
		if err = s.DB.Model(probe).Count(&before).Error; err != nil {
			return
		}
		err = s.DB.Transaction(func(tx *gorm.DB) error {
			return (&Store{DB: tx, cfg: s.cfg}).seedGameplaySpec(spec)
		})
		if err == nil {
			err = s.DB.Model(probe).Count(&after).Error
		}
		return
	}
	err = fmt.Errorf("未识别的玩法分类: %s", label)
	return
}

func gameplayConfigTable(label string) string {
	return map[string]string{
		"阵法": "formation_configs", "符箓": "talisman_configs", "傀儡": "puppet_configs",
		"争夺秘境": "secret_realm_conflict_configs", "上古传承": "inheritance_configs", "大道真法": "dao_insight_configs",
		"仙魔战场": "immortal_demon_battlefield_configs", "灵根进化": "spiritual_root_evolution_configs", "渡劫心魔": "inner_demon_configs",
		"道侣合体技": "couple_combination_skill_configs", "九天仙药": "immortal_herb_configs", "法宝炼化": "artifact_refinement_configs",
		"天机推演": "destiny_deduction_configs", "天地灵脉": "leyline_configs", "宗门战争": "sect_war_configs",
		"仙缘奇遇": "immortal_encounter_configs", "宇宙星河": "star_realm_configs",
	}[label]
}

func extendedSeedBase(spec gameplaySeedSpec, index int) model.GameplayConfigBase {
	n := cultivationSeedName(index)
	forms := map[string][]string{
		"阵法":    {"四象护道阵", "九宫困龙阵", "周天聚灵阵", "七杀诛邪阵"},
		"符箓":    {"镇邪符", "护身符", "神行符", "引雷符"},
		"傀儡":    {"玄甲战傀", "灵木药傀", "巡山剑傀", "破阵机关傀"},
		"争夺秘境":  {"灵泉洞天", "古剑遗府", "妖王禁地", "星陨秘藏"},
		"上古传承":  {"剑尊传承", "丹圣遗法", "阵祖道统", "御兽天书"},
		"大道真法":  {"金行真法", "太阴道法", "长生妙法", "虚空秘法"},
		"仙魔战场":  {"天河前线", "镇魔古关", "陨仙荒原", "九幽裂谷"},
		"灵根进化":  {"庚金淬灵篇", "乙木生灵篇", "玄水洗髓篇", "离火涅槃篇"},
		"渡劫心魔":  {"贪念心魔", "执念心魔", "杀念心魔", "恐惧心魔"},
		"道侣合体技": {"双星同辉剑", "鸾凤和鸣曲", "阴阳两仪印", "生死同心契"},
		"九天仙药":  {"紫府养魂芝", "九叶渡劫莲", "太阴凝露花", "龙血涅槃果"},
		"法宝炼化":  {"飞剑炼真篇", "仙衣开光篇", "古镜蕴神篇", "道钟融灵篇"},
		"天机推演":  {"命河观星术", "因果寻缘术", "天劫预兆术", "逆命改运术"},
		"天地灵脉":  {"青木灵脉", "赤火地脉", "玄水龙脉", "庚金矿脉"},
		"宗门战争":  {"山门攻防令", "灵脉争夺令", "护宗备战令", "诸宗会盟令"},
		"仙缘奇遇":  {"古洞逢仙缘", "灵狐报恩缘", "残碑悟道缘", "红尘问心缘"},
		"宇宙星河":  {"紫微星图", "北斗星魂", "天河星域", "太虚星门"},
	}
	types := map[string][]string{
		"阵法": {"护阵", "困阵", "聚灵阵", "杀阵"}, "符箓": {"镇邪", "防御", "遁行", "攻伐"},
		"傀儡": {"防御", "辅助", "攻击", "破阵"}, "争夺秘境": {"资源", "传承", "战斗", "探索"},
		"上古传承": {"剑道", "丹道", "阵道", "御兽"}, "大道真法": {"攻伐", "神魂", "长生", "空间"},
		"仙魔战场": {"前线", "守关", "混战", "险地"}, "灵根进化": {"金", "木", "水", "火"},
		"渡劫心魔": {"贪", "执", "杀", "惧"}, "道侣合体技": {"攻击", "辅助", "封印", "守护"},
		"九天仙药": {"养魂", "渡劫", "法力", "炼体"}, "法宝炼化": {"飞剑", "仙衣", "宝镜", "道钟"},
		"天机推演": {"命数", "因果", "天劫", "改命"}, "天地灵脉": {"木", "火", "水", "金"},
		"宗门战争": {"攻城", "争夺", "防守", "外交"}, "仙缘奇遇": {"传承", "灵兽", "悟道", "问心"},
		"宇宙星河": {"星图", "星魂", "星域", "传送"},
	}
	i := index - 1
	formList := forms[spec.Label]
	typeList := types[spec.Label]
	form := formList[i%len(formList)]
	kind := typeList[i%len(typeList)]
	materials := []string{"凝露草", "赤焰草", "月华花", "妖兽内丹", "玄铁", "星辰砂", "雷灵晶", "阵基石", "灵茶", "仙露"}
	material := materials[(i*3+len(spec.Prefix))%len(materials)]
	minimumRealm := 1 + i/10
	minimumLayer := 1 + i%10
	prerequisite := map[string]any{
		"minimum_realm_sequence": minimumRealm,
		"minimum_realm_level":    minimumLayer,
		"minimum_combat_power":   80 + index*18,
	}
	elements := []string{"庚金", "乙木", "玄水", "离火", "厚土", "风雷", "太阴", "太阳", "星辰", "时空"}
	cappedRootQuality := 10 + i/15
	if cappedRootQuality > 90 {
		cappedRootQuality = 90
	}
	switch spec.Label {
	case "阵法":
		prerequisite["minimum_perception"] = 8 + i/20
	case "符箓":
		prerequisite["minimum_spirit"] = 8 + i/18
		prerequisite["minimum_perception"] = 8 + i/25
	case "傀儡":
		prerequisite["minimum_spirit"] = 8 + i/16
		prerequisite["minimum_willpower"] = 8 + i/25
	case "争夺秘境":
		prerequisite["minimum_reputation"] = 1 + i*2
	case "上古传承":
		prerequisite["minimum_merit"] = 1 + i/3
		prerequisite["minimum_root_quality"] = cappedRootQuality
	case "大道真法":
		prerequisite["minimum_perception"] = 10 + i/10
		prerequisite["minimum_dao_heart"] = 20 + i/20
	case "仙魔战场":
		prerequisite["minimum_reputation"] = 5 + i
		prerequisite["minimum_willpower"] = 8 + i/20
	case "灵根进化":
		prerequisite["minimum_root_quality"] = cappedRootQuality
	case "渡劫心魔":
		prerequisite["minimum_willpower"] = 10 + i/10
		prerequisite["minimum_dao_heart"] = 25 + i/20
	case "道侣合体技":
		prerequisite["couple_required"] = true
		prerequisite["minimum_immortal_affinity"] = 10 + i/5
	case "九天仙药":
		prerequisite["mansion_required"] = true
		prerequisite["minimum_perception"] = 8 + i/25
	case "法宝炼化":
		prerequisite["minimum_spirit"] = 8 + i/18
		prerequisite["minimum_root_quality"] = cappedRootQuality
	case "天机推演":
		prerequisite["minimum_luck"] = minInt(10+i/10, 50)
		prerequisite["minimum_perception"] = 10 + i/15
	case "天地灵脉":
		prerequisite["minimum_spirit"] = 8 + i/15
		prerequisite["required_root_element"] = elements[(i/10)%len(elements)]
	case "宗门战争":
		prerequisite["sect_required"] = true
		prerequisite["minimum_reputation"] = 10 + i*2
	case "仙缘奇遇":
		prerequisite["minimum_luck"] = minInt(10+i/12, 50)
		prerequisite["minimum_immortal_affinity"] = 5 + i/8
	case "宇宙星河":
		prerequisite["minimum_spirit"] = 10 + i/10
		prerequisite["minimum_perception"] = 10 + i/12
		prerequisite["minimum_mana"] = 20 + i/5
	}
	prerequisiteJSON, _ := json.Marshal(prerequisite)
	return model.GameplayConfigBase{
		Code: fmt.Sprintf("%s_%d", spec.Prefix, index), Name: n + "·" + form,
		Type: kind, Level: 1 + i%10,
		Description:   fmt.Sprintf("%s一脉的%s，源自%s道意。其来历、成长、克制关系与解锁路线皆载于天机阁道藏。", spec.Label, form, n),
		EffectParams:  fmt.Sprintf(`{"power":%d,"duration":%d,"growth":%.2f}`, 20+index*4, 30+index%91, 1+float64(index%50)/100),
		CostMaterials: fmt.Sprintf(`{"灵石":%d,"%s":%d}`, 20+index*5, material, 1+index%5),
		Prerequisite:  string(prerequisiteJSON),
		SortOrder:     index, Status: "启用",
	}
}

func legacyExtendedPrerequisite(index int) string {
	i := index - 1
	return fmt.Sprintf(`{"minimum_realm_sequence":%d,"minimum_realm_level":%d,"minimum_combat_power":%d}`, 1+i/10, 1+i%10, 80+index*18)
}
