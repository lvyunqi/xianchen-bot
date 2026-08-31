package main

import (
	"encoding/json"
	"strings"
	"testing"

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

func TestPlainTextResponse(t *testing.T) {
	result := service.GameResult{Title: "状态", Content: "境界：炼气", Actions: []string{"菜单"}}
	text := result.Text()
	for _, want := range []string{"【状态】", "境界：炼气", "可用指令：菜单"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}

func TestStdioLoopPingPong(t *testing.T) {
	var output strings.Builder
	err := runStdioLoop(strings.NewReader(strings.Join([]string{
		"{\"type\":\"ping\"}",
		"",
		"not-json",
		"{\"type\":\"unknown_thing\"}",
		"{\"type\":\"shutdown\"}",
		"{\"type\":\"ping\"}",
	}, "\n")), &output)
	if err != nil {
		t.Fatalf("stdio loop returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 protocol replies, got %d: %q", len(lines), output.String())
	}
	var pong struct {
		Type            string "json:\"type\""
		ProtocolVersion int    "json:\"protocol_version\""
		Version         string "json:\"version\""
	}
	if err := json.Unmarshal([]byte(lines[0]), &pong); err != nil {
		t.Fatalf("ping reply is not valid JSON: %v", err)
	}
	if pong.Type != "pong" || pong.ProtocolVersion != protocolVersion || pong.Version == "" {
		t.Fatalf("unexpected pong: %+v", pong)
	}
	if !strings.Contains(lines[1], "无法解析的协议行") {
		t.Fatalf("expected parse error reply, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "未知消息类型") {
		t.Fatalf("expected unknown-type reply, got %q", lines[2])
	}
	if strings.TrimSpace(lines[3]) != "{\"type\":\"bye\"}" {
		t.Fatalf("expected bye reply, got %q", lines[3])
	}
}

func TestStdioLoopMsgIsNotImplementedYet(t *testing.T) {
	var output strings.Builder
	err := runStdioLoop(strings.NewReader("{\"type\":\"msg\",\"text\":\"状态\"}"+ "\n"), &output)
	if err != nil {
		t.Fatalf("stdio loop returned error: %v", err)
	}
	var reply OutboundReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &reply); err != nil {
		t.Fatalf("reply is not valid JSON: %v", err)
	}
	if reply.Type != "reply" || reply.Error == "" {
		t.Fatalf("expected P0 not-implemented reply, got %+v", reply)
	}
}
