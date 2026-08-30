package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xianlv/internal/appinfo"
	"xianlv/internal/config"
	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/service"
	"xianlv/internal/storage"
)

const (
	PluginName        = appinfo.PluginName
	PluginAuthor      = "随缘"
	PluginVersion     = appinfo.Version
	PluginDescription = "一款面向QQ开放平台官机的修仙仙侣文字游戏，支持共修、探索、战斗、宗门与全量游戏数据实时自定义。"
)

type pluginState struct {
	sync.RWMutex
	dataDir string
	store   *storage.Store
	game    *service.Game
}

var runtimeState pluginState

var messageDedup = struct {
	sync.Mutex
	seen map[string]time.Time
}{seen: make(map[string]time.Time)}

var knownGroupLock sync.Mutex
var releaseNoticeRetryAfter = make(map[string]time.Time)

func beeFromArgs(args [][]byte) (*BeeAPI, error) {
	if len(args) == 0 {
		return nil, errors.New("机器人上下文参数不足")
	}
	return NewBeeAPI(string(args[0]))
}

func onInitialize(args [][]byte) {
	bee, err := beeFromArgs(args)
	if err != nil {
		return
	}
	if err := ensureRuntime(bee); err != nil {
		_ = bee.Log("仙尘初始化失败: " + err.Error())
		return
	}
	_ = bee.Log("仙尘初始化完成")
}

func onEnable(args [][]byte) {
	if bee, err := beeFromArgs(args); err == nil {
		if err := ensureRuntime(bee); err != nil {
			_ = bee.Log("仙尘启用失败: " + err.Error())
			return
		}
		_ = bee.Log("仙尘已启用")
	}
}

func onDisable(args [][]byte) {
	if bee, err := beeFromArgs(args); err == nil {
		_ = bee.Log("仙尘已禁用")
	}
	closeSettingsWindow()
}

func onUnload(args [][]byte) {
	closeSettingsWindow()
	stopLicenseServer()
	stopAdminServer()
	runtimeState.Lock()
	if runtimeState.store != nil {
		_ = runtimeState.store.Close()
	}
	runtimeState.store = nil
	runtimeState.game = nil
	runtimeState.Unlock()
}

func onSettings(args [][]byte) {
	bee, err := beeFromArgs(args)
	if err != nil {
		return
	}
	dataDir, err := runtimeDataDir(bee)
	if err != nil {
		_ = bee.Log("获取插件数据目录失败: " + err.Error())
		return
	}
	if err := validateRuntimeLicense(dataDir); err != nil {
		activationURL, serverErr := startLicenseServer(dataDir)
		if serverErr != nil {
			_ = bee.Log("打开授权设置失败: " + serverErr.Error())
			return
		}
		setSettingsURL(activationURL)
		showSettingsWindow()
		return
	}
	if err := ensureRuntimeDataDir(dataDir); err != nil {
		_ = bee.Log("打开数据后台失败: " + err.Error())
		return
	}
	showSettingsWindow()
}

func ensureRuntime(bee *BeeAPI) error {
	if bee == nil {
		return errors.New("Bee API不可用")
	}
	dataDir, err := runtimeDataDir(bee)
	if err != nil {
		return err
	}
	return ensureRuntimeDataDir(dataDir)
}

func runtimeDataDir(bee *BeeAPI) (string, error) {
	dataDir, err := bee.GetAppDataDir()
	if err != nil {
		return "", fmt.Errorf("获取插件数据目录: %w", err)
	}
	dataDir, err = filepath.Abs(strings.TrimSpace(dataDir))
	if err != nil {
		return "", err
	}
	// Bee may derive a new application-data folder from the renamed plugin.
	// Reuse the legacy database when it is the only installed copy.
	newDatabase := filepath.Join(dataDir, "data", "xianlv.db")
	legacyDir := filepath.Join(filepath.Dir(dataDir), "仙侣奇缘")
	legacyDatabase := filepath.Join(legacyDir, "data", "xianlv.db")
	if _, newErr := os.Stat(newDatabase); os.IsNotExist(newErr) {
		if _, legacyErr := os.Stat(legacyDatabase); legacyErr == nil {
			return legacyDir, nil
		}
	}
	return dataDir, nil
}

func ensureRuntimeDataDir(dataDir string) error {
	if err := validateRuntimeLicense(dataDir); err != nil {
		return err
	}

	runtimeState.RLock()
	ready := runtimeState.game != nil && strings.EqualFold(runtimeState.dataDir, dataDir) && currentAdminURL() != ""
	runtimeState.RUnlock()
	if ready {
		return nil
	}

	runtimeState.Lock()
	defer runtimeState.Unlock()
	if runtimeState.game != nil && strings.EqualFold(runtimeState.dataDir, dataDir) {
		if currentAdminURL() != "" {
			return nil
		}
		adminURL, err := startAdminServer(runtimeState.store, dataDir)
		if err != nil {
			return err
		}
		setSettingsURL(adminURL)
		return nil
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "data"), 0o755); err != nil {
		return err
	}
	cfg := config.Runtime(dataDir)
	store, err := storage.Open(cfg)
	if err != nil {
		return err
	}
	if runtimeState.store != nil {
		_ = runtimeState.store.Close()
	}
	runtimeState.dataDir = dataDir
	runtimeState.store = store
	game, err := service.NewGame(store)
	if err != nil {
		_ = store.Close()
		return err
	}
	runtimeState.game = game
	adminURL, err := startAdminServer(store, dataDir)
	if err != nil {
		_ = store.Close()
		return err
	}
	setSettingsURL(adminURL)
	return nil
}

func onGroupMessage(robotJSON, groupID, userID, message, messageID string) int {
	bee, err := NewBeeAPI(robotJSON)
	if err != nil || bee == nil {
		writeRuntimeLog("群消息入口失败", fmt.Sprintf("group=%s user=%s error=%v", groupID, userID, err))
		return MessageContinue
	}
	bee.ctx.MessageID = messageID
	if err := ensureRuntime(bee); err != nil {
		writeRuntimeLog("群消息入口失败", fmt.Sprintf("group=%s user=%s error=%v", groupID, userID, err))
		_, _ = bee.ctx.SendGroupMessage(groupID, "【仙尘】\n数据正在加载中.....\n若长时间没有完成，请主人打开插件设置查看授权与初始化日志。", "", false, false)
		return MessageIntercept
	}
	rememberKnownGroup(groupID)
	// QQ only permits reliable passive replies for the group that produced the
	// current event. Deliver the release notice once to this group after the
	// command finishes; never fan out to stale groups on the hot message path.
	defer sendReleaseNoticeToCurrentGroup(bee, groupID)
	message = normalizeMessage(message)
	writeRuntimeLog("群消息已读取", fmt.Sprintf("group=%s user=%s message_id=%s content=%s", groupID, userID, messageID, message))
	if message == "" {
		return MessageContinue
	}
	if !acceptMessageOnce("group", groupID, userID, messageID, message) {
		return MessageIntercept
	}
	if strings.Contains(message, "上传图片") {
		result, handled := handleOwnerImageUpload(bee, userID, message)
		if !handled {
			return MessageContinue
		}
		if result != nil {
			_ = sendGroupResponse(bee, groupID, *result, false)
		}
		return MessageIntercept
	}
	if result := utilityResult(groupID, userID, message); result != nil {
		_ = sendGroupResponse(bee, groupID, *result, false)
		return MessageIntercept
	}

	runtimeState.RLock()
	game := runtimeState.game
	runtimeState.RUnlock()
	if game == nil {
		return MessageContinue
	}
	if birthdayResult, birthdayHandled, birthdayErr := game.BirthdayAmbientGreeting(groupID, userID); birthdayErr != nil {
		_ = bee.Log("仙尘生辰祝福检查失败: " + birthdayErr.Error())
	} else if birthdayHandled {
		if sendErr := sendGroupResponse(bee, groupID, birthdayResult, true); sendErr != nil {
			_ = bee.Log("仙尘生辰祝福发送失败: " + sendErr.Error())
		}
	}
	if gmCommand, ok := service.ParseGMCommand(message); ok {
		result, handled, executeErr := game.ExecuteGM(userID, gmCommand)
		if executeErr != nil {
			_ = bee.Log("仙尘神令执行失败: " + executeErr.Error())
			_ = sendGroupResponse(bee, groupID, service.GameResult{Title: "神令未成", Content: executeErr.Error()}, false)
			return MessageIntercept
		}
		if handled {
			if sendErr := sendGroupResponse(bee, groupID, result, false); sendErr != nil {
				_ = bee.Log("仙尘神令消息发送失败: " + sendErr.Error())
			}
			if strings.TrimSpace(result.BroadcastContent) != "" {
				broadcastToKnownGroups(bee, result.BroadcastContent)
			}
		}
		return MessageIntercept
	}
	parsed, ok := handler.ParseCommand(message)
	if !ok {
		parsed, ok = game.ResolveShortcut(userID, message)
	}
	if !ok {
		return MessageContinue
	}
	if parsed.Spec.ID == 3 {
		refreshPlayerAvatar(bee, game, userID)
	}
	result, handled, err := game.Execute(groupID, userID, parsed)
	if err != nil {
		_ = bee.Log("仙尘命令失败: " + err.Error())
		if parsed.Spec.ID == 3 {
			return MessageIntercept
		}
		_ = sendGroupResponse(bee, groupID, service.GameResult{Title: "天机紊乱", Content: "本次操作未能完成，请稍后重试。"}, false)
		return MessageIntercept
	}
	if !handled {
		return MessageContinue
	}
	if err := sendGroupResponse(bee, groupID, result, false); err != nil {
		_ = bee.Log("仙尘群消息发送失败: " + err.Error())
	}
	if strings.TrimSpace(result.BroadcastContent) != "" {
		broadcastToKnownGroups(bee, result.BroadcastContent)
	}
	return MessageIntercept
}

func onPrivateMessage(robotJSON, friendID, message, messageID string) int {
	bee, err := NewBeeAPI(robotJSON)
	if err != nil || bee == nil {
		writeRuntimeLog("私信入口失败", fmt.Sprintf("friend=%s error=%v", friendID, err))
		return MessageContinue
	}
	bee.ctx.MessageID = messageID
	if err := ensureRuntime(bee); err != nil {
		writeRuntimeLog("私信入口失败", fmt.Sprintf("friend=%s error=%v", friendID, err))
		_, _ = bee.ctx.SendFriendMessage(friendID, "【仙尘】\n数据正在加载中.....\n若长时间没有完成，请主人打开插件设置查看授权与初始化日志。", "", false, false, false)
		return MessageIntercept
	}
	message = normalizeMessage(message)
	writeRuntimeLog("私信已读取", fmt.Sprintf("friend=%s message_id=%s content=%s", friendID, messageID, message))
	if message == "" {
		return MessageContinue
	}
	if !acceptMessageOnce("private", friendID, friendID, messageID, message) {
		return MessageIntercept
	}
	if result := utilityResult("", friendID, message); result != nil {
		_ = sendFriendResponse(bee, friendID, *result)
		return MessageIntercept
	}
	runtimeState.RLock()
	game := runtimeState.game
	runtimeState.RUnlock()
	if game == nil {
		return MessageContinue
	}
	if gmCommand, ok := service.ParseGMCommand(message); ok {
		result, handled, executeErr := game.ExecuteGM(friendID, gmCommand)
		if executeErr != nil {
			_ = bee.Log("仙尘私聊神令执行失败: " + executeErr.Error())
			_ = sendFriendResponse(bee, friendID, service.GameResult{Title: "神令未成", Content: executeErr.Error()})
			return MessageIntercept
		}
		if handled {
			_ = sendFriendResponse(bee, friendID, result)
			if strings.TrimSpace(result.BroadcastContent) != "" {
				broadcastToKnownGroups(bee, result.BroadcastContent)
			}
		}
		return MessageIntercept
	}
	parsed, ok := handler.ParseCommand(message)
	if !ok {
		parsed, ok = game.ResolveShortcut(friendID, message)
	}
	if !ok {
		return MessageContinue
	}
	if parsed.Spec.ID == 3 {
		refreshPlayerAvatar(bee, game, friendID)
	}
	result, handled, err := game.Execute("私信", friendID, parsed)
	if err != nil {
		_ = bee.Log("仙尘私聊命令失败: " + err.Error())
		if parsed.Spec.ID == 3 {
			return MessageIntercept
		}
		_ = sendFriendResponse(bee, friendID, service.GameResult{Title: "天机紊乱", Content: "本次操作未能完成，请稍后重试。"})
		return MessageIntercept
	}
	if !handled {
		return MessageContinue
	}
	if err := sendFriendResponse(bee, friendID, result); err != nil {
		_ = bee.Log("仙尘私聊发送失败: " + err.Error())
	}
	if strings.TrimSpace(result.BroadcastContent) != "" {
		broadcastToKnownGroups(bee, result.BroadcastContent)
	}
	return MessageIntercept
}

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

func broadcastToKnownGroups(bee *BeeAPI, content string) {
	content = strings.TrimSpace(content)
	if bee == nil || content == "" {
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
	var value string
	if store.DB.Table("system_settings").Select("value").Where("key = ?", "runtime.known_groups").Scan(&value).Error != nil {
		return
	}
	var groups []string
	if json.Unmarshal([]byte(value), &groups) != nil {
		return
	}
	message := service.GameResult{Title: "全区通报", Content: content, Actions: []string{"全区通报", "公告"}}
	for _, groupID := range groups {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			continue
		}
		if err := sendGroupResponse(bee, groupID, message, true); err != nil {
			_ = bee.Log("仙尘全区通报发送失败: group=" + groupID + " error=" + err.Error())
		}
	}
}

func releaseNoticeResult() service.GameResult {
	return service.GameResult{
		Title: "仙尘 v" + appinfo.Version + " 已上线",
		Content: strings.Join([]string{
			"本次修复与补偿已经开放，可直接发送下方指令查看。",
			"━━━━━━━━━━━",
			"一、角色等级基础成长统一为每级气血+24、双攻各+7、双防各+5；旧道籍登录后自动回正，只增不减。",
			"二、修复渡劫覆盖永久属性、装备穿脱与榜位称号重复扣属性的问题，任何扣除都不能低于等级基础。",
			"三、装备按器谱真实十槽位归位；同槽替换、套装、锻造和卸下均使用事务结算，未成功或满重时不扣玄铁。",
			"四、移除失效群公告的同步重复推送，设置页与群内指令不再被旧群发送失败拖住。",
			"五、世界公告、更新公告、修复公告继续独立翻页；生辰档案与氪金菜单入口保持开放。",
			"六、符合范围的旧道籍可发送“全服补偿”查看全新万象归元礼，并发送“领取全服补偿”一次性领取丰厚物资。",
			"━━━━━━━━━━━",
			"发送“修复公告”查看完整说明；发送“全服补偿”查看领取资格与全部清单。",
		}, "\n"),
		Actions: []string{"全服补偿", "领取全服补偿", "世界公告", "更新公告", "修复公告", "生日", "氪金菜单", "功能菜单"},
	}
}

func sendReleaseNoticeToCurrentGroup(bee *BeeAPI, groupID string) {
	groupID = strings.TrimSpace(groupID)
	if bee == nil || groupID == "" {
		return
	}
	knownGroupLock.Lock()
	defer knownGroupLock.Unlock()
	if retryAt := releaseNoticeRetryAfter[groupID]; retryAt.After(time.Now()) {
		return
	}
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return
	}
	markerKey := "runtime.release_notice." + appinfo.Version + ".sent_groups"
	var sentJSON string
	_ = store.DB.Table("system_settings").Select("value").Where("key = ?", markerKey).Scan(&sentJSON).Error
	var sentGroups []string
	_ = json.Unmarshal([]byte(sentJSON), &sentGroups)
	sent := make(map[string]bool, len(sentGroups))
	for _, groupID := range sentGroups {
		sent[strings.TrimSpace(groupID)] = true
	}
	if sent[groupID] {
		delete(releaseNoticeRetryAfter, groupID)
		return
	}
	result := releaseNoticeResult()
	if err := sendGroupResponse(bee, groupID, result, false); err != nil {
		// A bad/stale group must not delay every later message. Retry at most once
		// per hour and only after that group produces another inbound event.
		releaseNoticeRetryAfter[groupID] = time.Now().Add(time.Hour)
		_ = bee.Log("仙尘版本告示发送失败: group=" + groupID + " error=" + err.Error())
		return
	}
	sentGroups = append(sentGroups, groupID)
	encoded, _ := json.Marshal(sentGroups)
	row := model.SystemSetting{Key: markerKey, Value: string(encoded), ValueType: "json", Description: "本版本告示已送达的群列表"}
	if err := store.DB.Where("key = ?", markerKey).Assign(map[string]any{"value": row.Value, "value_type": row.ValueType, "description": row.Description}).FirstOrCreate(&row).Error; err != nil {
		releaseNoticeRetryAfter[groupID] = time.Now().Add(time.Hour)
		_ = bee.Log("仙尘版本告示记录失败: group=" + groupID + " error=" + err.Error())
		return
	}
	delete(releaseNoticeRetryAfter, groupID)
}

func onChannelPrivate(robotJSON, channelID, subChannelID, userID, message, messageID string) int {
	return MessageContinue
}

func onChannelMessage(robotJSON, channelID, subChannelID, userID, message, messageID string) int {
	return MessageContinue
}

func onChannelEvent(robotJSON, channelID, subChannelID, userID, operatorID, eventType, rawMessage string) int {
	return MessageContinue
}

func onCommonEvent(robotJSON, sourceID, userID, operatorID, eventType, rawMessage string) int {
	return MessageContinue
}

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

func handleOwnerImageUpload(bee *BeeAPI, userID, message string) (*service.GameResult, bool) {
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return nil, false
	}
	var playerCount int64
	if store.DB.Table("players").Where("account_id = ? AND banned = ?", userID, false).Count(&playerCount).Error != nil || playerCount == 0 {
		return nil, false
	}
	var ownerID string
	_ = store.DB.Table("system_settings").Select("value").Where("key = ?", "owner.user_id").Scan(&ownerID).Error
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		result := service.GameResult{Title: "主人尚未设置", Content: "请先在后台系统参数中填写 `owner.user_id`，其值为主人QQ开放平台用户ID。"}
		return &result, true
	}
	if ownerID != strings.TrimSpace(userID) {
		result := service.GameResult{Title: "权限不足", Content: "上传图片仅限主人操作。"}
		return &result, true
	}
	source := strings.TrimSpace(ImageDownloadURL(message))
	if source == "" {
		result := service.GameResult{Title: "上传图片", Content: "请发送 `上传图片 菜单`、`上传图片 状态`、`上传图片 战斗` 或 `上传图片 Logo`，并在同一条消息中附带图片。"}
		return &result, true
	}
	target := "菜单"
	after := strings.TrimSpace(strings.SplitN(message, "上传图片", 2)[1])
	if fields := strings.Fields(after); len(fields) > 0 && !strings.HasPrefix(fields[0], "[") {
		target = fields[0]
	}
	settingKey := map[string]string{"菜单": "menu.cover_url", "状态": "image.status_url", "战斗": "image.battle_url", "Logo": "image.logo_url", "logo": "image.logo_url"}[target]
	if settingKey == "" {
		result := service.GameResult{Title: "上传目标不正确", Content: "可用目标：菜单、状态、战斗、Logo。"}
		return &result, true
	}
	uploadedURL, err := bee.ctx.UploadImage(source)
	if err != nil || strings.TrimSpace(uploadedURL) == "" {
		if err != nil {
			_ = bee.Log("仙尘图片上传失败: " + err.Error())
		}
		result := service.GameResult{Title: "图片上传失败", Content: "图片未能上传到图床，请稍后重试。"}
		return &result, true
	}
	row := model.SystemSetting{Key: settingKey, Value: strings.TrimSpace(uploadedURL), ValueType: "string", Description: target + "图片URL"}
	if err := store.DB.Where("key = ?", settingKey).Assign(map[string]any{"value": row.Value, "value_type": row.ValueType, "description": row.Description}).FirstOrCreate(&row).Error; err != nil {
		result := service.GameResult{Title: "图片保存失败", Content: "图片已上传，但配置写入失败，请在后台手动填写：`" + row.Value + "`"}
		return &result, true
	}
	result := service.GameResult{Title: "图片上传成功", Content: fmt.Sprintf("用途：%s\n配置键：`%s`\n图床URL：%s", target, settingKey, row.Value)}
	return &result, true
}

func sendGroupResponse(bee *BeeAPI, groupID string, result service.GameResult, active bool) error {
	if result.ImageOnly {
		return sendGroupImageOnly(bee, groupID, result, active)
	}
	if nativeMarkdownEnabled() {
		frameworkResult, err := bee.ctx.SendGroupMarkdown(groupID, MarkdownMessage{Native: result.Markdown()}, active)
		if err == nil {
			err = messageResultError(frameworkResult)
		}
		if err == nil {
			writeRuntimeLog("群原生Markdown成功", fmt.Sprintf("group=%s title=%s result=%s", groupID, result.Title, frameworkResult))
			return nil
		}
		writeRuntimeLog("群原生Markdown失败", fmt.Sprintf("group=%s title=%s error=%v result=%s", groupID, result.Title, err, frameworkResult))
		if !nativeFallbackEnabled() {
			return err
		}
	}
	frameworkResult, err := bee.ctx.SendGroupMessage(groupID, result.Text(), "", false, active)
	if err == nil {
		err = messageResultError(frameworkResult)
	}
	if err != nil {
		writeRuntimeLog("群普通消息失败", fmt.Sprintf("group=%s title=%s error=%v result=%s", groupID, result.Title, err, frameworkResult))
	} else {
		writeRuntimeLog("群普通消息成功", fmt.Sprintf("group=%s title=%s result=%s", groupID, result.Title, frameworkResult))
	}
	return err
}

func sendFriendResponse(bee *BeeAPI, friendID string, result service.GameResult) error {
	if result.ImageOnly {
		return sendFriendImageOnly(bee, friendID, result)
	}
	if nativeMarkdownEnabled() {
		frameworkResult, err := bee.ctx.SendFriendMarkdown(friendID, MarkdownMessage{Native: result.Markdown()}, false, false)
		if err == nil {
			err = messageResultError(frameworkResult)
		}
		if err == nil {
			writeRuntimeLog("私信原生Markdown成功", fmt.Sprintf("friend=%s title=%s result=%s", friendID, result.Title, frameworkResult))
			return nil
		}
		writeRuntimeLog("私信原生Markdown失败", fmt.Sprintf("friend=%s title=%s error=%v result=%s", friendID, result.Title, err, frameworkResult))
		if !nativeFallbackEnabled() {
			return err
		}
	}
	frameworkResult, err := bee.ctx.SendFriendMessage(friendID, result.Text(), "", false, false, false)
	if err != nil {
		writeRuntimeLog("私信普通消息失败", fmt.Sprintf("friend=%s title=%s error=%v", friendID, result.Title, err))
		return err
	}
	err = messageResultError(frameworkResult)
	if err != nil {
		writeRuntimeLog("私信普通消息失败", fmt.Sprintf("friend=%s title=%s error=%v result=%s", friendID, result.Title, err, frameworkResult))
	} else {
		writeRuntimeLog("私信普通消息成功", fmt.Sprintf("friend=%s title=%s result=%s", friendID, result.Title, frameworkResult))
	}
	return err
}

func refreshPlayerAvatar(bee *BeeAPI, game *service.Game, accountID string) {
	if bee == nil || bee.ctx == nil || game == nil || strings.TrimSpace(accountID) == "" {
		return
	}
	var value, avatarURL string
	var err error
	for _, size := range []int{640, 100, 40} {
		value, err = bee.ctx.GetAvatar(accountID, size)
		avatarURL = frameworkAvatarURL(value)
		if err == nil && avatarURL != "" {
			break
		}
	}
	if avatarURL == "" {
		writeRuntimeLog("玩家头像读取失败", fmt.Sprintf("user=%s error=%v result=%s", accountID, err, value))
		return
	}
	if err := game.CachePlayerAvatar(accountID, avatarURL); err != nil {
		writeRuntimeLog("玩家头像缓存失败", fmt.Sprintf("user=%s error=%v", accountID, err))
	}
}

func frameworkAvatarURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if extracted := ImageDownloadURL(value); extracted != "" {
		return extracted
	}
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		if extracted := nestedAvatarURL(decoded); extracted != "" {
			return extracted
		}
	}
	value = strings.Trim(value, "\"'")
	if strings.HasPrefix(strings.ToLower(value), "https://") || strings.HasPrefix(strings.ToLower(value), "http://") {
		return value
	}
	return ""
}

func nestedAvatarURL(value any) string {
	switch current := value.(type) {
	case string:
		return frameworkAvatarURL(current)
	case map[string]any:
		for _, key := range []string{"avatar", "avatar_url", "url", "image_url", "data"} {
			if child, exists := current[key]; exists {
				if result := nestedAvatarURL(child); result != "" {
					return result
				}
			}
		}
	case []any:
		for _, child := range current {
			if result := nestedAvatarURL(child); result != "" {
				return result
			}
		}
	}
	return ""
}

func sendGroupImageOnly(bee *BeeAPI, groupID string, result service.GameResult, active bool) error {
	imagePath := strings.TrimSpace(result.ImageURL)
	if imagePath == "" {
		return errors.New("状态图路径为空")
	}
	if info, err := os.Stat(imagePath); err == nil && !info.IsDir() {
		defer os.Remove(imagePath)
	}
	frameworkResult, err := bee.ctx.SendGroupMessage(groupID, "", imagePath, false, active)
	if err == nil {
		err = messageResultError(frameworkResult)
	}
	if err != nil {
		uploaded, uploadErr := bee.ctx.UploadImage(imagePath)
		if uploadErr == nil && strings.TrimSpace(uploaded) != "" {
			frameworkResult, err = bee.ctx.SendGroupMessage(groupID, "", strings.TrimSpace(uploaded), false, active)
			if err == nil {
				err = messageResultError(frameworkResult)
			}
		}
	}
	if err != nil {
		writeRuntimeLog("群状态图发送失败", fmt.Sprintf("group=%s error=%v result=%s", groupID, err, frameworkResult))
	} else {
		writeRuntimeLog("群状态图发送成功", fmt.Sprintf("group=%s result=%s", groupID, frameworkResult))
	}
	return err
}

func sendFriendImageOnly(bee *BeeAPI, friendID string, result service.GameResult) error {
	imagePath := strings.TrimSpace(result.ImageURL)
	if imagePath == "" {
		return errors.New("状态图路径为空")
	}
	if info, err := os.Stat(imagePath); err == nil && !info.IsDir() {
		defer os.Remove(imagePath)
	}
	frameworkResult, err := bee.ctx.SendFriendMessage(friendID, "", imagePath, false, false, false)
	if err == nil {
		err = messageResultError(frameworkResult)
	}
	if err != nil {
		uploaded, uploadErr := bee.ctx.UploadImage(imagePath)
		if uploadErr == nil && strings.TrimSpace(uploaded) != "" {
			frameworkResult, err = bee.ctx.SendFriendMessage(friendID, "", strings.TrimSpace(uploaded), false, false, false)
			if err == nil {
				err = messageResultError(frameworkResult)
			}
		}
	}
	if err != nil {
		writeRuntimeLog("私信状态图发送失败", fmt.Sprintf("friend=%s error=%v result=%s", friendID, err, frameworkResult))
	} else {
		writeRuntimeLog("私信状态图发送成功", fmt.Sprintf("friend=%s result=%s", friendID, frameworkResult))
	}
	return err
}

func nativeMarkdownEnabled() bool {
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return true
	}
	var value string
	if err := store.DB.Table("system_settings").Select("value").Where("key = ?", "message.mode").Scan(&value).Error; err != nil {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(value), "text")
}

func nativeFallbackEnabled() bool {
	runtimeState.RLock()
	store := runtimeState.store
	runtimeState.RUnlock()
	if store == nil {
		return true
	}
	var value string
	if err := store.DB.Table("system_settings").Select("value").Where("key = ?", "message.native_fallback").Scan(&value).Error; err != nil {
		return true
	}
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "false" && value != "0" && value != "关闭"
}

func messageResultError(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("Bee 消息接口未返回消息ID")
	}
	if value == "0" || strings.EqualFold(value, "false") {
		return fmt.Errorf("Bee 消息接口返回失败值: %s", value)
	}
	var response struct {
		Code    int    `json:"code"`
		ErrCode int    `json:"err_code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(value), &response); err != nil {
		lower := strings.ToLower(value)
		for _, marker := range []string{"失败", "错误", "无权限", "参数", "error", "failed", "forbidden", "invalid"} {
			if strings.Contains(lower, marker) {
				return fmt.Errorf("Bee 消息接口返回失败: %s", value)
			}
		}
		return nil
	}
	if response.Code == 0 && response.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s (code=%d, err_code=%d)", response.Message, response.Code, response.ErrCode)
}

type PluginMetadata struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Ver    string `json:"ver"`
	Text   string `json:"text"`
}

func pluginMetadata() PluginMetadata {
	return PluginMetadata{Name: PluginName, Author: PluginAuthor, Ver: PluginVersion, Text: PluginDescription}
}
