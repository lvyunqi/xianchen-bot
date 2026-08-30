package handler

import "testing"

func TestCommandTableCoversAll257Functions(t *testing.T) {
	if got := len(CommandTable); got != 257 {
		t.Fatalf("CommandTable length = %d, want 257", got)
	}
	for index, spec := range CommandTable {
		wantID := index + 1
		if spec.ID != wantID {
			t.Fatalf("CommandTable[%d].ID = %d, want %d", index, spec.ID, wantID)
		}
		if spec.Command == "" {
			t.Fatalf("CommandTable[%d] has an empty command", index)
		}
	}
}

func TestParseCommandRequiresNoPrefix(t *testing.T) {
	parsed, ok := ParseCommand("状态")
	if !ok || parsed.Spec.ID != 3 {
		t.Fatalf("bare command was not parsed: parsed=%+v ok=%v", parsed, ok)
	}

	for _, message := range []string{"#状态", "/状态", "！状态", "!状态"} {
		if parsed, ok := ParseCommand(message); ok {
			t.Errorf("prefixed command %q unexpectedly parsed as %+v", message, parsed)
		}
	}
}

func TestMenuCommandWithoutPrefix(t *testing.T) {
	parsed, ok := ParseCommand("菜单 修炼")
	if !ok || parsed.Spec.ID != 1000 || parsed.RawArguments != "修炼" {
		t.Fatalf("菜单 command did not parse: %+v, %v", parsed, ok)
	}
}

func TestCategoryMenuIsAStandaloneCommand(t *testing.T) {
	for _, test := range []struct {
		message  string
		category string
	}{{"角色菜单", "角色"}, {"秘境争夺菜单", "秘境争夺"}, {"宇宙星河菜单", "宇宙星河"}, {"系统菜单", "系统"}} {
		parsed, ok := ParseCommand(test.message)
		if !ok || parsed.Spec.ID != 1000 || parsed.RawArguments != test.category {
			t.Errorf("ParseCommand(%q) = %+v, %v", test.message, parsed, ok)
		}
	}
}

func TestParseNewCommandsWithoutPrefix(t *testing.T) {
	tests := []struct {
		message string
		wantID  int
	}{
		{"创宗 太一门", 101},
		{"炼药 筑基丹", 111},
		{"强宝 青冥剑", 120},
		{"副本", 121},
		{"竞榜", 129},
		{"抽签", 132},
		{"目标 飞升仙界", 139},
		{"评价", 140},
		{"地图", 241},
		{"前往 青云山脚", 242},
		{"系统", 243},
		{"位置", 252},
		{"攻击", 253},
		{"仙缘奇遇图鉴 2", 1121},
		{"仙缘奇遇图鉴 200", 1121},
		{"技能", 254},
		{"防御", 255},
		{"投降", 256},
		{"帮助", 257},
		{"指令大全 2", 1079},
		{"怎么玩", 1079},
		{"体力", 1115},
		{"修复公告 2", 1116},
		{"通知 2", 1117},
		{"通知未读", 1118},
		{"清理已读通知", 1119},
		{"大世界", 1114},
		{"领取挂机", 247},
		{"收获挂机", 247},
		{"挂机领取", 247},
		{"挂机收获", 247},
		{"结束挂机", 247},
		{"挂机结束", 247},
		{"挂机状态", 246},
	}

	for _, test := range tests {
		parsed, ok := ParseCommand(test.message)
		if !ok || parsed.Spec.ID != test.wantID {
			t.Errorf("ParseCommand(%q) = %+v, %v; want ID %d", test.message, parsed, ok, test.wantID)
		}
	}
}

func TestParseTransferCommandByArgumentType(t *testing.T) {
	for _, test := range []struct {
		message string
		wantID  int
	}{
		{"传功 123456 100", 41},
		{"传功 123456 太虚诀", 74},
	} {
		parsed, ok := ParseCommand(test.message)
		if !ok || parsed.Spec.ID != test.wantID {
			t.Errorf("ParseCommand(%q) = %+v, %v; want ID %d", test.message, parsed, ok, test.wantID)
		}
	}
}
