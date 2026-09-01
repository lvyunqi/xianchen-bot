package service

import (
	"strings"
	"testing"
	"time"

	"xianlv/internal/appinfo"
)

func TestRuntimeOverviewShowsLiveIdentityAndHealth(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "runtime-status-player", "巡天司辰")
	game.startedAt = time.Now().Add(-26*time.Hour - 3*time.Minute)

	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "运行状态"))
	if err != nil || !handled {
		t.Fatalf("runtime status: handled=%v err=%v result=%+v", handled, err, result)
	}
	wants := []string{
		"运行状态：正常",
		"插件载入：正常",
		"指令系统：正常",
		"框架名称：" + appinfo.FrameworkName,
		"插件名称：" + appinfo.PluginName,
		"插件版本：v" + appinfo.Version,
		"数据库版本：",
		"作者：" + appinfo.Authors,
		"开源地址：" + appinfo.SourceURL,
		"启动时间：",
		"持续运行：1天2小时",
		"数据库连接：正常",
		"数据存储：SQLite本地数据",
		"消息模式：",
		"状态显示：",
		"数据载入：",
	}
	for _, want := range wants {
		if !strings.Contains(result.Content, want) {
			t.Fatalf("runtime status missing %q:\n%s", want, result.Content)
		}
	}

	alias, handled, err := game.Execute("group", player.AccountID, mustParse(t, "插件状态"))
	if err != nil || !handled || alias.Title != result.Title || !strings.Contains(alias.Content, "插件版本：v"+appinfo.Version) || !strings.Contains(alias.Content, "数据库连接：正常") {
		t.Fatalf("plugin status alias mismatch: handled=%v err=%v result=%+v", handled, err, alias)
	}
}
