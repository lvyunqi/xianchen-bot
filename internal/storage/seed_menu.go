package storage

import "xianlv/internal/model"

type menuSeed struct {
	Label      string
	Icon       string
	Path       string
	Component  string
	Permission string
	Sort       int
}

func (s *Store) seedMenus() error {
	groups := []struct {
		menuSeed
		Children []menuSeed
	}{
		{menuSeed{"数据总览", "览", "/group/dashboard", "", "admin", 1}, []menuSeed{
			{"仪表盘", "览", "/dashboard", "dashboard", "admin", 10},
		}},
		{menuSeed{"核心数据", "核", "/group/core", "", "admin", 10}, []menuSeed{
			{"系统参数", "设", "/config", "config", "admin", 10}, {"功能开关", "开", "/features", "features", "admin", 20},
			{"游戏常量", "常", "/constants", "constants", "admin", 30}, {"冷却时间", "时", "/cooldowns", "cooldowns", "admin", 40},
			{"境界配置", "境", "/realms", "realms", "admin", 50}, {"灵根图鉴", "根", "/spiritual-roots", "spiritual_roots", "admin", 60},
		}},
		{menuSeed{"内容配置", "卷", "/group/content", "", "admin", 20}, []menuSeed{
			{"物品数据", "物", "/items", "items", "admin", 10}, {"事件数据", "缘", "/events", "events", "admin", 20},
			{"任务数据", "任", "/tasks", "tasks", "admin", 30}, {"功法数据", "诀", "/skills", "skills", "admin", 40},
			{"灵兽数据", "兽", "/pets", "pets", "admin", 50}, {"副本数据", "境", "/dungeons", "dungeons", "admin", 60},
			{"丹方数据", "丹", "/recipes", "recipes", "admin", 70}, {"器谱数据", "器", "/artifacts", "artifacts", "admin", 80},
			{"合成配方", "合", "/synthesis-recipes", "synthesis_recipes", "admin", 85}, {"地图数据", "图", "/locations", "locations", "admin", 90}, {"修仙界灵脉", "脉", "/world-leylines", "world_leylines", "admin", 100}, {"竞技段位", "竞", "/arena-tiers", "arena_tiers", "admin", 110},
		}},
		{menuSeed{"运营数据", "运", "/group/operations", "", "admin", 30}, []menuSeed{
			{"称号数据", "号", "/titles", "titles", "admin", 10}, {"活动数据", "活", "/activities", "activities", "admin", 20},
			{"邮件数据", "信", "/mails", "mails", "admin", 30}, {"签到配置", "签", "/checkin", "checkin", "admin", 40},
			{"商店数据", "店", "/shop", "shop", "admin", 50}, {"兑换码数据", "码", "/cdks", "cdks", "admin", 60},
			{"公告数据", "告", "/notices", "notices", "admin", 70},
		}},
		{menuSeed{"动态数据", "人", "/group/runtime", "", "admin", 40}, []menuSeed{
			{"玩家数据", "人", "/players", "players", "admin", 10}, {"仙侣数据", "侣", "/couples", "couples", "admin", 20},
		}},
		{menuSeed{"内容审核", "审", "/group/review", "", "admin", 45}, []menuSeed{
			{"内容与玩家反馈", "审", "/reviews", "reviews", "admin", 10}, {"敏感词管理", "词", "/sensitive-words", "sensitive_words", "admin", 20},
		}},
		{menuSeed{"系统管理", "管", "/group/system", "", "admin", 50}, []menuSeed{
			{"菜单管理", "单", "/menus", "menus", "admin", 10}, {"状态显示", "图", "/status-display", "status_display", "admin", 20},
			{"主人设置", "主", "/owner-settings", "owner_settings", "admin", 30}, {"管理设置", "管", "/managers", "managers", "admin", 40},
		}},
		{menuSeed{"系统监控", "监", "/group/monitor", "", "admin", 60}, []menuSeed{
			{"服务器状态", "服", "/monitor/server", "server_status", "admin", 10}, {"在线监控", "线", "/monitor/online", "online_monitor", "admin", 20},
			{"请求统计", "请", "/monitor/requests", "request_stats", "admin", 30}, {"慢查询", "慢", "/monitor/slow", "slow_queries", "admin", 40},
			{"性能监控", "性", "/monitor/performance", "performance", "admin", 50}, {"告警配置", "警", "/monitor/alerts", "alerts", "admin", 60},
		}},
		{menuSeed{"全新玩法", "新", "/group/extended", "", "admin", 70}, []menuSeed{
			{"阵法管理", "阵", "/extended/formations", "formations", "admin", 10}, {"符箓管理", "符", "/extended/talismans", "talismans", "admin", 20},
			{"傀儡管理", "傀", "/extended/puppets", "puppets_config", "admin", 30}, {"秘境争夺", "秘", "/extended/secret-conflicts", "secret_conflicts", "admin", 40},
			{"传承管理", "传", "/extended/inheritances", "inheritances", "admin", 50}, {"悟道管理", "道", "/extended/dao-insights", "dao_insights", "admin", 60},
			{"仙魔战场", "战", "/extended/battlefields", "battlefields", "admin", 70}, {"灵根进化", "根", "/extended/root-evolutions", "root_evolutions", "admin", 80},
			{"渡劫心魔", "魔", "/extended/inner-demons", "inner_demons", "admin", 90}, {"合体技管理", "合", "/extended/couple-skills", "couple_skills", "admin", 100},
			{"仙药培育", "药", "/extended/immortal-herbs", "immortal_herbs", "admin", 110}, {"法宝炼化", "宝", "/extended/artifact-refinements", "artifact_refinements", "admin", 120},
			{"天机推演", "机", "/extended/destiny-deductions", "destiny_deductions", "admin", 130}, {"天地灵脉", "脉", "/extended/leylines", "leylines", "admin", 140},
			{"宗门战争", "宗", "/extended/sect-wars", "sect_wars", "admin", 150}, {"仙缘奇遇", "缘", "/extended/immortal-encounters", "immortal_encounters", "admin", 160},
			{"宇宙星河", "星", "/extended/star-realms", "star_realms", "admin", 170},
		}},
	}
	for _, group := range groups {
		parent, err := s.firstOrCreateMenu(0, "side", group.menuSeed)
		if err != nil {
			return err
		}
		for _, child := range group.Children {
			if _, err := s.firstOrCreateMenu(parent.ID, "side", child); err != nil {
				return err
			}
		}
	}

	// Upgrade only the historical default; preserve an operator-customized label.
	if err := s.DB.Model(&model.AdminMenu{}).Where("path = ? AND label = ?", "/reviews", "内容审核队列").Update("label", "内容与玩家反馈").Error; err != nil {
		return err
	}

	gameCategories := []string{"角色", "修炼", "探索", "地图", "挂机", "道侣", "战斗", "渡劫", "仙府", "功法", "灵兽", "装备", "图鉴", "社交", "交易", "任务", "活动", "特殊", "宗门", "丹药", "炼器", "副本", "竞技", "奇遇", "生涯", "阵法", "符箓", "傀儡", "秘境争夺", "传承", "悟道", "仙魔战场", "灵根进化", "渡劫心魔", "合体技", "仙药培育", "法宝炼化", "天机推演", "天地灵脉", "宗门战争", "仙缘奇遇", "宇宙星河", "系统", "氪金"}
	for index, category := range gameCategories {
		seed := menuSeed{Label: category, Icon: "令", Path: "/game/" + category, Component: "GameMenuCategory:" + category, Permission: "player", Sort: (index + 1) * 10}
		if _, err := s.firstOrCreateMenu(0, "top", seed); err != nil {
			return err
		}
	}
	// Migrate the old system-owned label without changing permissions or custom rows.
	if err := s.DB.Model(&model.AdminMenu{}).Where("component = ? AND permission = ? AND label = ?", "GameMenuCategory:管理", "admin", "管理").Updates(map[string]any{"label": "神令", "icon": "令", "path": "/game/神令"}).Error; err != nil {
		return err
	}
	management := menuSeed{Label: "神令", Icon: "令", Path: "/game/神令", Component: "GameMenuCategory:管理", Permission: "admin", Sort: 9990}
	if _, err := s.firstOrCreateMenu(0, "top", management); err != nil {
		return err
	}
	return nil
}

func (s *Store) firstOrCreateMenu(parentID uint, menuType string, seed menuSeed) (model.AdminMenu, error) {
	row := model.AdminMenu{
		ParentID: parentID, MenuType: menuType, Label: seed.Label, Icon: seed.Icon,
		Path: seed.Path, Component: seed.Component, Permission: seed.Permission,
		SortOrder: seed.Sort, Target: "_self", Status: "active",
	}
	err := s.DB.Where("path = ?", seed.Path).FirstOrCreate(&row).Error
	return row, err
}
