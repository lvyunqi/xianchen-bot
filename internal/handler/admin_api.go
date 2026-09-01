package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type listResponse struct {
	Items any   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

type resourceSpec struct {
	New        func() any
	NewSlice   func() any
	SearchCols []string
}

var adminResources = map[string]resourceSpec{
	"items":                {func() any { return &model.Item{} }, func() any { return &[]model.Item{} }, []string{"code", "name", "category_name", "rarity_name"}},
	"events":               {func() any { return &model.Event{} }, func() any { return &[]model.Event{} }, []string{"name", "type", "description"}},
	"tasks":                {func() any { return &model.TaskTemplate{} }, func() any { return &[]model.TaskTemplate{} }, []string{"name", "type", "description"}},
	"skills":               {func() any { return &model.Skill{} }, func() any { return &[]model.Skill{} }, []string{"name", "type", "rarity"}},
	"pets":                 {func() any { return &model.PetTemplate{} }, func() any { return &[]model.PetTemplate{} }, []string{"code", "name"}},
	"dungeons":             {func() any { return &model.Dungeon{} }, func() any { return &[]model.Dungeon{} }, []string{"code", "name", "difficulty"}},
	"arena-tiers":          {func() any { return &model.ArenaTier{} }, func() any { return &[]model.ArenaTier{} }, []string{"code", "name", "description"}},
	"titles":               {func() any { return &model.Title{} }, func() any { return &[]model.Title{} }, []string{"code", "name", "type", "condition"}},
	"activities":           {func() any { return &model.Activity{} }, func() any { return &[]model.Activity{} }, []string{"code", "name", "type", "status"}},
	"mails":                {func() any { return &model.Mail{} }, func() any { return &[]model.Mail{} }, []string{"code", "title", "sender", "target_id"}},
	"checkin":              {func() any { return &model.CheckinReward{} }, func() any { return &[]model.CheckinReward{} }, []string{"item_name", "special_reward"}},
	"shop":                 {func() any { return &model.ShopEntry{} }, func() any { return &[]model.ShopEntry{} }, []string{"code", "item_name", "currency"}},
	"cdks":                 {func() any { return &model.RedemptionCode{} }, func() any { return &[]model.RedemptionCode{} }, []string{"code", "status"}},
	"notices":              {func() any { return &model.Notice{} }, func() any { return &[]model.Notice{} }, []string{"code", "title", "type", "content"}},
	"recipes":              {func() any { return &model.AlchemyRecipe{} }, func() any { return &[]model.AlchemyRecipe{} }, []string{"code", "name", "description"}},
	"artifacts":            {func() any { return &model.ArtifactTemplate{} }, func() any { return &[]model.ArtifactTemplate{} }, []string{"code", "name", "type"}},
	"synthesis-recipes":    {func() any { return &model.SynthesisRecipe{} }, func() any { return &[]model.SynthesisRecipe{} }, []string{"code", "name", "category", "output_name"}},
	"reviews":              {func() any { return &model.ContentReview{} }, func() any { return &[]model.ContentReview{} }, []string{"type", "player_name", "content", "status"}},
	"sensitive-words":      {func() any { return &model.SensitiveWord{} }, func() any { return &[]model.SensitiveWord{} }, []string{"word", "replacement"}},
	"slow-queries":         {func() any { return &model.SlowQueryLog{} }, func() any { return &[]model.SlowQueryLog{} }, []string{"sql", "source"}},
	"formations":           {func() any { return &model.FormationConfig{} }, func() any { return &[]model.FormationConfig{} }, []string{"code", "name", "type", "description"}},
	"talismans":            {func() any { return &model.TalismanConfig{} }, func() any { return &[]model.TalismanConfig{} }, []string{"code", "name", "type", "description"}},
	"puppets-config":       {func() any { return &model.PuppetConfig{} }, func() any { return &[]model.PuppetConfig{} }, []string{"code", "name", "type", "description"}},
	"secret-conflicts":     {func() any { return &model.SecretRealmConflictConfig{} }, func() any { return &[]model.SecretRealmConflictConfig{} }, []string{"code", "name", "type", "description"}},
	"inheritances":         {func() any { return &model.InheritanceConfig{} }, func() any { return &[]model.InheritanceConfig{} }, []string{"code", "name", "type", "description"}},
	"dao-insights":         {func() any { return &model.DaoInsightConfig{} }, func() any { return &[]model.DaoInsightConfig{} }, []string{"code", "name", "type", "description"}},
	"battlefields":         {func() any { return &model.ImmortalDemonBattlefieldConfig{} }, func() any { return &[]model.ImmortalDemonBattlefieldConfig{} }, []string{"code", "name", "type", "description"}},
	"root-evolutions":      {func() any { return &model.SpiritualRootEvolutionConfig{} }, func() any { return &[]model.SpiritualRootEvolutionConfig{} }, []string{"code", "name", "type", "description"}},
	"inner-demons":         {func() any { return &model.InnerDemonConfig{} }, func() any { return &[]model.InnerDemonConfig{} }, []string{"code", "name", "type", "description"}},
	"couple-skills":        {func() any { return &model.CoupleCombinationSkillConfig{} }, func() any { return &[]model.CoupleCombinationSkillConfig{} }, []string{"code", "name", "type", "description"}},
	"immortal-herbs":       {func() any { return &model.ImmortalHerbConfig{} }, func() any { return &[]model.ImmortalHerbConfig{} }, []string{"code", "name", "type", "description"}},
	"artifact-refinements": {func() any { return &model.ArtifactRefinementConfig{} }, func() any { return &[]model.ArtifactRefinementConfig{} }, []string{"code", "name", "type", "description"}},
	"destiny-deductions":   {func() any { return &model.DestinyDeductionConfig{} }, func() any { return &[]model.DestinyDeductionConfig{} }, []string{"code", "name", "type", "description"}},
	"leylines":             {func() any { return &model.LeylineConfig{} }, func() any { return &[]model.LeylineConfig{} }, []string{"code", "name", "type", "description"}},
	"sect-wars":            {func() any { return &model.SectWarConfig{} }, func() any { return &[]model.SectWarConfig{} }, []string{"code", "name", "type", "description"}},
	"immortal-encounters":  {func() any { return &model.ImmortalEncounterConfig{} }, func() any { return &[]model.ImmortalEncounterConfig{} }, []string{"code", "name", "type", "description"}},
	"star-realms":          {func() any { return &model.StarRealmConfig{} }, func() any { return &[]model.StarRealmConfig{} }, []string{"code", "name", "type", "description"}},
	"locations":            {func() any { return &model.WorldLocation{} }, func() any { return &[]model.WorldLocation{} }, []string{"code", "name", "region", "description"}},
	"spiritual-roots":      {func() any { return &model.SpiritualRootTemplate{} }, func() any { return &[]model.SpiritualRootTemplate{} }, []string{"code", "name", "element", "grade", "combat_description"}},
	"world-leylines":       {func() any { return &model.WorldLeyline{} }, func() any { return &[]model.WorldLeyline{} }, []string{"code", "name", "region", "location_name", "element", "grade", "description"}},
	"managers":             {func() any { return &model.ManagerAccount{} }, func() any { return &[]model.ManagerAccount{} }, []string{"user_id", "name", "permissions"}},
}

type AdminAPI struct {
	store     *storage.Store
	static    fs.FS
	uploadDir string
}

func NewAdminMux(store *storage.Store, static fs.FS, uploadDir string) http.Handler {
	api := &AdminAPI{store: store, static: static, uploadDir: uploadDir}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", api.handleAPI)
	mux.HandleFunc("/uploads/", api.serveUpload)
	mux.HandleFunc("/admin", api.serveAdmin)
	mux.HandleFunc("/admin/", api.serveAdmin)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusFound)
	})
	return monitorMiddleware(recoveryMiddleware(corsMiddleware(mux)))
}

func (a *AdminAPI) handleAPI(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(strings.TrimPrefix(r.URL.Path, "/api/"))
	if len(segments) == 0 {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	if segments[0] == "config" {
		a.handleConfig(w, r, segments[1:])
		return
	}
	if segments[0] == "realms" {
		a.handleRealms(w, r, segments[1:])
		return
	}
	if segments[0] == "players" {
		a.handlePlayers(w, r, segments[1:])
		return
	}
	if segments[0] == "couples" {
		a.handleCouples(w, r, segments[1:])
		return
	}
	if segments[0] == "menus" {
		a.handleMenus(w, r, segments[1:])
		return
	}
	if segments[0] == "dashboard" {
		a.handleDashboard(w, r)
		return
	}
	if segments[0] == "monitor" {
		a.handleMonitor(w, r)
		return
	}
	if segments[0] == "export" && len(segments) == 2 {
		a.exportData(w, r, segments[1])
		return
	}
	if segments[0] == "import" && len(segments) == 2 {
		a.importData(w, r, segments[1])
		return
	}
	if segments[0] == "upload" {
		a.uploadImage(w, r)
		return
	}
	if spec, ok := adminResources[segments[0]]; ok {
		a.handleResource(w, r, segments[0], segments[1:], spec)
		return
	}
	writeError(w, http.StatusNotFound, "接口不存在")
}

func (a *AdminAPI) handleConfig(w http.ResponseWriter, r *http.Request, segments []string) {
	switch {
	case r.Method == http.MethodGet && len(segments) == 0:
		query := a.store.DB.Model(&model.SystemSetting{})
		if prefix := strings.TrimSpace(r.URL.Query().Get("prefix")); prefix != "" {
			query = query.Where("key LIKE ?", prefix+"%")
		}
		if keyword := strings.TrimSpace(r.URL.Query().Get("keyword")); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("key LIKE ? OR value LIKE ? OR description LIKE ?", like, like, like)
		}
		// Older clients expect the original array response. The admin UI sends a
		// page parameter and receives a bounded result so thousands of generated
		// settings never freeze the browser while rendering one giant table.
		if _, paged := r.URL.Query()["page"]; paged {
			page, size := pagination(r)
			var total int64
			if err := query.Count(&total).Error; err != nil {
				writeDBError(w, err)
				return
			}
			var rows []model.SystemSetting
			if err := query.Order("key").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
				writeDBError(w, err)
				return
			}
			writeOK(w, listResponse{Items: rows, Total: total, Page: page, Size: size}, "读取成功")
			return
		}
		var rows []model.SystemSetting
		if err := query.Order("key").Find(&rows).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, rows, "读取成功")
	case r.Method == http.MethodPut && len(segments) == 1:
		var input struct {
			Value       any    `json:"value"`
			ValueType   string `json:"value_type"`
			Description string `json:"description"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		value := fmt.Sprint(input.Value)
		if raw, ok := input.Value.(map[string]any); ok {
			encoded, _ := json.Marshal(raw)
			value = string(encoded)
		}
		row := model.SystemSetting{Key: segments[0]}
		err := a.store.DB.Where("key = ?", segments[0]).FirstOrCreate(&row).Error
		if err == nil {
			updates := map[string]any{"value": value}
			if input.ValueType != "" {
				updates["value_type"] = input.ValueType
			}
			if input.Description != "" {
				updates["description"] = input.Description
			}
			err = a.store.DB.Model(&row).Updates(updates).Error
		}
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "配置已更新，立即生效")
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的配置操作")
	}
}

func (a *AdminAPI) handleRealms(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) > 0 {
		spec := resourceSpec{func() any { return &model.Realm{} }, func() any { return &[]model.Realm{} }, []string{"name", "description"}}
		a.handleResource(w, r, "realms", segments, spec)
		return
	}
	switch r.Method {
	case http.MethodGet:
		var rows []model.Realm
		if err := a.store.DB.Order("sequence").Find(&rows).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, listResponse{Items: rows, Total: int64(len(rows)), Page: 1, Size: len(rows)}, "读取成功")
	case http.MethodPut:
		var rows []model.Realm
		if err := decodeJSON(r, &rows); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		err := a.store.DB.Transaction(func(tx *gorm.DB) error {
			for index := range rows {
				if rows[index].Sequence == 0 {
					rows[index].Sequence = index + 1
				}
				if err := tx.Save(&rows[index]).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, rows, "境界配置已保存")
	case http.MethodPost:
		var row model.Realm
		if err := decodeJSON(r, &row); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.store.DB.Create(&row).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "境界已新增")
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的境界操作")
	}
}

func (a *AdminAPI) handleResource(w http.ResponseWriter, r *http.Request, resource string, segments []string, spec resourceSpec) {
	if len(segments) == 2 && r.Method == http.MethodPost {
		a.handleResourceAction(w, resource, segments[0], segments[1])
		return
	}
	if len(segments) == 0 {
		switch r.Method {
		case http.MethodGet:
			a.listResource(w, r, resource, spec)
		case http.MethodPost:
			row := spec.New()
			if err := decodeJSON(r, row); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := a.prepareResourceCreate(resource, row); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := a.store.DB.Create(row).Error; err != nil {
				writeDBError(w, err)
				return
			}
			writeOK(w, row, "新增成功")
		default:
			writeError(w, http.StatusMethodNotAllowed, "不支持的操作")
		}
		return
	}
	if len(segments) != 1 {
		writeError(w, http.StatusNotFound, "资源路径错误")
		return
	}
	id, err := strconv.ParseUint(segments[0], 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "ID不正确")
		return
	}
	row := spec.New()
	switch r.Method {
	case http.MethodGet:
		if err := a.store.DB.First(row, id).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "读取成功")
	case http.MethodPut:
		var changes map[string]any
		if err := decodeJSON(r, &changes); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		stripProtectedFields(changes)
		if err := a.prepareResourceUpdate(resource, changes); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.store.DB.Model(row).Where("id = ?", id).Updates(changes).Error; err != nil {
			writeDBError(w, err)
			return
		}
		if err := a.store.DB.First(row, id).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "保存成功，游戏内立即生效")
	case http.MethodDelete:
		if err := a.store.DB.Delete(row, id).Error; err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "删除成功")
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的操作")
	}
}

func (a *AdminAPI) listResource(w http.ResponseWriter, r *http.Request, resource string, spec resourceSpec) {
	page, size := pagination(r)
	rows := spec.NewSlice()
	query := a.store.DB.Model(spec.New())
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword != "" && len(spec.SearchCols) > 0 {
		like := "%" + keyword + "%"
		conditions := make([]string, 0, len(spec.SearchCols))
		args := make([]any, 0, len(spec.SearchCols))
		for _, column := range spec.SearchCols {
			conditions = append(conditions, column+" LIKE ?")
			args = append(args, like)
		}
		query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && resource != "notices" {
		query = query.Where("status = ?", status)
	}
	if resource == "notices" {
		if noticeType := strings.TrimSpace(r.URL.Query().Get("type")); noticeType != "" {
			query = query.Where("type = ?", noticeType)
		}
		if rawPublished := strings.TrimSpace(r.URL.Query().Get("published")); rawPublished != "" {
			published, err := strconv.ParseBool(rawPublished)
			if err != nil {
				writeError(w, http.StatusBadRequest, "发布状态必须是true或false")
				return
			}
			query = query.Where("published = ?", published)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeDBError(w, err)
		return
	}
	if err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(rows).Error; err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, listResponse{Items: rows, Total: total, Page: page, Size: size}, "读取成功")
}

func (a *AdminAPI) handleResourceAction(w http.ResponseWriter, resource, idText, action string) {
	id, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID不正确")
		return
	}
	now := time.Now()
	switch {
	case resource == "mails" && action == "send":
		err = a.store.DB.Model(&model.Mail{}).Where("id = ?", id).Updates(map[string]any{"sent": true, "sent_at": &now}).Error
	case resource == "notices" && action == "publish":
		err = a.store.DB.Model(&model.Notice{}).Where("id = ?", id).Updates(map[string]any{"published": true, "published_at": &now}).Error
	case resource == "cdks" && action == "use":
		err = a.store.DB.Model(&model.RedemptionCode{}).Where("id = ? AND status = ? AND used_count < max_uses", id, "有效").Updates(map[string]any{"used_count": gorm.Expr("used_count + 1")}).Error
	case resource == "reviews" && (action == "approve" || action == "reject" || action == "resolve"):
		var review model.ContentReview
		if err = a.store.DB.First(&review, id).Error; err != nil {
			break
		}
		updates := map[string]any{"reviewed_at": &now}
		switch action {
		case "approve":
			updates["status"] = "已通过"
			if strings.TrimSpace(review.ResolutionType) == "" {
				updates["resolution_type"] = "人工评审"
			}
			if strings.TrimSpace(review.Resolution) == "" {
				updates["resolution"] = "管理员审核通过；玩法建议进入排期，其他内容完成审核。"
			}
		case "reject":
			updates["status"] = "已拒绝"
			updates["resolved_at"] = &now
			if strings.TrimSpace(review.ResolutionType) == "" {
				updates["resolution_type"] = "人工评审"
			}
			if strings.TrimSpace(review.Resolution) == "" {
				updates["resolution"] = "管理员审核未通过，未执行任何数据或程序修改。"
			}
		case "resolve":
			if review.Type != "BUG反馈" {
				writeError(w, http.StatusBadRequest, "只有BUG反馈可以标记为已修复")
				return
			}
			updates["status"] = "已修复"
			updates["resolved_at"] = &now
			if strings.TrimSpace(review.ResolutionType) == "" || review.ResolutionType == "人工排查" || review.ResolutionType == "执行链排查" {
				updates["resolution_type"] = "人工修复"
			}
			if strings.TrimSpace(review.Resolution) == "" {
				updates["resolution"] = "管理员已完成修复并核验。"
			}
		}
		err = a.store.DB.Model(&model.ContentReview{}).Where("id = ?", id).Updates(updates).Error
	default:
		writeError(w, http.StatusNotFound, "操作不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil, "操作成功")
}

func (a *AdminAPI) handlePlayers(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "不支持的玩家操作")
			return
		}
		page, size := pagination(r)
		rows, total, err := storage.NewPlayerRepository(a.store.DB).List(r.URL.Query().Get("keyword"), (page-1)*size, size)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, listResponse{Items: rows, Total: total, Page: page, Size: size}, "读取成功")
		return
	}
	accountID := segments[0]
	var player model.Player
	if err := a.store.DB.Where("account_id = ?", accountID).First(&player).Error; err != nil {
		writeDBError(w, err)
		return
	}
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			inventory, _ := storage.NewPlayerRepository(a.store.DB).Inventory(player.ID)
			writeOK(w, map[string]any{"player": player, "inventory": inventory}, "读取成功")
		case http.MethodPut:
			var changes map[string]any
			if err := decodeJSON(r, &changes); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			stripProtectedFields(changes)
			delete(changes, "account_id")
			if err := a.store.DB.Model(&player).Updates(changes).Error; err != nil {
				writeDBError(w, err)
				return
			}
			writeOK(w, nil, "玩家数据已保存")
		case http.MethodDelete:
			if err := storage.NewPlayerRepository(a.store.DB).Delete(player.ID); err != nil {
				writeDBError(w, err)
				return
			}
			writeOK(w, nil, "玩家及关联动态数据已永久删除，道号已释放，可重新注册")
		default:
			writeError(w, http.StatusMethodNotAllowed, "不支持的玩家操作")
		}
		return
	}
	action := segments[1]
	switch {
	case action == "ban" && r.Method == http.MethodPost:
		var input struct {
			Reason string `json:"reason"`
		}
		_ = decodeJSON(r, &input)
		err := a.store.DB.Model(&player).Updates(map[string]any{"banned": true, "ban_reason": input.Reason}).Error
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "玩家已封禁")
	case action == "unban" && r.Method == http.MethodPost:
		err := a.store.DB.Model(&player).Updates(map[string]any{"banned": false, "ban_reason": ""}).Error
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "玩家已解禁")
	case action == "items" && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		var input struct {
			ItemID   uint  `json:"item_id"`
			Quantity int64 `json:"quantity"`
		}
		if err := decodeJSON(r, &input); err != nil || input.ItemID == 0 || input.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, "物品和数量不正确")
			return
		}
		delta := input.Quantity
		if r.Method == http.MethodDelete {
			delta = -delta
		}
		if err := storage.NewPlayerRepository(a.store.DB).AdjustItem(player.ID, input.ItemID, delta); err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "背包已更新")
	default:
		writeError(w, http.StatusNotFound, "玩家操作不存在")
	}
}

func (a *AdminAPI) handleCouples(w http.ResponseWriter, r *http.Request, segments []string) {
	repo := storage.NewCoupleRepository(a.store.DB)
	if len(segments) == 0 {
		if r.Method == http.MethodGet {
			page, size := pagination(r)
			rows, total, err := repo.List((page-1)*size, size)
			if err != nil {
				writeDBError(w, err)
				return
			}
			writeOK(w, listResponse{Items: rows, Total: total, Page: page, Size: size}, "读取成功")
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "不支持的仙侣操作")
		return
	}
	if segments[0] == "force" && r.Method == http.MethodPost {
		var input struct {
			PlayerAID uint `json:"player_a_id"`
			PlayerBID uint `json:"player_b_id"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		row, err := repo.ForceBond(input.PlayerAID, input.PlayerBID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "强制结缘成功")
		return
	}
	id, err := strconv.ParseUint(segments[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID不正确")
		return
	}
	switch r.Method {
	case http.MethodGet:
		row, err := repo.Get(uint(id))
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, row, "读取成功")
	case http.MethodPut:
		var changes map[string]any
		if err := decodeJSON(r, &changes); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		stripProtectedFields(changes)
		if err := repo.Update(uint(id), changes); err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "仙侣数据已保存")
	case http.MethodDelete:
		if err := repo.ForceDissolve(uint(id)); err != nil {
			writeDBError(w, err)
			return
		}
		writeOK(w, nil, "强制解缘成功")
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的仙侣操作")
	}
}

func (a *AdminAPI) exportData(w http.ResponseWriter, r *http.Request, resource string) {
	spec, ok := adminResources[resource]
	if !ok {
		writeError(w, http.StatusNotFound, "不支持导出该类型")
		return
	}
	rows := spec.NewSlice()
	if err := a.store.DB.Order("id").Find(rows).Error; err != nil {
		writeDBError(w, err)
		return
	}
	if strings.EqualFold(r.URL.Query().Get("format"), "xlsx") {
		writeExcelResource(w, resource, rows)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+resource+`.json"`)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(rows)
}

func (a *AdminAPI) importData(w http.ResponseWriter, r *http.Request, resource string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请使用POST上传")
		return
	}
	spec, ok := adminResources[resource]
	if !ok {
		writeError(w, http.StatusNotFound, "不支持导入该类型")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择JSON文件")
		return
	}
	defer file.Close()
	var rows any
	if strings.EqualFold(filepath.Ext(header.Filename), ".xlsx") {
		rows, err = readExcelResource(file, spec)
	} else {
		rows = spec.NewSlice()
		err = json.NewDecoder(io.LimitReader(file, 20<<20)).Decode(rows)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "导入文件解析失败: "+err.Error())
		return
	}
	mode := r.URL.Query().Get("mode")
	err = a.store.DB.Transaction(func(tx *gorm.DB) error {
		if mode == "replace" {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(spec.New()).Error; err != nil {
				return err
			}
		}
		return tx.Save(rows).Error
	})
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, nil, "导入完成，游戏内立即生效")
}

func (a *AdminAPI) uploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请使用POST上传")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择图片")
		return
	}
	defer file.Close()
	if header.Size > 10<<20 {
		writeError(w, http.StatusBadRequest, "图片不能超过10MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		writeError(w, http.StatusBadRequest, "仅支持PNG/JPG/WEBP/GIF")
		return
	}
	if err := os.MkdirAll(a.uploadDir, 0o755); err != nil {
		writeDBError(w, err)
		return
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	target := filepath.Join(a.uploadDir, name)
	if err := saveMultipartFile(file, target); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]string{"url": "/uploads/" + name, "path": target}, "图片上传成功")
}

func (a *AdminAPI) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	if name == "." || name == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.uploadDir, name))
}

func (a *AdminAPI) serveAdmin(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		path = "index.html"
	}
	data, err := fs.ReadFile(a.static, path)
	if err != nil {
		data, err = fs.ReadFile(a.static, "index.html")
	}
	if err != nil {
		http.Error(w, "管理界面未嵌入", http.StatusInternalServerError)
		return
	}
	switch filepath.Ext(path) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(data)
}

func splitPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	result := parts[:0]
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	sizeText := strings.TrimSpace(r.URL.Query().Get("page_size"))
	if sizeText == "" {
		sizeText = r.URL.Query().Get("size")
	}
	size, _ := strconv.Atoi(sizeText)
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 10<<20))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("请求数据错误: %w", err)
	}
	return nil
}

func stripProtectedFields(changes map[string]any) {
	for _, field := range []string{"id", "created_at", "updated_at", "deleted_at"} {
		delete(changes, field)
	}
}

func saveMultipartFile(source multipart.File, target string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(source, 10<<20))
	return err
}

func writeOK(w http.ResponseWriter, data any, message string) {
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: message, Data: data})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiResponse{Code: status, Message: message})
}

func writeDBError(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "数据不存在")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprint(recovered))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
