package model

import "time"

// AdminMenu drives both the data-backend navigation and the in-game menu.
// Side menus are shown by the backend; top menus with permission=player are
// shown by the prefix-free 菜单 command.
type AdminMenu struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ParentID    uint      `gorm:"index;default:0" json:"parent_id"`
	MenuType    string    `gorm:"size:16;index;default:side" json:"menu_type"`
	Label       string    `gorm:"size:64;index;not null" json:"label"`
	Icon        string    `gorm:"size:32" json:"icon"`
	Path        string    `gorm:"size:128;index" json:"path"`
	Component   string    `gorm:"size:128" json:"component"`
	Permission  string    `gorm:"size:64;index" json:"permission"`
	SortOrder   int       `gorm:"index;default:0" json:"sort_order"`
	IsHidden    bool      `gorm:"index;default:false" json:"is_hidden"`
	IsExternal  bool      `gorm:"default:false" json:"is_external"`
	ExternalURL string    `gorm:"size:255" json:"external_url"`
	Target      string    `gorm:"size:16;default:_self" json:"target"`
	BadgeType   string    `gorm:"size:16" json:"badge_type"`
	BadgeValue  int       `gorm:"default:0" json:"badge_value"`
	Status      string    `gorm:"size:16;index;default:active" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AdminMenuPermission struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	MenuID uint   `gorm:"uniqueIndex:idx_menu_role;index" json:"menu_id"`
	Role   string `gorm:"size:32;uniqueIndex:idx_menu_role" json:"role"`
}

type AdminMenuLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MenuID    uint      `gorm:"index" json:"menu_id"`
	Action    string    `gorm:"size:32;index" json:"action"`
	OldData   string    `gorm:"type:text" json:"old_data"`
	NewData   string    `gorm:"type:text" json:"new_data"`
	Operator  string    `gorm:"size:32" json:"operator"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
