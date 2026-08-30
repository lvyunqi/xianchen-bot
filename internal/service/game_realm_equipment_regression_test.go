package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestRealmTransitionThenUnequipPreservesPermanentStats(t *testing.T) {
	game, store := testGame(t)
	original := registerPlayer(t, game, "realm-equipment-ledger", "守真验账使")

	var current, next, following model.Realm
	if err := store.DB.First(&current, original.RealmID).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Where("sequence > ?", current.Sequence).Order("sequence").First(&next).Error; err != nil {
		t.Fatal(err)
	}
	cultivationRequired := realmStageCost(next, model.Realm{})
	if err := store.DB.Where("sequence > ?", next.Sequence).Order("sequence").First(&following).Error; err == nil {
		cultivationRequired = realmStageCost(next, following)
	}

	template := model.ArtifactTemplate{
		Code: "realm_equipment_ledger_artifact", Name: "守真五行佩", Type: "灵佩", Slot: "腰佩", Archetype: "灵佩",
		AttributeJSON: `{"attack":37,"defense":19,"health":240,"mana":90,"speed":13,"power":11}`, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{
		PlayerID: original.ID, TemplateID: template.ID, Name: template.Name, Level: 1,
		Quality: "凡品", Slot: template.Slot, Equipped: true,
	}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	equipment := game.equipmentStats(artifact)
	equipmentAttack := equipment.Attack + equipment.Power

	const (
		permanentHealth          int64 = 321
		permanentMana            int64 = 177
		permanentPhysicalAttack  int64 = 43
		permanentMagicAttack     int64 = 57
		permanentPhysicalDefense int64 = 29
		permanentMagicDefense    int64 = 35
		permanentSpeed           int64 = 17
		permanentStrength        int64 = 23
		permanentLifespan        int64 = 44
		transitionCost           int64 = 500
	)
	prepared := original
	prepared.RealmLevel = realmStageCount
	prepared.Cultivation = transitionCost + 1_000
	prepared.MaxHealth += permanentHealth + equipment.Health
	prepared.Health += permanentHealth + equipment.Health
	prepared.MaxMana += permanentMana + equipment.Mana
	prepared.Mana += permanentMana + equipment.Mana
	prepared.PhysicalAttack += permanentPhysicalAttack + equipmentAttack
	prepared.MagicAttack += permanentMagicAttack + equipmentAttack
	prepared.PhysicalDefense += permanentPhysicalDefense + equipment.Defense
	prepared.MagicDefense += permanentMagicDefense + equipment.Defense
	prepared.Agility += permanentSpeed + equipment.Speed
	prepared.Strength += permanentStrength
	prepared.DodgeRate += .07
	prepared.MaxLifespan += permanentLifespan
	prepared.Lifespan += permanentLifespan
	prepared.Title = "守真道君"
	if err := store.DB.Model(&model.Player{}).Where("id = ?", original.ID).Updates(map[string]any{
		"realm_level": prepared.RealmLevel, "cultivation": prepared.Cultivation,
		"health": prepared.Health, "max_health": prepared.MaxHealth,
		"mana": prepared.Mana, "max_mana": prepared.MaxMana,
		"physical_attack": prepared.PhysicalAttack, "magic_attack": prepared.MagicAttack,
		"physical_defense": prepared.PhysicalDefense, "magic_defense": prepared.MagicDefense,
		"agility": prepared.Agility, "strength": prepared.Strength,
		"dodge_rate": prepared.DodgeRate, "lifespan": prepared.Lifespan, "max_lifespan": prepared.MaxLifespan,
		"title": prepared.Title,
	}).Error; err != nil {
		t.Fatal(err)
	}
	talisman, err := game.itemByName("引劫玉符")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(original.ID, talisman.ID, 1); err != nil {
		t.Fatal(err)
	}

	transitioned, err := game.advanceRealmAfterTribulation(original.ID, current, next, transitionCost, cultivationRequired)
	if err != nil {
		t.Fatal(err)
	}
	if transitioned.RealmID != next.ID || transitioned.RealmLevel != 1 || transitioned.Title != prepared.Title || transitioned.Strength != prepared.Strength {
		t.Fatalf("realm transition discarded non-realm state: %+v", transitioned)
	}
	if game.itemQuantity(original.ID, talisman.ID) != 0 {
		t.Fatal("realm transition did not consume exactly one tribulation talisman")
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil || !artifact.Equipped {
		t.Fatalf("realm transition changed equipped artifact: artifact=%+v err=%v", artifact, err)
	}

	result, handled, err := game.unequipAllEquipment(&transitioned)
	if err != nil || !handled || !strings.Contains(result.Content, "已卸下1件装备") {
		t.Fatalf("unequip all: handled=%v err=%v result=%+v", handled, err, result)
	}
	latest, err := game.players.Get(original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil || artifact.Equipped {
		t.Fatalf("artifact was not unequipped atomically: artifact=%+v err=%v", artifact, err)
	}

	healthDelta := next.BaseHealth - current.BaseHealth
	manaDelta := next.BaseMana - current.BaseMana
	attackDelta := next.BaseAttack - current.BaseAttack
	defenseDelta := next.BaseDefense - current.BaseDefense
	speedDelta := next.BaseSpeed - current.BaseSpeed
	lifespanDelta := next.BaseLifespan - current.BaseLifespan
	dodgeDelta := next.BaseDodge - current.BaseDodge
	checks := map[string][2]int64{
		"health":           {latest.Health, original.Health + permanentHealth + healthDelta},
		"max_health":       {latest.MaxHealth, original.MaxHealth + permanentHealth + healthDelta},
		"mana":             {latest.Mana, original.Mana + permanentMana + manaDelta},
		"max_mana":         {latest.MaxMana, original.MaxMana + permanentMana + manaDelta},
		"physical_attack":  {latest.PhysicalAttack, original.PhysicalAttack + permanentPhysicalAttack + attackDelta},
		"magic_attack":     {latest.MagicAttack, original.MagicAttack + permanentMagicAttack + attackDelta},
		"physical_defense": {latest.PhysicalDefense, original.PhysicalDefense + permanentPhysicalDefense + defenseDelta},
		"magic_defense":    {latest.MagicDefense, original.MagicDefense + permanentMagicDefense + defenseDelta},
		"agility":          {latest.Agility, original.Agility + permanentSpeed + speedDelta},
		"strength":         {latest.Strength, original.Strength + permanentStrength},
		"lifespan":         {latest.Lifespan, original.Lifespan + permanentLifespan + lifespanDelta},
		"max_lifespan":     {latest.MaxLifespan, original.MaxLifespan + permanentLifespan + lifespanDelta},
		"cultivation":      {latest.Cultivation, prepared.Cultivation - transitionCost},
		"realm_level":      {int64(latest.RealmLevel), 1},
	}
	for name, values := range checks {
		if values[0] != values[1] {
			t.Errorf("%s after transition and unequip=%d want %d", name, values[0], values[1])
		}
	}
	wantDodge := original.DodgeRate + .07 + dodgeDelta
	if latest.DodgeRate != wantDodge {
		t.Errorf("dodge after transition=%v want %v", latest.DodgeRate, wantDodge)
	}
	if latest.Title != prepared.Title {
		t.Errorf("title changed: got %q want %q", latest.Title, prepared.Title)
	}
	if latest.CombatPower <= 1 {
		t.Errorf("combat power collapsed after unequip: %d", latest.CombatPower)
	}
}

func TestUnequipNeverDropsCoreStatsBelowRealmBase(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "unequip-realm-floor", "护基验账使")
	var realm model.Realm
	if err := store.DB.First(&realm, player.RealmID).Error; err != nil {
		t.Fatal(err)
	}
	template := model.ArtifactTemplate{
		Code: "unequip_realm_floor_artifact", Name: "护基重器", Type: "重器", Slot: "本命法器", Archetype: "重器",
		AttributeJSON: `{"attack":999,"defense":999,"health":9999,"mana":9999,"speed":999}`, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: template.Slot, Equipped: true}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"physical_attack": realm.BaseAttack, "magic_attack": realm.BaseAttack,
		"physical_defense": realm.BaseDefense, "magic_defense": realm.BaseDefense,
		"max_health": realm.BaseHealth, "health": realm.BaseHealth,
		"max_mana": realm.BaseMana, "mana": realm.BaseMana,
		"agility": realm.BaseSpeed,
	}).Error; err != nil {
		t.Fatal(err)
	}
	corrupt, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := game.unequipAllEquipment(&corrupt); err != nil {
		t.Fatal(err)
	}
	latest, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.PhysicalAttack < realm.BaseAttack || latest.MagicAttack < realm.BaseAttack ||
		latest.PhysicalDefense < realm.BaseDefense || latest.MagicDefense < realm.BaseDefense ||
		latest.MaxHealth < realm.BaseHealth || latest.MaxMana < realm.BaseMana || latest.Agility < realm.BaseSpeed {
		t.Fatalf("unequip hid an inconsistent ledger by collapsing stats: player=%+v realm=%+v", latest, realm)
	}
}

func TestSingleUnequipPreservesPermanentStatsAndRejectsRepeat(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "single-unequip", "单件验收")
	var realm model.Realm
	if err := store.DB.First(&realm, player.RealmID).Error; err != nil {
		t.Fatal(err)
	}
	template := model.ArtifactTemplate{
		Code: "single_unequip_ledger_artifact", Name: "守元照心佩", Type: "灵佩", Slot: "腰佩", Archetype: "灵佩",
		AttributeJSON: `{"attack":41,"defense":23,"health":280,"mana":120,"speed":17,"power":13}`, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: template.Slot, Equipped: true}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	stats := game.equipmentStats(artifact)
	const permanent int64 = 73
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"physical_attack":  player.PhysicalAttack + permanent + stats.Attack + stats.Power,
		"magic_attack":     player.MagicAttack + permanent + stats.Attack + stats.Power,
		"physical_defense": player.PhysicalDefense + permanent + stats.Defense,
		"magic_defense":    player.MagicDefense + permanent + stats.Defense,
		"max_health":       player.MaxHealth + permanent + stats.Health,
		"health":           player.Health + permanent + stats.Health,
		"max_mana":         player.MaxMana + permanent + stats.Mana,
		"mana":             player.Mana + permanent + stats.Mana,
		"agility":          player.Agility + permanent + stats.Speed,
	}).Error; err != nil {
		t.Fatal(err)
	}
	prepared, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.changeEquipment(&prepared, artifact.Name, false)
	if err != nil || !handled || !strings.Contains(result.Title, "卸下成功") {
		t.Fatalf("single unequip: handled=%v err=%v result=%+v", handled, err, result)
	}
	latest, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]int64{
		"physical_attack":  {latest.PhysicalAttack, player.PhysicalAttack + permanent},
		"magic_attack":     {latest.MagicAttack, player.MagicAttack + permanent},
		"physical_defense": {latest.PhysicalDefense, player.PhysicalDefense + permanent},
		"magic_defense":    {latest.MagicDefense, player.MagicDefense + permanent},
		"max_health":       {latest.MaxHealth, player.MaxHealth + permanent},
		"health":           {latest.Health, player.Health + permanent},
		"max_mana":         {latest.MaxMana, player.MaxMana + permanent},
		"mana":             {latest.Mana, player.Mana + permanent},
		"agility":          {latest.Agility, player.Agility + permanent},
	}
	for name, values := range checks {
		if values[0] != values[1] {
			t.Errorf("%s after single unequip=%d want %d", name, values[0], values[1])
		}
	}
	if latest.PhysicalAttack < realm.BaseAttack || latest.PhysicalDefense < realm.BaseDefense || latest.MaxHealth < realm.BaseHealth {
		t.Fatalf("single unequip dropped below realm floor: player=%+v realm=%+v", latest, realm)
	}

	result, handled, err = game.changeEquipment(&prepared, artifact.Name, false)
	if err != nil || !handled || !strings.Contains(result.Title, "装备不存在") {
		t.Fatalf("repeat unequip should be a no-op: handled=%v err=%v result=%+v", handled, err, result)
	}
	repeated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.PhysicalAttack != latest.PhysicalAttack || repeated.MaxHealth != latest.MaxHealth || repeated.Agility != latest.Agility {
		t.Fatalf("repeat unequip changed stats: before=%+v after=%+v", latest, repeated)
	}
}

func TestUnequipAllRemovesEquipmentAndSetBonusExactlyOnce(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "unequip-set", "套装校验")
	setBonus := `{"two":{"attack":31,"defense":17,"health":190,"mana":80,"speed":9}}`
	templates := []model.ArtifactTemplate{
		{Code: "unequip_set_ledger_a", Name: "两仪冠", Type: "冠", Slot: "冠冕", Archetype: "冠", AttributeJSON: `{"attack":13,"defense":7,"health":90}`, SetName: "两仪守真", SetBonusJSON: setBonus, Enabled: true},
		{Code: "unequip_set_ledger_b", Name: "两仪袍", Type: "道袍", Slot: "道袍", Archetype: "道袍", AttributeJSON: `{"attack":11,"defense":9,"mana":60}`, SetName: "两仪守真", SetBonusJSON: setBonus, Enabled: true},
	}
	withEquipment := player
	for index := range templates {
		if err := store.DB.Create(&templates[index]).Error; err != nil {
			t.Fatal(err)
		}
		artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: templates[index].ID, Name: templates[index].Name, Level: 1, Quality: "凡品", Slot: templates[index].Slot, Equipped: true}
		if err := store.DB.Create(&artifact).Error; err != nil {
			t.Fatal(err)
		}
		stats := game.equipmentStats(artifact)
		withEquipment.PhysicalAttack += stats.Attack + stats.Power
		withEquipment.MagicAttack += stats.Attack + stats.Power
		withEquipment.PhysicalDefense += stats.Defense
		withEquipment.MagicDefense += stats.Defense
		withEquipment.MaxHealth += stats.Health
		withEquipment.Health += stats.Health
		withEquipment.MaxMana += stats.Mana
		withEquipment.Mana += stats.Mana
		withEquipment.Agility += stats.Speed
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(equipmentPlayerStatUpdates(withEquipment)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := game.syncEquipmentSetBonuses(player.ID); err != nil {
		t.Fatal(err)
	}
	withSet, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := game.unequipAllEquipment(&withSet); err != nil {
		t.Fatal(err)
	}
	latest, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.PhysicalAttack != player.PhysicalAttack || latest.MagicAttack != player.MagicAttack ||
		latest.PhysicalDefense != player.PhysicalDefense || latest.MagicDefense != player.MagicDefense ||
		latest.MaxHealth != player.MaxHealth || latest.MaxMana != player.MaxMana || latest.Agility != player.Agility {
		t.Fatalf("unequip all did not remove equipment and set stats exactly once: got=%+v want base=%+v", latest, player)
	}
	var equipped int64
	if err := store.DB.Model(&model.PlayerArtifact{}).Where("player_id = ? AND equipped = ?", player.ID, true).Count(&equipped).Error; err != nil {
		t.Fatal(err)
	}
	if equipped != 0 {
		t.Fatalf("equipped artifacts after unequip all=%d", equipped)
	}
	if _, err := game.playerValue(player.ID, "equipment.set.applied"); err == nil {
		t.Fatal("set bonus ledger still exists after unequip all")
	}
}

func TestSingleUnequipRollsBackWhenSetLedgerCannotSync(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "unequip-rollback", "回滚校验")
	template := model.ArtifactTemplate{
		Code: "unequip_rollback_artifact", Name: "回元法冠", Type: "冠", Slot: "冠冕", Archetype: "冠",
		AttributeJSON: `{"attack":19,"defense":11,"health":130}`, SetName: "回元道装",
		SetBonusJSON: `{"two":{"health":90}}`, Enabled: true,
	}
	if err := store.DB.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.PlayerArtifact{PlayerID: player.ID, TemplateID: template.ID, Name: template.Name, Level: 1, Quality: "凡品", Slot: template.Slot, Equipped: true}
	if err := store.DB.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	stats := game.equipmentStats(artifact)
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"physical_attack":  player.PhysicalAttack + stats.Attack + stats.Power,
		"magic_attack":     player.MagicAttack + stats.Attack + stats.Power,
		"physical_defense": player.PhysicalDefense + stats.Defense,
		"magic_defense":    player.MagicDefense + stats.Defense,
		"max_health":       player.MaxHealth + stats.Health,
		"health":           player.Health + stats.Health,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "equipment.set.applied", "{invalid-json", nil); err != nil {
		t.Fatal(err)
	}
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := game.changeEquipment(&before, artifact.Name, false); err == nil {
		t.Fatal("single unequip unexpectedly succeeded with an invalid set ledger")
	}
	if err := store.DB.First(&artifact, artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !artifact.Equipped {
		t.Fatal("artifact state committed even though set ledger synchronization failed")
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.PhysicalAttack != before.PhysicalAttack || after.MagicAttack != before.MagicAttack ||
		after.PhysicalDefense != before.PhysicalDefense || after.MaxHealth != before.MaxHealth || after.Health != before.Health {
		t.Fatalf("player stats changed despite transaction rollback: before=%+v after=%+v", before, after)
	}
}
