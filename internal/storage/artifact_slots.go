package storage

import (
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const ArtifactSlotSyncMigrationKey = "migration.artifact_slot_sync"

var artifactEquipmentSlots = []string{"本命法器", "冠冕", "道袍", "护腕", "腰佩", "灵靴", "戒指", "项链", "护符", "阵盘"}

type artifactFormRule struct {
	terms     []string
	slot      string
	archetype string
}

var artifactFormRules = []artifactFormRule{
	{[]string{"葫芦", "宝葫", "吞天葫", "葫"}, "腰佩", "宝葫"},
	{[]string{"飞舟", "仙舟", "御风舟", "舟"}, "灵靴", "飞舟"},
	{[]string{"道袍", "法袍", "仙衣", "法衣", "衣", "袍"}, "道袍", "仙衣"},
	{[]string{"灵索", "法索", "缚龙索", "索"}, "护腕", "灵索"},
	{[]string{"法尺", "量天尺", "尺"}, "护腕", "法尺"},
	{[]string{"宝镜", "法镜", "照魂镜", "镜"}, "项链", "宝镜"},
	{[]string{"道琴", "仙琴", "问心琴", "琴"}, "项链", "道琴"},
	{[]string{"法钟", "仙钟", "渡厄钟", "钟"}, "冠冕", "法钟"},
	{[]string{"护道塔", "宝塔", "仙塔", "塔"}, "冠冕", "护道塔"},
	{[]string{"灵扇", "法扇", "离火扇", "扇"}, "戒指", "灵扇"},
	{[]string{"道珠", "宝珠", "玄水珠", "珠"}, "戒指", "道珠"},
	{[]string{"道幡", "法幡", "星河幡", "幡"}, "阵盘", "道幡"},
	{[]string{"丹鼎", "仙鼎", "太虚鼎", "鼎"}, "阵盘", "丹鼎"},
	{[]string{"法印", "仙印", "镇岳印", "印"}, "护符", "法印"},
	{[]string{"宝伞", "仙伞", "混元伞", "伞"}, "护符", "宝伞"},
	{[]string{"仙剑", "法剑", "斩仙剑", "剑"}, "本命法器", "仙剑"},
	{[]string{"神枪", "仙枪", "雷罚枪", "枪"}, "本命法器", "神枪"},
	{[]string{"灵弓", "仙弓", "逐日弓", "弓"}, "灵靴", "灵弓"},
	{[]string{"山河图", "道图", "仙图", "图"}, "道袍", "山河图"},
	{[]string{"法轮", "因果轮", "轮"}, "腰佩", "法轮"},
	{[]string{"灵佩", "玉佩", "腰佩", "佩"}, "腰佩", "灵佩"},
}

func artifactForm(text string) (artifactFormRule, bool) {
	text = strings.TrimSpace(text)
	bestIndex, bestLength := -1, -1
	best := artifactFormRule{}
	for _, rule := range artifactFormRules {
		for _, term := range rule.terms {
			index := strings.LastIndex(text, term)
			if index > bestIndex || (index == bestIndex && len(term) > bestLength) {
				bestIndex, bestLength, best = index, len(term), rule
			}
		}
	}
	return best, bestIndex >= 0
}

func isArtifactEquipmentSlot(value string) bool {
	for _, slot := range artifactEquipmentSlots {
		if strings.TrimSpace(value) == slot {
			return true
		}
	}
	return false
}

// ArtifactSlot maps a器型 or legacy category to one wearable position. Unknown
// forms use the neutral护符 slot instead of crowding the本命法器 slot.
func ArtifactSlot(kind string) string {
	if form, ok := artifactForm(kind); ok {
		return form.slot
	}
	for _, slot := range artifactEquipmentSlots {
		if strings.Contains(kind, slot) {
			return slot
		}
	}
	switch strings.TrimSpace(kind) {
	case "攻击":
		return "本命法器"
	case "防御":
		return "道袍"
	case "辅助", "均衡":
		return "护符"
	default:
		return "护符"
	}
}

func ArtifactTemplateSlot(row model.ArtifactTemplate) string {
	if isArtifactEquipmentSlot(row.Slot) {
		return strings.TrimSpace(row.Slot)
	}
	if form, ok := artifactForm(strings.Join([]string{row.Archetype, row.Name, row.Type}, " ")); ok {
		return form.slot
	}
	return ArtifactSlot(row.Type)
}

func canonicalArtifactArchetype(row model.ArtifactTemplate) string {
	current := strings.TrimSpace(row.Archetype)
	if current != "" && !isArtifactEquipmentSlot(current) && !isLegacyArtifactCategory(current) {
		return current
	}
	if form, ok := artifactForm(strings.Join([]string{row.Archetype, row.Name, row.Type}, " ")); ok {
		return form.archetype
	}
	current = strings.TrimSpace(row.Type)
	if current != "" && !isArtifactEquipmentSlot(current) && !isLegacyArtifactCategory(current) {
		return current
	}
	return "法器"
}

func isLegacyArtifactCategory(value string) bool {
	switch strings.TrimSpace(value) {
	case "攻击", "防御", "辅助", "均衡":
		return true
	default:
		return false
	}
}

// normalizeArtifactSlots repairs template and owned-instance slots in place.
// Only slot/type metadata changes; every instance ID and all cultivation fields
// (level, forge, inscription, stars and sockets) remain untouched.
func (s *Store) normalizeArtifactSlots() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var templates []model.ArtifactTemplate
		if err := tx.Find(&templates).Error; err != nil {
			return err
		}
		targets := make(map[uint]string, len(templates))
		for _, row := range templates {
			slot := ArtifactTemplateSlot(row)
			targets[row.ID] = slot
			updates := map[string]any{}
			if row.Slot != slot {
				updates["slot"] = slot
			}
			archetype := canonicalArtifactArchetype(row)
			if strings.TrimSpace(row.Archetype) == "" || isArtifactEquipmentSlot(row.Archetype) || isLegacyArtifactCategory(row.Archetype) {
				updates["archetype"] = archetype
			}
			if strings.TrimSpace(row.Type) == "" || isArtifactEquipmentSlot(row.Type) || isLegacyArtifactCategory(row.Type) {
				updates["type"] = archetype
			}
			if len(updates) > 0 {
				if err := tx.Model(&model.ArtifactTemplate{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
		}

		var owned []model.PlayerArtifact
		if err := tx.Find(&owned).Error; err != nil {
			return err
		}
		for _, row := range owned {
			slot := targets[row.TemplateID]
			if slot == "" {
				if form, ok := artifactForm(row.Name); ok {
					slot = form.slot
				}
			}
			if slot == "" || row.Slot == slot {
				continue
			}
			if err := tx.Model(&model.PlayerArtifact{}).Where("id = ?", row.ID).Update("slot", slot).Error; err != nil {
				return err
			}
			marker := model.PlayerValue{PlayerID: row.PlayerID, Key: ArtifactSlotSyncMigrationKey, Value: "true"}
			if err := tx.Where("player_id = ? AND key = ?", row.PlayerID, marker.Key).
				Assign(map[string]any{"value": marker.Value, "expires_at": nil}).FirstOrCreate(&marker).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
