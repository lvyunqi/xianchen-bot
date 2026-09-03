package service

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type GameResult struct {
	Title            string
	Content          string
	MarkdownContent  string
	ImageURL         string
	ImageOnly        bool
	Actions          []string
	BroadcastContent string
}

func (r GameResult) Markdown() string {
	if r.ImageOnly {
		return ""
	}
	var output strings.Builder
	output.WriteString("## ")
	output.WriteString(strings.TrimSpace(r.Title))
	output.WriteString("\n")
	if strings.TrimSpace(r.ImageURL) != "" {
		output.WriteString("![#250px #100px](")
		output.WriteString(strings.TrimSpace(r.ImageURL))
		output.WriteString(")\n")
	}
	content := strings.TrimSpace(r.MarkdownContent)
	if content == "" {
		content = strings.TrimSpace(r.Content)
		menuCommands := []string{"功能菜单"}
		seenCategories := make(map[string]struct{})
		for _, spec := range handler.CommandTable {
			if _, exists := seenCategories[spec.Category]; exists {
				continue
			}
			seenCategories[spec.Category] = struct{}{}
			menuCommands = append(menuCommands, spec.Category+"菜单")
		}
		for _, command := range menuCommands {
			content = strings.ReplaceAll(content, command, markdownInlineCommand(command))
		}
	}
	output.WriteString(content)
	if len(r.Actions) > 0 {
		output.WriteString("\n━━━━━━━━━━━")
		for _, action := range r.Actions {
			command := strings.TrimSpace(action)
			if command == "" {
				continue
			}
			output.WriteString("\n")
			output.WriteString(markdownInlineCommand(command))
		}
	}
	return output.String()
}

func markdownInlineCommand(label string, command ...string) string {
	target := label
	if len(command) > 0 && strings.TrimSpace(command[0]) != "" {
		target = command[0]
	}
	escaped := strings.ReplaceAll(url.QueryEscape(target), "+", "%20")
	return "[" + label + "](mqqapi://aio/inlinecmd?command=" + escaped + "&enter=false&reply=false)"
}

func (r GameResult) Text() string {
	if r.ImageOnly {
		return ""
	}
	var output strings.Builder
	output.WriteString("【")
	output.WriteString(strings.TrimSpace(r.Title))
	output.WriteString("】\n")
	content := strings.NewReplacer("**", "", "### ", "", "## ", "", "`", "").Replace(strings.TrimSpace(r.Content))
	output.WriteString(content)
	if len(r.Actions) > 0 {
		output.WriteString("\n━━━━━━━━━━━\n可用指令：")
		output.WriteString(strings.Join(r.Actions, " ｜ "))
	}
	return output.String()
}

type Game struct {
	store     *storage.Store
	players   *storage.PlayerRepository
	counters  PlayerCounters
	social    *storage.SocialRepository
	shop      *storage.ShopRepository
	logs      *storage.LogRepository
	startedAt time.Time
}

func NewGame(store *storage.Store) (*Game, error) {
	if err := SeedPlayerCommandMenus(store); err != nil {
		return nil, err
	}
	players := storage.NewPlayerRepository(store.DB)
	game := &Game{store: store, players: players, counters: players, social: storage.NewSocialRepository(store.DB), shop: storage.NewShopRepository(store.DB), logs: storage.NewLogRepository(store.DB), startedAt: time.Now()}
	if err := game.repairMigratedArtifactSlots(); err != nil {
		return nil, err
	}
	if err := game.repairPlayerLevelFloors(); err != nil {
		return nil, err
	}
	game.syncPendingMigrationCombatPower()
	return game, nil
}

// Execute stays silent for unregistered users. Only 入道 is available before
// registration, as required by the QQ bot interaction rules.
func (g *Game) Execute(groupID, accountID string, command handler.ParsedCommand) (GameResult, bool, error) {
	player, err := g.players.GetByAccount(accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if command.Spec.ID == 1190 {
			return g.importAccountMigration(groupID, accountID, command.RawArguments)
		}
		if command.Spec.ID == 1189 {
			return g.migrationGuide(nil)
		}
		if command.Spec.ID == 1191 {
			if command.Spec.Command == "群审核状态" {
				return g.groupAccessStatus(groupID)
			}
			return g.submitGroupAccess(nil, groupID, accountID, command.RawArguments)
		}
		if command.Spec.ID != 1 {
			return GameResult{}, false, nil
		}
		return g.register(groupID, accountID, command)
	}
	if err != nil {
		return GameResult{}, false, err
	}
	if err := g.ensurePlayerLevelFloor(&player); err != nil {
		return GameResult{}, false, err
	}
	if player.Banned {
		return GameResult{}, false, nil
	}
	if command.Spec.ID != 2 && command.Spec.ID != 1082 && command.Spec.ID != 1083 && command.Spec.ID != 1084 && command.Spec.ID != 1189 && command.Spec.ID != 1190 && command.Spec.ID != 1191 {
		invalid := validateDaoName(player.DaoName)
		critical := criticalSensitiveTerm(normalizeModerationText(player.DaoName))
		if invalid != "" || critical != "" {
			reason := invalid
			if critical != "" {
				reason = "当前道号命中高风险违规词。"
			}
			return GameResult{Title: "道号复核未通过", Content: fmt.Sprintf("当前道号：%s\n原因：%s\n━━━━━━━━━━━\n该道号来自旧版宽松审核，现已限制继续游戏。请发送 `改名 新道号` 免费更换；也可以发送 `申请删号` 永久删除道籍。", player.DaoName, reason), Actions: []string{"改名 新道号", "申请删号"}}, true, nil
		}
	}
	returningOnline := player.LastLoginAt == nil || time.Since(*player.LastLoginAt) >= 30*time.Minute
	now := time.Now()
	_ = g.store.DB.Model(&player).Updates(map[string]any{"online": true, "last_login_at": &now}).Error
	if command.Spec.ID == 1 {
		return GameResult{Title: "道籍已存", Content: "你已入道，无需重复建立道籍。", Actions: []string{"状态", "日常"}}, true, nil
	}
	if command.Spec.ID == 1191 {
		if command.Spec.Command == "群审核状态" {
			return g.groupAccessStatus(groupID)
		}
		return g.submitGroupAccess(&player, groupID, accountID, command.RawArguments)
	}
	beforeCommand := player
	result, handled, err := g.executeRegistered(&player, command)
	if handled && err == nil {
		if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
			if cultivationGain := latest.Cultivation - beforeCommand.Cultivation; cultivationGain > 0 {
				progress, experienceErr := g.applyCultivationExperience(player.ID, cultivationGain)
				if experienceErr != nil {
					return GameResult{}, true, experienceErr
				}
				if refreshed, refreshErr := g.players.Get(player.ID); refreshErr == nil {
					latest = refreshed
				}
				appendPlayerLevelSettlement(&result, latest, progress)
			}
			if combatPowerInputsChanged(beforeCommand, latest) || g.playerLevelPowerSyncPending(latest.ID) {
				if syncErr := g.syncPlayerCombatPower(&latest); syncErr == nil {
					g.clearPlayerLevelPowerSync(latest.ID)
				}
			}
			player = latest
		}
	}
	if handled && err == nil && returningOnline && g.isCombatPowerChampion(player.ID) {
		broadcast := fmt.Sprintf("【天骄临世】战力之王%s%s已踏入仙尘，当前境界%s·%d层，战力%d。", displayOr(player.Title, "【无上天骄】"), player.DaoName, player.RealmName, player.RealmLevel, player.CombatPower)
		_ = g.publishWorldBroadcast("上线", player.DaoName+"降临仙尘", broadcast)
		if strings.TrimSpace(result.BroadcastContent) == "" {
			result.BroadcastContent = broadcast
		} else {
			result.BroadcastContent += "\n" + broadcast
		}
	}
	if handled && err == nil && !result.ImageOnly && len(result.Actions) == 0 && command.Spec.ID != 1000 {
		if command.Spec.Category != "系统" && strings.TrimSpace(command.Spec.Category) != "" {
			result.Actions = []string{command.Spec.Category + "菜单", "状态"}
		} else {
			result.Actions = []string{"系统", "状态"}
		}
	}
	if handled && err == nil && !result.ImageOnly && command.Spec.ID != 1000 && command.Spec.Input != "" && !strings.Contains(result.Content, "操作指引") && !strings.Contains(result.Content, "请输入") {
		guide := "\n\n操作指引：发送 `" + command.Spec.Input + "`。"
		result.Content += guide
		if result.MarkdownContent != "" {
			result.MarkdownContent += guide
		}
	}
	if handled && err == nil && !result.ImageOnly && command.Spec.ID != 1067 {
		result.Title = decorateGameTitle(command.Spec.Category, result.Title)
	}
	if handled && err == nil && !result.ImageOnly && !singlePanelCommand(command.Spec) {
		result, err = g.paginateGameResult(&player, result)
	}
	return result, handled, err
}

func singlePanelCommand(spec handler.CommandSpec) bool {
	if strings.Contains(spec.Name, "排行") || strings.HasSuffix(spec.Name, "榜") {
		return true
	}
	switch spec.ID {
	case 3, 15, 53, 54, 55, 98, 113, 122, 123, 130, 143, 149, 154, 159, 164, 167, 172, 181, 183, 189, 196, 206, 214, 215, 219, 230, 233, 241, 246, 247, 252, 253, 254, 255, 256, 257,
		1000, 1007, 1010, 1011, 1012, 1021, 1028, 1035, 1036, 1038, 1039, 1040, 1042, 1043, 1044,
		1048, 1056, 1061, 1063, 1066, 1067, 1075, 1076, 1078, 1079, 1085,
		1086, 1087, 1088, 1089, 1090, 1091, 1092, 1093, 1094, 1095, 1096, 1097, 1098, 1099, 1100, 1101, 1102, 1103,
		1104, 1105, 1106, 1107, 1110, 1111, 1112, 1113, 1115, 1116, 1117, 1118, 1119, 1194, 1198, 1218:
		return true
	case 1120, 1121, 1122, 1144, 1145, 1151, 1154, 1163, 1182:
		return true
	default:
		return false
	}
}

func (g *Game) isCombatPowerChampion(playerID uint) bool {
	var player model.Player
	if g.store.DB.Select("id", "combat_power").First(&player, playerID).Error != nil {
		return false
	}
	stronger, _ := g.counters.CountStrongerThan(player)
	return stronger == 0
}

func (g *Game) currencyWallet(player *model.Player) GameResult {
	var contribution int64
	_ = g.store.DB.Model(&model.SectMember{}).Select("contribution").Where("player_id = ?", player.ID).Scan(&contribution).Error
	return GameResult{Title: "仙尘钱庄", Content: fmt.Sprintf("道号：%s\n━━━━━━━━━━━\n灵石：%d · 修炼世界通用流通货币\n银币：%d · 签到、任务与活动免费获得\n仙金：%d · 充值或主人管理发放\n功德：%d · 行善、首领与渡劫所得\n宗门贡献：%d · 宗务与宗战所得\n竞技币：%d · 问剑竞技所得\n━━━━━━━━━━━\n银币与仙金相互独立，商城不会混扣；竞技积分只决定段位，不再充当消费货币。", player.DaoName, player.SpiritStones, player.SilverCoins, player.ImmortalJade, player.Merit, contribution, player.ArenaCoins), Actions: []string{"银币来源", "银币商城", "仙金商城", "竞技商店", "签到", "状态"}}
}

func (g *Game) register(groupID, accountID string, command handler.ParsedCommand) (GameResult, bool, error) {
	if len(command.Arguments) == 0 || strings.TrimSpace(command.RawArguments) == "" {
		return GameResult{Title: "入道指引", Content: "请输入：`入道 道号 男/女`。\n性别可在入道后发送“性别 男”或“性别 女”补录。", Actions: []string{"帮助 角色"}}, true, nil
	}
	gender := unsetPlayerGender
	daoNameArguments := append([]string(nil), command.Arguments...)
	if len(daoNameArguments) > 1 {
		if parsedGender := normalizePlayerGender(daoNameArguments[len(daoNameArguments)-1]); parsedGender != "" {
			gender = parsedGender
			daoNameArguments = daoNameArguments[:len(daoNameArguments)-1]
		}
	}
	daoName := strings.TrimSpace(strings.Join(daoNameArguments, " "))
	if invalid := validateDaoName(daoName); invalid != "" {
		return GameResult{Title: "道号格式审核未通过", Content: invalid + "\n请使用清晰、可读且符合修仙世界设定的全服唯一道号。", Actions: []string{"帮助"}}, true, nil
	}
	if _, _, matched, err := g.matchSensitiveWord(daoName); err != nil {
		return GameResult{}, true, err
	} else if matched {
		return GameResult{Title: "道号审核未通过", Content: "该道号触发仙盟禁用词，请更换合规道号后重新入道。", Actions: []string{"帮助"}}, true, nil
	}
	existingNames, err := g.counters.CountByDaoName(daoName, 0)
	if err != nil {
		return GameResult{}, true, err
	} else if existingNames > 0 {
		return GameResult{Title: "道号已被占用", Content: "“" + daoName + "”已经记入他人道籍，请更换一个全服唯一道号。"}, true, nil
	}
	var firstRealm model.Realm
	if err := g.store.DB.Order("sequence").First(&firstRealm).Error; err != nil {
		return GameResult{}, true, err
	}
	nextRequired := realmStageCost(firstRealm, model.Realm{})
	var nextRealm model.Realm
	if err := g.store.DB.Where("sequence > ?", firstRealm.Sequence).Order("sequence").First(&nextRealm).Error; err == nil {
		nextRequired = realmStageCost(firstRealm, nextRealm)
	}
	now := time.Now()
	rootTemplate, rootErr := g.randomSpiritualRoot()
	if rootErr != nil {
		return GameResult{}, true, rootErr
	}
	root := rootTemplate.Name
	rootQuality := rootTemplate.BaseQuality + int(now.UnixNano()%6)
	if rootQuality > 100 {
		rootQuality = 100
	}
	player := model.Player{
		AccountID:           accountID,
		DaoName:             daoName,
		Gender:              gender,
		ServerName:          "仙尘一区",
		RealmID:             firstRealm.ID,
		RealmName:           firstRealm.Name,
		RealmLevel:          1,
		CultivationRequired: nextRequired,
		SpiritualRoot:       root,
		RootQuality:         rootQuality,
		Level:               1,
		Health:              100,
		MaxHealth:           100,
		Mana:                50,
		MaxMana:             50,
		PhysicalAttack:      10,
		MagicAttack:         10,
		PhysicalDefense:     5,
		MagicDefense:        5,
		Agility:             10,
		Strength:            10,
		Constitution:        10,
		Spirit:              10,
		Perception:          10,
		Willpower:           10,
		Luck:                initialPlayerLuck,
		CritRate:            0.05,
		CritDamage:          1.5,
		DodgeRate:           0.05,
		Lifespan:            firstRealm.BaseLifespan,
		MaxLifespan:         firstRealm.BaseLifespan,
		SpiritStones:        100,
		DaoHeart:            50,
		ImmortalAffinity:    10,
		Location:            "青云山脚",
		State:               model.PlayerStateIdle,
		Online:              true,
		LastLoginAt:         &now,
		DailyTaskDate:       now.Format("2006-01-02"),
		CreatedAt:           now,
	}
	g.applyInitialSpiritualRootBonus(&player)
	player.CombatPower = calculateCombatPower(player)
	if err := g.store.DB.Create(&player).Error; err != nil {
		return GameResult{}, true, err
	}
	var starterPack model.Item
	if err := g.store.DB.Where("code = ?", "gift_starter_qingyun").First(&starterPack).Error; err == nil {
		_ = g.players.AdjustItem(player.ID, starterPack.ID, 1)
	}
	_ = g.queueContentReview("道号", &player, player.DaoName)
	return GameResult{
		Title:   "入道成功",
		Content: fmt.Sprintf("**%s** 的道籍已录入。\n\n性别：%s\n灵根：%s · %s · 纯度%d\n灵根主加成：%s\n灵根副加成：%s\n修炼倍率：×%.3f\n境界：%s · 一层\n运气：%d/%d\n气血：%d/%d\n法力：%d/%d\n战力：%d", player.DaoName, displayPlayerGender(player.Gender), player.SpiritualRoot, rootTemplate.Grade, player.RootQuality, rootTemplate.PrimaryBonus, rootTemplate.SecondaryBonus, g.spiritualRootBonuses(player.SpiritualRoot, player.RootQuality).Cultivation, player.RealmName, player.Luck, maximumPlayerLuck, player.Health, player.MaxHealth, player.Mana, player.MaxMana, player.CombatPower),
		Actions: []string{"性别", "礼包", "开启礼包 青云入道礼匣", "状态", "帮助"},
	}, true, nil
}

func (g *Game) executeRegistered(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 2:
		return g.rename(player, command)
	case 3:
		result, err := g.status(player)
		return result, true, err
	case 4:
		return GameResult{Title: "星盘占卜", Content: fmt.Sprintf("灵根：%s\n%s\n今日卦象：风雷益，利有攸往。", player.SpiritualRoot, luckEffectSummary(player.Luck)), Actions: []string{"探索", "抽签", "仙缘"}}, true, nil
	case 5:
		return g.rebirth(player)
	case 6:
		return g.inventory(player)
	case 7:
		return GameResult{Title: "寿元", Content: fmt.Sprintf("道龄：%d年\n寿元：%d/%d年\n当前境界：%s", player.Age, player.Lifespan, player.MaxLifespan, player.RealmName)}, true, nil
	case 8:
		return GameResult{Title: "修仙日历", Content: fmt.Sprintf("入道时间：%s\n已修行：%d天", player.CreatedAt.Format("2006-01-02 15:04"), int(time.Since(player.CreatedAt).Hours()/24))}, true, nil
	case 9:
		return GameResult{Title: "道心", Content: fmt.Sprintf("道心：%d/100\n%s", player.DaoHeart, daoHeartText(player.DaoHeart))}, true, nil
	case 10:
		return GameResult{Title: "仙缘与运气", Content: fmt.Sprintf("仙缘：%d\n仙缘影响奇遇前置、结缘与部分奖励。\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n成功承接仙缘奇遇时，会按当前天缘概率永久增加1点运气。", player.ImmortalAffinity, luckEffectSummary(player.Luck)), Actions: []string{"仙遇", "探索", "抽签", "状态"}}, true, nil
	case 11:
		return GameResult{Title: "道侣数据管理", Content: "此项为主人数据操作。请在插件设置中打开仙侣数据页面，可强制结缘、解缘或修改道缘深度。"}, true, nil
	case 12:
		return g.archive(player), true, nil
	case 13, 14, 15, 16, 17, 18, 19, 20, 21, 22:
		return g.executeCultivation(player, command)
	case 23, 24, 25, 26, 27, 28, 29, 30, 31, 32:
		return g.executeExplore(player, command)
	case 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46:
		return g.executeCouple(player, command)
	case 47, 48, 49, 50, 51, 52:
		return g.executeBattle(player, command)
	case 53, 54, 55, 58:
		return g.executeTribulation(player, command)
	case 59, 60, 61, 62, 63, 64, 65, 66, 67, 68:
		return g.executeMansion(player, command)
	case 69, 70, 71, 72, 73, 74:
		return g.executeSkill(player, command)
	case 75, 76, 77, 78, 79, 80:
		return g.executePet(player, command)
	case 81, 82, 83, 84, 85, 86:
		return g.executeSocial(player, command)
	case 87, 88, 89, 90:
		return g.executeTrade(player, command)
	case 91, 92, 93, 94, 95, 96:
		return g.executeTask(player, command)
	case 97, 98, 99, 100:
		return g.executeSpecial(player, command)
	case 101, 102, 103, 104, 105, 106, 107, 108:
		return g.executeSect(player, command)
	case 109, 110, 111, 112, 113, 114:
		return g.executeAlchemy(player, command)
	case 115, 116, 117, 118, 119, 120:
		return g.executeArtifact(player, command)
	case 121, 122, 123, 124, 125, 126:
		return g.executeDungeon(player, command)
	case 127, 128, 129, 130:
		return g.executeArena(player, command)
	case 131, 132, 133, 134, 135, 136:
		return g.executeEncounter(player, command)
	case 137, 138, 139, 140:
		return g.executeCareer(player, command)
	case 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158,
		159, 160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175, 176,
		177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194,
		195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212,
		213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230,
		231, 232, 233, 234, 235, 236, 237, 238, 239, 240:
		return g.executeExtended(player, command)
	case 241, 242, 244, 245, 252:
		return g.executeMap(player, command)
	case 243:
		return g.systemOverview(player)
	case 246, 247:
		return g.executeAFK(player, command)
	case 248, 249, 250, 251:
		return g.executeInventoryCommand(player, command)
	case 253, 254, 255, 256:
		return g.executePVPTurn(player, command)
	case 257:
		return g.helpGuide(player, command.RawArguments)
	case 1002:
		return g.checkIn(player)
	case 1003:
		return g.checkInRecord(player)
	case 1007:
		if command.Spec.Command == "商城" {
			return g.shopHub(player), true, nil
		}
		return g.shopList(player, command.RawArguments, false)
	case 1008:
		return g.buyMarketByName(player, command.RawArguments)
	case 1009:
		return g.buyShopCommand(player, command.Arguments, false)
	case 1010:
		return g.shopList(player, command.RawArguments, true)
	case 1011:
		return g.buyShopCommand(player, command.Arguments, true)
	case 1012:
		return g.giftPackList(player, command.RawArguments)
	case 1013:
		return g.openGiftPack(player, command.RawArguments)
	case 1014:
		if command.Spec.Command == "装备系统" || command.Spec.Command == "装备菜单" {
			return g.equipmentMenu(player)
		}
		return g.equipmentOverview(player)
	case 1015:
		return g.equipmentBag(player, command.RawArguments)
	case 1016:
		return g.changeEquipment(player, command.RawArguments, true)
	case 1017:
		return g.changeEquipment(player, command.RawArguments, false)
	case 1018:
		return g.unequipAllEquipment(player)
	case 1019:
		return g.forgeEquipment(player, command.RawArguments)
	case 1020:
		return g.inscribeEquipment(player, command.RawArguments)
	case 1021:
		return g.farmOverview(player, command.RawArguments)
	case 1022:
		return g.plantCrop(player, command.RawArguments)
	case 1023:
		return g.plantAllAvailable(player, command.RawArguments)
	case 1024:
		return g.harvestFarmPlot(player, command.RawArguments)
	case 1025, 1026, 1027:
		return g.tendFarmPlot(player, command.Spec.Command, command.RawArguments)
	case 1028:
		return g.farmWarehouse(player, command.RawArguments)
	case 1029:
		return g.sellFarmProduce(player, command.Arguments)
	case 1036:
		return g.rankingCenter(player, command.RawArguments)
	case 1037:
		return g.claimRankingReward(player, command.RawArguments)
	case 1038:
		return g.noticeBoard(command.RawArguments, "")
	case 1039:
		return g.noticeBoard(command.RawArguments, "更新")
	case 1040:
		return g.noticeBoard(command.RawArguments, "全区通报")
	case 1041:
		return g.resolveEventChoice(player, command.RawArguments)
	case 1042:
		if command.Spec.Command == "钱庄" {
			return g.bankOverview(player)
		}
		if command.Spec.Command == "银币来源" {
			return g.silverIncomeGuide(player)
		}
		if command.Spec.Command == "赚银币" {
			return g.earnSilverCoins(player)
		}
		return g.currencyWallet(player), true, nil
	case 1043:
		return g.currencyShopList(player, command.RawArguments, "银币")
	case 1044:
		return g.currencyShopList(player, command.RawArguments, "仙金")
	case 1045:
		return g.buyCurrencyShop(player, command.Arguments, "银币")
	case 1046:
		return g.buyCurrencyShop(player, command.Arguments, "仙金")
	case 1047:
		return g.arenaProfile(player)
	case 1048:
		return g.arenaTierInfo(player, command.RawArguments)
	case 1049:
		return g.claimArenaDailyReward(player)
	case 1050:
		return g.arenaGuide(player), true, nil
	case 1051:
		return g.shortcutList(player)
	case 1052:
		return g.setShortcut(player, command.RawArguments)
	case 1053:
		return g.deleteShortcut(player, command.RawArguments)
	case 1054:
		return g.requestDaoNameTransfer(player, command.Arguments)
	case 1055:
		return g.acceptDaoNameTransfer(player)
	case 1056:
		return g.worldLeylineMap(player, command.RawArguments)
	case 1057:
		return g.worldLeylineDetails(player, command.RawArguments)
	case 1058:
		return g.startLeylineMeditation(player, command.RawArguments)
	case 1059:
		return g.finishLeylineMeditation(player)
	case 1060:
		return g.gatherLeylineAura(player, command.RawArguments)
	case 1061:
		return g.leylineCultivationRanking(player, command.RawArguments)
	case 1062:
		return g.discoverLocalLeylines(player)
	case 1063:
		return g.spiritualRootCatalog(player, command.RawArguments)
	case 1064:
		return g.spiritualRootDetails(player, command.RawArguments)
	case 1065:
		return g.talkToLocalNPC(player, command.RawArguments)
	case 1066:
		return g.startMapMonsterBattle(player, command.RawArguments)
	case 1067:
		return g.cachedResultPage(player, command.RawArguments)
	case 1068:
		return g.rechargePriceTable(player, command.RawArguments), true, nil
	case 1069:
		return g.customizationMenu(player), true, nil
	case 1070:
		return g.customizeSpiritualRoot(player, command.RawArguments)
	case 1071:
		return g.customizeTitle(player, command.RawArguments)
	case 1072:
		return g.customizeMansion(player, command.RawArguments)
	case 1073:
		return g.customizePet(player, command.RawArguments)
	case 1074:
		return g.customizeArtifact(player, command.RawArguments)
	case 1075:
		return g.synthesisMenu(player), true, nil
	case 1076:
		return g.synthesisCatalog(player, command.RawArguments)
	case 1077:
		return g.synthesizeItem(player, command.RawArguments)
	case 1078:
		return g.synthesisRecord(player), true, nil
	case 1079:
		raw := command.RawArguments
		if command.Spec.Command == "指令大全" {
			raw = strings.TrimSpace("全部 " + raw)
		}
		return g.helpGuide(player, raw)
	case 1080:
		return g.upgradeFarm(player)
	case 1081:
		return g.resurrectPlayer(player)
	case 1082:
		return g.requestSelfDelete(player)
	case 1083:
		return g.confirmSelfDelete(player, command.RawArguments)
	case 1084:
		return g.cancelSelfDelete(player)
	case 1085:
		return g.synthesisRecipeDetails(player, command.RawArguments)
	case 1086, 1087, 1088, 1089, 1090, 1091, 1092, 1093, 1094, 1095, 1096, 1097, 1098, 1099, 1100, 1101, 1102, 1103:
		return g.executeActivity(player, command)
	case 1104:
		return g.fuseSpiritualRoots(player, command.Arguments)
	case 1105:
		return g.pendingSpiritualRoot(player)
	case 1106:
		return g.absorbFusedSpiritualRoot(player)
	case 1107:
		return g.discardFusedSpiritualRoot(player)
	case 1108:
		return g.playerGender(player, command.RawArguments)
	case 1109:
		return GameResult{Title: "运气属性", Content: luckEffectSummary(player.Luck) + "\n━━━━━━━━━━━\n成功承接仙缘奇遇时有概率永久+1；普通寻宝、抽签和失败事件不会扣除永久运气。", Actions: []string{"仙遇", "探索", "寻宝", "捕获", "丹方", "合成图鉴", "状态"}}, true, nil
	case 1110, 1111, 1112, 1113:
		return g.executeFeedback(player, command)
	case 1114:
		return g.xianchenIntroduction(player, command.Spec.Command), true, nil
	case 1115:
		return g.staminaOverview(player)
	case 1116:
		return g.noticeBoard(command.RawArguments, "修复")
	case 1117:
		return g.notificationInbox(player, command.RawArguments, false)
	case 1118:
		return g.notificationInbox(player, command.RawArguments, true)
	case 1119:
		return g.clearReadNotifications(player)
	case 1120, 1121, 1122:
		return g.executeCatalog(player, command)
	case 1140:
		return g.bankDeposit(player, command.RawArguments)
	case 1141:
		return g.bankWithdraw(player, command.RawArguments)
	case 1142:
		return g.bankBorrow(player, command.RawArguments)
	case 1143:
		return g.bankRepay(player, command.RawArguments)
	case 1144:
		return g.bankLedger(player, command.RawArguments)
	case 1145:
		return g.bankRules(player), true, nil
	case 1150, 1151, 1152, 1153, 1154, 1155, 1156, 1157, 1158, 1159, 1160, 1161, 1162, 1163, 1164, 1165, 1166, 1167, 1168:
		return g.executeEquipmentExtended(player, command)
	case 1169:
		return g.sellItemToAlliance(player, command.RawArguments)
	case 1170:
		return g.transferCurrency(player, command.RawArguments)
	case 1171:
		return g.myTitles(player, command.RawArguments)
	case 1172:
		return g.unlockTitle(player, command.RawArguments)
	case 1173:
		return g.wearTitle(player, command.RawArguments)
	case 1174:
		return g.removeTitle(player)
	case 1175:
		return g.npcShop(player, command.RawArguments)
	case 1176:
		return g.giftLocalNPC(player, command.RawArguments)
	case 1177:
		return g.buyFromLocalNPC(player, command.RawArguments)
	case 1178:
		return g.localNPCRelationship(player, command.RawArguments)
	case 1179:
		return g.farmWeatherOverview(player)
	case 1180:
		return g.protectFarmCrops(player, command.RawArguments)
	case 1181:
		return g.farmDisasterLog(player, command.RawArguments)
	case 1182:
		return g.runtimeOverview()
	case 1183:
		return g.acceptBarter(player, command.RawArguments)
	case 1184:
		return g.rejectBarter(player, command.RawArguments)
	case 1185:
		return g.barterRequests(player, command.RawArguments)
	case 1186:
		return g.teleportHub(player)
	case 1187:
		if command.Spec.Command == "诸界列表" || command.Spec.Command == "世界列表" {
			return g.worldRegionList(player)
		}
		return g.teleportList(player, command.RawArguments)
	case 1188:
		return g.teleportTo(player, command.RawArguments, command.Spec.Command == "跨界传送")
	case 1189:
		if command.Spec.Command == "生成迁移码" {
			return g.createAccountMigrationCode(player)
		}
		return g.migrationGuide(player)
	case 1190:
		return g.importAccountMigration("", player.AccountID, command.RawArguments)
	case 1192:
		return g.fertilizeFarmPlot(player, command.RawArguments)
	case 1193:
		return g.fertilizeAllFarmPlots(player, command.RawArguments)
	case 1194:
		return g.fertilizerCatalog(player, command.RawArguments)
	case 1195:
		return g.setSkillPublication(player, command.RawArguments, command.Spec.Command == "上传功法")
	case 1196:
		return g.sharedSkillLibrary(player, command.RawArguments)
	case 1197:
		return g.createdSkills(player, command.RawArguments)
	case 1198:
		return g.playerLevelOverview(player)
	case 1199, 1200:
		return g.tendAllFarmPlots(player, command.Spec.Command)
	case 1201:
		return g.cumulativeRecharge(player), true, nil
	case 1202:
		return g.rotatingShopList(player, mysteryShopConfig.Code)
	case 1203:
		return g.buyRotatingShop(player, command.Arguments, mysteryShopConfig.Code)
	case 1204:
		return g.rotatingShopList(player, limitedShopConfig.Code)
	case 1205:
		return g.buyRotatingShop(player, command.Arguments, limitedShopConfig.Code)
	case 1206:
		return g.birthdayMenu(player)
	case 1207:
		return g.registerBirthday(player, command.RawArguments)
	case 1208:
		return g.birthdayCheckin(player)
	case 1209:
		return g.claimBirthdayGift(player)
	case 1210:
		return g.todayBirthdayList(player)
	case 1211:
		return g.birthdayBlessing(player, command.RawArguments)
	case 1212:
		return g.birthdayPresent(player, command.RawArguments)
	case 1213:
		return g.birthdayLottery(player, command.RawArguments)
	case 1214:
		return g.birthdayExchange(player, command.RawArguments)
	case 1215:
		return g.birthdayTaskList(player)
	case 1216:
		return g.claimBirthdayTask(player, command.RawArguments)
	case 1217:
		return g.birthdayRanking(player, command.RawArguments)
	case 1218:
		if command.Spec.Command == "领取全服补偿" {
			return g.claimV221ServerCompensation(player)
		}
		if command.Spec.Command == "补偿公告" {
			return g.v221CompensationNotice()
		}
		return g.v221ServerCompensation(player), true, nil
	case 1030:
		return g.sellAllFarmProduce(player)
	case 1031:
		return g.gatherFromFriendFarm(player, command.RawArguments)
	case 1032:
		return g.farmPlotDetails(player, command.RawArguments)
	case 1033:
		return g.farmGuide(player), true, nil
	case 1034:
		return g.farmGuardLog(player)
	case 1035:
		return g.farmRanking(player)
	case 1000:
		return g.gameMenu(player, command)
	case 1001:
		return g.managementMenu(player)
	default:
		return GameResult{}, false, fmt.Errorf("命令%d没有业务路由", command.Spec.ID)
	}
}

func (g *Game) gameMenu(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	var menus []model.AdminMenu
	err := g.store.DB.Where("menu_type IN ? AND permission IN ? AND is_hidden = ? AND status = ?", []string{"top", "both"}, []string{"player", "admin"}, false, "active").Order("parent_id, sort_order, id").Find(&menus).Error
	if err != nil {
		return GameResult{}, true, err
	}
	requested, menuPage := parseMenuPage(command.RawArguments)
	if requested == "管理" {
		return g.managementMenu(player)
	}
	_, _, canManage := g.gmAuthority(player.AccountID)
	if !canManage {
		filtered := menus[:0]
		for _, menu := range menus {
			if menu.Permission != "admin" {
				filtered = append(filtered, menu)
			}
		}
		menus = filtered
	}
	if requested == "" {
		lines := []string{g.menuCover(), "━━━━━━━━━━━"}
		markdownLines := []string{g.menuCover(), "━━━━━━━━━━━"}
		var topMenus []model.AdminMenu
		var managementEntry *model.AdminMenu
		for _, menu := range menus {
			if menu.ParentID != 0 {
				continue
			}
			if strings.TrimPrefix(menu.Component, "GameMenuCategory:") == "管理" {
				copy := menu
				managementEntry = &copy
				continue
			}
			topMenus = append(topMenus, menu)
		}
		if len(topMenus) == 0 {
			return GameResult{Title: "仙途菜单", Content: "仙途菜单尚未载入，请主人在插件设置的菜单管理中启用玩家顶部菜单。"}, true, nil
		}
		byCategory := make(map[string]model.AdminMenu, len(topMenus))
		for _, menu := range topMenus {
			category := strings.TrimPrefix(menu.Component, "GameMenuCategory:")
			if category == menu.Component || category == "" {
				category = menu.Label
			}
			byCategory[category] = menu
		}
		sections := []struct {
			title      string
			categories []string
		}{
			{"🧘 修行根基", []string{"角色", "修炼", "功法", "灵根进化", "悟道", "渡劫", "渡劫心魔", "生涯"}},
			{"🗺️ 诸界游历", []string{"地图", "探索", "天地灵脉", "奇遇", "仙缘奇遇", "副本", "秘境争夺", "宇宙星河"}},
			{"⚔️ 斗法养成", []string{"战斗", "竞技", "装备", "炼器", "丹药", "灵兽", "阵法", "符箓", "傀儡", "法宝炼化"}},
			{"🏯 宗门社交", []string{"道侣", "合体技", "社交", "宗门", "宗门战争", "仙魔战场", "传承"}},
			{"🌿 仙府经营", []string{"仙府", "仙药培育", "交易", "挂机", "任务", "活动", "特殊"}},
			{"🪪 万象资料", []string{"图鉴", "天机推演", "系统"}},
		}
		used := map[string]bool{}
		for _, section := range sections {
			plainLinks, richLinks := []string{}, []string{}
			for _, category := range section.categories {
				menu, exists := byCategory[category]
				if !exists {
					continue
				}
				used[category] = true
				label := mainMenuIcon(category) + menu.Label + "菜单"
				commandName := category + "菜单"
				if category == "图鉴" {
					commandName = "图鉴菜单"
				}
				plainLinks = append(plainLinks, label)
				richLinks = append(richLinks, markdownInlineCommand(label, commandName))
			}
			if len(plainLinks) == 0 {
				continue
			}
			lines = append(lines, section.title)
			markdownLines = append(markdownLines, section.title)
			lines = append(lines, pairMenuLines(plainLinks)...)
			lines = append(lines, "━━━━━━━━━━━")
			markdownLines = append(markdownLines, pairMenuLines(richLinks)...)
			markdownLines = append(markdownLines, "━━━━━━━━━━━")
		}
		if _, exists := byCategory["氪金"]; exists {
			used["氪金"] = true
			lines = append(lines, "💎 仙缘珍阁", "💎 氪金菜单", "━━━━━━━━━━━")
			markdownLines = append(markdownLines, "💎 仙缘珍阁", markdownInlineCommand("💎 氪金菜单", "氪金菜单"), "━━━━━━━━━━━")
		}
		otherPlain, otherRich := []string{}, []string{}
		for category, menu := range byCategory {
			if used[category] {
				continue
			}
			label := mainMenuIcon(category) + menu.Label + "菜单"
			otherPlain = append(otherPlain, label)
			otherRich = append(otherRich, markdownInlineCommand(label, category+"菜单"))
		}
		if len(otherPlain) > 0 {
			sort.Strings(otherPlain)
			sort.Strings(otherRich)
			lines = append(lines, "✨ 其他道藏")
			markdownLines = append(markdownLines, "✨ 其他道藏")
			lines = append(lines, pairMenuLines(otherPlain)...)
			lines = append(lines, "━━━━━━━━━━━")
			markdownLines = append(markdownLines, pairMenuLines(otherRich)...)
			markdownLines = append(markdownLines, "━━━━━━━━━━━")
		}
		lines = append(lines, commandText("🎂 生辰档案", "生日"))
		markdownLines = append(markdownLines, markdownInlineCommand("🎂 生辰档案", "生日"))
		if g.isPlayerBirthdayToday(player.ID, time.Now()) {
			lines = append(lines, "🎊 生日专属菜单")
			markdownLines = append(markdownLines, markdownInlineCommand("🎊 生日专属菜单", "生日菜单"))
		}
		lines = append(lines, "━━━━━━━━━━━", "📜 仙门告示", "世界公告 ┆ 更新公告", "修复公告", "━━━━━━━━━━━")
		markdownLines = append(markdownLines, "━━━━━━━━━━━", "📜 仙门告示")
		markdownLines = append(markdownLines, pairMenuLines([]string{
			markdownInlineCommand("世界公告", "世界公告"),
			markdownInlineCommand("更新公告", "更新公告"),
			markdownInlineCommand("修复公告", "修复公告"),
		})...)
		markdownLines = append(markdownLines, "━━━━━━━━━━━")
		if managementEntry != nil {
			lines = append(lines, "🛡️ 神令系统", "━━━━━━━━━━━")
			markdownLines = append(markdownLines, "🛡️ "+markdownInlineCommand("神令系统", "神令系统"), "━━━━━━━━━━━")
		}
		countLine := fmt.Sprintf("共%d个玩家系统", len(topMenus))
		lines = append(lines, "━━━━━━━━━━━", countLine)
		markdownLines = append(markdownLines, "━━━━━━━━━━━", countLine)
		footer := []string{"发送或点击对应文字查看系统；长列表在原指令后加页码。", "发现异常可发送“提交BUG 指令、现象与期望结果”。", "🪐 世界消息 🪐", g.latestWorldNotice()}
		lines = append(lines, footer...)
		markdownLines = append(markdownLines, footer...)
		return GameResult{Title: "功能菜单", Content: strings.Join(nonEmptyLines(lines), "\n"), MarkdownContent: strings.Join(nonEmptyLines(markdownLines), "\n")}, true, nil
	}

	var selected *model.AdminMenu
	for index := range menus {
		category := strings.TrimPrefix(menus[index].Component, "GameMenuCategory:")
		if menus[index].Label == requested || menus[index].Path == requested || category == requested {
			selected = &menus[index]
			break
		}
	}
	if selected == nil {
		return GameResult{Title: "菜单分类不存在", Content: "发送 `菜单` 查看当前可用分类。", Actions: []string{"菜单"}}, true, nil
	}
	if strings.TrimPrefix(selected.Component, "GameMenuCategory:") == "管理" {
		return g.managementMenu(player)
	}
	lines := []string{g.menuCover(), "━━━━━━━━━━━"}
	markdownLines := []string{g.menuCover(), "━━━━━━━━━━━"}
	var commandLinks, markdownCommandLinks []string
	seenCommands := make(map[string]struct{})
	for _, menu := range menus {
		if menu.ParentID == selected.ID && strings.TrimSpace(menu.Path) != "" {
			path := strings.TrimSpace(menu.Path)
			if _, exists := seenCommands[path]; exists {
				continue
			}
			seenCommands[path] = struct{}{}
			commandLinks = append(commandLinks, commandText(path, path))
			markdownCommandLinks = append(markdownCommandLinks, markdownCommandText(path, path))
		}
	}
	if len(commandLinks) == 0 {
		category := strings.TrimPrefix(selected.Component, "GameMenuCategory:")
		for _, spec := range handler.CommandTable {
			if spec.Category == category && !spec.EventOnly {
				if _, exists := seenCommands[spec.Command]; exists {
					continue
				}
				seenCommands[spec.Command] = struct{}{}
				commandLinks = append(commandLinks, commandText(spec.Command, spec.Command))
				markdownCommandLinks = append(markdownCommandLinks, markdownCommandText(spec.Command, spec.Command))
			}
		}
	}
	category := strings.TrimPrefix(selected.Component, "GameMenuCategory:")
	commandLinks, markdownCommandLinks = prioritizeCategoryMenuLinks(category, commandLinks, markdownCommandLinks)
	allCommandCount := len(commandLinks)
	const menuPageSize = 14
	menuPages := maxInt((allCommandCount+menuPageSize-1)/menuPageSize, 1)
	menuPage = minInt(maxInt(menuPage, 1), menuPages)
	menuStart := minInt((menuPage-1)*menuPageSize, allCommandCount)
	menuEnd := minInt(menuStart+menuPageSize, allCommandCount)
	if allCommandCount > 0 {
		pageLine := fmt.Sprintf("第%d/%d页 · 共%d项功能", menuPage, menuPages, allCommandCount)
		lines = append(lines, pageLine)
		markdownLines = append(markdownLines, pageLine)
		lines = append(lines, pairMenuLines(commandLinks[menuStart:menuEnd])...)
		markdownLines = append(markdownLines, pairMenuLines(markdownCommandLinks[menuStart:menuEnd])...)
	}
	if allCommandCount == 0 {
		lines = append(lines, "该分类尚未配置可用指令。")
		markdownLines = append(markdownLines, "该分类尚未配置可用指令。")
	}
	var examples []string
	seenExamples := make(map[string]struct{})
	for _, spec := range handler.CommandTable {
		if spec.Category != category || spec.EventOnly {
			continue
		}
		if _, exists := seenExamples[spec.Input]; !exists && len(examples) < 4 {
			seenExamples[spec.Input] = struct{}{}
			examples = append(examples, spec.Input)
		}
	}
	lines = append(lines, "━━━━━━━━━━━", "例如:")
	markdownLines = append(markdownLines, "━━━━━━━━━━━", "例如:")
	lines = append(lines, pairPlainLines(examples)...)
	markdownLines = append(markdownLines, pairPlainLines(examples)...)
	if allCommandCount > 0 {
		summary := fmt.Sprintf("当前%s分类共%d项功能", selected.Label, allCommandCount)
		lines = append(lines, "━━━━━━━━━━━", summary)
		markdownLines = append(markdownLines, "━━━━━━━━━━━", summary)
	}
	lines = append(lines, "━━━━━━━━━━━", commandText("功能菜单", "功能菜单"), "🪐世界消息🪐", g.latestWorldNotice())
	markdownLines = append(markdownLines, "━━━━━━━━━━━", markdownCommandText("功能菜单", "功能菜单"), "🪐世界消息🪐", g.latestWorldNotice())
	actions := []string{"功能菜单"}
	categoryCommand := strings.TrimPrefix(selected.Component, "GameMenuCategory:") + "菜单"
	if strings.TrimPrefix(selected.Component, "GameMenuCategory:") == "图鉴" {
		categoryCommand = "图鉴菜单"
	}
	if menuPage > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", categoryCommand, menuPage-1))
	}
	if menuPage < menuPages {
		actions = append(actions, fmt.Sprintf("%s %d", categoryCommand, menuPage+1))
	}
	return GameResult{Title: selected.Label + "系统", Content: strings.Join(nonEmptyLines(lines), "\n"), MarkdownContent: strings.Join(nonEmptyLines(markdownLines), "\n"), Actions: actions}, true, nil
}

func parseMenuPage(argument string) (string, int) {
	parts := strings.Fields(strings.TrimSpace(argument))
	page := 1
	if len(parts) > 0 {
		if parsed, err := strconv.Atoi(parts[len(parts)-1]); err == nil && parsed > 0 {
			page = parsed
			parts = parts[:len(parts)-1]
		}
	}
	return strings.Join(parts, " "), page
}

func prioritizeCategoryMenuLinks(category string, plain, markdown []string) ([]string, []string) {
	priorities := map[string][]string{
		"仙府": {"仙府", "灵田", "种田", "施肥", "一键施肥", "收获", "灵肥图鉴", "灵田天象", "护持灵田", "灵田灾异录", "种子商店", "购买种子"},
		"交易": {"商城", "神秘商城", "限时商城", "银币商城", "仙金商城", "货铺", "集市", "摆摊", "易物", "钱庄"},
	}
	priority := priorities[category]
	if len(priority) == 0 || len(plain) != len(markdown) {
		return plain, markdown
	}
	used := make([]bool, len(plain))
	orderedPlain := make([]string, 0, len(plain))
	orderedMarkdown := make([]string, 0, len(markdown))
	for _, command := range priority {
		for index, value := range plain {
			if used[index] || strings.TrimSpace(value) != command {
				continue
			}
			used[index] = true
			orderedPlain = append(orderedPlain, value)
			orderedMarkdown = append(orderedMarkdown, markdown[index])
			break
		}
	}
	for index, value := range plain {
		if used[index] {
			continue
		}
		orderedPlain = append(orderedPlain, value)
		orderedMarkdown = append(orderedMarkdown, markdown[index])
	}
	return orderedPlain, orderedMarkdown
}

func (g *Game) managementMenu(player *model.Player) (GameResult, bool, error) {
	name, level, authorized := g.gmAuthority(player.AccountID)
	if !authorized {
		return GameResult{Title: "管理权限", Content: "此处为主人和已授权管理员专属入口。请在插件设置的“管理设置”中配置管理员用户ID，普通玩家无法查看神令。", Actions: []string{"功能菜单", "帮助"}}, true, nil
	}
	lines := []string{
		"🛡️ 管理系统",
		"━━━━━━━━━━━",
		fmt.Sprintf("当前身份：%s · 权阶%d", name, level),
		"神令会写入审计记录，并按权限阶位执行。",
		"",
		"快捷管理：",
		"- 发放道具 道号 物品名 数量",
		"- 充值 道号 灵石/仙金/银币 数量",
		"- 封号 道号 / 解封 道号 / 删号 道号",
		"- 乾坤令 世界公告内容",
		"",
		"可用神令：",
	}
	markdownLines := append([]string(nil), lines...)
	for _, info := range GMCommandCatalog() {
		line := fmt.Sprintf("- %s · 需要%s", info.Name, info.MinRole)
		lines = append(lines, line)
		markdownLines = append(markdownLines, "- "+markdownInlineCommand(info.Name, info.Name)+" · 需要"+info.MinRole)
	}
	lines = append(lines, "━━━━━━━━━━━", "示例：天赐修为 @玩家 100、天赐灵石 @玩家 500", "所有神令均不带前缀。")
	markdownLines = append(markdownLines, "━━━━━━━━━━━", "示例："+markdownInlineCommand("天赐修为 @玩家 100", "天赐修为 @玩家 100"), "、"+markdownInlineCommand("天赐灵石 @玩家 500", "天赐灵石 @玩家 500"), "所有神令均不带前缀。")
	return GameResult{Title: "管理系统", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: []string{"发放道具 道号 物品名 1", "充值 道号 灵石 2000000", "充值 道号 仙金 2000", "封号 道号", "解封 道号", "删号 道号", "乾坤令 世界公告内容", "管理菜单", "功能菜单"}}, true, nil
}

func commandText(label, command string) string {
	if strings.TrimSpace(label) == strings.TrimSpace(command) {
		return strings.TrimSpace(command)
	}
	return strings.TrimSpace(label) + "：" + strings.TrimSpace(command)
}

func markdownCommandText(label, command string) string {
	label = strings.TrimSpace(label)
	command = strings.TrimSpace(command)
	if label == command {
		return markdownInlineCommand(command)
	}
	return label + "：" + markdownInlineCommand(command)
}

func pairMenuLines(values []string) []string { return pairPlainLines(values) }

func pairPlainLines(values []string) []string {
	lines := make([]string, 0, (len(values)+1)/2)
	for index := 0; index < len(values); index += 2 {
		if index+1 < len(values) {
			lines = append(lines, values[index]+" ┆ "+values[index+1])
		} else {
			lines = append(lines, values[index])
		}
	}
	return lines
}

func mainMenuIcon(category string) string {
	icons := map[string]string{
		"角色": "🧑 ", "修炼": "🧘 ", "功法": "📖 ", "灵根进化": "🌱 ", "悟道": "🪷 ", "渡劫": "⚡ ", "渡劫心魔": "👁️ ", "生涯": "🏅 ",
		"地图": "🗺️ ", "探索": "🧭 ", "天地灵脉": "🔆 ", "奇遇": "🌌 ", "仙缘奇遇": "✨ ", "副本": "🏯 ", "秘境争夺": "🗝️ ", "宇宙星河": "🌠 ",
		"战斗": "⚔️ ", "竞技": "🏆 ", "装备": "🛡️ ", "炼器": "🔥 ", "丹药": "🧪 ", "灵兽": "🐉 ", "阵法": "☯️ ", "符箓": "🧿 ", "傀儡": "🪆 ", "法宝炼化": "💠 ",
		"道侣": "💞 ", "合体技": "🤝 ", "社交": "👥 ", "宗门": "⛩️ ", "宗门战争": "🏳️ ", "仙魔战场": "⚔️ ", "传承": "📚 ",
		"仙府": "🏡 ", "仙药培育": "🌿 ", "交易": "🏪 ", "挂机": "⏳ ", "任务": "📜 ", "活动": "🎯 ", "特殊": "🎁 ",
		"图鉴": "🪪 ", "天机推演": "🔮 ", "系统": "⚙️ ",
	}
	return icons[category]
}

func (g *Game) menuCover() string {
	return "仙尘"
}

func (g *Game) latestWorldNotice() string {
	query := g.store.DB.Where("published = ? AND type = ?", true, "公告")
	for _, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
		pattern := "%" + forbidden + "%"
		query = query.Where("title NOT LIKE ? AND content NOT LIKE ?", pattern, pattern)
	}
	var row model.Notice
	if err := query.Order("published_at DESC, id DESC").First(&row).Error; err == nil && worldNoticePlayerSafe(row.Title, row.Content) {
		return strings.TrimSpace(row.Content)
	}
	return "诸界今日安稳，道友可发送“世界公告”查看最新仙门告示。"
}

func worldNoticePlayerSafe(title, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	combined := title + "\n" + content
	for _, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
		if strings.Contains(combined, forbidden) {
			return false
		}
	}
	return true
}

func nonEmptyLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

func (g *Game) rebirth(player *model.Player) (GameResult, bool, error) {
	count := g.playerValueInt(player.ID, "rebirth.count", 0)
	maximum := g.settingInt("player.max_rebirth", 3)
	if count >= maximum {
		return GameResult{Title: "转世上限", Content: fmt.Sprintf("已完成%d/%d次转世。", count, maximum)}, true, nil
	}
	if player.RealmName != "飞升" && player.Cultivation < player.CultivationRequired {
		return GameResult{Title: "转世条件不足", Content: "需要当前境界修为圆满，或达到飞升境。"}, true, nil
	}
	var first, second model.Realm
	if err := g.store.DB.Order("sequence").First(&first).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Where("sequence > ?", first.Sequence).Order("sequence").First(&second).Error
	retainedAffinity := player.ImmortalAffinity
	bonus := (count + 1) * 5
	updates := map[string]any{
		"realm_id": first.ID, "realm_name": first.Name, "realm_level": 1,
		"cultivation": 0, "cultivation_required": realmStageCost(first, second),
		"health": first.BaseHealth, "max_health": first.BaseHealth,
		"mana": first.BaseMana, "max_mana": first.BaseMana,
		"physical_attack": first.BaseAttack + bonus, "magic_attack": first.BaseAttack + bonus,
		"physical_defense": first.BaseDefense + bonus/2, "magic_defense": first.BaseDefense + bonus/2,
		"agility": first.BaseSpeed, "dodge_rate": first.BaseDodge,
		"lifespan": first.BaseLifespan, "max_lifespan": first.BaseLifespan,
		"immortal_affinity": retainedAffinity, "state": model.PlayerStateIdle,
		"cultivation_started_at": nil,
	}
	preview := *player
	preview.PhysicalAttack, preview.MagicAttack = first.BaseAttack+bonus, first.BaseAttack+bonus
	preview.PhysicalDefense, preview.MagicDefense = first.BaseDefense+bonus/2, first.BaseDefense+bonus/2
	preview.MaxHealth, preview.Agility = first.BaseHealth, first.BaseSpeed
	updates["combat_power"] = calculateCombatPower(preview)
	if err := g.store.DB.Model(player).Updates(updates).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.setPlayerValueInt(player.ID, "rebirth.count", count+1)
	return GameResult{Title: "兵解转世", Content: fmt.Sprintf("前尘散去，道籍重开。\n转世次数：%d/%d\n保留仙缘：%d\n轮回属性加成：+%d", count+1, maximum, retainedAffinity, bonus), Actions: []string{"状态", "修炼"}}, true, nil
}

func (g *Game) rename(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	name := strings.TrimSpace(command.RawArguments)
	if name == "" {
		return GameResult{Title: "改名", Content: "请输入：`改名 新道号`"}, true, nil
	}
	if invalid := validateDaoName(name); invalid != "" {
		return GameResult{Title: "道号格式审核未通过", Content: invalid + "\n本次没有扣除修为。", Actions: []string{"改名"}}, true, nil
	}
	if _, _, matched, err := g.matchSensitiveWord(name); err != nil {
		return GameResult{}, true, err
	} else if matched {
		return GameResult{Title: "道号审核未通过", Content: "新道号触发仙盟禁用词，修为未扣除。", Actions: []string{"改名"}}, true, nil
	}
	existing, err := g.counters.CountByDaoName(name, player.ID)
	if err != nil {
		return GameResult{}, true, err
	} else if existing > 0 {
		return GameResult{Title: "道号已被占用", Content: "道号必须全服唯一，修为未扣除。"}, true, nil
	}
	cost := int64(100)
	if validateDaoName(player.DaoName) != "" || criticalSensitiveTerm(normalizeModerationText(player.DaoName)) != "" {
		cost = 0
	}
	if player.Cultivation < cost {
		return GameResult{Title: "改名失败", Content: "修改道号需要100点修为。"}, true, nil
	}
	if err := g.store.DB.Model(player).Updates(map[string]any{"dao_name": name, "cultivation": gorm.Expr("cultivation - ?", cost)}).Error; err != nil {
		return GameResult{}, true, err
	}
	costText := "消耗修为：100"
	if cost == 0 {
		costText = "旧版违规道号强制整改：本次免费"
	}
	return GameResult{Title: "道号已更改", Content: "从今往后，你的道号是 **" + name + "**。\n" + costText}, true, nil
}

func (g *Game) status(player *model.Player) (GameResult, error) {
	if err := g.syncPlayerCombatPower(player); err != nil {
		return GameResult{}, err
	}
	stamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, err
	}
	staminaMaximum, err := g.staminaMaximum(player.ID)
	if err != nil {
		return GameResult{}, err
	}
	displayPlayer := g.playerWithActiveSkillStats(player)
	if g.settingBool("display.status_image_mode", true) && statusImageRenderingSupported() {
		path, err := g.renderStatusImage(&displayPlayer, stamina, staminaMaximum)
		if err != nil {
			return GameResult{}, err
		}
		return GameResult{Title: "状态图", ImageURL: path, ImageOnly: true}, nil
	}
	return g.textStatus(&displayPlayer, stamina, staminaMaximum), nil
}

func (g *Game) textStatus(player *model.Player, stamina, staminaMaximum int64) GameResult {
	relations := g.statusRelations(player)
	medicineBonus := g.activeItemBonuses(player.ID)
	physicalDefense, magicDefense, agility, daoHeart := medicineAdjustedDisplayStats(player, medicineBonus)
	medicineLine := "当前药力：无持续增益"
	if text := activeMedicineBonusText(medicineBonus); text != "" {
		medicineLine = "当前药力：" + text
	}
	return GameResult{
		Title:    player.DaoName,
		ImageURL: g.playerAvatarURL(player),
		Content: strings.Join([]string{
			"### 道行修为",
			fmt.Sprintf("境界：%s · %d层", player.RealmName, player.RealmLevel),
			fmt.Sprintf("角色等级：LV%d", maxInt(player.Level, 1)),
			"等级进度 " + playerExperienceProgress(*player),
			fmt.Sprintf("灵根：%s（资质%d）", player.SpiritualRoot, player.RootQuality),
			fmt.Sprintf("修为：%d/%d", player.Cultivation, player.CultivationRequired),
			fmt.Sprintf("战力：%d", player.CombatPower),
			fmt.Sprintf("运气：%d/%d · 仙缘：%d", normalizedPlayerLuck(player.Luck), maximumPlayerLuck, player.ImmortalAffinity),
			"", "### 战斗属性",
			fmt.Sprintf("气血：%d/%d", player.Health, player.MaxHealth),
			fmt.Sprintf("法力：%d/%d", player.Mana, player.MaxMana),
			fmt.Sprintf("体力：%d/%d（每个大境上限+%d）", stamina, staminaMaximum, g.settingInt("player.stamina_growth_per_realm", 100)),
			fmt.Sprintf("攻击：%d | 法强：%d", player.PhysicalAttack, player.MagicAttack),
			fmt.Sprintf("防御：%d | 法抗：%d", physicalDefense, magicDefense),
			fmt.Sprintf("身法：%d | 闪避：%.0f%%", agility, player.DodgeRate*100),
			fmt.Sprintf("道心：%d", daoHeart),
			medicineLine,
			"", "### 仙途相伴",
			"性别：" + displayPlayerGender(player.Gender), "仙侣：" + relations.Couple, "宗门：" + relations.Sect,
			"", "### 钱庄余额",
			fmt.Sprintf("灵石：%d · 银币：%d · 仙金：%d · 竞技币：%d", player.SpiritStones, player.SilverCoins, player.ImmortalJade, player.ArenaCoins),
			"", "### 所在之处", "位置：" + player.Location, fmt.Sprintf("道龄：%d年", player.Age),
			"", "### 仙籍档案", "账号：" + player.AccountID, "区服：" + player.ServerName,
		}, "\n"),
		Actions: []string{"当前药效", "等级", "体力", "性别", "仙缘", "修炼", "探索", "背包", "日常"},
	}
}

func (g *Game) inventory(player *model.Player) (GameResult, bool, error) {
	type inventoryRow struct {
		Name         string
		CategoryName string
		RarityName   string
		Description  string
		EffectType   string
		EffectFunc   string
		EffectParams string
		EffectValue  float64
		Quantity     int64
	}
	var rows []inventoryRow
	err := g.store.DB.Table("player_items").Select("items.name, items.category_name, items.rarity_name, items.description, items.effect_type, items.effect_func, items.effect_params, items.effect_value, player_items.quantity").Joins("JOIN items ON items.id = player_items.item_id").Where("player_items.player_id = ? AND player_items.quantity > 0", player.ID).Order("items.category_name, items.rarity_name DESC, items.name").Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	wallet := fmt.Sprintf("道号：%s\n灵石：%d · 银币：%d · 仙金：%d · 竞技币：%d", player.DaoName, player.SpiritStones, player.SilverCoins, player.ImmortalJade, player.ArenaCoins)
	if len(rows) == 0 {
		return GameResult{Title: "乾坤袋", Content: wallet + "\n物品种类：0 · 容量：0/" + fmt.Sprint(g.settingInt("inventory.capacity", 50)) + "\n━━━━━━━\n背包空空如也。\n可通过探索、秘境、任务和商城获取物品。", Actions: []string{"商城", "银币商城", "仙金商城", "探索", "秘境", "货币"}}, true, nil
	}
	var lines, markdownLines []string
	total := int64(0)
	lastCategory := ""
	for _, row := range rows {
		total += row.Quantity
		if row.CategoryName != lastCategory {
			if lastCategory != "" {
				lines = append(lines, "")
			}
			lastCategory = row.CategoryName
			lines = append(lines, "### "+displayOr(row.CategoryName, "未分类"))
			markdownLines = append(markdownLines, "### "+displayOr(row.CategoryName, "未分类"))
		}
		detail := fmt.Sprintf("- %s × %d · %s", row.Name, row.Quantity, displayOr(row.RarityName, "凡品"))
		markdownDetail := fmt.Sprintf("- %s × %d · %s", markdownInlineCommand(row.Name, "物品 "+row.Name), row.Quantity, displayOr(row.RarityName, "凡品"))
		if strings.TrimSpace(row.EffectType) != "" {
			effect := fmt.Sprintf(" · %s", row.EffectType)
			if row.EffectValue != 0 {
				effect += fmt.Sprintf("%.0f", row.EffectValue)
			}
			detail += effect
			markdownDetail += effect
		}
		if row.CategoryName == "丹药" && strings.TrimSpace(row.EffectFunc) != "" {
			medicine := model.Item{Name: row.Name, CategoryName: row.CategoryName, RarityName: row.RarityName, Description: row.Description, EffectType: row.EffectType, EffectFunc: row.EffectFunc, EffectParams: row.EffectParams, EffectValue: row.EffectValue}
			drugEffect := "  实际药效：" + itemEffectSummary(medicine, 1)
			detail += " · 可使用"
			markdownDetail += " · " + markdownInlineCommand("使用", "使用 "+row.Name)
			lines = append(lines, detail, drugEffect)
			markdownLines = append(markdownLines, markdownDetail, drugEffect+" · "+markdownInlineCommand("药效详情", "药效 "+row.Name))
			if strings.TrimSpace(row.Description) != "" {
				lines = append(lines, "  用途："+row.Description)
				markdownLines = append(markdownLines, "  用途："+row.Description)
			}
			continue
		}
		switch {
		case row.EffectFunc == "open_gift_pack":
			markdownDetail += " · " + markdownInlineCommand("开启", "开启礼包 "+row.Name)
		case strings.TrimSpace(row.EffectFunc) != "":
			markdownDetail += " · " + markdownInlineCommand("使用", "使用 "+row.Name)
		case row.CategoryName == "装备":
			markdownDetail += " · " + markdownInlineCommand("装备", "装备背包")
		}
		lines = append(lines, detail)
		markdownLines = append(markdownLines, markdownDetail)
		if strings.TrimSpace(row.Description) != "" {
			lines = append(lines, "  用途："+row.Description)
			markdownLines = append(markdownLines, "  用途："+row.Description)
		}
	}
	lines = append([]string{wallet, fmt.Sprintf("物品种类：%d · 总数量：%d/%d", len(rows), total, g.settingInt("inventory.capacity", 50)), "━━━━━━━"}, lines...)
	markdownLines = append([]string{wallet, fmt.Sprintf("物品种类：%d · 总数量：%d/%d", len(rows), total, g.settingInt("inventory.capacity", 50)), "━━━━━━━"}, markdownLines...)
	return GameResult{Title: "乾坤袋", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), ImageURL: g.settingText("image.inventory_url", ""), Actions: []string{"货币", "商城", "礼包", "背包搜索 功法", "装备背包", "合成菜单", "炼丹", "地图"}}, true, nil
}

func decorateGameTitle(category, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return title
	}
	for _, prefix := range []string{"🌱", "🌿", "🗺️", "⚔️", "🎒", "🏪", "📜", "🏯", "📢", "⚡", "🧘", "🔆", "💎", "🧪", "🛡️", "🎁", "🐉", "🌌", "🪆", "💧", "🕯️", "⚠️", "✅", "👻", "🐾", "🌙", "✨", "🧭", "⏳", "💞", "🤝", "⛩️", "🏆", "🔯", "🧷", "⚙️", "🌋", "☯️", "👿", "✦"} {
		if strings.HasPrefix(title, prefix) {
			return title
		}
	}
	icon := map[string]string{
		"角色": "🪪", "修炼": "🧘", "探索": "🧭", "地图": "🗺️", "挂机": "⏳", "道侣": "💞",
		"战斗": "⚔️", "渡劫": "⚡", "仙府": "🏯", "功法": "📜", "灵兽": "🐉", "社交": "🤝",
		"交易": "🏪", "任务": "📜", "特殊": "🎁", "宗门": "⛩️", "丹药": "🧪", "炼器": "🛡️",
		"副本": "🏯", "竞技": "⚔️", "奇遇": "✨", "生涯": "🏆", "阵法": "🔯", "符箓": "🧷",
		"傀儡": "⚙️", "秘境争夺": "🌋", "传承": "📜", "悟道": "☯️", "仙魔战场": "⚔️", "灵根进化": "🌱",
		"渡劫心魔": "👿", "合体技": "💞", "仙药培育": "🌿", "法宝炼化": "💎", "天机推演": "🌌",
		"天地灵脉": "🔆", "宗门战争": "⚔️", "仙缘奇遇": "✨", "宇宙星河": "🌌", "活动": "🎊", "系统": "⚙️",
	}[category]
	if strings.Contains(title, "背包") || strings.Contains(title, "乾坤袋") {
		icon = "🎒"
	} else if strings.Contains(title, "采集") || strings.Contains(title, "灵植") || strings.Contains(title, "仙药") {
		icon = "🌿"
	} else if strings.Contains(title, "公告") || strings.Contains(title, "通报") {
		icon = "📢"
	} else if strings.Contains(title, "快捷") {
		icon = "⚡"
	} else if strings.Contains(title, "商城") || strings.Contains(title, "货铺") || strings.Contains(title, "商店") {
		icon = "🏪"
	}
	if icon == "" {
		icon = "✦"
	}
	return icon + " " + title
}

func (g *Game) archive(player *model.Player) GameResult {
	return GameResult{Title: "修仙档案", Content: fmt.Sprintf("道号：%s\n性别：%s\n账号：%s\n境界：%s%d层\n角色等级：LV%d\n等级进度：%d/%d\n修为：%d\n战力：%d\n灵根：%s\n道心：%d\n运气：%d/%d\n仙缘：%d\n功德：%d\n声望：%d\n宗门：%s\n位置：%s\n入道：%s", player.DaoName, displayPlayerGender(player.Gender), player.AccountID, player.RealmName, player.RealmLevel, maxInt(player.Level, 1), max64(player.Experience, 0), model.PlayerExperienceRequired(maxInt(player.Level, 1)), player.Cultivation, player.CombatPower, player.SpiritualRoot, player.DaoHeart, normalizedPlayerLuck(player.Luck), maximumPlayerLuck, player.ImmortalAffinity, player.Merit, player.Reputation, displayOr(player.SectName, "散修"), player.Location, player.CreatedAt.Format("2006-01-02"))}
}

func calculateCombatPower(player model.Player) int64 {
	positive := func(value int64) float64 {
		if value < 0 {
			return 0
		}
		return float64(value)
	}
	rate := func(value float64) float64 {
		if value < 0 || math.IsNaN(value) {
			return 0
		}
		return value
	}
	score := positive(player.PhysicalAttack)*2.4 + positive(player.MagicAttack)*2.4
	score += positive(player.PhysicalDefense)*1.8 + positive(player.MagicDefense)*1.8
	score += positive(player.MaxHealth)*0.12 + positive(player.MaxMana)*0.10
	score += positive(player.Agility)*1.5 + positive(player.Strength)*1.3 + positive(player.Constitution)*1.3 + positive(player.Spirit)*1.3
	score += positive(player.Perception) + positive(player.Willpower) + positive(player.Luck)*0.6
	score += float64(maxInt(player.Level, 1))*8 + float64(maxInt(player.RealmLevel, 1))*15
	score += float64(maxInt(player.RootQuality, 0))*0.5 + positive(player.DaoHeart)*0.8 + positive(player.ImmortalAffinity)*0.3
	score += rate(player.CritRate)*800 + math.Max(rate(player.CritDamage)-1, 0)*300
	score += rate(player.DodgeRate)*600 + rate(player.DamageReduction)*1000
	if math.IsInf(score, 0) || score >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	return max64(int64(math.Round(score)), 1)
}

func combatPowerInputsChanged(before, after model.Player) bool {
	return before.Level != after.Level || before.RealmLevel != after.RealmLevel || before.RootQuality != after.RootQuality ||
		before.MaxHealth != after.MaxHealth || before.MaxMana != after.MaxMana ||
		before.PhysicalAttack != after.PhysicalAttack || before.MagicAttack != after.MagicAttack ||
		before.PhysicalDefense != after.PhysicalDefense || before.MagicDefense != after.MagicDefense ||
		before.Agility != after.Agility || before.Strength != after.Strength || before.Constitution != after.Constitution ||
		before.Spirit != after.Spirit || before.Perception != after.Perception || before.Willpower != after.Willpower || before.Luck != after.Luck ||
		before.CritRate != after.CritRate || before.CritDamage != after.CritDamage || before.DodgeRate != after.DodgeRate ||
		before.DamageReduction != after.DamageReduction || before.DaoHeart != after.DaoHeart || before.ImmortalAffinity != after.ImmortalAffinity ||
		before.ActivePetID != after.ActivePetID || before.CurrentSkillID != after.CurrentSkillID
}

func (g *Game) syncPlayerCombatPower(player *model.Player) error {
	if player == nil || player.ID == 0 {
		return nil
	}
	calculated := calculateCombatPower(*player)
	calculated += g.activePetCombatPower(player)
	calculated += g.activeSkillCombatPower(player)
	if calculated == player.CombatPower {
		return nil
	}
	if err := g.players.Update(player.ID, map[string]any{"combat_power": calculated}); err != nil {
		return err
	}
	player.CombatPower = calculated
	return nil
}

func (g *Game) activePetCombatPower(player *model.Player) int64 {
	if player == nil || player.ID == 0 || player.ActivePetID == 0 {
		return 0
	}
	var pet model.Pet
	if err := g.store.DB.Where("id = ? AND player_id = ? AND active = ?", player.ActivePetID, player.ID, true).First(&pet).Error; err != nil {
		return 0
	}
	return petCombatPower(pet)
}

func daoHeartText(value int64) string {
	switch {
	case value >= 80:
		return "道心澄明，修行时不易走火入魔。"
	case value >= 40:
		return "道心平稳，可正常修行。"
	default:
		return "道心动摇，突破成功率会降低。"
	}
}

func displayOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
