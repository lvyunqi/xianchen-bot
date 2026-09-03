package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressionMiddleware(t *testing.T) {
	body := strings.Repeat("<html>仙尘管理后台</html>", 100) // ~4KB 文本
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})

	// 声明 gzip：应压缩
	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()
	compressionMiddleware(inner).ServeHTTP(res, req)
	if res.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("文本响应应启用 gzip，实际头=%v", res.Header())
	}
	if res.Body.Len() >= len(body) {
		t.Fatalf("压缩后应更小: got=%d raw=%d", res.Body.Len(), len(body))
	}
	if res.Header().Get("Vary") == "" {
		t.Fatal("应携带 Vary: Accept-Encoding")
	}

	// 不声明 gzip：应原样返回
	req2 := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	res2 := httptest.NewRecorder()
	compressionMiddleware(inner).ServeHTTP(res2, req2)
	if res2.Header().Get("Content-Encoding") != "" {
		t.Fatal("未声明 gzip 时不应压缩")
	}
	if res2.Body.Len() != len(body) {
		t.Fatalf("未压缩响应应等长: got=%d want=%d", res2.Body.Len(), len(body))
	}
}
