package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

type leaderboardDefinition struct {
	Key         string
	Title       string
	Description string
}

type leaderboardRow struct {
	PlayerID          uint   `gorm:"column:player_id"`
	SecondaryPlayerID uint   `gorm:"column:secondary_player_id"`
	Name              string `gorm:"column:name"`
	Score             int64  `gorm:"column:score"`
	Aux               int64  `gorm:"column:aux"`
	Extra             string `gorm:"column:extra"`
}

var leaderboardDefinitions = []leaderboardDefinition{
	{"综合", "太虚总榜", "境界、层数与战力综合评定"},
	{"境界", "大道境界榜", "先比大境界，再比层数与当前修为"},
	{"战力", "诸天战力榜", "按实时综合战力排序"},
	{"修为", "万载修为榜", "按当前境界内积累修为排序"},
	{"财富", "乾坤财富榜", "按持有灵石排序"},
	{"功德", "玄门功德榜", "按累计功德排序"},
	{"声望", "四海声望榜", "按仙门声望排序"},
	{"道心", "问心无愧榜", "按道心稳固程度排序"},
	{"仙缘", "天眷仙缘榜", "按仙缘气数排序"},
	{"灵根", "先天灵根榜", "按灵根纯度排序"},
	{"灵兽", "万兽御灵榜", "按名下灵兽总战力排序"},
	{"装备", "百炼神兵榜", "按装备等级与锻造层次排序"},
	{"宗门", "仙宗气运榜", "按宗门声望、等级与底蕴排序"},
	{"道缘", "三生道缘榜", "按仙侣道缘与同心层次排序"},
	{"副本", "秘境征伐榜", "按副本最高评分与通关次数排序"},
	{"竞技", "问剑天骄榜", "按竞技积分与胜场排序"},
	{"灵田", "洞天灵植榜", "按仙府繁荣与灵田等级排序"},
	{"首领", "镇域诛魔榜", "按击败区域首领次数排序"},
	{"成就", "仙途成就榜", "按已解锁核心成就数量排序"},
	{"活跃", "诸界活跃榜", "按探索、战斗、副本与修行行为排序"},
	{"炼丹", "丹道宗师榜", "按成功炼丹次数排序"},
	{"锻造", "炼器宗师榜", "按成功锻造次数排序"},
	{"渡劫", "九霄渡劫榜", "按成功渡劫次数排序"},
	{"生辰", "寿星福缘榜", "按历年收到的唯一祝福与有效赠礼福缘排序"},
}

func (g *Game) rankingCenter(player *model.Player, raw string) (GameResult, bool, error) {
	kind, page := parseRankingRequest(raw)
	if kind == "" {
		definitions := g.visibleLeaderboardDefinitions()
		var titleChanges []string
		for _, definition := range definitions {
			rows, err := g.loadLeaderboard(definition.Key)
			if err != nil {
				return GameResult{}, true, err
			}
			changes, err := g.syncLeaderboardTitles(definition, rows)
			if err != nil {
				return GameResult{}, true, err
			}
			titleChanges = append(titleChanges, changes...)
		}
		lines := []string{
			"诸天万榜已经开卷，每榜每日结算一次前十俸禄。",
			"榜单只显示道号，不公开QQ号、账号ID或内部玩家ID。",
			"每榜前三各有独立尊号与真实属性；榜位更迭后旧尊号和已佩戴属性会同步失效。",
			"━━━━━━━━━━━",
		}
		actions := make([]string, 0, len(definitions))
		for index := 0; index < len(definitions); index += 2 {
			left := definitions[index]
			line := left.Key + "榜"
			if index+1 < len(definitions) {
				line += " ┆ " + definitions[index+1].Key + "榜"
			}
			lines = append(lines, line)
		}
		for _, definition := range definitions {
			actions = append(actions, "排行 "+definition.Key)
		}
		lines = append(lines, "━━━━━━━━━━━", "查看：排行 战力", "领奖：领取排行奖励 战力", "奖励名次：第一至第十名；每个榜单每日限领一次。")
		result := GameResult{Title: "诸天排行榜", Content: strings.Join(lines, "\n"), Actions: append(actions, "我的称号")}
		if len(titleChanges) > 0 {
			broadcast := "【诸天尊号更迭】" + strings.Join(titleChanges, "；") + "。榜位尊号及属性已经同步。"
			_ = g.publishWorldBroadcast("排行", "诸天尊号更迭", broadcast)
			result.BroadcastContent = broadcast
		}
		return result, true, nil
	}
	definition, found := findLeaderboardDefinition(kind)
	if !found {
		return GameResult{Title: "榜单不存在", Content: "没有找到“" + kind + "”榜。发送 `排行榜` 查看全部榜单。", Actions: []string{"排行榜"}}, true, nil
	}
	if definition.Key == "生辰" && !g.birthdayLeaderboardOpen() {
		return GameResult{Title: "寿星榜今日未开", Content: "今天没有已登记生日的寿星，生辰榜单与对应尊号暂不显示。", Actions: []string{"排行榜"}}, true, nil
	}
	rows, err := g.loadLeaderboard(definition.Key)
	if err != nil {
		return GameResult{}, true, err
	}
	titleChanges, err := g.syncLeaderboardTitles(definition, rows)
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 10
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	start := minInt((page-1)*pageSize, len(rows))
	end := minInt(start+pageSize, len(rows))
	lines := []string{definition.Description, fmt.Sprintf("第%d/%d页 · 收录%d名道友", page, pages, len(rows)), "━━━━━━━━━━━"}
	for index, row := range rows[start:end] {
		rank := start + index + 1
		marker := rankingMarker(rank)
		titleLine := ""
		if spec, ok := rankingTitleSpec(definition.Key, rank); ok {
			titleLine = "\n   尊号：" + spec.Name + " · 属性：" + displayConfigText(spec.BonusJSON)
		}
		lines = append(lines, fmt.Sprintf("%s 第%d名 · %s\n   %s%s", marker, rank, row.Name, leaderboardScoreText(definition.Key, row), titleLine))
	}
	if len(rows) == 0 {
		lines = append(lines, "此榜尚无人留名，第一位完成对应玩法的道友将成为开榜者。")
	}
	if rank := leaderboardPlayerRank(definition.Key, rows, player); rank > 0 {
		lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("你的当前名次：第%d名", rank))
	} else {
		lines = append(lines, "━━━━━━━━━━━", "你尚未进入该榜。")
	}
	lines = append(lines, "前十奖励：名次越高，灵石与功德俸禄越丰厚；前三尊号随当前榜位生效。")
	actions := []string{"领取排行奖励 " + definition.Key, "我的称号", "排行榜"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("排行 %s %d", definition.Key, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("排行 %s %d", definition.Key, page+1))
	}
	result := GameResult{Title: definition.Title, Content: strings.Join(lines, "\n"), Actions: actions}
	if len(titleChanges) > 0 {
		broadcast := "【" + definition.Title + "尊号更迭】" + strings.Join(titleChanges, "；") + "。"
		_ = g.publishWorldBroadcast("排行", definition.Title+"尊号更迭", broadcast)
		result.BroadcastContent = broadcast
	}
	return result, true, nil
}

func (g *Game) birthdayLeaderboardOpen() bool {
	rows, err := g.todayBirthdayPlayers(time.Now())
	return err == nil && len(rows) > 0
}

func (g *Game) visibleLeaderboardDefinitions() []leaderboardDefinition {
	definitions := make([]leaderboardDefinition, 0, len(leaderboardDefinitions))
	for _, definition := range leaderboardDefinitions {
		if definition.Key == "生辰" && !g.birthdayLeaderboardOpen() {
			continue
		}
		definitions = append(definitions, definition)
	}
	return definitions
}

func rankingTitleSpec(kind string, rank int) (model.RankingTitleSpec, bool) {
	if rank < 1 || rank > 3 {
		return model.RankingTitleSpec{}, false
	}
	for _, spec := range model.RankingTitleCatalog() {
		if spec.Leaderboard == kind && spec.Rank == rank {
			return spec, true
		}
	}
	return model.RankingTitleSpec{}, false
}

func leaderboardTitleRecipients(row leaderboardRow) []uint {
	result := make([]uint, 0, 2)
	if row.PlayerID != 0 {
		result = append(result, row.PlayerID)
	}
	if row.SecondaryPlayerID != 0 && row.SecondaryPlayerID != row.PlayerID {
		result = append(result, row.SecondaryPlayerID)
	}
	return result
}

func (g *Game) syncLeaderboardTitles(definition leaderboardDefinition, rows []leaderboardRow) ([]string, error) {
	type titleState struct {
		spec   model.RankingTitleSpec
		title  model.Title
		owners map[uint]struct{}
	}
	states := make(map[string]titleState, 3)
	keys := make([]string, 0, 3)
	for rank := 1; rank <= 3; rank++ {
		spec, ok := rankingTitleSpec(definition.Key, rank)
		if !ok {
			continue
		}
		var title model.Title
		if err := g.store.DB.Where("code = ? AND enabled = ?", spec.Code, true).First(&title).Error; err != nil {
			return nil, err
		}
		owners := make(map[uint]struct{})
		if rank <= len(rows) {
			for _, playerID := range leaderboardTitleRecipients(rows[rank-1]) {
				owners[playerID] = struct{}{}
			}
		}
		states[spec.Code] = titleState{spec: spec, title: title, owners: owners}
		keys = append(keys, titleUnlockKey(title))
	}
	if len(states) == 0 {
		return nil, nil
	}

	var existing []model.PlayerValue
	if err := g.store.DB.Where("key IN ?", keys).Find(&existing).Error; err != nil {
		return nil, err
	}
	for _, value := range existing {
		code := strings.TrimPrefix(value.Key, "title.unlocked.")
		state, exists := states[code]
		if !exists {
			continue
		}
		if _, keep := state.owners[value.PlayerID]; keep {
			delete(state.owners, value.PlayerID)
			states[code] = state
			continue
		}
		var former model.Player
		if err := g.store.DB.First(&former, value.PlayerID).Error; err == nil && former.Title == state.title.Name {
			if _, _, err := g.removeTitle(&former); err != nil {
				return nil, err
			}
		}
		if err := g.store.DB.Delete(&value).Error; err != nil {
			return nil, err
		}
		_ = g.createPlayerNotification(value.PlayerID, "榜位尊号变更", fmt.Sprintf("你已不再执掌%s榜对应席位，尊号【%s】及其佩戴属性已经失效。", definition.Key, state.title.Name))
	}

	var championChanges []string
	for _, state := range states {
		for playerID := range state.owners {
			if err := g.setPlayerValue(playerID, titleUnlockKey(state.title), "ranking", nil); err != nil {
				return nil, err
			}
			var owner model.Player
			if err := g.store.DB.First(&owner, playerID).Error; err != nil {
				return nil, err
			}
			_ = g.createPlayerNotification(playerID, "榜位尊号", fmt.Sprintf("你当前执掌%s榜尊席，获授【%s】。属性：%s；可在尊号玉册中佩戴。", definition.Key, state.title.Name, displayConfigText(state.title.AttributeBonus)))
			if state.spec.Rank == 1 {
				championChanges = append(championChanges, fmt.Sprintf("%s获授【%s】", owner.DaoName, state.title.Name))
			}
		}
	}
	return championChanges, nil
}

func parseRankingRequest(raw string) (string, int) {
	parts := strings.Fields(strings.TrimSpace(raw))
	page := 1
	if len(parts) > 0 {
		if value, err := strconv.Atoi(parts[len(parts)-1]); err == nil && value > 0 {
			page = value
			parts = parts[:len(parts)-1]
		}
	}
	kind := strings.TrimSpace(strings.Join(parts, " "))
	kind = strings.TrimSuffix(kind, "排行榜")
	kind = strings.TrimSuffix(kind, "排行")
	kind = strings.TrimSuffix(kind, "榜")
	aliases := map[string]string{"总": "综合", "总榜": "综合", "灵石": "财富", "财力": "财富", "宠物": "灵兽", "法宝": "装备", "仙侣": "道缘", "秘境": "副本", "竞技场": "竞技", "农场": "灵田", "BOSS": "首领", "Boss": "首领", "boss": "首领"}
	if canonical := aliases[kind]; canonical != "" {
		kind = canonical
	}
	return kind, page
}

func findLeaderboardDefinition(kind string) (leaderboardDefinition, bool) {
	for _, definition := range leaderboardDefinitions {
		if definition.Key == kind {
			return definition, true
		}
	}
	return leaderboardDefinition{}, false
}

func (g *Game) loadLeaderboard(kind string) ([]leaderboardRow, error) {
	var rows []leaderboardRow
	playerField := map[string]string{
		"战力": "combat_power", "修为": "cultivation", "财富": "spirit_stones", "功德": "merit",
		"声望": "reputation", "道心": "dao_heart", "仙缘": "immortal_affinity", "灵根": "root_quality",
	}[kind]
	if playerField != "" {
		extra := "''"
		if kind == "灵根" {
			extra = "spiritual_root"
		}
		query := fmt.Sprintf("SELECT id AS player_id, 0 AS secondary_player_id, dao_name AS name, %s AS score, 0 AS aux, %s AS extra FROM players WHERE deleted_at IS NULL AND banned = ? ORDER BY %s DESC, id ASC", playerField, extra, playerField)
		return rows, g.store.DB.Raw(query, false).Scan(&rows).Error
	}
	switch kind {
	case "综合":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			(COALESCE(r.sequence,0) * 1000000000 + p.realm_level * 1000000 + p.combat_power) AS score,
			p.realm_level AS aux, p.realm_name AS extra
			FROM players p LEFT JOIN realms r ON r.id = p.realm_id
			WHERE p.deleted_at IS NULL AND p.banned = ?
			ORDER BY COALESCE(r.sequence,0) DESC, p.realm_level DESC, p.combat_power DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "境界":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			(COALESCE(r.sequence,0) * 1000000 + p.realm_level * 1000 + p.cultivation) AS score,
			p.realm_level AS aux, p.realm_name AS extra
			FROM players p LEFT JOIN realms r ON r.id = p.realm_id
			WHERE p.deleted_at IS NULL AND p.banned = ?
			ORDER BY COALESCE(r.sequence,0) DESC, p.realm_level DESC, p.cultivation DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "灵兽":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			SUM((pet.attack * 2 + pet.defense * 2 + pet.health / 10) / 4) AS score, COUNT(pet.id) AS aux, '' AS extra
			FROM pets pet JOIN players p ON p.id = pet.player_id WHERE p.deleted_at IS NULL AND p.banned = ?
			GROUP BY p.id, p.dao_name ORDER BY score DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "装备":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			SUM(a.level * 100 + a.forge_level * 300 + CASE WHEN a.equipped THEN 200 ELSE 0 END) AS score,
			COUNT(a.id) AS aux, '' AS extra FROM player_artifacts a JOIN players p ON p.id = a.player_id
			WHERE p.deleted_at IS NULL AND p.banned = ? GROUP BY p.id, p.dao_name ORDER BY score DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "宗门":
		err := g.store.DB.Raw(`SELECT s.owner_id AS player_id, 0 AS secondary_player_id, s.name AS name,
			(s.reputation * 100 + s.level * 10000 + s.funds) AS score, s.level AS aux, '' AS extra
			FROM sects s ORDER BY score DESC, s.id ASC`).Scan(&rows).Error
		return rows, err
	case "道缘":
		err := g.store.DB.Raw(`SELECT player_a_id AS player_id, player_b_id AS secondary_player_id,
			(player_a_name || ' · ' || player_b_name) AS name, affinity AS score, bond_level AS aux, '' AS extra
			FROM couples WHERE status = ? ORDER BY affinity DESC, id ASC`, model.CoupleStatusActive).Scan(&rows).Error
		return rows, err
	case "副本":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			MAX(d.score) AS score, COUNT(d.id) AS aux, '' AS extra FROM dungeon_runs d JOIN players p ON p.id = d.player_id
			WHERE d.success = ? AND p.deleted_at IS NULL AND p.banned = ? GROUP BY p.id, p.dao_name ORDER BY score DESC, aux DESC, p.id ASC`, true, false).Scan(&rows).Error
		return rows, err
	case "竞技":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			a.rating AS score, a.wins AS aux, '' AS extra FROM arena_records a JOIN players p ON p.id = a.player_id
			WHERE p.deleted_at IS NULL AND p.banned = ? ORDER BY a.rating DESC, a.wins DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "灵田":
		err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
			m.prosperity AS score, m.farm_level AS aux, '' AS extra FROM mansions m JOIN players p ON p.id = m.player_id
			WHERE p.deleted_at IS NULL AND p.banned = ? ORDER BY m.prosperity DESC, m.farm_level DESC, p.id ASC`, false).Scan(&rows).Error
		return rows, err
	case "成就":
		err := g.store.DB.Raw(`SELECT id AS player_id, 0 AS secondary_player_id, dao_name AS name,
			(1 + CASE WHEN couple_id > 0 THEN 1 ELSE 0 END + CASE WHEN realm_name = ? THEN 1 ELSE 0 END) AS score,
			0 AS aux, title AS extra FROM players WHERE deleted_at IS NULL AND banned = ? ORDER BY score DESC, id ASC`, "飞升", false).Scan(&rows).Error
		return rows, err
	case "活跃":
		return g.statSumLeaderboard([]string{"stats.explores", "stats.battles", "stats.dungeons", "stats.cultivation_minutes"})
	case "首领":
		return g.statLeaderboard("stats.boss_wins")
	case "炼丹":
		return g.statLeaderboard("stats.alchemy")
	case "锻造":
		return g.statLeaderboard("stats.forges")
	case "渡劫":
		return g.statLeaderboard("stats.tribulation_successes")
	case "生辰":
		return g.statLeaderboard(birthdayLifetimeScoreKey)
	default:
		return nil, fmt.Errorf("unsupported leaderboard: %s", kind)
	}
}

func (g *Game) statLeaderboard(key string) ([]leaderboardRow, error) {
	var rows []leaderboardRow
	err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
		CAST(v.value AS BIGINT) AS score, 0 AS aux, '' AS extra FROM player_values v JOIN players p ON p.id = v.player_id
		WHERE v.key = ? AND CAST(v.value AS BIGINT) > 0 AND p.deleted_at IS NULL AND p.banned = ?
		ORDER BY score DESC, p.id ASC`, key, false).Scan(&rows).Error
	return rows, err
}

func (g *Game) statSumLeaderboard(keys []string) ([]leaderboardRow, error) {
	var rows []leaderboardRow
	err := g.store.DB.Raw(`SELECT p.id AS player_id, 0 AS secondary_player_id, p.dao_name AS name,
		SUM(CAST(v.value AS BIGINT)) AS score, COUNT(v.id) AS aux, '' AS extra
		FROM player_values v JOIN players p ON p.id = v.player_id
		WHERE v.key IN ? AND p.deleted_at IS NULL AND p.banned = ? GROUP BY p.id, p.dao_name
		ORDER BY score DESC, p.id ASC`, keys, false).Scan(&rows).Error
	return rows, err
}

func leaderboardPlayerRank(kind string, rows []leaderboardRow, player *model.Player) int {
	for index, row := range rows {
		eligible := row.PlayerID == player.ID || row.SecondaryPlayerID == player.ID
		if kind == "宗门" && player.SectName != "" {
			eligible = row.Name == player.SectName
		}
		if eligible {
			return index + 1
		}
	}
	return 0
}

func leaderboardScoreText(kind string, row leaderboardRow) string {
	switch kind {
	case "综合":
		return fmt.Sprintf("%s · 第%d层 · 综合道行%d", displayOr(row.Extra, "未知境界"), row.Aux, row.Score)
	case "境界":
		return fmt.Sprintf("%s · 第%d层", displayOr(row.Extra, "未知境界"), row.Aux)
	case "战力":
		return fmt.Sprintf("战力 %d", row.Score)
	case "修为":
		return fmt.Sprintf("当前修为 %d", row.Score)
	case "财富":
		return fmt.Sprintf("灵石 %d", row.Score)
	case "功德":
		return fmt.Sprintf("功德 %d", row.Score)
	case "声望":
		return fmt.Sprintf("声望 %d", row.Score)
	case "道心":
		return fmt.Sprintf("道心 %d", row.Score)
	case "仙缘":
		return fmt.Sprintf("仙缘 %d", row.Score)
	case "灵根":
		return fmt.Sprintf("%s · 纯度%d", displayOr(row.Extra, "未知灵根"), row.Score)
	case "灵兽":
		return fmt.Sprintf("%d只灵兽 · 御灵总战力%d", row.Aux, row.Score)
	case "装备":
		return fmt.Sprintf("%d件法宝装备 · 炼器评分%d", row.Aux, row.Score)
	case "宗门":
		return fmt.Sprintf("宗门%d级 · 气运评分%d", row.Aux, row.Score)
	case "道缘":
		return fmt.Sprintf("同心%d阶 · 道缘%d", row.Aux, row.Score)
	case "副本":
		return fmt.Sprintf("最高评分%d · 通关%d次", row.Score, row.Aux)
	case "竞技":
		return fmt.Sprintf("问剑积分%d · 胜%d场", row.Score, row.Aux)
	case "灵田":
		return fmt.Sprintf("灵田%d级 · 洞天繁荣%d", row.Aux, row.Score)
	case "首领":
		return fmt.Sprintf("镇域首领击破%d次", row.Score)
	case "成就":
		return fmt.Sprintf("核心成就%d项 · 当前称号%s", row.Score, displayOr(row.Extra, "初入仙途"))
	case "活跃":
		return fmt.Sprintf("仙途活跃度%d", row.Score)
	case "炼丹":
		return fmt.Sprintf("成功开炉%d次", row.Score)
	case "锻造":
		return fmt.Sprintf("成功锻造%d次", row.Score)
	case "渡劫":
		return fmt.Sprintf("渡劫成功%d次", row.Score)
	case "生辰":
		return fmt.Sprintf("生辰祝福值%d", row.Score)
	default:
		return fmt.Sprintf("评分%d", row.Score)
	}
}

func rankingMarker(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "◆"
	}
}

func (g *Game) claimRankingReward(player *model.Player, raw string) (GameResult, bool, error) {
	kind, _ := parseRankingRequest(raw)
	definition, found := findLeaderboardDefinition(kind)
	if !found {
		return GameResult{Title: "排行俸禄", Content: "请输入榜单类型，例如：`领取排行奖励 战力`。", Actions: []string{"排行榜"}}, true, nil
	}
	if definition.Key == "生辰" && !g.birthdayLeaderboardOpen() {
		return GameResult{Title: "寿星榜今日未开", Content: "今天没有寿星，不能查看或领取生辰榜俸禄。", Actions: []string{"排行榜"}}, true, nil
	}
	rows, err := g.loadLeaderboard(kind)
	if err != nil {
		return GameResult{}, true, err
	}
	titleChanges, err := g.syncLeaderboardTitles(definition, rows)
	if err != nil {
		return GameResult{}, true, err
	}
	titleBroadcast := ""
	if len(titleChanges) > 0 {
		titleBroadcast = "【" + definition.Title + "尊号更迭】" + strings.Join(titleChanges, "；") + "。"
		_ = g.publishWorldBroadcast("排行", definition.Title+"尊号更迭", titleBroadcast)
	}
	rank := leaderboardPlayerRank(kind, rows, player)
	if rank == 0 || rank > 10 {
		return GameResult{Title: "未入前十", Content: fmt.Sprintf("你当前未进入%s前十，暂无今日俸禄。继续提升后可再次查看。", definition.Title), Actions: []string{"排行 " + kind, "帮助"}, BroadcastContent: titleBroadcast}, true, nil
	}
	today := time.Now().Format("2006-01-02")
	claimKey := "ranking.claim." + today + "." + kind
	if _, valueErr := g.playerValue(player.ID, claimKey); valueErr == nil {
		return GameResult{Title: "俸禄已领", Content: fmt.Sprintf("%s第%d名的今日俸禄已经领取，明日榜单重新结算。", definition.Title, rank), Actions: []string{"排行 " + kind, "排行榜", "我的称号"}, BroadcastContent: titleBroadcast}, true, nil
	}
	stones := int64(11-rank) * 30
	merit := int64(11 - rank)
	errAlreadyClaimed := errors.New("ranking reward already claimed")
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		marker := model.PlayerValue{PlayerID: player.ID, Key: claimKey, Value: strconv.Itoa(rank)}
		if createErr := tx.Create(&marker).Error; createErr != nil {
			return errAlreadyClaimed
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
			"spirit_stones": gorm.Expr("spirit_stones + ?", stones),
			"merit":         gorm.Expr("merit + ?", merit),
		}).Error
	})
	if errors.Is(err, errAlreadyClaimed) {
		return GameResult{Title: "俸禄已领", Content: "今日该榜俸禄已经领取，不能重复领取。", Actions: []string{"排行 " + kind, "我的称号"}, BroadcastContent: titleBroadcast}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	titleReward := ""
	if spec, ok := rankingTitleSpec(kind, rank); ok {
		titleReward = fmt.Sprintf("\n当前榜位尊号：【%s】\n佩戴属性：%s", spec.Name, displayConfigText(spec.BonusJSON))
	}
	content := fmt.Sprintf("%s第%d名\n灵石：+%d\n功德：+%d%s\n结算日期：%s\n该榜今日不可重复领取。", definition.Title, rank, stones, merit, titleReward, today)
	_ = g.publishWorldBroadcast("排行", player.DaoName+"名列"+definition.Title, fmt.Sprintf("%s位列第%d名，领取今日天榜俸禄：灵石%d、功德%d。", player.DaoName, rank, stones, merit))
	rewardBroadcast := fmt.Sprintf("【排行天赐】%s位列%s第%d名，获赐灵石%d、功德%d。", player.DaoName, definition.Title, rank, stones, merit)
	if titleBroadcast != "" {
		rewardBroadcast = titleBroadcast + "\n" + rewardBroadcast
	}
	return GameResult{Title: "前十俸禄", Content: content, Actions: []string{"排行 " + kind, "我的称号", "全区通报", "状态"}, BroadcastContent: rewardBroadcast}, true, nil
}

// noticeBoardLimit 限制公告板单次加载的公告数量。
const noticeBoardLimit = 200

func (g *Game) noticeBoard(raw, typeFilter string) (GameResult, bool, error) {
	query := g.store.DB.Model(&model.Notice{}).Where("published = ?", true)
	if typeFilter != "" {
		query = query.Where("type = ?", typeFilter)
	} else {
		query = query.Where("type = ?", "公告")
	}
	var notices []model.Notice
	// 公告上限：种子目录会生成上千条公告，无界加载会拖垮内存与响应；置顶优先，取最近 noticeBoardLimit 条。
	if err := query.Order("pinned DESC, published_at DESC, id DESC").Limit(noticeBoardLimit).Find(&notices).Error; err != nil {
		return GameResult{}, true, err
	}
	title := "仙门公告"
	if typeFilter == "更新" {
		title = "版本更新公告"
	} else if typeFilter == "修复" {
		title = "已确认修复公告"
	} else if typeFilter == "全区通报" {
		title = "诸天全区通报"
	}
	baseCommand := "世界公告"
	if typeFilter == "更新" {
		baseCommand = "更新公告"
	} else if typeFilter == "修复" {
		baseCommand = "修复公告"
	} else if typeFilter == "全区通报" {
		baseCommand = "全区通报"
	}
	if len(notices) == 0 {
		return GameResult{Title: title, Content: "第1/1页 · 共0则\n━━━━━━━━━━━\n暂时没有已经发布的内容。", Actions: []string{baseCommand}}, true, nil
	}
	type noticePage struct {
		noticeIndex int
		partIndex   int
		partCount   int
		notice      model.Notice
		content     string
	}
	virtualPages := make([]noticePage, 0, len(notices))
	for noticeIndex, notice := range notices {
		parts := splitResultContent(notice.Content, 1500, 12)
		if len(parts) == 0 {
			parts = []string{"暂无正文。"}
		}
		for partIndex, content := range parts {
			virtualPages = append(virtualPages, noticePage{noticeIndex: noticeIndex + 1, partIndex: partIndex + 1, partCount: len(parts), notice: notice, content: content})
		}
	}
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	pages := len(virtualPages)
	if page > pages {
		page = pages
	}
	selected := virtualPages[page-1]
	notice := selected.notice
	publishedAt := notice.CreatedAt
	if notice.PublishedAt != nil {
		publishedAt = *notice.PublishedAt
	}
	pin := ""
	if notice.Pinned {
		pin = "【置顶】"
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d则公告", page, pages, len(notices)), "━━━━━━━━━━━", fmt.Sprintf("%s%s · %s", pin, notice.Title, publishedAt.Format("01-02 15:04"))}
	if selected.partCount > 1 {
		lines = append(lines, fmt.Sprintf("正文分段：%d/%d", selected.partIndex, selected.partCount))
	}
	lines = append(lines, selected.content, "━━━━━━━━━━━", fmt.Sprintf("公告序位：%d/%d", selected.noticeIndex, len(notices)))
	// 公告频道彼此独立，避免世界公告引用更新或修复公告。
	actions := []string{baseCommand}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", baseCommand, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", baseCommand, page+1))
	}
	return GameResult{Title: title, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) publishWorldBroadcast(kind, title, content string) error {
	now := time.Now()
	row := model.Notice{
		Code:        fmt.Sprintf("world_%d", now.UnixNano()),
		Title:       "【" + strings.TrimSpace(kind) + "】" + strings.TrimSpace(title),
		Content:     strings.TrimSpace(content),
		Type:        "全区通报",
		Published:   true,
		PublishedAt: &now,
	}
	return g.store.DB.Create(&row).Error
}
