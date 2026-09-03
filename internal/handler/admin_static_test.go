package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"xianlv/internal/config"
	"xianlv/internal/storage"
)

func staticTestMux(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "static.db")
	store, err := storage.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	assets, _ := fs.Sub(fstest.MapFS{
		"index.html":         {Data: []byte("<html>SPA</html>")},
		"assets/index.js":    {Data: []byte("console.log('module')")},
		"assets/index.css":   {Data: []byte("body{}")},
	}, ".")
	return NewAdminMux(store, assets, filepath.Join(t.TempDir(), "uploads"))
}

// 回归：/assets/* 模块脚本曾因未注册被根重定向 302 到 /admin，
// 浏览器把重定向后的 text/html 当 module 加载 → Strict MIME 白屏。
// 修复 = vite base /admin/ + /admin 规范化重定向 + 回落按 HTML 声明 MIME。
func TestServeAdminEntryAndAssetMIME(t *testing.T) {
	h := staticTestMux(t)

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if res.Code != http.StatusMovedPermanently || res.Header().Get("Location") != "/admin/" {
		t.Fatalf("GET /admin 应 301 到 /admin/，实际 %d %s", res.Code, res.Header().Get("Location"))
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET /admin/ 应 200，实际 %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("index.html Content-Type 应为 text/html，实际 %s", ct)
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin/assets/index.js", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("module 资产应 200，实际 %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("module 资产 Content-Type 应为 application/javascript，实际 %s", ct)
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin/assets/index.css", nil))
	if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("css Content-Type 应为 text/css，实际 %s", ct)
	}

	// SPA 回落：缺失路径回 index.html 且必须按 text/html 声明（不得沿用 .js 的 Content-Type）。
	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin/assets/missing.js", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("回落应 200，实际 %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("回落 index.html Content-Type 应为 text/html，实际 %s", ct)
	}
	if body := res.Body.String(); body != "<html>SPA</html>" {
		t.Fatalf("回落应返回 index.html 内容，实际 %q", body)
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusFound || res.Header().Get("Location") != "/admin/" {
		t.Fatalf("GET / 应 302 到 /admin/，实际 %d %s", res.Code, res.Header().Get("Location"))
	}
}
