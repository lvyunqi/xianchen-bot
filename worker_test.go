package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"xianlv/internal/service"
)

// 回归测试：initRuntime 曾在持有 runtimeState 写锁时调用 writeRuntimeLog
// （其内部取读锁），Go RWMutex 不可重入导致永久自死锁，“内核状态”永远初始化中。
func TestInitRuntimeCompletesWithoutSelfDeadlock(t *testing.T) {
	// 自管临时目录：异步 runtime.log 写入协程可能在测试结束后仍创建文件，
	// t.TempDir 的严格清理会因“directory not empty”误报失败，这里尽力清理即可。
	dir, err := os.MkdirTemp("", "xianlv-init-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	done := make(chan error, 1)
	go func() {
		done <- initRuntime(dir, "127.0.0.1", "")
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("initRuntime 失败: %v", err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("initRuntime 90 秒未返回（疑似持锁调用 writeRuntimeLog 自死锁）")
	}
	stopAdminServer()
}

func TestAdminListenAddress(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{host: "", port: 8088, want: "127.0.0.1:8088"},
		{host: "   ", port: 8088, want: "127.0.0.1:8088"},
		{host: "127.0.0.1", port: 8088, want: "127.0.0.1:8088"},
		{host: "0.0.0.0", port: 8088, want: "0.0.0.0:8088"},
		{host: "192.168.1.10", port: 8090, want: "192.168.1.10:8090"},
		{host: "::", port: 8088, want: "[::]:8088"},
		{host: "::1", port: 8088, want: "[::1]:8088"},
	}
	for _, item := range cases {
		if got := adminListenAddress(item.host, item.port); got != item.want {
			t.Fatalf("adminListenAddress(%q, %d) = %q, want %q", item.host, item.port, got, item.want)
		}
	}
}

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

func TestGameResultReplyProjectsRichPayload(t *testing.T) {
	reply, err := gameResultReply(service.GameResult{
		Title: "状态", Content: "境界：炼气", MarkdownContent: "**境界**：炼气",
		Actions: []string{"菜单"}, BroadcastContent: "世界公告",
	})
	if err != nil {
		t.Fatalf("game result projection failed: %v", err)
	}
	if reply.Type != "reply" || !reply.Handled || reply.Result == nil {
		t.Fatalf("unexpected projected reply: %+v", reply)
	}
	for _, want := range []string{"状态", "境界", "菜单"} {
		if !strings.Contains(reply.Result.Markdown, want) && !strings.Contains(reply.Result.TextFallback, want) {
			t.Fatalf("projected payload does not contain %q: %+v", want, reply.Result)
		}
	}
	if reply.Result.Broadcast != "世界公告" {
		t.Fatalf("broadcast was not projected: %+v", reply.Result)
	}
}

func TestStdioLoopPingPong(t *testing.T) {
	var output strings.Builder
	initDone := make(chan error, 1)
	close(initDone)
	err := runStdioLoop(strings.NewReader(strings.Join([]string{
		"{\"type\":\"ping\"}",
		"",
		"not-json",
		"{\"type\":\"unknown_thing\"}",
		"{\"type\":\"shutdown\"}",
		"{\"type\":\"ping\"}",
	}, "\n")), &output, initDone)
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
		KernelReady     bool   "json:\"kernel_ready\""
	}
	if err := json.Unmarshal([]byte(lines[0]), &pong); err != nil {
		t.Fatalf("ping reply is not valid JSON: %v", err)
	}
	if pong.Type != "pong" || pong.ProtocolVersion != protocolVersion || pong.Version == "" {
		t.Fatalf("unexpected pong: %+v", pong)
	}
	if !pong.KernelReady {
		t.Fatalf("expected kernel ready after resolved init, got %+v", pong)
	}
	if !strings.Contains(lines[1], "无法解析的协议行") {
		t.Fatalf("expected parse error reply, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "未知消息类型") {
		t.Fatalf("expected unknown-type reply, got %q", lines[2])
	}
	var bye OutboundReply
	if err := json.Unmarshal([]byte(lines[3]), &bye); err != nil {
		t.Fatalf("bye reply is not valid JSON: %v", err)
	}
	if bye.Type != "bye" {
		t.Fatalf("expected bye reply, got %+v", bye)
	}
}

func TestStdioLoopMsgWithoutCommandIsIgnored(t *testing.T) {
	var output strings.Builder
	initDone := make(chan error, 1)
	close(initDone)
	err := runStdioLoop(strings.NewReader("{\"type\":\"msg\",\"text\":\"状态\"}"+"\n"), &output, initDone)
	if err != nil {
		t.Fatalf("stdio loop returned error: %v", err)
	}
	var reply OutboundReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &reply); err != nil {
		t.Fatalf("reply is not valid JSON: %v", err)
	}
	if reply.Type != "reply" || reply.Handled || reply.Error != "" {
		t.Fatalf("expected unmatched message to be ignored, got %+v", reply)
	}
}

func TestStdioLoopMsgDuringInitIsDowngraded(t *testing.T) {
	var output strings.Builder
	initDone := make(chan error, 1) // 保持未写入：模拟内核初始化尚未完成
	err := runStdioLoop(strings.NewReader("{\"type\":\"msg\",\"sender_id\":\"u1\",\"text\":\"菜单\"}"+"\n"), &output, initDone)
	if err != nil {
		t.Fatalf("stdio loop returned error: %v", err)
	}
	var reply OutboundReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &reply); err != nil {
		t.Fatalf("reply is not valid JSON: %v", err)
	}
	if reply.Type != "reply" || reply.Handled {
		t.Fatalf("expected downgraded unhandled reply, got %+v", reply)
	}
	if !strings.Contains(reply.Error, "初始化中") {
		t.Fatalf("expected init-pending downgrade error, got %+v", reply)
	}
}
