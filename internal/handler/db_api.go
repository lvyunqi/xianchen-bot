package handler

import (
	"net/http"
)

// registerDatabaseOps 把数据体积/压缩/保留期端点接入 /api 分发表。
// GET  /api/db-stats     体积概览（dbstat top 表 + 文件大小）
// POST /api/db-vacuum    手动整库压缩（SQLite）
// POST /api/db-retention 手动执行一轮保留期清理
func (a *AdminAPI) registerDatabaseOps() {
	a.customRoutes["db-stats"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "仅支持 GET")
			return
		}
		stats, err := a.store.DatabaseStats()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取数据体积失败："+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
	a.customRoutes["db-vacuum"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
			return
		}
		if err := a.store.VacuumDatabase(); err != nil {
			writeError(w, http.StatusInternalServerError, "压缩失败："+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
	a.customRoutes["db-retention"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
			return
		}
		stats, err := a.store.RunRetention(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "清理失败："+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": stats})
	}
}
