package model

type RankingTitleSpec struct {
	Leaderboard string
	Rank        int
	Code        string
	Name        string
	BonusJSON   string
}

type rankingTitleGroup struct {
	Leaderboard string
	CodePrefix  string
	Names       [3]string
	Bonuses     [3]string
}

var rankingTitleGroups = []rankingTitleGroup{
	{"综合", "rank_overall", [3]string{"万象道主", "太虚行者", "诸法鸿儒"}, [3]string{`{"all_percent":12,"power":600}`, `{"all_percent":8,"power":360}`, `{"all_percent":5,"power":220}`}},
	{"境界", "rank_realm", [3]string{"登天至尊", "凌霄真君", "叩道上仙"}, [3]string{`{"health":1200,"mana":600,"cultivation_percent":12}`, `{"health":800,"mana":400,"cultivation_percent":8}`, `{"health":500,"mana":250,"cultivation_percent":5}`}},
	{"战力", "rank_power", [3]string{"诸天武圣", "镇世战仙", "破军真君"}, [3]string{`{"attack":180,"defense":120,"power":800}`, `{"attack":120,"defense":80,"power":480}`, `{"attack":75,"defense":50,"power":300}`}},
	{"修为", "rank_cultivation", [3]string{"万古道藏", "岁月真人", "苦海行者"}, [3]string{`{"mana":1000,"cultivation_percent":15}`, `{"mana":650,"cultivation_percent":10}`, `{"mana":400,"cultivation_percent":6}`}},
	{"财富", "rank_wealth", [3]string{"乾坤财神", "聚宝天君", "金阙散仙"}, [3]string{`{"health":800,"drop_percent":15}`, `{"health":520,"drop_percent":10}`, `{"health":320,"drop_percent":6}`}},
	{"功德", "rank_merit", [3]string{"万善圣人", "济世真君", "护生上人"}, [3]string{`{"defense":160,"health":1000}`, `{"defense":105,"health":650}`, `{"defense":65,"health":400}`}},
	{"声望", "rank_reputation", [3]string{"四海共尊", "名动八荒", "誉满九州"}, [3]string{`{"speed":150,"power":550}`, `{"speed":100,"power":340}`, `{"speed":60,"power":210}`}},
	{"道心", "rank_daoheart", [3]string{"无垢道君", "明镜真人", "守一居士"}, [3]string{`{"defense":180,"mana":800,"tribulation_percent":8}`, `{"defense":115,"mana":520,"tribulation_percent":5}`, `{"defense":70,"mana":320,"tribulation_percent":3}`}},
	{"仙缘", "rank_affinity", [3]string{"天眷福主", "紫气仙君", "承运真人"}, [3]string{`{"speed":140,"mana":700,"fortune_percent":15}`, `{"speed":90,"mana":450,"fortune_percent":10}`, `{"speed":55,"mana":280,"fortune_percent":6}`}},
	{"灵根", "rank_root", [3]string{"太初道胎", "天生仙骨", "灵蕴真人"}, [3]string{`{"attack":140,"mana":900,"all_percent":5}`, `{"attack":90,"mana":580,"all_percent":3}`, `{"attack":55,"mana":360,"all_percent":2}`}},
	{"灵兽", "rank_pet", [3]string{"万灵御主", "御兽仙君", "通灵真人"}, [3]string{`{"health":1200,"power":650,"pet_percent":15}`, `{"health":780,"power":400,"pet_percent":10}`, `{"health":480,"power":250,"pet_percent":6}`}},
	{"装备", "rank_equipment", [3]string{"百炼器尊", "神兵天匠", "藏锋宗师"}, [3]string{`{"attack":150,"defense":150,"forge_percent":15}`, `{"attack":95,"defense":95,"forge_percent":10}`, `{"attack":60,"defense":60,"forge_percent":6}`}},
	{"宗门", "rank_sect", [3]string{"仙盟共主", "开宗圣贤", "护宗真君"}, [3]string{`{"all_percent":10,"health":1000}`, `{"all_percent":6,"health":650}`, `{"all_percent":4,"health":400}`}},
	{"道缘", "rank_bond", [3]string{"三生眷主", "同心仙君", "红尘知己"}, [3]string{`{"health":900,"mana":900,"joint_attack_percent":15}`, `{"health":580,"mana":580,"joint_attack_percent":10}`, `{"health":360,"mana":360,"joint_attack_percent":6}`}},
	{"副本", "rank_dungeon", [3]string{"秘境征服者", "破界先锋", "探幽真人"}, [3]string{`{"attack":150,"defense":100,"dungeon_percent":15}`, `{"attack":95,"defense":65,"dungeon_percent":10}`, `{"attack":60,"defense":40,"dungeon_percent":6}`}},
	{"竞技", "rank_arena", [3]string{"问剑魁首", "剑台无双", "论武宗师"}, [3]string{`{"attack":170,"speed":140}`, `{"attack":110,"speed":90}`, `{"attack":70,"speed":55}`}},
	{"灵田", "rank_farm", [3]string{"百草仙君", "灵圃宗师", "耕云上人"}, [3]string{`{"health":900,"mana":600,"harvest_percent":15}`, `{"health":580,"mana":390,"harvest_percent":10}`, `{"health":360,"mana":240,"harvest_percent":6}`}},
	{"首领", "rank_boss", [3]string{"镇域战神", "斩魔天君", "伏妖真人"}, [3]string{`{"attack":190,"power":700,"boss_percent":15}`, `{"attack":125,"power":430,"boss_percent":10}`, `{"attack":75,"power":270,"boss_percent":6}`}},
	{"成就", "rank_achievement", [3]string{"万相功成", "百业真人", "勋业上人"}, [3]string{`{"all_percent":11,"power":500}`, `{"all_percent":7,"power":310}`, `{"all_percent":4,"power":190}`}},
	{"活跃", "rank_activity", [3]string{"诸界行者", "不倦真人", "勤修居士"}, [3]string{`{"speed":160,"health":700}`, `{"speed":105,"health":450}`, `{"speed":65,"health":280}`}},
	{"炼丹", "rank_alchemy", [3]string{"丹道圣手", "九转丹君", "妙火宗师"}, [3]string{`{"mana":900,"alchemy_percent":15}`, `{"mana":580,"alchemy_percent":10}`, `{"mana":360,"alchemy_percent":6}`}},
	{"锻造", "rank_forge", [3]string{"炼器神匠", "天工仙师", "铸灵宗师"}, [3]string{`{"defense":170,"forge_percent":15}`, `{"defense":110,"forge_percent":10}`, `{"defense":70,"forge_percent":6}`}},
	{"渡劫", "rank_tribulation", [3]string{"九霄劫主", "驭雷真君", "渡厄上人"}, [3]string{`{"health":1300,"defense":150,"tribulation_percent":15}`, `{"health":850,"defense":95,"tribulation_percent":10}`, `{"health":520,"defense":60,"tribulation_percent":6}`}},
	{"生辰", "rank_birthday", [3]string{"福曜天官", "瑞岁真君", "长乐仙使"}, [3]string{`{"health":588,"mana":288,"power":188}`, `{"health":388,"mana":188,"power":108}`, `{"health":288,"mana":128,"power":68}`}},
}

func RankingTitleCatalog() []RankingTitleSpec {
	result := make([]RankingTitleSpec, 0, len(rankingTitleGroups)*3)
	rankCodes := [3]string{"crown", "jade", "cloud"}
	for _, group := range rankingTitleGroups {
		for index := range group.Names {
			result = append(result, RankingTitleSpec{
				Leaderboard: group.Leaderboard,
				Rank:        index + 1,
				Code:        group.CodePrefix + "_" + rankCodes[index],
				Name:        group.Names[index],
				BonusJSON:   group.Bonuses[index],
			})
		}
	}
	return result
}
