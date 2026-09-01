package storage

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

func TestGiftPackCatalogHasOneThousandUniqueCultivationRewards(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "gift-catalog.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.seedGiftPackCatalogLimit(1000); err != nil {
		t.Fatal(err)
	}
	var rows []model.Item
	if err := store.DB.Where("code LIKE ?", "cultivation_gift_%").Order("code").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1000 {
		t.Fatalf("generated gift count=%d, want 1000", len(rows))
	}
	digits := regexp.MustCompile(`[0-9]`)
	names := make(map[string]struct{}, len(rows))
	rewardItems := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if digits.MatchString(row.Name) || strings.Contains(row.Name, "后台") || strings.Contains(row.Description, "后台") {
			t.Fatalf("gift contains forbidden generated wording: name=%q description=%q", row.Name, row.Description)
		}
		if _, exists := names[row.Name]; exists {
			t.Fatalf("duplicate gift name %q", row.Name)
		}
		names[row.Name] = struct{}{}
		var rewards seededGiftRewards
		if err := json.Unmarshal([]byte(row.EffectParams), &rewards); err != nil {
			t.Fatalf("gift %s reward JSON: %v", row.Name, err)
		}
		if len(rewards.Items) != 1 || len(rewards.Artifacts) != 1 {
			t.Fatalf("gift %s incomplete rewards: %+v", row.Name, rewards)
		}
		for itemName := range rewards.Items {
			if _, exists := rewardItems[itemName]; exists {
				t.Fatalf("reward item reused across gift packs: %s", itemName)
			}
			rewardItems[itemName] = struct{}{}
		}
	}
	var shopCount int64
	if err := store.DB.Model(&model.ShopEntry{}).Where("code LIKE ? AND enabled = ? AND purchase_limit = ? AND refresh_cycle = ?", "cultivation_gift_shop_%", true, 0, "永不").Count(&shopCount).Error; err != nil || shopCount != 1000 {
		t.Fatalf("gift shop entries=%d err=%v", shopCount, err)
	}
}
