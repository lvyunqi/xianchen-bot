package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) shortcutList(player *model.Player) (GameResult, bool, error) {
	shortcuts, err := g.playerShortcuts(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"专属快捷只对你本人有效，可把常用的长指令缩成一个短词。", fmt.Sprintf("当前已设置：%d条", len(shortcuts)), "━━━━━━━━━━━"}
	actions := []string{"设置快捷 回家=位置", "设置快捷 收田=收菜"}
	if len(shortcuts) == 0 {
		lines = append(lines, "尚未设置快捷。", "示例：设置快捷 回家=位置", "设置后直接发送“回家”即可执行“位置”。")
	} else {
		aliases := make([]string, 0, len(shortcuts))
		for alias := range shortcuts {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		for index, alias := range aliases {
			target := shortcuts[alias]
			lines = append(lines, fmt.Sprintf("%d. %s → %s", index+1, alias, target))
			actions = append(actions, alias, "删除快捷 "+alias)
		}
	}
	lines = append(lines, "━━━━━━━━━━━", "点击快捷名可立即执行；点击“删除快捷”可移除对应别名。", "限制：别名最多十二个字，不能带空格，不能覆盖系统指令，也不能映射到管理神令。")
	return GameResult{Title: "专属快捷", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) setShortcut(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
	if len(parts) != 2 {
		return GameResult{Title: "设置快捷", Content: "格式：`设置快捷 别名=完整指令`\n示例：`设置快捷 回家=位置`", Actions: []string{"快捷列表"}}, true, nil
	}
	alias, target := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if alias == "" || strings.ContainsAny(alias, " \t\r\n") || utf8.RuneCountInString(alias) > 12 {
		return GameResult{Title: "快捷别名无效", Content: "别名必须是一至十二个字，且不能包含空格。", Actions: []string{"快捷列表"}}, true, nil
	}
	if _, exists := handler.ParseCommand(alias); exists {
		return GameResult{Title: "快捷别名冲突", Content: "“" + alias + "”已经是系统指令，不能覆盖。", Actions: []string{"快捷列表"}}, true, nil
	}
	parsed, valid := handler.ParseCommand(target)
	if !valid || parsed.Spec.ID == 1001 || parsed.Spec.Category == "管理" {
		return GameResult{Title: "快捷目标无效", Content: "目标必须是一条可以直接执行的普通游戏指令，不能是管理菜单或神令。", Actions: []string{"帮助", "快捷列表"}}, true, nil
	}
	shortcuts, err := g.playerShortcuts(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	shortcuts[alias] = target
	encoded, _ := json.Marshal(shortcuts)
	if err := g.setPlayerValue(player.ID, "player.shortcuts", string(encoded), nil); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "快捷已设置", Content: fmt.Sprintf("%s → %s\n以后直接发送“%s”即可。", alias, target, alias), Actions: []string{alias, "快捷列表"}}, true, nil
}

func (g *Game) deleteShortcut(player *model.Player, raw string) (GameResult, bool, error) {
	alias := strings.TrimSpace(raw)
	if alias == "" {
		return GameResult{Title: "删除快捷", Content: "请输入：`删除快捷 别名`。", Actions: []string{"快捷列表"}}, true, nil
	}
	shortcuts, err := g.playerShortcuts(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if _, exists := shortcuts[alias]; !exists {
		return GameResult{Title: "快捷不存在", Content: "没有设置过“" + alias + "”。", Actions: []string{"快捷列表"}}, true, nil
	}
	delete(shortcuts, alias)
	encoded, _ := json.Marshal(shortcuts)
	if err := g.setPlayerValue(player.ID, "player.shortcuts", string(encoded), nil); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "快捷已删除", Content: "已删除“" + alias + "”，原始游戏指令不受影响。", Actions: []string{"快捷列表"}}, true, nil
}

func (g *Game) playerShortcuts(playerID uint) (map[string]string, error) {
	shortcuts := make(map[string]string)
	value, err := g.playerValue(playerID, "player.shortcuts")
	if err != nil {
		return shortcuts, nil
	}
	if strings.TrimSpace(value) == "" {
		return shortcuts, nil
	}
	if err := json.Unmarshal([]byte(value), &shortcuts); err != nil {
		return nil, err
	}
	return shortcuts, nil
}

func (g *Game) ResolveShortcut(accountID, message string) (handler.ParsedCommand, bool) {
	player, err := g.players.GetByAccount(accountID)
	if err != nil || player.Banned {
		return handler.ParsedCommand{}, false
	}
	shortcuts, err := g.playerShortcuts(player.ID)
	if err != nil {
		return handler.ParsedCommand{}, false
	}
	target := strings.TrimSpace(shortcuts[strings.TrimSpace(message)])
	if target == "" {
		return handler.ParsedCommand{}, false
	}
	parsed, ok := handler.ParseCommand(target)
	if !ok || parsed.Spec.ID == 1001 || parsed.Spec.Category == "管理" {
		return handler.ParsedCommand{}, false
	}
	return parsed, true
}
