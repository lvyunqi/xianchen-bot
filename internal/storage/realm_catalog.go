package storage

import (
	"fmt"
	"math"

	"xianlv/internal/model"
)

const realmCatalogSize = 1000

// realmCatalog builds the complete cultivation ladder. RequiredCultivation is
// cumulative; the difference between adjacent realms is split into ten stage
// costs by the game service.
func realmCatalog() []model.Realm {
	opening := []struct {
		name string
		lore string
	}{
		{"炼气", "引天地灵气入经脉，洗去凡尘浊气，初窥长生门径。"},
		{"筑基", "灵气凝液，道台筑成，修士自此拥有承载大道的根基。"},
		{"金丹", "精气神熔作一粒金丹，丹光不灭，术法威能由此蜕变。"},
		{"元婴", "金丹破壳而成元婴，神魂可以离体巡游，感知天地脉络。"},
		{"化神", "元婴与神识相合，一念观山河，一念御万法。"},
		{"合体", "法身、元神与肉身归一，举手投足皆可引动天地大势。"},
		{"渡劫", "大道降下雷劫问心，唯有道基无缺者才能在毁灭中重生。"},
		{"大乘", "诸法圆融，道果将成，只待叩开隔绝仙凡的天门。"},
		{"飞升", "褪去凡胎，仙籍留名，自此踏入更加辽阔的诸天仙域。"},
	}
	domains := []string{
		"太初", "鸿蒙", "混元", "玄黄", "无极", "太虚", "紫霄", "上清",
		"玉清", "太清", "九曜", "星罗", "天璇", "瑶光", "青冥", "碧落",
		"沧溟", "扶摇", "昆吾", "蓬莱", "瀛洲", "方丈", "洞玄", "归墟",
		"寂灭", "涅槃", "轮回", "因果", "命河", "岁月", "彼岸", "永恒",
	}
	phases := []string{
		"凝真境", "聚魄境", "开玄境", "照神境", "御虚境", "问道境", "通幽境", "洞天境",
		"福地境", "法相境", "天人境", "圣胎境", "道宫境", "神桥境", "彼岸境", "星君境",
		"月皇境", "日尊境", "界王境", "域主境", "天尊境", "道祖境", "仙王境", "仙帝境",
		"神君境", "神王境", "神帝境", "圣尊境", "主宰境", "超脱境", "无上境",
	}
	domainLore := []string{
		"追溯开天前的第一缕道炁", "观想鸿蒙未判时的无量气海", "熔炼诸法归于混元一炁", "参悟天地初分后的玄黄母气",
		"守无极而生太极万象", "横渡虚实交界的太虚长河", "接引九霄紫电淬炼仙躯", "以清灵道炁洗炼三魂七魄",
		"登玉京天阙参修无垢仙法", "返璞归真以求太上忘情", "采炼九曜星辉铸就道骨", "排列周天星斗推演命数",
		"循北斗天璇定位大道枢机", "借瑶光净火照破心中妄障", "御青冥罡风锻造不灭法身", "穿行碧落天海寻觅仙机",
		"听沧海潮生领悟生灭循环", "乘扶摇天风直上九重云阙", "承昆吾神山厚重镇压诸邪", "采蓬莱仙霞温养长生道果",
		"渡瀛洲弱水磨炼无漏真身", "在方丈仙岛开辟内景天地", "洞察玄关一窍映照诸天", "深入万法归墟直面大道终点",
		"于寂灭中守住一线真灵", "历涅槃神火重塑血肉神魂", "遍观轮回百世而不失本心", "梳理因果丝线斩断宿业",
		"溯命河而上改写自身命格", "凝固岁月长河中的刹那永恒", "跨过苦海抵达大道彼岸", "以永恒道心承载不朽神国",
	}
	phaseLore := []string{
		"凝炼本真", "聚合魂魄", "开启玄关", "照见元神", "驾驭虚空", "叩问大道", "通达幽明", "开辟洞天",
		"演化福地", "铸就法相", "感应天人", "孕育圣胎", "构筑道宫", "横架神桥", "横渡彼岸", "执掌星辰",
		"统御月华", "驾驭日轮", "镇守一界", "统摄万域", "受命于天", "开宗立道", "号令群仙", "君临仙域",
		"凝聚神格", "执掌神国", "演化神域", "言出法随", "主宰万道", "挣脱诸界", "证得无上",
	}

	rows := make([]model.Realm, 0, realmCatalogSize)
	cumulative := int64(0)
	appendRealm := func(name, description string) {
		sequence := len(rows) + 1
		step := int64(sequence - 1)
		dodge := math.Min(.05+float64(step)*.0004, .45)
		rate := math.Max(.90-float64(step)*.00035, .55)
		rows = append(rows, model.Realm{
			Name: name, Sequence: sequence, RequiredCultivation: cumulative,
			AttributeMultiplier: 1 + float64(step)*.08,
			BaseHealth:          100 + step*35 + step*step/10,
			BaseMana:            50 + step*22 + step*step/16,
			BaseAttack:          10 + step*6 + step*step/200,
			BaseDefense:         5 + step*4 + step*step/250,
			BaseSpeed:           10 + step/4,
			BaseDodge:           dodge,
			BaseLifespan:        100 + step*step*25,
			TribulationBaseRate: rate,
			Description:         description,
		})
		cumulative += realmLayerCost(sequence) * 10
	}
	for _, row := range opening {
		appendRealm(row.name, row.lore)
	}
	for _, domain := range domains {
		for phaseIndex, phase := range phases {
			if len(rows) == realmCatalogSize {
				return rows
			}
			domainIndex := indexOf(domains, domain)
			description := fmt.Sprintf("%s，于此境%s；十层道基全部圆满后，方能叩问下一重天。", domainLore[domainIndex], phaseLore[phaseIndex])
			appendRealm(domain+"·"+phase, description)
		}
	}
	return rows
}

func realmLayerCost(sequence int) int64 {
	step := int64(sequence - 1)
	return 5000 + step*750 + step*step*25
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return 0
}

func (s *Store) seedRealmCatalog() error {
	catalog := realmCatalog()
	if contentSeedLimit() < 1000 {
		catalog = catalog[:20]
	}
	var existing []model.Realm
	if err := s.DB.Find(&existing).Error; err != nil {
		return err
	}
	byName := make(map[string]model.Realm, len(existing))
	bySequence := make(map[int]model.Realm, len(existing))
	for _, row := range existing {
		byName[row.Name] = row
		bySequence[row.Sequence] = row
	}
	missing := make([]model.Realm, 0, len(catalog))
	for _, row := range catalog {
		if _, ok := byName[row.Name]; ok {
			continue
		}
		if occupied, ok := bySequence[row.Sequence]; ok {
			return fmt.Errorf("境界序号%d已被自定义境界“%s”占用，无法导入完整境界链", row.Sequence, occupied.Name)
		}
		missing = append(missing, row)
	}
	if len(missing) == 0 {
		return nil
	}
	return s.DB.CreateInBatches(&missing, 50).Error
}
