package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"io/fs"

	"xianlv/internal/config"
	"xianlv/internal/storage"
)

func authTestMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "auth.db")
	store, err := storage.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	dataDir := t.TempDir()
	assets, _ := fs.Sub(fstest.MapFS{"index.html": {Data: []byte("<html>SPA</html>")}}, ".")
	t.Cleanup(func() { SetAdminAuth("", "") })
	return NewAdminMux(store, assets, filepath.Join(t.TempDir(), "uploads")), dataDir
}

func doJSON(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(`{"token":"`+token+`"}`))
	if token != "" && path != "/api/auth/verify" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func TestAdminAuthDisabledByDefault(t *testing.T) {
	h, _ := authTestMux(t)
	SetAdminAuth("", "")
	if res := doJSON(h, http.MethodGet, "/api/dashboard", ""); res.Code != http.StatusOK {
		t.Fatalf("未启用鉴权应放行，实际 %d", res.Code)
	}
}

func TestAdminAuthBearerAndVerify(t *testing.T) {
	h, _ := authTestMux(t)
	SetAdminAuth("s3cret-token", "")

	if res := doJSON(h, http.MethodGet, "/api/dashboard", ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401，实际 %d", res.Code)
	}
	if res := doJSON(h, http.MethodGet, "/api/dashboard", "wrong"); res.Code != http.StatusUnauthorized {
		t.Fatalf("错误令牌应 401，实际 %d", res.Code)
	}
	if res := doJSON(h, http.MethodGet, "/api/dashboard", "s3cret-token"); res.Code != http.StatusOK {
		t.Fatalf("正确令牌应 200，实际 %d", res.Code)
	}
	// 上传目录同样受保护
	if res := doJSON(h, http.MethodGet, "/uploads/x.png", ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("uploads 无令牌应 401，实际 %d", res.Code)
	}
	// SPA 壳放行
	if res := doJSON(h, http.MethodGet, "/admin/", ""); res.Code != http.StatusOK {
		t.Fatalf("SPA 壳应放行，实际 %d", res.Code)
	}

	// verify：错误令牌 401，正确令牌 200
	if res := doJSON(h, http.MethodPost, "/api/auth/verify", "nope"); res.Code != http.StatusUnauthorized {
		t.Fatalf("verify 错误令牌应 401，实际 %d", res.Code)
	}
	res := doJSON(h, http.MethodPost, "/api/auth/verify", "s3cret-token")
	if res.Code != http.StatusOK {
		t.Fatalf("verify 正确令牌应 200，实际 %d", res.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	if body["enabled"] != true {
		data, _ := body["data"].(map[string]any)
		if data == nil || data["enabled"] != true {
			t.Fatalf("verify 应报告 enabled=true，实际 %#v", body)
		}
	}
}

func TestAdminAuthHotReloadFile(t *testing.T) {
	h, dataDir := authTestMux(t)
	SetAdminAuth("boot-token", dataDir)

	if res := doJSON(h, http.MethodGet, "/api/dashboard", "boot-token"); res.Code != http.StatusOK {
		t.Fatalf("启动令牌应通过，实际 %d", res.Code)
	}

	// 热更新：写入 admin-token.txt 覆盖启动令牌
	tokenFile := filepath.Join(dataDir, "admin-token.txt")
	if err := os.WriteFile(tokenFile, []byte("hot-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // 越过 1s 热更新缓存

	if res := doJSON(h, http.MethodGet, "/api/dashboard", "boot-token"); res.Code != http.StatusUnauthorized {
		t.Fatalf("旧启动令牌应失效，实际 %d", res.Code)
	}
	if res := doJSON(h, http.MethodGet, "/api/dashboard", "hot-token"); res.Code != http.StatusOK {
		t.Fatalf("热更新令牌应通过，实际 %d", res.Code)
	}

	// 删除文件回落启动令牌
	if err := os.Remove(tokenFile); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if res := doJSON(h, http.MethodGet, "/api/dashboard", "boot-token"); res.Code != http.StatusOK {
		t.Fatalf("删除热更新文件后应回落启动令牌，实际 %d", res.Code)
	}
}
