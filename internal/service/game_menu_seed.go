package service

import (
	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

// SeedPlayerCommandMenus materializes all 257 commands as editable menu rows.
func SeedPlayerCommandMenus(store *storage.Store) error {
	return store.DB.Transaction(func(tx *gorm.DB) error {
		return seedPlayerCommandMenus(tx)
	})
}

func seedPlayerCommandMenus(db *gorm.DB) error {
	// 旧版“收获”同时指向灵田和挂机，解析时挂机入口永远无法到达。
	// 只迁移挂机分类的系统菜单行，运营人员的其他自定义菜单不受影响。
	var afkParent model.AdminMenu
	if err := db.Where("component = ?", "GameMenuCategory:挂机").First(&afkParent).Error; err == nil {
		if err := db.Model(&model.AdminMenu{}).Where("parent_id = ? AND label IN ? AND path = ? AND component = ?", afkParent.ID, []string{"收获挂机", "挂机结算"}, "收获", "GameCommand").Updates(map[string]any{"label": "挂机结算", "path": "挂机结算"}).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&model.AdminMenu{}).Where("label = ? AND path = ? AND component = ?", "诸界传送列表", "传送列表", "GameCommand").Update("label", "当前界域传送阵图").Error; err != nil {
		return err
	}
	if err := db.Model(&model.AdminMenu{}).Where("label IN ? AND path IN ? AND component = ?", []string{"灵根融合", "灵根合成"}, []string{"灵融", "灵根合成", "合成灵根"}, "GameCommand").Update("label", "灵根随机重铸").Error; err != nil {
		return err
	}
	for _, spec := range handler.CommandTable {
		if spec.EventOnly {
			continue
		}
		var parent model.AdminMenu
		if err := db.Where("component = ?", "GameMenuCategory:"+spec.Category).First(&parent).Error; err != nil {
			continue
		}
		row := model.AdminMenu{
			ParentID: parent.ID, MenuType: "top", Label: spec.Name, Icon: "令", Path: spec.Command,
			Component: "GameCommand", Permission: "player", SortOrder: spec.ID * 10, Target: "_self", Status: "active",
		}
		// Command text can legitimately be shared by overloaded commands such as 传功.
		if err := db.Where("parent_id = ? AND label = ?", parent.ID, row.Label).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	// 辅助指令也必须自动进入对应玩家子菜单，避免新功能“可执行但找不到入口”。
	for _, spec := range handler.AuxiliaryCommands() {
		if spec.ID == 1000 || spec.ID == 1001 || spec.Category == "管理" {
			continue
		}
		var parent model.AdminMenu
		if err := db.Where("component = ?", "GameMenuCategory:"+spec.Category).First(&parent).Error; err != nil {
			continue
		}
		row := model.AdminMenu{
			ParentID: parent.ID, MenuType: "top", Label: spec.Name, Icon: "令", Path: spec.Command,
			Component: "GameCommand", Permission: "player", SortOrder: spec.ID * 10, Target: "_self", Status: "active",
		}
		if err := db.Where("parent_id = ? AND label = ?", parent.ID, row.Label).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	var taskParent model.AdminMenu
	if err := db.Where("component = ?", "GameMenuCategory:任务").First(&taskParent).Error; err == nil {
		for _, row := range []model.AdminMenu{
			{ParentID: taskParent.ID, MenuType: "top", Label: "每日签到", Icon: "签", Path: "签到", Component: "GameCommand", Permission: "player", SortOrder: 905, Target: "_self", Status: "active"},
			{ParentID: taskParent.ID, MenuType: "top", Label: "签到记录", Icon: "录", Path: "签到记录", Component: "GameCommand", Permission: "player", SortOrder: 906, Target: "_self", Status: "active"},
		} {
			if err := db.Where("parent_id = ? AND label = ?", row.ParentID, row.Label).FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
	}
	auxiliaryMenus := []struct {
		category string
		rows     []model.AdminMenu
	}{
		{"交易", []model.AdminMenu{
			{MenuType: "top", Label: "仙门货铺", Icon: "铺", Path: "货铺", Component: "GameCommand", Permission: "player", SortOrder: 895, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "买下摊品", Icon: "购", Path: "买下", Component: "GameCommand", Permission: "player", SortOrder: 896, Target: "_self", Status: "active"},
		}},
		{"仙府", []model.AdminMenu{
			{MenuType: "top", Label: "种子商店", Icon: "种", Path: "种子商店", Component: "GameCommand", Permission: "player", SortOrder: 615, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "仙府灵田", Icon: "田", Path: "灵田", Component: "GameCommand", Permission: "player", SortOrder: 616, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "灵田仓库", Icon: "仓", Path: "灵田仓库", Component: "GameCommand", Permission: "player", SortOrder: 617, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "灵田说明", Icon: "说", Path: "灵田说明", Component: "GameCommand", Permission: "player", SortOrder: 618, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "灵肥图鉴", Icon: "肥", Path: "灵肥图鉴", Component: "GameCommand", Permission: "player", SortOrder: 619, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "一键施肥", Icon: "施", Path: "一键施肥", Component: "GameCommand", Permission: "player", SortOrder: 620, Target: "_self", Status: "active"},
		}},
		{"特殊", []model.AdminMenu{
			{MenuType: "top", Label: "仙途礼包", Icon: "礼", Path: "礼包", Component: "GameCommand", Permission: "player", SortOrder: 1005, Target: "_self", Status: "active"},
		}},
		{"装备", []model.AdminMenu{
			{MenuType: "top", Label: "装备系统", Icon: "装", Path: "装备系统", Component: "GameCommand", Permission: "player", SortOrder: 1205, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "装备背包", Icon: "袋", Path: "装备背包", Component: "GameCommand", Permission: "player", SortOrder: 1206, Target: "_self", Status: "active"},
			{MenuType: "top", Label: "装备图鉴", Icon: "鉴", Path: "装备图鉴", Component: "GameCommand", Permission: "player", SortOrder: 1207, Target: "_self", Status: "active"},
		}},
	}
	for _, group := range auxiliaryMenus {
		var parent model.AdminMenu
		if err := db.Where("component = ?", "GameMenuCategory:"+group.category).First(&parent).Error; err != nil {
			continue
		}
		for _, row := range group.rows {
			row.ParentID = parent.ID
			if err := db.Where("parent_id = ? AND label = ?", row.ParentID, row.Label).FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
	}
	// Move only known system equipment entries out of the legacy炼器分类.
	var equipmentParent model.AdminMenu
	if err := db.Where("component = ?", "GameMenuCategory:装备").First(&equipmentParent).Error; err == nil {
		var forgeParent model.AdminMenu
		if db.Where("component = ?", "GameMenuCategory:炼器").First(&forgeParent).Error == nil {
			labels := []string{"装备系统", "当前装备", "装备背包", "穿戴装备", "卸下装备", "一键卸下", "装备锻造", "装备篆刻", "装备图鉴", "装备详情", "器谱详情"}
			if err := db.Model(&model.AdminMenu{}).Where("parent_id = ? AND label IN ? AND component = ?", forgeParent.ID, labels, "GameCommand").Update("parent_id", equipmentParent.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
