package handler

import (
  "compress/gzip"
  "encoding/json"
  "io"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
)

// 复刻生产中间件链，定位 gzip 头与明文体不一致的层。
func TestCompressionFullChain(t *testing.T) {
  inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "读取成功"})
  })

  chain := monitorMiddleware(recoveryMiddleware(compressionMiddleware(corsMiddleware(inner))))
  req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
  req.Header.Set("Accept-Encoding", "gzip")
  res := httptest.NewRecorder()
  chain.ServeHTTP(res, req)

  enc := res.Header().Get("Content-Encoding")
  raw := res.Body.Bytes()
  t.Logf("encoding=%q body_len=%d head=%q", enc, len(raw), string(raw[:min(30, len(raw))]))
  if enc != "gzip" {
    t.Fatal("应声明 gzip")
  }
  zr, err := gzip.NewReader(strings.NewReader(string(raw)))
  if err != nil {
    t.Fatalf("gzip 流损坏: %v", err)
  }
  plain, err := io.ReadAll(zr)
  if err != nil {
    t.Fatalf("解压失败: %v", err)
  }
  if !strings.Contains(string(plain), "读取成功") {
    t.Fatalf("解压内容不符: %q", string(plain))
  }
}
