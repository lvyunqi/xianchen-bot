package service

import (
	"xianlv/internal/model"
)

// 本文件定义 service 层依赖的持久化窄接口（consumer-side interface）。
// storage 包的具体 Repository 实现隐式满足这些接口；单测可以注入内存实现。
// 收口原则：新增代码一律走接口；存量直查按热域渐进迁移。

// PlayerStore 玩家档案与背包的读写能力。
type PlayerStore interface {
	Get(id uint) (model.Player, error)
	GetByAccount(accountID string) (model.Player, error)
	Inventory(playerID uint) ([]model.PlayerItem, error)
	Update(id uint, changes map[string]any) error
	AdjustItem(playerID, itemID uint, delta int64) error
	Delete(id uint) error
}

// PlayerCounters 玩家只读统计（榜单、查重、战力对比等聚合查询）。
type PlayerCounters interface {
	CountStrongerThan(player model.Player) (int64, error)
	CountByDaoName(daoName string, excludeID uint) (int64, error)
}

// SocialWriter 社交消息写入（私信/通知/请求，玩家即时可见，逐条落库）。
// 事务内写入走 storage.SocialRepository.CreateInTx，属实现细节，不进接口。
type SocialWriter interface {
	Create(msg *model.SocialMessage) error
}

// LogWriter 游戏流水日志写入。
type LogWriter interface {
	Game(level, kind string, playerID uint, message, metadata string) error
}
