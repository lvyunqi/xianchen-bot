package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminAllDataPagesAreReadable(t *testing.T) {
	h := testAdminAPI(t)
	paths := []string{
		"/api/config", "/api/realms", "/api/items", "/api/events", "/api/tasks", "/api/skills", "/api/pets", "/api/dungeons",
		"/api/recipes", "/api/artifacts", "/api/locations", "/api/titles", "/api/activities", "/api/mails", "/api/checkin",
		"/api/shop", "/api/cdks", "/api/notices", "/api/reviews", "/api/sensitive-words", "/api/slow-queries", "/api/managers",
		"/api/formations", "/api/talismans", "/api/puppets-config", "/api/secret-conflicts", "/api/inheritances", "/api/dao-insights",
		"/api/battlefields", "/api/root-evolutions", "/api/inner-demons", "/api/couple-skills", "/api/immortal-herbs",
		"/api/artifact-refinements", "/api/destiny-deductions", "/api/leylines", "/api/sect-wars", "/api/immortal-encounters", "/api/star-realms",
		"/api/players", "/api/couples", "/api/menus?type=side", "/api/dashboard", "/api/monitor",
	}
	for _, path := range paths {
		requestAPI(t, h, http.MethodGet, path, nil)
	}
}

func TestAdminFriendlyItemAndGameplayCRUD(t *testing.T) {
	h := testAdminAPI(t)
	created := requestAPI(t, h, http.MethodPost, "/api/items", map[string]any{
		"name": "后台试炼回元丹", "category_name": "丹药", "rarity_name": "灵品",
		"description": "后台中文表单创建的测试丹药。", "effect_type": "治疗比例", "effect_func": "heal_hp",
		"effect_params": `{"max_health_percent":42}`, "effect_value": 42, "base_value": 180,
		"stack_limit": 99, "stackable": true, "tradable": true,
	})
	item := created["data"].(map[string]any)
	id := int(item["id"].(float64))
	if !strings.HasPrefix(item["code"].(string), "admin_items_") || item["category_id"].(float64) == 0 || item["rarity_id"].(float64) == 0 {
		t.Fatalf("friendly item defaults were not persisted: %#v", item)
	}
	updated := requestAPI(t, h, http.MethodPut, "/api/items/"+itoa(id), map[string]any{
		"description": "修改后立即生效", "effect_params": map[string]any{"max_health_percent": 55}, "effect_value": 55,
	})
	updatedItem := updated["data"].(map[string]any)
	if updatedItem["description"] != "修改后立即生效" || updatedItem["effect_params"] != `{"max_health_percent":55}` {
		t.Fatalf("friendly item update mismatch: %#v", updatedItem)
	}

	formation := requestAPI(t, h, http.MethodPost, "/api/formations", map[string]any{
		"name": "后台试炼聚灵阵", "type": "辅助", "level": 1, "description": "中文字段创建的阵法。",
		"effect_params": `{"cultivation_multiplier":1.1}`, "cost_materials": `{"阵基石":1}`,
		"prerequisite": `{"minimum_realm_sequence":1}`, "sort_order": 9999, "status": "启用",
	})
	formationData := formation["data"].(map[string]any)
	if !strings.HasPrefix(formationData["code"].(string), "admin_formations_") {
		t.Fatalf("gameplay code was not generated: %#v", formationData)
	}

	badRequest := httptest.NewRequest(http.MethodPost, "/api/items", strings.NewReader(`{"name":"坏配置丹","category_name":"丹药","rarity_name":"凡品","effect_params":"{错误"}`))
	badRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	badResponse := httptest.NewRecorder()
	h.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || !strings.Contains(badResponse.Body.String(), "中文项目编辑器") {
		t.Fatalf("invalid structured item was accepted: status=%d body=%s", badResponse.Code, badResponse.Body.String())
	}
}

func TestAdminNoticePublishLifecycleFiltersAndPagination(t *testing.T) {
	h := testAdminAPI(t)
	const titlePrefix = "公告接口筛选验收"

	createNotice := func(suffix, noticeType string, published bool) map[string]any {
		t.Helper()
		response := requestAPI(t, h, http.MethodPost, "/api/notices", map[string]any{
			"title": titlePrefix + suffix, "content": "完整公告正文" + suffix,
			"type": noticeType, "published": published,
		})
		return response["data"].(map[string]any)
	}

	first := createNotice("一", "公告", true)
	createNotice("二", "公告", false)
	createNotice("三", "更新", true)
	createNotice("四", "公告", true)
	createNotice("五", "公告", true)

	if first["published_at"] == nil {
		t.Fatalf("published notice did not receive published_at: %#v", first)
	}
	if _, err := time.Parse(time.RFC3339Nano, first["published_at"].(string)); err != nil {
		t.Fatalf("published_at is not a valid timestamp: %v", err)
	}

	filtered := requestAPI(t, h, http.MethodGet, "/api/notices?keyword="+titlePrefix+"&type=公告&published=true&page=2&page_size=2", nil)
	page := filtered["data"].(map[string]any)
	if page["total"].(float64) != 3 || page["page"].(float64) != 2 || page["size"].(float64) != 2 {
		t.Fatalf("notice filters or page_size mismatch: %#v", page)
	}
	items := page["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["type"] != "公告" || items[0].(map[string]any)["published"] != true {
		t.Fatalf("notice filter returned unexpected rows: %#v", items)
	}

	// Notice has no status column. A stale generic status parameter must not turn the request into SQL 500.
	requestAPI(t, h, http.MethodGet, "/api/notices?keyword="+titlePrefix+"&status=已发布", nil)

	id := int(first["id"].(float64))
	unpublished := requestAPI(t, h, http.MethodPut, "/api/notices/"+itoa(id), map[string]any{"published": false})
	unpublishedRow := unpublished["data"].(map[string]any)
	if unpublishedRow["published"] != false || unpublishedRow["published_at"] != nil {
		t.Fatalf("unpublishing did not clear publication time: %#v", unpublishedRow)
	}

	republished := requestAPI(t, h, http.MethodPut, "/api/notices/"+itoa(id), map[string]any{"published": true})
	republishedRow := republished["data"].(map[string]any)
	if republishedRow["published"] != true || republishedRow["published_at"] == nil {
		t.Fatalf("republishing did not restore publication time: %#v", republishedRow)
	}
}
