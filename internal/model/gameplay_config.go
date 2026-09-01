package model

import "time"

// GameplayConfigBase is shared by the 17 independent fourth-batch gameplay
// tables. Each named type below maps to its own physical table.
type GameplayConfigBase struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name          string    `gorm:"size:64;uniqueIndex" json:"name"`
	Type          string    `gorm:"size:32;index" json:"type"`
	Level         int       `gorm:"index" json:"level"`
	Description   string    `gorm:"type:text" json:"description"`
	EffectParams  string    `gorm:"type:text" json:"effect_params"`
	CostMaterials string    `gorm:"type:text" json:"cost_materials"`
	Prerequisite  string    `gorm:"type:text" json:"prerequisite"`
	SortOrder     int       `gorm:"index" json:"sort_order"`
	Status        string    `gorm:"size:16;index" json:"status"`
	ImageURL      string    `gorm:"size:500" json:"image_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type FormationConfig GameplayConfigBase
type TalismanConfig GameplayConfigBase
type PuppetConfig GameplayConfigBase
type SecretRealmConflictConfig GameplayConfigBase
type InheritanceConfig GameplayConfigBase
type DaoInsightConfig GameplayConfigBase
type ImmortalDemonBattlefieldConfig GameplayConfigBase
type SpiritualRootEvolutionConfig GameplayConfigBase
type InnerDemonConfig GameplayConfigBase
type CoupleCombinationSkillConfig GameplayConfigBase
type ImmortalHerbConfig GameplayConfigBase
type ArtifactRefinementConfig GameplayConfigBase
type DestinyDeductionConfig GameplayConfigBase
type LeylineConfig GameplayConfigBase
type SectWarConfig GameplayConfigBase
type ImmortalEncounterConfig GameplayConfigBase
type StarRealmConfig GameplayConfigBase

func (FormationConfig) TableName() string                { return "formation_configs" }
func (TalismanConfig) TableName() string                 { return "talisman_configs" }
func (PuppetConfig) TableName() string                   { return "puppet_configs" }
func (SecretRealmConflictConfig) TableName() string      { return "secret_realm_conflict_configs" }
func (InheritanceConfig) TableName() string              { return "inheritance_configs" }
func (DaoInsightConfig) TableName() string               { return "dao_insight_configs" }
func (ImmortalDemonBattlefieldConfig) TableName() string { return "immortal_demon_battlefield_configs" }
func (SpiritualRootEvolutionConfig) TableName() string   { return "spiritual_root_evolution_configs" }
func (InnerDemonConfig) TableName() string               { return "inner_demon_configs" }
func (CoupleCombinationSkillConfig) TableName() string   { return "couple_combination_skill_configs" }
func (ImmortalHerbConfig) TableName() string             { return "immortal_herb_configs" }
func (ArtifactRefinementConfig) TableName() string       { return "artifact_refinement_configs" }
func (DestinyDeductionConfig) TableName() string         { return "destiny_deduction_configs" }
func (LeylineConfig) TableName() string                  { return "leyline_configs" }
func (SectWarConfig) TableName() string                  { return "sect_war_configs" }
func (ImmortalEncounterConfig) TableName() string        { return "immortal_encounter_configs" }
func (StarRealmConfig) TableName() string                { return "star_realm_configs" }
