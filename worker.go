package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"xianlv/internal/appinfo"
	"xianlv/internal/config"
	"xianlv/internal/service"
	"xianlv/internal/storage"
)

// xianchen-worker：QimenBot 动态插件的 Go 游戏内核进程。
//
// 由 Rust 插件（plugin/）spawn，通过 stdin/stdout 的 JSON 行协议通信：
//   入站 {"type":"ping"}               → {"type":"pong"}
//   入站 {"type":"msg", ...}           → P1 起实现，当前返回 not_implemented
//   入站 {"type":"shutdown"}           → 进程退出
// 协议详见 docs/qimenbot-migration-plan.md。

const (
	protocolVersion = 1
	stdioReadLimit  = 8 * 1024 * 1024
)

// InboundMessage 是 Rust 插件转发来的一条聊天消息（群聊或私信）。
// AccountID 是宿主 qimen_context 的稳定账号标识，用于多 Bot 数据分区。
type InboundMessage struct {
	Type       string `json:"type"`
	GroupID    string `json:"group_id"`     // 群聊为群 openid；私信为空
	SenderID   string `json:"sender_id"`    // 发送者 openid（字符串，禁止数字强转）
	SenderName string `json:"sender_name"`  // 发送者昵称
	Text       string `json:"text"`         // 消息纯文本（@ 段已由插件解析拼入）
	IsPrivate  bool   `json:"is_private"`   // true=好友私信
	AccountID  string `json:"account_id"`   // 宿主稳定账号；空表示宿主未配置
	MessageID  string `json:"message_id"`   // 平台消息 ID，用于去重
}

// OutboundReply 是 worker 对一条入站消息的应答。
type OutboundReply struct {
	Type    string      `json:"type"`
	Handled bool        `json:"handled"`
	Result  *GamePayload `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// GamePayload 是 service.GameResult 的协议投影：发送动作全部交给 Rust 侧执行。
type GamePayload struct {
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	Markdown     string   `json:"markdown"`
	TextFallback string   `json:"text_fallback"`
	ImageBase64  string   `json:"image_base64,omitempty"`
	ImageOnly    bool     `json:"image_only"`
	Actions      []string `json:"actions,omitempty"`
	Broadcast    string   `json:"broadcast,omitempty"`
}

func main() {
	dataDir := flag.String("data-dir", "", "运行数据目录（默认：可执行文件所在目录）")
	hostPID := flag.Int("host-pid", 0, "宿主插件进程 PID；进程退出时 worker 自行退出（看门狗）")
	flag.Parse()

	dir := strings.TrimSpace(*dataDir)
	if dir == "" {
		if executable, err := os.Executable(); err == nil {
			dir = filepath.Dir(executable)
		} else if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fatal(err)
	}
	if *hostPID > 0 {
		startHostWatcher(uint32(*hostPID))
	}
	if err := initRuntime(absDir); err != nil {
		writeRuntimeLog("worker初始化失败", err.Error())
		fatal(err)
	}
	writeRuntimeLog("worker就绪", fmt.Sprintf("data_dir=%s version=%s", absDir, appinfo.Version))
	// stdout 只用于协议输出；运行日志走 stderr 与 runtime.log，绝不污染协议流。
	if err := runStdioLoop(os.Stdin, os.Stdout); err != nil {
		writeRuntimeLog("worker主循环退出", err.Error())
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// startHostWatcher 看门狗：宿主插件进程退出后 worker 自行退出，避免孤儿进程。
func startHostWatcher(hostPID uint32) {
	go func() {
		process, err := os.FindProcess(int(hostPID))
		if err != nil {
			return
		}
		_, _ = process.Wait()
		os.Exit(0)
	}()
}

// initRuntime 初始化数据目录、数据库、游戏引擎与管理后台（幂等）。
func initRuntime(dataDir string) error {
	runtimeState.Lock()
	defer runtimeState.Unlock()
	if runtimeState.game != nil && strings.EqualFold(runtimeState.dataDir, dataDir) {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "data"), 0o755); err != nil {
		return err
	}
	cfg := config.Runtime(dataDir)
	store, err := storage.Open(cfg)
	if err != nil {
		return err
	}
	if runtimeState.store != nil {
		_ = runtimeState.store.Close()
	}
	game, err := service.NewGame(store)
	if err != nil {
		_ = store.Close()
		return err
	}
	runtimeState.dataDir = dataDir
	runtimeState.store = store
	runtimeState.game = game
	adminURL, err := startAdminServer(store, dataDir)
	if err != nil {
		// 管理后台起不来不阻塞游戏主链路，记录后继续。
		writeRuntimeLog("管理后台启动失败", err.Error())
		return nil
	}
	writeRuntimeLog("管理后台就绪", adminURL)
	return nil
}

// runStdioLoop 是协议 v0 主循环：逐行读 JSON、应答、直到 EOF 或 shutdown。
func runStdioLoop(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), stdioReadLimit)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			if err := encoder.Encode(OutboundReply{Type: "error", Error: "无法解析的协议行: " + err.Error()}); err != nil {
				return err
			}
			continue
		}
		switch envelope.Type {
		case "ping":
			reply := struct {
				Type             string `json:"type"`
				ProtocolVersion  int    `json:"protocol_version"`
				Version          string `json:"version"`
				PluginName       string `json:"plugin_name"`
				AdminURL         string `json:"admin_url,omitempty"`
			}{Type: "pong", ProtocolVersion: protocolVersion, Version: appinfo.Version, PluginName: appinfo.PluginName, AdminURL: currentAdminURL()}
			if err := encoder.Encode(reply); err != nil {
				return err
			}
		case "shutdown":
			encoder.Encode(OutboundReply{Type: "bye"})
			return nil
		case "msg":
			// P1（桥接打通）实现完整消息处理链；P0 仅回执。
			if err := encoder.Encode(OutboundReply{Type: "reply", Error: "not_implemented: P1 将接入消息处理链"}); err != nil {
				return err
			}
		default:
			if err := encoder.Encode(OutboundReply{Type: "error", Error: "未知消息类型: " + envelope.Type}); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// pluginState 汇总运行期共享状态；runtime_log 与 pipeline 都依赖它。
type pluginState struct {
	sync.RWMutex
	dataDir string
	store   *storage.Store
	game    *service.Game
}

var runtimeState pluginState
