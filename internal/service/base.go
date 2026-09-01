package service

import (
	"xianlv/internal/config"
	"xianlv/internal/model"
	"xianlv/internal/storage"
	"xianlv/internal/util"
)

type Base struct {
	Store  *storage.Store
	Cache  *storage.Cache
	Config config.Config
	Logs   *storage.LogRepository
}

func NewBase(store *storage.Store, cache *storage.Cache, cfg config.Config) *Base {
	return &Base{Store: store, Cache: cache, Config: cfg, Logs: storage.NewLogRepository(store.DB)}
}
func (b *Base) Audit(gm, action, targetType, targetID string, before, after any, ip string) {
	_ = b.Logs.Operation(&model.OperationLog{GMName: gm, Action: action, TargetType: targetType, TargetID: targetID, BeforeJSON: util.JSON(before), AfterJSON: util.JSON(after), IP: ip})
}
