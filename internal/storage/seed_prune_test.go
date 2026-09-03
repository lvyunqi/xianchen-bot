package storage

import (
	"fmt"
	"testing"

	"xianlv/internal/model"
)

func TestCatalogSequence(t *testing.T) {
	cases := map[string]int{
		"catalog_notice_1":     1,
		"catalog_notice_100":   100,
		"catalog_mail_42":      42,
		"world_notice_v2":      0,
		"catalog_notice_":      0,
		"catalog_notice_abc":   0,
	}
	for code, want := range cases {
		if got := catalogSequence(code); got != want {
			t.Errorf("catalogSequence(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestPruneCatalogContentKeepsInLimit(t *testing.T) {
	store := retentionTestStore(t)
	// 造 5 条目录公告 + 3 封目录邮件，limit=3（测试模式 contentSeedLimit=3）
	for i := 1; i <= 5; i++ {
		notice := model.Notice{Code: fmt.Sprintf("catalog_notice_%d", i), Title: "样板", Content: "样板正文", Type: "公告", Published: true}
		if err := store.DB.Create(&notice).Error; err != nil {
			t.Fatalf("seed notice: %v", err)
		}
	}
	for i := 1; i <= 3; i++ {
		mail := model.Mail{Code: fmt.Sprintf("catalog_mail_%d", i), Title: "样板信", Content: "信", Sender: "仙尘", RewardJSON: "[]"}
		if err := store.DB.Create(&mail).Error; err != nil {
			t.Fatalf("seed mail: %v", err)
		}
	}
	// 一条不受影响的真实公告
	keep := model.Notice{Code: "world_notice_real", Title: "真实公告", Content: "重要", Type: "公告", Published: true}
	if err := store.DB.Create(&keep).Error; err != nil {
		t.Fatalf("seed keep: %v", err)
	}

	if err := store.pruneCatalogContent(); err != nil {
		t.Fatalf("pruneCatalogContent: %v", err)
	}
	var notices []model.Notice
	if err := store.DB.Where("code LIKE ?", "catalog_notice_%").Find(&notices).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(notices) != 3 {
		t.Errorf("目录公告应保留 3 条，实际 %d", len(notices))
	}
	var mails int64
	store.DB.Model(&model.Mail{}).Where("code LIKE ?", "catalog_mail_%").Count(&mails)
	if mails != 3 {
		t.Errorf("目录邮件应保留 3 封，实际 %d", mails)
	}
	var real int64
	store.DB.Model(&model.Notice{}).Where("code = ?", "world_notice_real").Count(&real)
	if real != 1 {
		t.Error("非目录公告不应被删除")
	}
	// 幂等：再跑一次不报错不误删
	if err := store.pruneCatalogContent(); err != nil {
		t.Fatalf("second prune: %v", err)
	}
	store.DB.Model(&model.Notice{}).Where("code LIKE ?", "catalog_notice_%").Count(&mails)
	if mails != 3 {
		t.Errorf("二次清理后目录公告应仍为 3 条，实际 %d", mails)
	}
}
