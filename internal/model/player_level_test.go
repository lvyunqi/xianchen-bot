package model

import "testing"

func TestApplyPlayerExperienceSupportsContinuousLevelUps(t *testing.T) {
	player := Player{
		Level: 1, Health: 80, MaxHealth: 100, Mana: 40, MaxMana: 50,
		PhysicalAttack: 10, MagicAttack: 11, PhysicalDefense: 5, MagicDefense: 6,
		Agility: 10, Strength: 10, Constitution: 10, Spirit: 10, Perception: 10, Willpower: 10,
	}
	progress := ApplyPlayerExperience(&player, 500)
	if player.Level != 3 || player.Experience != 0 || progress.BeforeLevel != 1 || progress.AfterLevel != 3 || progress.NextRequired != 900 {
		t.Fatalf("continuous level result player=%+v progress=%+v", player, progress)
	}
	if progress.HealthGain != 48 || progress.ManaGain != 8 || progress.AttackGain != 14 || progress.DefenseGain != 10 || progress.AgilityGain != 1 {
		t.Fatalf("continuous growth=%+v", progress)
	}
	if player.Health != 128 || player.MaxHealth != 148 || player.Mana != 48 || player.MaxMana != 58 || player.PhysicalAttack != 24 || player.MagicAttack != 25 || player.PhysicalDefense != 15 || player.MagicDefense != 16 || player.Agility != 11 {
		t.Fatalf("grown attributes=%+v", player)
	}
}

func TestPlayerLevelStatsMatchesPublishedGrowth(t *testing.T) {
	stats := PlayerLevelStats(168)
	if stats.MaxHealth != 4108 || stats.PhysicalAttack != 1179 || stats.MagicAttack != 1179 || stats.PhysicalDefense != 840 || stats.MagicDefense != 840 {
		t.Fatalf("level 168 floor=%+v", stats)
	}
}

func TestApplyPlayerExperienceAddsMilestoneAttributesAndKeepsRemainder(t *testing.T) {
	player := Player{Level: 4, Experience: 1595, Health: 100, MaxHealth: 100, Mana: 50, MaxMana: 50}
	progress := ApplyPlayerExperience(&player, 10)
	if player.Level != 5 || player.Experience != 5 || progress.AfterExp != 5 {
		t.Fatalf("milestone remainder player=%+v progress=%+v", player, progress)
	}
	if progress.StrengthGain != 1 || progress.ConstitutionGain != 1 || progress.SpiritGain != 1 || progress.PerceptionGain != 0 || progress.WillpowerGain != 0 {
		t.Fatalf("milestone growth=%+v", progress)
	}
	before := player
	negative := ApplyPlayerExperience(&player, -100)
	if player != before || negative.ExperienceGain != 0 {
		t.Fatalf("negative gain changed player: before=%+v after=%+v progress=%+v", before, player, negative)
	}
}
