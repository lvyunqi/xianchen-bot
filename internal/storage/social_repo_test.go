package storage

import (
	"testing"
	"time"

	"xianlv/internal/model"
)

func socialTestStore(t *testing.T) *Store {
	t.Helper()
	store := retentionTestStore(t)
	if err := store.DB.AutoMigrate(&model.ShopEntry{}, &model.Player{}); err != nil {
		t.Fatalf("migrate shop: %v", err)
	}
	return store
}

func TestSocialRepositoryCreateAndListPaged(t *testing.T) {
	store := socialTestStore(t)
	social := NewSocialRepository(store.DB)
	for i := 1; i <= 7; i++ {
		msg := model.SocialMessage{SenderID: 1, ReceiverID: 2, Type: "message", Content: "信", Read: i <= 2}
		if err := social.Create(&msg); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	rows, total, err := social.ListReceivedPaged(2, "message", 1, 5)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 7 || len(rows) != 5 {
		t.Errorf("total=%d len(rows)=%d，期望 7/5", total, len(rows))
	}
	if rows[0].ID != 7 {
		t.Errorf("应按 id DESC 返回最新一条，实际 ID=%d", rows[0].ID)
	}
}

func TestSocialRepositoryCountUnreadAndMark(t *testing.T) {
	store := socialTestStore(t)
	social := NewSocialRepository(store.DB)
	var ids []uint
	for i := 1; i <= 4; i++ {
		msg := model.SocialMessage{SenderID: 1, ReceiverID: 2, Type: "message", Content: "信"}
		if err := social.Create(&msg); err != nil {
			t.Fatalf("create: %v", err)
		}
		ids = append(ids, msg.ID)
	}
	unread, err := social.CountUnread(2, []string{"message"})
	if err != nil || unread != 4 {
		t.Fatalf("unread=%d err=%v，期望 4", unread, err)
	}
	if err := social.MarkReadByIDs(ids[:2]); err != nil {
		t.Fatalf("mark: %v", err)
	}
	unread, _ = social.CountUnread(2, []string{"message"})
	if unread != 2 {
		t.Errorf("标记后 unread=%d，期望 2", unread)
	}
	if err := social.MarkTypeRead(2, "message"); err != nil {
		t.Fatalf("mark type: %v", err)
	}
	unread, _ = social.CountUnread(2, []string{"message"})
	if unread != 0 {
		t.Errorf("全量标记后 unread=%d，期望 0", unread)
	}
}

func TestShopRepositoryFilterAndPaging(t *testing.T) {
	store := socialTestStore(t)
	shop := NewShopRepository(store.DB)
	entries := []model.ShopEntry{
		{Code: "shop_a", ItemName: "灵石商品", Currency: "灵石", Price: 100, Enabled: true, Sort: 2},
		{Code: "shop_b", ItemName: "灵石商品二", Currency: "灵石", Price: 200, Enabled: true, Sort: 1},
		{Code: "seed_shop_c", ItemName: "种子", Currency: "灵石", Price: 50, Enabled: true, Sort: 0},
		{Code: "shop_d", ItemName: "下架商品", Currency: "灵石", Price: 300, Enabled: false, Sort: 1},
		{Code: "shop_e", ItemName: "贡献商品", Currency: "贡献", Price: 10, Enabled: true, Sort: 1},
	}
	if err := store.DB.Create(&entries).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows, total, err := shop.ListEnabledPaged(ShopFilter{Currency: "灵石", CodeNotIn: "seed_shop_%"}, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Errorf("total=%d len=%d，期望 2/2（排除种子与下架）", total, len(rows))
	}
	if rows[0].Code != "shop_b" {
		t.Errorf("应按 sort ASC 排在前（shop_b sort=1），实际 %s", rows[0].Code)
	}
	count, err := shop.CountEnabled(ShopFilter{})
	if err != nil || count != 4 {
		t.Errorf("count=%d err=%v，期望 4（启用中的全部）", count, err)
	}
	saleRows, _, err := shop.ListEnabledPaged(ShopFilter{CodeLike: "seed_shop_%"}, 1, 10)
	if err != nil || len(saleRows) != 1 {
		t.Errorf("seed 前缀过滤 len=%d err=%v，期望 1", len(saleRows), err)
	}
}

func TestPlayerRepositoryGMBatch(t *testing.T) {
	store := socialTestStore(t)
	now := time.Now()
	players := []model.Player{
		{AccountID: "a1", DaoName: "甲", SpiritStones: 100, Cultivation: 500, ImmortalJade: 10},
		{AccountID: "a2", DaoName: "乙", SpiritStones: 200, Cultivation: 1000, ImmortalJade: 20, DeletedAt: &now},
	}
	if err := store.DB.Create(&players).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := NewPlayerRepository(store.DB)
	if err := repo.GrantAllSpiritStones(50); err != nil {
		t.Fatalf("grant: %v", err)
	}
	var a1 model.Player
	store.DB.Where("account_id = ?", "a1").First(&a1)
	if a1.SpiritStones != 150 {
		t.Errorf("a1 灵石应 150，实际 %d", a1.SpiritStones)
	}
	var a2 model.Player
	store.DB.Unscoped().Where("account_id = ?", "a2").First(&a2)
	if a2.SpiritStones != 200 {
		t.Errorf("软删玩家不应收到全服发放，实际 %d", a2.SpiritStones)
	}
	if err := repo.ClearAllSpiritStones(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	store.DB.Where("account_id = ?", "a1").First(&a1)
	if a1.SpiritStones != 0 {
		t.Errorf("清空后灵石应 0，实际 %d", a1.SpiritStones)
	}
	if err := repo.ReduceAllCultivationPercent(20); err != nil {
		t.Fatalf("reduce: %v", err)
	}
	store.DB.Where("account_id = ?", "a1").First(&a1)
	if a1.Cultivation != 400 {
		t.Errorf("削减 20%% 后修为应 400，实际 %d", a1.Cultivation)
	}
	if err := repo.BoostAllImmortalAffinity(5); err != nil {
		t.Fatalf("boost: %v", err)
	}
	if err := repo.ReduceAllCultivationFixed(100); err != nil {
		t.Fatalf("fixed: %v", err)
	}
	store.DB.Where("account_id = ?", "a1").First(&a1)
	if a1.Cultivation != 300 {
		t.Errorf("固定扣除 100 后修为应 300，实际 %d", a1.Cultivation)
	}
}

func TestPlayerRepositoryUpdateColumnWhere(t *testing.T) {
	store := socialTestStore(t)
	player := model.Player{AccountID: "p1", DaoName: "守卫", Mana: 10}
	if err := store.DB.Create(&player).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := NewPlayerRepository(store.DB)
	hit, err := repo.UpdateColumnWhere(player.ID, "mana", 4, "mana >= ?", 5)
	if err != nil || !hit {
		t.Fatalf("守卫通过应命中，hit=%v err=%v", hit, err)
	}
	var after model.Player
	store.DB.First(&after, player.ID)
	if after.Mana != 4 {
		t.Errorf("法力应 4，实际 %d", after.Mana)
	}
	hit, err = repo.UpdateColumnWhere(player.ID, "mana", 0, "mana >= ?", 5)
	if err != nil || hit {
		t.Fatalf("守卫不足不应命中，hit=%v err=%v", hit, err)
	}
	store.DB.First(&after, player.ID)
	if after.Mana != 4 {
		t.Errorf("守卫拒绝后法力应仍 4，实际 %d", after.Mana)
	}
}
