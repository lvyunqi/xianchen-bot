package main

import (
	"strings"
	"testing"

	"xianlv/internal/appinfo"
	"xianlv/internal/service"
)

func TestMessageDedupUsesCallbackMessageID(t *testing.T) {
	if !acceptMessageOnce("group", "group-a", "user-a", "message-unique-a", "状态") {
		t.Fatal("first callback was rejected")
	}
	if acceptMessageOnce("group", "group-a", "user-a", "message-unique-a", "状态") {
		t.Fatal("duplicate callback was accepted")
	}
	if !acceptMessageOnce("group", "group-a", "user-a", "message-unique-b", "状态") {
		t.Fatal("different message ID was rejected")
	}
}

func TestMessageErrorRecognition(t *testing.T) {
	err := messageResultError(`{"message":"消息参数错误","code":40034124,"err_code":40034124}`)
	if err == nil || !strings.Contains(err.Error(), "40034124") {
		t.Fatalf("platform error was not recognized: %v", err)
	}
	if err := messageResultError(""); err == nil {
		t.Fatal("empty framework result must be treated as failure")
	}
	if err := messageResultError("message-id"); err != nil {
		t.Fatalf("message ID was rejected: %v", err)
	}
	if err := messageResultError("普通消息无权限，发送失败"); err == nil {
		t.Fatal("textual framework error was accepted as a message ID")
	}
}

func TestPlainTextResponse(t *testing.T) {
	result := service.GameResult{Title: "状态", Content: "境界：炼气", Actions: []string{"菜单"}}
	text := result.Text()
	for _, want := range []string{"【状态】", "境界：炼气", "可用指令：菜单"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}

func TestNativeMarkdownResponseContainsInlineCommand(t *testing.T) {
	result := service.GameResult{Title: "状态", Content: "境界：炼气", Actions: []string{"菜单"}}
	content := result.Markdown()
	for _, want := range []string{"## 状态", "境界：炼气", "mqqapi://aio/inlinecmd?command=%E8%8F%9C%E5%8D%95"} {
		if !strings.Contains(content, want) {
			t.Fatalf("markdown %q does not contain %q", content, want)
		}
	}
}

func TestReleaseNoticeIsPlayerFacingAndComplete(t *testing.T) {
	result := releaseNoticeResult()
	if !strings.Contains(result.Title, "v"+appinfo.Version) {
		t.Fatalf("release title does not contain current version: %q", result.Title)
	}
	for _, want := range []string{"每级气血+24", "双攻各+7", "双防各+5", "永久属性", "真实十槽位", "不扣玄铁", "万象归元礼", "全服补偿"} {
		if !strings.Contains(result.Content, want) {
			t.Fatalf("release content does not contain %q: %q", want, result.Content)
		}
	}
	for _, forbidden := range []string{"后台", "接口", "数据库", "配置"} {
		if strings.Contains(result.Content, forbidden) {
			t.Fatalf("release content contains internal wording %q: %q", forbidden, result.Content)
		}
	}
}
