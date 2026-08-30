package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

const selfDeleteConfirmationKey = "account.self_delete_confirmation"

func (g *Game) requestSelfDelete(player *model.Player) (GameResult, bool, error) {
	expires := time.Now().Add(10 * time.Minute)
	if err := g.setPlayerValue(player.ID, selfDeleteConfirmationKey, player.DaoName, &expires); err != nil {
		return GameResult{}, true, err
	}
	content := fmt.Sprintf("道号：%s\n确认时限：10分钟\n━━━━━━━━━━━\n此操作不可恢复，将永久清除：\n- 境界、修为、货币与角色属性\n- 乾坤袋、装备、法宝、功法与灵兽\n- 仙府、灵田、灵植与仓库\n- 任务、副本、竞技、排行与玩法进度\n- 仙侣、好友、宗门、交易与社交关系\n━━━━━━━━━━━\n删除完成后，道号“%s”立即释放，可以被任何账号重新注册。\n若确定删除，请发送：确认删号 %s", player.DaoName, player.DaoName, player.DaoName)
	return GameResult{Title: "⚠️ 删除道籍确认", Content: content, Actions: []string{"确认删号 " + player.DaoName, "取消删号", "状态"}}, true, nil
}

func (g *Game) confirmSelfDelete(player *model.Player, argument string) (GameResult, bool, error) {
	confirmation, err := g.playerValue(player.ID, selfDeleteConfirmationKey)
	if err != nil || strings.TrimSpace(confirmation) == "" {
		return GameResult{Title: "⚠️ 删号确认已失效", Content: "没有有效的删号申请，或十分钟确认期已经结束。请重新发送“申请删号”。", Actions: []string{"申请删号", "状态"}}, true, nil
	}
	provided := strings.TrimSpace(argument)
	if provided != player.DaoName || confirmation != player.DaoName {
		return GameResult{Title: "⚠️ 道号校验失败", Content: fmt.Sprintf("必须完整输入当前道号，任何错字都不会执行删除。\n当前道号：%s\n正确格式：确认删号 %s", player.DaoName, player.DaoName), Actions: []string{"确认删号 " + player.DaoName, "取消删号"}}, true, nil
	}
	name := player.DaoName
	accountID := player.AccountID
	_ = g.store.DB.Create(&model.GameLog{Level: "warn", Type: "self_delete", PlayerID: player.ID, Message: fmt.Sprintf("玩家%s（%s）完成二次确认并自行删除道籍", name, accountID)}).Error
	if err := g.players.Delete(player.ID); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🕯️ 道籍已归虚", Content: fmt.Sprintf("道号“%s”的角色与关联玩法数据已经永久删除。\n━━━━━━━━━━━\n该道号现已释放，可重新发送“入道 新道号”建立全新角色。\n旧角色的境界、物品、充值货币和进度无法恢复。", name), Actions: []string{"入道 " + name}}, true, nil
}

func (g *Game) cancelSelfDelete(player *model.Player) (GameResult, bool, error) {
	result := g.store.DB.Where("player_id = ? AND key = ?", player.ID, selfDeleteConfirmationKey).Delete(&model.PlayerValue{})
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return GameResult{}, true, result.Error
	}
	return GameResult{Title: "✅ 删号申请已取消", Content: "道籍保持不变，角色、仙府、物品和全部玩法进度均未被删除。", Actions: []string{"状态", "角色菜单"}}, true, nil
}
