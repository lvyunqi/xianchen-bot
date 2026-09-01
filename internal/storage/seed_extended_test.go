package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestExtendedGameplayCatalogProfilesAreUniqueAndValid(t *testing.T) {
	specs := gameplaySeedSpecs()
	if len(specs) != 17 {
		t.Fatalf("extended gameplay categories=%d, want 17", len(specs))
	}
	validMaterials := map[string]bool{
		"灵石": true, "凝露草": true, "赤焰草": true, "月华花": true, "妖兽内丹": true,
		"玄铁": true, "星辰砂": true, "雷灵晶": true, "阵基石": true, "灵茶": true, "仙露": true,
	}
	requiredKeys := map[string][]string{
		"阵法": {"minimum_perception"}, "符箓": {"minimum_spirit", "minimum_perception"},
		"傀儡": {"minimum_spirit", "minimum_willpower"}, "争夺秘境": {"minimum_reputation"},
		"上古传承": {"minimum_merit", "minimum_root_quality"}, "大道真法": {"minimum_perception", "minimum_dao_heart"},
		"仙魔战场": {"minimum_reputation", "minimum_willpower"}, "灵根进化": {"minimum_root_quality"},
		"渡劫心魔": {"minimum_willpower", "minimum_dao_heart"}, "道侣合体技": {"couple_required", "minimum_immortal_affinity"},
		"九天仙药": {"mansion_required", "minimum_perception"}, "法宝炼化": {"minimum_spirit", "minimum_root_quality"},
		"天机推演": {"minimum_luck", "minimum_perception"}, "天地灵脉": {"minimum_spirit", "required_root_element"},
		"宗门战争": {"sect_required", "minimum_reputation"}, "仙缘奇遇": {"minimum_luck", "minimum_immortal_affinity"},
		"宇宙星河": {"minimum_spirit", "minimum_perception", "minimum_mana"},
	}
	globalCodes := make(map[string]string, len(specs)*1000)

	for _, spec := range specs {
		codes := make(map[string]bool, 1000)
		names := make(map[string]bool, 1000)
		payloads := make(map[string]bool, 1000)
		for index := 1; index <= 1000; index++ {
			row := extendedSeedBase(spec, index)
			if strings.TrimSpace(row.Code) == "" || strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.Type) == "" || strings.TrimSpace(row.Description) == "" {
				t.Fatalf("%s row %d has blank identity fields: %+v", spec.Label, index, row)
			}
			if codes[row.Code] || names[row.Name] {
				t.Fatalf("%s row %d duplicates code or name: %s / %s", spec.Label, index, row.Code, row.Name)
			}
			if owner := globalCodes[row.Code]; owner != "" {
				t.Fatalf("code %s reused by %s and %s", row.Code, owner, spec.Label)
			}
			codes[row.Code], names[row.Name], globalCodes[row.Code] = true, true, spec.Label
			if row.Level < 1 || row.Level > 10 || row.SortOrder != index || row.Status != "启用" {
				t.Fatalf("%s row %d invalid level/order/status: %+v", spec.Label, index, row)
			}
			if len([]rune(row.Description)) < 35 || strings.Contains(row.Description, "唯一配置") {
				t.Fatalf("%s row %d description is placeholder-like: %s", spec.Label, index, row.Description)
			}

			var effect map[string]float64
			if err := json.Unmarshal([]byte(row.EffectParams), &effect); err != nil || effect["power"] <= 0 || effect["duration"] < 10 || effect["growth"] < 1 || effect["growth"] > 5 {
				t.Fatalf("%s row %d invalid effect %q: %v", spec.Label, index, row.EffectParams, err)
			}
			var costs map[string]int64
			if err := json.Unmarshal([]byte(row.CostMaterials), &costs); err != nil || len(costs) < 2 {
				t.Fatalf("%s row %d invalid costs %q: %v", spec.Label, index, row.CostMaterials, err)
			}
			for material, amount := range costs {
				if !validMaterials[material] || amount <= 0 {
					t.Fatalf("%s row %d references unusable cost %s=%d", spec.Label, index, material, amount)
				}
			}
			var prerequisite map[string]any
			if err := json.Unmarshal([]byte(row.Prerequisite), &prerequisite); err != nil {
				t.Fatalf("%s row %d invalid prerequisite %q: %v", spec.Label, index, row.Prerequisite, err)
			}
			for _, key := range []string{"minimum_realm_sequence", "minimum_realm_level", "minimum_combat_power"} {
				if _, ok := prerequisite[key]; !ok {
					t.Fatalf("%s row %d missing common prerequisite %s", spec.Label, index, key)
				}
			}
			for _, key := range requiredKeys[spec.Label] {
				if _, ok := prerequisite[key]; !ok {
					t.Fatalf("%s row %d missing gameplay prerequisite %s", spec.Label, index, key)
				}
			}
			payload := fmt.Sprintf("%s|%s|%s|%s", row.EffectParams, row.CostMaterials, row.Prerequisite, row.Description)
			if payloads[payload] {
				t.Fatalf("%s row %d duplicates its complete gameplay payload", spec.Label, index)
			}
			payloads[payload] = true
		}
		if len(codes) != 1000 || len(names) != 1000 || len(payloads) != 1000 {
			t.Fatalf("%s catalogue incomplete: codes=%d names=%d payloads=%d", spec.Label, len(codes), len(names), len(payloads))
		}
	}
}
