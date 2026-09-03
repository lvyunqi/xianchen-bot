package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const adminTokenFileName = "admin-token.txt"

// 管理后台单令牌鉴权：生效令牌 = 数据目录 admin-token.txt（热更新，mtime 缓存 1s）
// 高于启动令牌（--admin-token / admin.bootstrap_token），两者皆空表示未启用鉴权，
// 所有请求放行（本机默认部署的向后兼容行为）。比对使用 constant-time 避免计时侧信道。
// SPA 壳与静态资产放行（/admin 前缀），未授权由前端路由守卫呈现登录页；
// 数据接口 /api/*（除 verify）与上传文件 /uploads/* 必须携带正确令牌。

type adminAuthState struct {
	mu        sync.RWMutex
	bootstrap string
	dataDir   string

	fileMu       sync.Mutex
	fileToken    string
	fileLoadedAt time.Time
	fileSize     int64
	fileModTime  time.Time
}

var adminAuth adminAuthState

// SetAdminAuth 设置启动令牌与数据目录（热更新文件位于该目录下）。
func SetAdminAuth(bootstrapToken, dataDir string) {
	adminAuth.mu.Lock()
	defer adminAuth.mu.Unlock()
	adminAuth.bootstrap = strings.TrimSpace(bootstrapToken)
	adminAuth.dataDir = dataDir
	adminAuth.fileLoadedAt = time.Time{}
}

func (s *adminAuthState) currentToken() string {
	if token := s.hotToken(); token != "" {
		return token
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bootstrap
}

// hotToken 读取 admin-token.txt，1s 内复用缓存；文件不存在或为空返回空串。
func (s *adminAuthState) hotToken() string {
	s.mu.RLock()
	dir := s.dataDir
	s.mu.RUnlock()
	if dir == "" {
		return ""
	}
	path := dir + string(os.PathSeparator) + adminTokenFileName
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if !s.fileLoadedAt.IsZero() && s.fileModTime.Equal(info.ModTime()) && s.fileSize == info.Size() && time.Since(s.fileLoadedAt) < time.Second {
		return s.fileToken
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.fileToken = ""
		s.fileLoadedAt = time.Now()
		s.fileModTime = info.ModTime()
		s.fileSize = info.Size()
		return ""
	}
	s.fileToken = strings.TrimSpace(string(data))
	s.fileLoadedAt = time.Now()
	s.fileModTime = info.ModTime()
	s.fileSize = info.Size()
	return s.fileToken
}

// adminAuthEnabled 报告当前是否启用鉴权。
func adminAuthEnabled() bool {
	return adminAuth.currentToken() != ""
}

// verifyAdminToken constant-time 校验；未启用时恒通过。
func verifyAdminToken(candidate string) bool {
	token := adminAuth.currentToken()
	if token == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(strings.TrimSpace(candidate))) == 1
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "message": message})
}

// authMiddleware 管理后台统一入口鉴权。
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !adminAuthEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if path == "/admin" || strings.HasPrefix(path, "/admin/") || path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		if path == "/api/auth/verify" {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/uploads/") {
			if !verifyAdminToken(bearerToken(r)) {
				writeAuthError(w, http.StatusUnauthorized, "未登录或访问令牌无效")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

// handleAuthVerify 登录验证：POST 请求体 {"token":"..."}。
// 返回 enabled=false 表示服务端未启用鉴权，前端直接放行进入。
func (a *AdminAPI) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !adminAuthEnabled() {
		writeOK(w, map[string]any{"enabled": false, "ok": true}, "未启用鉴权")
		return
	}
	if !verifyAdminToken(body.Token) {
		writeAuthError(w, http.StatusUnauthorized, "访问令牌不正确")
		return
	}
	writeOK(w, map[string]any{"enabled": true, "ok": true}, "验证通过")
}
