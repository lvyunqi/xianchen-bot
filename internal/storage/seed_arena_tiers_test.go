package storage

import (
	"testing"
	"unicode"
)

func TestArenaTierCatalogContainsOneThousandUniqueNamedTiers(t *testing.T) {
	rows := arenaTierCatalog(1000)
	if len(rows) != 1000 {
		t.Fatalf("arena tiers=%d want=1000", len(rows))
	}
	names := make(map[string]struct{}, len(rows))
	minimums := make(map[int64]struct{}, len(rows))
	coins := make(map[int64]struct{}, len(rows))
	silvers := make(map[int64]struct{}, len(rows))
	for index, row := range rows {
		if row.Sequence != index+1 || row.Name == "" || row.Description == "" || !row.Enabled {
			t.Fatalf("invalid arena tier at %d: %+v", index, row)
		}
		for _, r := range row.Name {
			if unicode.IsDigit(r) {
				t.Fatalf("arena tier name contains a number: %s", row.Name)
			}
		}
		if _, exists := names[row.Name]; exists {
			t.Fatalf("duplicate arena tier name: %s", row.Name)
		}
		if _, exists := minimums[row.MinimumRating]; exists {
			t.Fatalf("duplicate minimum rating: %d", row.MinimumRating)
		}
		if _, exists := coins[row.DailyCoin]; exists {
			t.Fatalf("duplicate daily coin reward: %d", row.DailyCoin)
		}
		if _, exists := silvers[row.DailySilver]; exists {
			t.Fatalf("duplicate daily silver reward: %d", row.DailySilver)
		}
		names[row.Name] = struct{}{}
		minimums[row.MinimumRating] = struct{}{}
		coins[row.DailyCoin] = struct{}{}
		silvers[row.DailySilver] = struct{}{}
	}
	if rows[999].MinimumRating <= rows[998].MinimumRating {
		t.Fatal("arena tier thresholds are not strictly increasing")
	}
}
