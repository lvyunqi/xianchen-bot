package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"xianlv/internal/model"
	"xianlv/internal/service"
)

// 从 plugin_main.go 原样保留的平台无关消息管线原语：
// 全角空格归一、消息去重、内置工具指令、已知群持久化。
// P1 的 processInbound 将基于这些原语重建完整处理链。

var messageDedup = struct {
	sync.Mutex
	seen map[string]time.Time
}{seen: make(map[string]time.Time)}

var knownGroupLock sync.Mutex

func normalizeMessage(message string) string {
	return strings.TrimSpace(strings.ReplaceAll(message, "　", " "))
}

func acceptMessageOnce(scope, target, userID, messageID, message string) bool {
	now := time.Now()
	key := strings.Join([]string{scope, target, userID, strings.TrimSpace(messageID)}, "|")
	ttl := 2 * time.Minute
	if strings.TrimSpace(messageID) == "" {
		key = strings.Join([]string{scope, target, userID, message}, "|")
		ttl = 3 * time.Second
	}
	messageDedup.Lock()
	defer messageDedup.Unlock()
	for existing, expiresAt := range messageDedup.seen {
		if !expiresAt.After(now) {
			delete(messageDedup.seen, existing)
		}
	}
	if expiresAt, exists := messageDedup.seen[key]; exists && expiresAt.After(now) {
		return false
	}
	messageDedup.seen[key] = now.Add(ttl)
	return true
}

func utilityResult(groupID, userID, message string) *service.GameResult {
	switch message {
	case "获取ID", "我的ID", "查询ID":
		result := service.GameResult{Title: "身份玉牌", Content: "你的QQ开放平台用户ID：`" + userID + "`"}
		return &result
	case "获取群ID", "本群ID", "查询群ID":
		if groupID == "" {
			return nil
		}
		result := service.GameResult{Title: "宗门坐标", Content: "当前QQ群ID：`" + groupID + "`"}
		return &result
	default:
		return nil
	}
}

// rememberKnownGroup 将活跃群写入 system_settings（runtime.known_groups），
// 供全区通报广播使用；Rust 插件通过协议读取同一份数据。
func rememberKnownGroup(groupID string) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return
	}
	knownGroupLock.Lock()
	defer knownGroupLock.Unlock()
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return
	}
	var row model.SystemSetting
	groups := make([]string, 0, 8)
	if store.DB.Where("key = ?", "runtime.known_groups").First(&row).Error == nil {
		_ = json.Unmarshal([]byte(row.Value), &groups)
	}
	for _, existing := range groups {
		if existing == groupID {
			return
		}
	}
	groups = append(groups, groupID)
	data, _ := json.Marshal(groups)
	row = model.SystemSetting{Key: "runtime.known_groups", Value: string(data), ValueType: "json", Description: "自动全区通报的已知QQ群，请勿手动修改"}
	_ = store.DB.Where("key = ?", row.Key).Assign(map[string]any{"value": row.Value, "value_type": row.ValueType, "description": row.Description}).FirstOrCreate(&row).Error
}

// knownGroups 返回持久化的已知群列表（快照）。
func knownGroups() []string {
	knownGroupLock.Lock()
	defer knownGroupLock.Unlock()
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return nil
	}
	var value string
	if store.DB.Table("system_settings").Select("value").Where("key = ?", "runtime.known_groups").Scan(&value).Error != nil {
		return nil
	}
	var groups []string
	if json.Unmarshal([]byte(value), &groups) != nil {
		return nil
	}
	return groups
}
