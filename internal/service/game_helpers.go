package service

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

var mentionIDPattern = regexp.MustCompile(`[0-9A-Za-z_-]{5,}`)

func (g *Game) settingInt(key string, fallback int64) int64 {
	var row model.SystemSetting
	if err := g.store.DB.Where("key = ?", key).First(&row).Error; err != nil {
		return fallback
	}
	value, err := strconv.ParseInt(strings.TrimSpace(row.Value), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func (g *Game) settingFloat(key string, fallback float64) float64 {
	var row model.SystemSetting
	if err := g.store.DB.Where("key = ?", key).First(&row).Error; err != nil {
		return fallback
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(row.Value), 64)
	if err != nil {
		return fallback
	}
	return value
}

func (g *Game) settingBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(g.settingText(key, "")))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on", "开启", "启用", "图片", "图片模式":
		return true
	case "0", "false", "no", "off", "关闭", "停用", "文字", "文字模式":
		return false
	default:
		return fallback
	}
}

func (g *Game) settingText(key, fallback string) string {
	var row model.SystemSetting
	if err := g.store.DB.Where("key = ?", key).First(&row).Error; err != nil || strings.TrimSpace(row.Value) == "" {
		return fallback
	}
	return strings.TrimSpace(row.Value)
}

func (g *Game) playerAvatarURL(player *model.Player) string {
	if strings.TrimSpace(player.AvatarURL) != "" {
		return strings.TrimSpace(player.AvatarURL)
	}
	template := g.settingText("image.avatar_template", "")
	if template != "" {
		return strings.ReplaceAll(template, "{user_id}", player.AccountID)
	}
	return g.settingText("image.status_url", "")
}

// CachePlayerAvatar stores the framework-resolved QQ Open Platform avatar URL.
// Status rendering reads it immediately on the same command and again as a
// fallback when the framework avatar endpoint has a transient failure.
func (g *Game) CachePlayerAvatar(accountID, avatarURL string) error {
	avatarURL = strings.TrimSpace(avatarURL)
	if accountID == "" || avatarURL == "" {
		return errors.New("玩家账号或头像地址为空")
	}
	lower := strings.ToLower(avatarURL)
	if (!strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://")) || len(avatarURL) > 500 {
		return errors.New("头像地址不是有效的HTTP地址")
	}
	return g.players.UpdateAvatarByAccount(accountID, avatarURL)
}

func (g *Game) playerValue(playerID uint, key string) (string, error) {
	var row model.PlayerValue
	err := g.store.DB.Where("player_id = ? AND key = ?", playerID, key).First(&row).Error
	if err != nil {
		return "", err
	}
	if row.ExpiresAt != nil && row.ExpiresAt.Before(time.Now()) {
		_ = g.store.DB.Delete(&row).Error
		return "", gorm.ErrRecordNotFound
	}
	return row.Value, nil
}

func (g *Game) setPlayerValue(playerID uint, key, value string, expiresAt *time.Time) error {
	row := model.PlayerValue{PlayerID: playerID, Key: key, Value: value, ExpiresAt: expiresAt}
	return g.store.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "player_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "expires_at", "updated_at"}),
	}).Create(&row).Error
}

func (g *Game) playerValueInt(playerID uint, key string, fallback int64) int64 {
	value, err := g.playerValue(playerID, key)
	if err != nil {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (g *Game) setPlayerValueInt(playerID uint, key string, value int64) error {
	return g.setPlayerValue(playerID, key, strconv.FormatInt(value, 10), nil)
}

func (g *Game) addPlayerValueInt(playerID uint, key string, delta int64) (int64, error) {
	value := g.playerValueInt(playerID, key, 0) + delta
	return value, g.setPlayerValueInt(playerID, key, value)
}

func (g *Game) cooldown(playerID uint, key string, duration time.Duration) (time.Duration, bool, error) {
	value, err := g.playerValue(playerID, "cooldown."+key)
	if err == nil {
		until, parseErr := time.Parse(time.RFC3339Nano, value)
		if parseErr == nil && until.After(time.Now()) {
			return time.Until(until), false, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, err
	}
	until := time.Now().Add(duration)
	if err := g.setPlayerValue(playerID, "cooldown."+key, until.Format(time.RFC3339Nano), &until); err != nil {
		return 0, false, err
	}
	return 0, true, nil
}

func (g *Game) findPlayer(argument string) (model.Player, error) {
	argument = strings.TrimSpace(argument)
	argument = strings.TrimPrefix(argument, "@")
	if match := mentionIDPattern.FindString(argument); match != "" {
		argument = match
	}
	var player model.Player
	err := g.store.DB.Where("account_id = ? OR dao_name = ?", argument, strings.TrimSpace(argument)).First(&player).Error
	return player, err
}

func (g *Game) itemByName(name string) (model.Item, error) {
	var item model.Item
	err := g.store.DB.Where("name = ? OR code = ?", strings.TrimSpace(name), strings.TrimSpace(name)).First(&item).Error
	return item, err
}

func (g *Game) itemQuantity(playerID uint, itemID uint) int64 {
	var row model.PlayerItem
	if err := g.store.DB.Where("player_id = ? AND item_id = ?", playerID, itemID).First(&row).Error; err != nil {
		return 0
	}
	return row.Quantity
}

func (g *Game) adjustNamedItem(playerID uint, itemName string, delta int64) error {
	item, err := g.itemByName(itemName)
	if err != nil {
		return err
	}
	return g.players.AdjustItem(playerID, item.ID, delta)
}

func consumeNamedItemTx(tx *gorm.DB, playerID uint, itemName string, quantity int64) error {
	if quantity <= 0 {
		return nil
	}
	var item model.Item
	if err := tx.Where("name = ? OR code = ?", itemName, itemName).First(&item).Error; err != nil {
		return err
	}
	var row model.PlayerItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND item_id = ?", playerID, item.ID).First(&row).Error; err != nil {
		return err
	}
	if row.Quantity < quantity {
		return errors.New("insufficient item quantity")
	}
	if row.Quantity == quantity {
		return tx.Delete(&row).Error
	}
	return tx.Model(&row).Update("quantity", gorm.Expr("quantity - ?", quantity)).Error
}

func (g *Game) randomEnabledItem() (model.Item, error) {
	var items []model.Item
	if err := g.store.DB.Where("tradable = ?", true).Find(&items).Error; err != nil {
		return model.Item{}, err
	}
	if len(items) == 0 {
		return model.Item{}, errors.New("物品库为空")
	}
	return items[rand.Intn(len(items))], nil
}

func formatDuration(duration time.Duration) string {
	if duration < time.Minute {
		seconds := int(duration.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		return fmt.Sprintf("%d秒", seconds)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

func firstArgument(commandArgs []string) string {
	if len(commandArgs) == 0 {
		return ""
	}
	return commandArgs[0]
}

// parseStackQuantity accepts the player-facing "名称*数量" form. There is no
// gameplay ceiling; strconv's int64 range is the only technical boundary.
func parseStackQuantity(raw string) (string, int64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "×", "*"))
	index := strings.LastIndex(raw, "*")
	if index < 0 {
		return raw, 1, nil
	}
	name := strings.TrimSpace(raw[:index])
	quantityText := strings.TrimSpace(raw[index+1:])
	quantity, err := strconv.ParseInt(quantityText, 10, 64)
	if err != nil || quantity <= 0 || name == "" {
		return "", 0, errors.New("数量格式不正确，请使用：名称*正整数")
	}
	return name, quantity, nil
}

// parseFlexibleStackQuantity keeps the former "名称 数量" input while making
// "名称*数量" the common form used by every stackable gameplay action.
func parseFlexibleStackQuantity(raw string) (string, int64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "×", "*"))
	if strings.Contains(raw, "*") {
		return parseStackQuantity(raw)
	}
	parts := strings.Fields(raw)
	if len(parts) > 1 {
		if quantity, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
			name := strings.TrimSpace(strings.Join(parts[:len(parts)-1], " "))
			if quantity <= 0 || name == "" {
				return "", 0, errors.New("数量格式不正确，请使用：名称*正整数")
			}
			return name, quantity, nil
		}
	}
	if raw == "" {
		return "", 0, errors.New("名称不能为空")
	}
	return raw, 1, nil
}

func parseNamedAmount(raw, expectedName string) (int64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "×", "*"))
	if amount, err := strconv.ParseInt(raw, 10, 64); err == nil && amount > 0 {
		return amount, nil
	}
	name, amount, err := parseStackQuantity(raw)
	if err != nil || !strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(expectedName)) {
		return 0, errors.New("数量必须是正整数，可使用" + expectedName + "*数量")
	}
	return amount, nil
}

func randomPercent() int {
	return rand.Intn(100) + 1
}

func (g *Game) queueContentReview(contentType string, player *model.Player, content string) error {
	content = strings.TrimSpace(content)
	if player == nil || content == "" {
		return nil
	}
	status := "待审核"
	reason := ""
	if word, _, matched, _ := g.matchSensitiveWord(content); matched {
		status = "已拒绝"
		reason = "自动审核命中敏感词：" + word
	}
	row := model.ContentReview{Type: contentType, PlayerID: player.ID, PlayerName: player.DaoName, Content: content, Status: status, Reason: reason}
	return g.store.DB.Create(&row).Error
}

func (g *Game) matchSensitiveWord(content string) (string, string, bool, error) {
	normalized := normalizeModerationText(content)
	if normalized == "" {
		return "", "", false, nil
	}
	if term := criticalSensitiveTerm(normalized); term != "" {
		return term, "[违规内容已屏蔽]", true, nil
	}
	var words []model.SensitiveWord
	if err := g.store.DB.Where("enabled = ?", true).Find(&words).Error; err != nil {
		return "", "", false, err
	}
	for _, word := range words {
		candidate := normalizeModerationText(word.Word)
		if candidate != "" && strings.Contains(normalized, candidate) {
			return word.Word, displayOr(word.Replacement, "[内容已屏蔽]"), true, nil
		}
	}
	return "", "", false, nil
}

func criticalSensitiveTerm(normalized string) string {
	criticalTerms := []string{
		"草泥马", "操你妈", "草你妈", "你妈死了", "全家去死", "傻逼", "煞笔", "脑残", "去死吧",
		"加微信", "加威信", "低价代充", "游戏外挂", "盗号教程", "网络赌博", "色情网站", "毒品交易", "枪支交易",
	}
	for _, term := range criticalTerms {
		if strings.Contains(normalized, normalizeModerationText(term)) {
			return term
		}
	}
	return ""
}

func normalizeModerationText(value string) string {
	var output strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			output.WriteRune(r)
		}
	}
	return output.String()
}

func (g *Game) rejectSensitiveContent(contentType string, player *model.Player, content string) (GameResult, bool, error) {
	word, _, matched, err := g.matchSensitiveWord(content)
	if err != nil || !matched {
		return GameResult{}, false, err
	}
	row := model.ContentReview{Type: contentType, PlayerID: player.ID, PlayerName: player.DaoName, Content: content, Status: "已拒绝", Reason: "自动审核命中敏感词：" + word}
	if err := g.store.DB.Create(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "内容审核未通过", Content: "内容触发仙盟禁用词，已阻止发送并记录审核，请修改后重试。", Actions: []string{"帮助"}}, true, nil
}
