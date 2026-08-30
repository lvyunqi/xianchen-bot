package handler

import (
	"net/http"
	"time"

	"xianlv/internal/model"
)

func (a *AdminAPI) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仪表盘仅支持读取")
		return
	}
	today := time.Now().Format("2006-01-02")
	counts := map[string]int64{}
	queries := map[string]any{
		"players": &model.Player{}, "couples": &model.Couple{}, "items": &model.Item{},
		"pending_reviews": &model.ContentReview{},
	}
	for key, target := range queries {
		query := a.store.DB.Model(target)
		if key == "pending_reviews" {
			query = query.Where("status = ?", "待审核")
		}
		var count int64
		_ = query.Count(&count).Error
		counts[key] = count
	}
	var activeToday int64
	_ = a.store.DB.Model(&model.GameLog{}).Where("created_at >= ?", today+" 00:00:00").Distinct("player_id").Count(&activeToday).Error
	counts["active_today"] = activeToday

	type trendRow struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	trend := make([]trendRow, 0, 7)
	for offset := 6; offset >= 0; offset-- {
		day := time.Now().AddDate(0, 0, -offset)
		next := day.AddDate(0, 0, 1)
		var count int64
		_ = a.store.DB.Model(&model.Player{}).Where("created_at >= ? AND created_at < ?", day.Format("2006-01-02 00:00:00"), next.Format("2006-01-02 00:00:00")).Count(&count).Error
		trend = append(trend, trendRow{Day: day.Format("01-02"), Count: count})
	}
	var logs []model.OperationLog
	_ = a.store.DB.Order("id DESC").Limit(8).Find(&logs).Error
	writeOK(w, map[string]any{"metrics": counts, "trend": trend, "recent": logs}, "读取成功")
}
