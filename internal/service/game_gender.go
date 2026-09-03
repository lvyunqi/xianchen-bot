package service

import (
	"fmt"
	"strings"

	"xianlv/internal/model"
)

const unsetPlayerGender = "未定"

func normalizePlayerGender(raw string) string {
	switch strings.TrimSpace(raw) {
	case "男", "男修", "男性":
		return "男修"
	case "女", "女修", "女性":
		return "女修"
	default:
		return ""
	}
}

func displayPlayerGender(raw string) string {
	if gender := normalizePlayerGender(raw); gender != "" {
		return gender
	}
	return unsetPlayerGender
}

func hasPlayerGender(player *model.Player) bool {
	return player != nil && normalizePlayerGender(player.Gender) != ""
}

func (g *Game) playerGender(player *model.Player, raw string) (GameResult, bool, error) {
	target := normalizePlayerGender(raw)
	current := displayPlayerGender(player.Gender)
	if strings.TrimSpace(raw) == "" {
		content := fmt.Sprintf("道号：%s\n性别：%s\n━━━━━━━━━━━\n入道时可发送“入道 道号 男/女”；旧角色可发送“性别 男”或“性别 女”补录。\n性别用于角色档案、仙侣资料和双修叙事，不限制可结缘对象。", player.DaoName, current)
		return GameResult{Title: "角色性别", Content: content, Actions: []string{"性别 男", "性别 女", "状态", "道侣菜单"}}, true, nil
	}
	if target == "" {
		return GameResult{Title: "性别格式不正确", Content: "目前可登记为男修或女修。\n请输入：`性别 男` 或 `性别 女`。", Actions: []string{"性别 男", "性别 女"}}, true, nil
	}
	if hasPlayerGender(player) && player.Gender == target {
		return GameResult{Title: "性别已经登记", Content: fmt.Sprintf("道号：%s\n性别：%s\n无需重复设置。", player.DaoName, target), Actions: []string{"状态", "道侣菜单"}}, true, nil
	}
	if hasPlayerGender(player) && player.CoupleID != 0 {
		return GameResult{Title: "道籍性别暂不可更改", Content: fmt.Sprintf("当前性别：%s\n已有生效中的仙侣因果。为避免仙侣档案前后矛盾，结缘期间不能改写已登记性别；未登记的旧角色仍可正常补录。", current), Actions: []string{"心意", "道侣菜单", "性别"}}, true, nil
	}
	if err := g.players.UpdateColumn(player.ID, "gender", target); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "性别登记完成", Content: fmt.Sprintf("道号：%s\n性别：%s\n━━━━━━━━━━━\n角色状态、修仙档案、寻缘名单与仙侣互动现已使用这份道籍资料。", player.DaoName, target), Actions: []string{"状态", "档案", "寻缘", "道侣菜单"}}, true, nil
}
