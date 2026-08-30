package handler

import (
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"xianlv/internal/model"
)

var monitorState = struct {
	startedAt time.Time
	requests  atomic.Int64
	errors    atomic.Int64
	totalNS   atomic.Int64
	maximumNS atomic.Int64
}{startedAt: time.Now()}

func monitorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		elapsed := time.Since(started).Nanoseconds()
		monitorState.requests.Add(1)
		monitorState.totalNS.Add(elapsed)
		if recorder.status >= 400 {
			monitorState.errors.Add(1)
		}
		for {
			old := monitorState.maximumNS.Load()
			if elapsed <= old || monitorState.maximumNS.CompareAndSwap(old, elapsed) {
				break
			}
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (a *AdminAPI) handleMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "监控接口仅支持读取")
		return
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	requests := monitorState.requests.Load()
	totalNS := monitorState.totalNS.Load()
	averageMS := float64(0)
	if requests > 0 {
		averageMS = float64(totalNS) / float64(requests) / float64(time.Millisecond)
	}
	var online int64
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	_ = a.store.DB.Model(&model.GameLog{}).Where("created_at >= ?", fiveMinutesAgo).Distinct("player_id").Count(&online).Error
	sqlDB, _ := a.store.DB.DB()
	dbStats := map[string]any{}
	if sqlDB != nil {
		stats := sqlDB.Stats()
		dbStats = map[string]any{"open_connections": stats.OpenConnections, "in_use": stats.InUse, "idle": stats.Idle, "wait_count": stats.WaitCount}
	}
	writeOK(w, map[string]any{
		"server": map[string]any{
			"cpu_cores": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(), "memory_alloc_mb": float64(memory.Alloc) / 1024 / 1024,
			"memory_system_mb": float64(memory.Sys) / 1024 / 1024, "disk_total_gb": diskTotalGB(), "disk_free_gb": diskFreeGB(),
			"uptime_seconds": int64(time.Since(monitorState.startedAt).Seconds()), "go_version": runtime.Version(),
		},
		"online":      map[string]any{"players_5m": online, "window_minutes": 5},
		"requests":    map[string]any{"total": requests, "errors": monitorState.errors.Load()},
		"performance": map[string]any{"average_ms": averageMS, "maximum_ms": float64(monitorState.maximumNS.Load()) / float64(time.Millisecond), "database": dbStats},
	}, "读取成功")
}
