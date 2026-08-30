package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

type menuNode struct {
	model.AdminMenu
	Children []menuNode `json:"children,omitempty"`
}

func (a *AdminAPI) handleMenus(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 {
		switch r.Method {
		case http.MethodGet:
			a.getMenuTree(w, r)
		case http.MethodPost:
			a.createMenu(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "不支持的菜单操作")
		}
		return
	}
	switch segments[0] {
	case "list":
		if len(segments) == 1 && r.Method == http.MethodGet {
			a.listMenus(w, r)
			return
		}
	case "export":
		if len(segments) == 1 && r.Method == http.MethodGet {
			a.exportMenus(w)
			return
		}
	case "import":
		if len(segments) == 1 && r.Method == http.MethodPost {
			a.importMenus(w, r)
			return
		}
	case "sort":
		if len(segments) == 1 && r.Method == http.MethodPut {
			a.sortMenus(w, r)
			return
		}
	}

	id, err := strconv.ParseUint(segments[0], 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "菜单ID不正确")
		return
	}
	menuID := uint(id)
	if len(segments) == 2 && r.Method == http.MethodPut {
		switch segments[1] {
		case "move":
			a.moveMenu(w, r, menuID)
			return
		case "hide":
			a.hideMenu(w, r, menuID)
			return
		}
	}
	if len(segments) != 1 {
		writeError(w, http.StatusNotFound, "菜单路径不存在")
		return
	}
	switch r.Method {
	case http.MethodGet:
		var row model.AdminMenu
		if err := a.store.DB.First(&row, menuID).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "读取成功")
	case http.MethodPut:
		a.updateMenu(w, r, menuID)
	case http.MethodDelete:
		a.deleteMenu(w, menuID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的菜单操作")
	}
}

func (a *AdminAPI) getMenuTree(w http.ResponseWriter, r *http.Request) {
	menuType := strings.TrimSpace(r.URL.Query().Get("type"))
	if menuType == "" {
		menuType = "side"
	}
	query := a.store.DB.Model(&model.AdminMenu{})
	if menuType != "all" {
		query = query.Where("menu_type IN ?", []string{menuType, "both"})
	}
	if r.URL.Query().Get("include_hidden") != "1" {
		query = query.Where("is_hidden = ? AND status = ?", false, "active")
	}
	var rows []model.AdminMenu
	if err := query.Order("parent_id, sort_order, id").Find(&rows).Error; err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, buildMenuTree(rows, 0), "读取成功")
}

func buildMenuTree(rows []model.AdminMenu, parentID uint) []menuNode {
	result := make([]menuNode, 0)
	for _, row := range rows {
		if row.ParentID != parentID {
			continue
		}
		result = append(result, menuNode{AdminMenu: row, Children: buildMenuTree(rows, row.ID)})
	}
	return result
}

func (a *AdminAPI) listMenus(w http.ResponseWriter, r *http.Request) {
	page, size := pagination(r)
	query := a.store.DB.Model(&model.AdminMenu{})
	if keyword := strings.TrimSpace(r.URL.Query().Get("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("label LIKE ? OR path LIKE ? OR component LIKE ? OR permission LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeDBError(w, err)
		return
	}
	var rows []model.AdminMenu
	if err := query.Order("menu_type, parent_id, sort_order, id").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, listResponse{Items: rows, Total: total, Page: page, Size: size}, "读取成功")
}

func (a *AdminAPI) createMenu(w http.ResponseWriter, r *http.Request) {
	var row model.AdminMenu
	if err := decodeJSON(r, &row); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row.ID = 0
	applyMenuDefaults(&row)
	if err := a.validateMenu(row, 0); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.DB.Create(&row).Error; err != nil {
		writeDBError(w, err)
		return
	}
	a.logMenu("create", row.ID, nil, row)
	writeOK(w, row, "菜单已新增并立即生效")
}

func (a *AdminAPI) updateMenu(w http.ResponseWriter, r *http.Request, id uint) {
	var old model.AdminMenu
	if err := a.store.DB.First(&old, id).Error; err != nil {
		writeDBError(w, err)
		return
	}
	var changes map[string]any
	if err := decodeJSON(r, &changes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stripProtectedFields(changes)
	preview := old
	encoded, _ := json.Marshal(changes)
	if err := json.Unmarshal(encoded, &preview); err != nil {
		writeError(w, http.StatusBadRequest, "菜单字段格式不正确")
		return
	}
	applyMenuDefaults(&preview)
	if err := a.validateMenu(preview, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.DB.Model(&old).Updates(changes).Error; err != nil {
		writeDBError(w, err)
		return
	}
	var updated model.AdminMenu
	if err := a.store.DB.First(&updated, id).Error; err != nil {
		writeDBError(w, err)
		return
	}
	a.logMenu("update", id, old, updated)
	writeOK(w, updated, "菜单已保存并立即生效")
}

func (a *AdminAPI) deleteMenu(w http.ResponseWriter, id uint) {
	var row model.AdminMenu
	if err := a.store.DB.First(&row, id).Error; err != nil {
		writeDBError(w, err)
		return
	}
	var children int64
	if err := a.store.DB.Model(&model.AdminMenu{}).Where("parent_id = ?", id).Count(&children).Error; err != nil {
		writeDBError(w, err)
		return
	}
	if children > 0 {
		writeError(w, http.StatusConflict, "该菜单含有子菜单，请先移动或删除子菜单")
		return
	}
	if err := a.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", id).Delete(&model.AdminMenuPermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	}); err != nil {
		writeDBError(w, err)
		return
	}
	a.logMenu("delete", id, row, nil)
	writeOK(w, nil, "菜单已删除")
}

func (a *AdminAPI) hideMenu(w http.ResponseWriter, r *http.Request, id uint) {
	var input struct {
		IsHidden bool `json:"is_hidden"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.DB.Model(&model.AdminMenu{}).Where("id = ?", id).Update("is_hidden", input.IsHidden).Error; err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil, "显示状态已更新")
}

func (a *AdminAPI) moveMenu(w http.ResponseWriter, r *http.Request, id uint) {
	var input struct {
		TargetParentID uint `json:"target_parent_id"`
		TargetPosition int  `json:"target_position"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var row model.AdminMenu
	if err := a.store.DB.First(&row, id).Error; err != nil {
		writeDBError(w, err)
		return
	}
	preview := row
	preview.ParentID = input.TargetParentID
	preview.SortOrder = input.TargetPosition
	if err := a.validateMenu(preview, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	old := row
	err := a.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&row).Updates(map[string]any{"parent_id": input.TargetParentID, "sort_order": input.TargetPosition}).Error; err != nil {
			return err
		}
		return normalizeMenuOrder(tx, input.TargetParentID)
	})
	if err != nil {
		writeDBError(w, err)
		return
	}
	a.logMenu("move", id, old, preview)
	writeOK(w, nil, "菜单已移动")
}

func normalizeMenuOrder(tx *gorm.DB, parentID uint) error {
	var rows []model.AdminMenu
	if err := tx.Where("parent_id = ?", parentID).Order("sort_order, id").Find(&rows).Error; err != nil {
		return err
	}
	for index := range rows {
		if err := tx.Model(&model.AdminMenu{}).Where("id = ?", rows[index].ID).Update("sort_order", (index+1)*10).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *AdminAPI) sortMenus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Menus []struct {
			ID        uint `json:"id"`
			SortOrder int  `json:"sort_order"`
		} `json:"menus"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err := a.store.DB.Transaction(func(tx *gorm.DB) error {
		for _, row := range input.Menus {
			if row.ID == 0 {
				continue
			}
			if err := tx.Model(&model.AdminMenu{}).Where("id = ?", row.ID).Update("sort_order", row.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil, "菜单排序已保存")
}

func (a *AdminAPI) exportMenus(w http.ResponseWriter) {
	var rows []model.AdminMenu
	if err := a.store.DB.Order("menu_type, parent_id, sort_order, id").Find(&rows).Error; err != nil {
		writeDBError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="menus.json"`)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(rows)
}

func (a *AdminAPI) importMenus(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择菜单JSON文件")
		return
	}
	defer file.Close()
	var rows []model.AdminMenu
	if err := json.NewDecoder(io.LimitReader(file, 20<<20)).Decode(&rows); err != nil {
		writeError(w, http.StatusBadRequest, "菜单JSON解析失败: "+err.Error())
		return
	}
	for index := range rows {
		applyMenuDefaults(&rows[index])
		if strings.TrimSpace(rows[index].Label) == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("第%d条菜单缺少名称", index+1))
			return
		}
	}
	mode := r.URL.Query().Get("mode")
	err = a.store.DB.Transaction(func(tx *gorm.DB) error {
		if mode == "replace" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.AdminMenuPermission{}).Error; err != nil {
				return err
			}
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.AdminMenu{}).Error; err != nil {
				return err
			}
			return tx.Create(&rows).Error
		}
		for index := range rows {
			row := rows[index]
			if row.ID != 0 {
				var existing model.AdminMenu
				if tx.First(&existing, row.ID).Error == nil {
					if err := tx.Model(&existing).Updates(row).Error; err != nil {
						return err
					}
					continue
				}
			}
			row.ID = 0
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil, "菜单导入完成并立即生效")
}

func applyMenuDefaults(row *model.AdminMenu) {
	row.Label = strings.TrimSpace(row.Label)
	row.MenuType = strings.TrimSpace(row.MenuType)
	if row.MenuType == "" {
		row.MenuType = "side"
	}
	row.Status = strings.TrimSpace(row.Status)
	if row.Status == "" {
		row.Status = "active"
	}
	row.Target = strings.TrimSpace(row.Target)
	if row.Target == "" {
		row.Target = "_self"
	}
	row.Path = strings.TrimSpace(row.Path)
	row.Component = strings.TrimSpace(row.Component)
	row.Permission = strings.TrimSpace(row.Permission)
}

func (a *AdminAPI) validateMenu(row model.AdminMenu, selfID uint) error {
	if row.Label == "" {
		return fmt.Errorf("菜单名称不能为空")
	}
	if row.MenuType != "side" && row.MenuType != "top" && row.MenuType != "both" {
		return fmt.Errorf("菜单类型只能是 side、top 或 both")
	}
	if row.Status != "active" && row.Status != "inactive" {
		return fmt.Errorf("菜单状态只能是 active 或 inactive")
	}
	if row.Target != "_self" && row.Target != "_blank" {
		return fmt.Errorf("打开方式只能是 _self 或 _blank")
	}
	if row.IsExternal && strings.TrimSpace(row.ExternalURL) == "" {
		return fmt.Errorf("外部菜单必须填写链接")
	}
	if row.ParentID == 0 {
		return nil
	}
	if row.ParentID == selfID {
		return fmt.Errorf("菜单不能成为自己的父菜单")
	}
	var parent model.AdminMenu
	if err := a.store.DB.First(&parent, row.ParentID).Error; err != nil {
		return fmt.Errorf("父菜单不存在")
	}
	for parent.ParentID != 0 {
		if parent.ParentID == selfID {
			return fmt.Errorf("不能把菜单移动到自己的子菜单下")
		}
		if err := a.store.DB.First(&parent, parent.ParentID).Error; err != nil {
			break
		}
	}
	return nil
}

func (a *AdminAPI) logMenu(action string, id uint, oldValue, newValue any) {
	row := model.AdminMenuLog{MenuID: id, Action: action, Operator: "local-admin"}
	if oldValue != nil {
		data, _ := json.Marshal(oldValue)
		row.OldData = string(data)
	}
	if newValue != nil {
		data, _ := json.Marshal(newValue)
		row.NewData = string(data)
	}
	_ = a.store.DB.Create(&row).Error
}
