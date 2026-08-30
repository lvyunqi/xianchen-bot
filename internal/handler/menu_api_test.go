package handler

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"xianlv/internal/config"
	"xianlv/internal/storage"
)

func testAdminAPI(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "admin.db")
	store, err := storage.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	assets, _ := fs.Sub(fstest.MapFS{"index.html": {Data: []byte("ok")}}, ".")
	return NewAdminMux(store, assets, filepath.Join(t.TempDir(), "uploads"))
}

func TestMenuCRUDPersistsUnicodeData(t *testing.T) {
	h := testAdminAPI(t)
	payload := map[string]any{"parent_id": 0, "menu_type": "side", "label": "保存测试菜单", "icon": "测", "path": "/save-test", "component": "items", "permission": "admin", "sort_order": 999, "status": "active", "target": "_self"}
	created := requestAPI(t, h, http.MethodPost, "/api/menus", payload)
	data := created["data"].(map[string]any)
	id := int(data["id"].(float64))

	updated := requestAPI(t, h, http.MethodPut, "/api/menus/"+itoa(id), map[string]any{"label": "中文已写入数据库"})
	updatedData := updated["data"].(map[string]any)
	if updatedData["label"] != "中文已写入数据库" {
		t.Fatalf("unicode menu was not persisted: %#v", updatedData)
	}
	requestAPI(t, h, http.MethodPut, "/api/menus/"+itoa(id)+"/hide", map[string]any{"is_hidden": true})
	requestAPI(t, h, http.MethodDelete, "/api/menus/"+itoa(id), nil)
}

func requestAPI(t *testing.T, h http.Handler, method, path string, payload any) map[string]any {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code >= 300 {
		t.Fatalf("%s %s: status=%d body=%s", method, path, res.Code, res.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["code"].(float64) != 0 {
		t.Fatalf("%s %s failed: %#v", method, path, decoded)
	}
	return decoded
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = digits[value%10]
		value /= 10
	}
	return string(buffer[index:])
}
