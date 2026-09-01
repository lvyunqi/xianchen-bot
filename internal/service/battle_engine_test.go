package service

import (
	"math/rand"
	"testing"
)

func TestResolveCombatUsesDefenseCriticalAndTurns(t *testing.T) {
	rules := defaultCombatRules()
	rules.MaxRounds = 20
	player := combatStats{Name: "甲", Health: 300, MaxHealth: 300, Mana: 50, MaxMana: 50, PhysicalAttack: 55, MagicAttack: 65, PhysicalDefense: 22, MagicDefense: 20, Agility: 30, CritRate: .3, CritDamage: 1.8, DodgeRate: .1}
	enemy := combatStats{Name: "乙", Health: 220, MaxHealth: 220, Mana: 30, MaxMana: 30, PhysicalAttack: 35, MagicAttack: 38, PhysicalDefense: 15, MagicDefense: 14, Agility: 18, CritRate: .05, CritDamage: 1.5, DodgeRate: .03}
	outcome := resolveCombat(player, enemy, rules, rand.New(rand.NewSource(7)))
	if !outcome.PlayerWon {
		t.Fatalf("expected player win, got %+v", outcome)
	}
	if outcome.Rounds < 1 || outcome.PlayerDamage < enemy.MaxHealth || len(outcome.Log) == 0 {
		t.Fatalf("incomplete combat outcome: %+v", outcome)
	}
}

func TestResolveCombatCapsDodgeAndAlwaysTerminates(t *testing.T) {
	rules := defaultCombatRules()
	rules.MaxRounds = 5
	player := combatStats{Name: "甲", Health: 100, MaxHealth: 100, PhysicalAttack: 1, MagicAttack: 1, Agility: 1, CritDamage: 1.5, DodgeRate: 1}
	enemy := combatStats{Name: "乙", Health: 100, MaxHealth: 100, PhysicalAttack: 1, MagicAttack: 1, Agility: 1, CritDamage: 1.5, DodgeRate: 1}
	outcome := resolveCombat(player, enemy, rules, rand.New(rand.NewSource(9)))
	if outcome.Rounds != rules.MaxRounds {
		t.Fatalf("rounds=%d want=%d", outcome.Rounds, rules.MaxRounds)
	}
	if !outcome.Draw && outcome.PlayerRemaining == outcome.EnemyRemaining {
		t.Fatalf("equal health must produce a draw: %+v", outcome)
	}
}
