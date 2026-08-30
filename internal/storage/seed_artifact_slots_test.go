package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

func TestGeneratedArtifactCatalogBalancesAllTenSlots(t *testing.T) {
	expected := map[string]int{
		"本命法器": 100, "冠冕": 100, "道袍": 100, "护腕": 100, "腰佩": 100,
		"灵靴": 100, "戒指": 100, "项链": 100, "护符": 100, "阵盘": 100,
	}
	counts := make(map[string]int, len(expected))
	names := make(map[string]struct{}, 1000)
	for index := 1; index <= 1000; index++ {
		seedName := cultivationSeedName(index)
		profile := cultivationArtifactProfile(index, seedName)
		counts[profile.Slot]++
		if strings.TrimSpace(profile.Name) == "" || strings.TrimSpace(profile.Archetype) == "" || strings.TrimSpace(profile.Positioning) == "" ||
			profile.AttributeJSON == "{}" || !strings.Contains(profile.Description, profile.Slot+"槽位") {
			t.Fatalf("incomplete artifact profile %d: %+v", index, profile)
		}
		if _, exists := names[profile.Name]; exists {
			t.Fatalf("duplicate generated artifact name: %s", profile.Name)
		}
		names[profile.Name] = struct{}{}
	}
	if len(names) != 1000 || len(counts) != len(expected) {
		t.Fatalf("artifact names=%d slot kinds=%d counts=%v", len(names), len(counts), counts)
	}
	for slot, want := range expected {
		if counts[slot] != want {
			t.Fatalf("slot %s count=%d want=%d; all=%v", slot, counts[slot], want, counts)
		}
	}
}

func TestArtifactSlotMappingUsesRealFormAndKeepsExplicitSlot(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"太虚斩仙剑", "本命法器"}, {"雷罚神枪", "本命法器"}, {"青冥吞天葫", "腰佩"},
		{"九霄御风舟", "灵靴"}, {"万法仙衣", "道袍"}, {"缚龙灵索", "护腕"},
		{"照魂宝镜", "项链"}, {"问心道琴", "项链"}, {"渡厄法钟", "冠冕"}, {"护道宝塔", "冠冕"},
		{"离火灵扇", "戒指"}, {"玄水道珠", "戒指"}, {"星河道幡", "阵盘"}, {"太虚丹鼎", "阵盘"},
		{"镇岳法印", "护符"}, {"混元宝伞", "护符"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := model.ArtifactTemplate{Name: test.name}
			if got := ArtifactTemplateSlot(row); got != test.want {
				t.Fatalf("slot=%s want=%s", got, test.want)
			}
		})
	}
	explicit := model.ArtifactTemplate{Name: "运营自定义吞天葫", Archetype: "宝葫", Slot: "本命法器"}
	if got := ArtifactTemplateSlot(explicit); got != "本命法器" {
		t.Fatalf("explicit valid slot was overridden: %s", got)
	}
	generated := cultivationArtifactProfile(8, cultivationSeedName(8))
	if generated.Slot != "腰佩" || ArtifactTemplateSlot(model.ArtifactTemplate{Name: generated.Name, Type: generated.Archetype, Archetype: generated.Archetype, Slot: generated.Slot}) != "腰佩" {
		t.Fatalf("generated gourd profile not stable: %+v", generated)
	}
}

func TestArtifactSlotMigrationPreservesOwnedCultivation(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "artifact-slot-migration.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	player := model.Player{AccountID: "artifact-slot-migration", DaoName: "藏器真人", RealmName: "炼气", RealmLevel: 1}
	if err := store.DB.Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	template := model.ArtifactTemplate{Code: "legacy_gourd_slot_test", Name: "迁移验收吞天葫", Type: "宝葫", Archetype: "宝葫", AttributeJSON: `{"attack":12}`, Enabled: true}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	owned := model.PlayerArtifact{
		PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Slot: "本命法器", Equipped: true,
		Level: 7, Quality: "仙品", ForgeLevel: 9, Inscription: "庚金破军纹", StarLevel: 6, SocketCount: 2, SocketJSON: `["赤曜攻伐石"]`, Activated: true,
	}
	if err := store.DB.Create(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.normalizeArtifactSlots(); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.First(&template, template.ID).Error; err != nil || template.Slot != "腰佩" {
		t.Fatalf("template slot=%q err=%v", template.Slot, err)
	}
	if err := store.DB.First(&owned, owned.ID).Error; err != nil {
		t.Fatal(err)
	}
	if owned.Slot != "腰佩" || owned.Level != 7 || owned.Quality != "仙品" || owned.ForgeLevel != 9 || owned.Inscription != "庚金破军纹" || owned.StarLevel != 6 || owned.SocketCount != 2 || owned.SocketJSON != `["赤曜攻伐石"]` || !owned.Equipped || !owned.Activated {
		t.Fatalf("owned artifact cultivation changed during migration: %+v", owned)
	}
	var marker model.PlayerValue
	if err := store.DB.Where("player_id = ? AND key = ?", player.ID, ArtifactSlotSyncMigrationKey).First(&marker).Error; err != nil {
		t.Fatalf("slot repair marker missing: %v", err)
	}
}
