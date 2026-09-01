package model

import (
	"encoding/json"
	"strings"
	"time"
)

type TaskTemplate struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Name             string    `gorm:"size:64;uniqueIndex" json:"name"`
	Type             string    `gorm:"size:32;index" json:"type"`
	Description      string    `gorm:"size:1000" json:"description"`
	PrerequisiteJSON string    `gorm:"type:text" json:"prerequisite_json"`
	ObjectiveJSON    string    `gorm:"type:text" json:"objective_json"`
	RewardJSON       string    `gorm:"type:text" json:"reward_json"`
	Weight           int       `json:"weight"`
	Daily            bool      `gorm:"index" json:"daily"`
	Enabled          bool      `gorm:"index" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type PlayerTask struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PlayerID       uint      `gorm:"index" json:"player_id"`
	TaskTemplateID uint      `gorm:"index" json:"task_template_id"`
	ProgressJSON   string    `gorm:"type:text" json:"progress_json"`
	Status         string    `gorm:"size:16;index" json:"status"`
	AssignedDate   string    `gorm:"size:10;index" json:"assigned_date"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TaskSilverReward returns an explicitly configured silver reward or the
// global free-currency baseline for ordinary gameplay tasks. Achievements are
// excluded because their title and item rewards are intentionally bespoke.
func TaskSilverReward(task TaskTemplate) int64 {
	var rewards map[string]any
	if json.Unmarshal([]byte(task.RewardJSON), &rewards) == nil {
		for _, key := range []string{"silver_coins", "silver", "银币"} {
			if amount := taskJSONInt64(rewards[key]); amount > 0 {
				return amount
			}
		}
	}
	taskType := strings.TrimSpace(task.Type)
	if taskType == "成就" || taskType == "称号" {
		return 0
	}
	var prerequisite struct {
		MinimumRealmSequence int `json:"minimum_realm_sequence"`
		MinimumRealmLevel    int `json:"minimum_realm_level"`
	}
	_ = json.Unmarshal([]byte(task.PrerequisiteJSON), &prerequisite)
	var objective struct {
		Count int64 `json:"count"`
	}
	_ = json.Unmarshal([]byte(task.ObjectiveJSON), &objective)
	if prerequisite.MinimumRealmSequence < 1 {
		prerequisite.MinimumRealmSequence = 1
	}
	if prerequisite.MinimumRealmLevel < 1 {
		prerequisite.MinimumRealmLevel = 1
	}
	if objective.Count < 1 {
		objective.Count = 1
	}
	base := int64(40+prerequisite.MinimumRealmSequence*8+prerequisite.MinimumRealmLevel*4) + objective.Count*6
	multiplier := int64(100)
	switch taskType {
	case "悬赏":
		multiplier = 160
	case "主线":
		multiplier = 140
	case "地图":
		multiplier = 125
	case "宗门", "支线":
		multiplier = 115
	}
	return base * multiplier / 100
}

func taskJSONInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}
