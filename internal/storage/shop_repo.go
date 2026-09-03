package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

// ShopFilter 商城列表过滤条件；零值字段表示不过滤。
type ShopFilter struct {
	Currency  string // 限定货币（灵石/贡献/仙金…）
	CodeNotIn string // 排除的 code 前缀（如 seed_shop_%）
	CodeLike  string // 限定 code 前缀（如 event_sale_%）
}

// ShopRepository 商城条目读取（写入由管理端资源 CRUD 负责，不经此仓库）。
type ShopRepository struct{ db *gorm.DB }

func NewShopRepository(db *gorm.DB) *ShopRepository { return &ShopRepository{db: db} }

func (r *ShopRepository) applyFilter(query *gorm.DB, filter ShopFilter) *gorm.DB {
	query = query.Where("enabled = ?", true)
	if filter.Currency != "" {
		query = query.Where("currency = ?", filter.Currency)
	}
	if filter.CodeNotIn != "" {
		query = query.Where("code NOT LIKE ?", filter.CodeNotIn)
	}
	if filter.CodeLike != "" {
		query = query.Where("code LIKE ?", filter.CodeLike)
	}
	return query
}

// ListEnabledPaged 启用中的商城条目分页（含总数）。
func (r *ShopRepository) ListEnabledPaged(filter ShopFilter, page, pageSize int) (rows []model.ShopEntry, total int64, err error) {
	query := r.applyFilter(r.db.Model(&model.ShopEntry{}), filter)
	if err = query.Count(&total).Error; err != nil {
		return
	}
	err = query.Order("sort ASC, id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&rows).Error
	return
}

// CountEnabled 按过滤条件统计启用条目数（仪表盘/目录概览用）。
func (r *ShopRepository) CountEnabled(filter ShopFilter) (int64, error) {
	var count int64
	err := r.applyFilter(r.db.Model(&model.ShopEntry{}), filter).Count(&count).Error
	return count, err
}

// PriceSourceByItems 指定物品的商城售价来源（送礼成本估算等场景）。
func (r *ShopRepository) PriceSourceByItems(itemIDs []uint) (rows []model.ShopEntry, err error) {
	err = r.db.Select("item_id, currency, price").
		Where("enabled = ? AND item_id IN ?", true, itemIDs).
		Order("price ASC").Find(&rows).Error
	return
}
