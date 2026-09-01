package main

import (
	"bufio"
	"encoding/base64"
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
	"xianlv/internal/handler"
	"xianlv/internal/service"
	"xianlv/internal/storage"
)

// xianchen-worker：QimenBot 动态插件的 Go 游戏内核进程。
//
// 由 Rust 插件（plugin/）spawn，通过 stdin/stdout 的 JSON 行协议通信：
//   入站 {"type":"ping"}               → {"type":"pong"}
//   入站 {"type":"msg", ...}           → 执行游戏命令并返回 GameResult
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
	Title            string   `json:"title"`
	Content          string   `json:"content"`
	Markdown        string   `json:"markdown"`
	MarkdownContent string   `json:"markdown_content,omitempty"`
	TextFallback    string   `json:"text_fallback"`
	ImageURL        string   `json:"image_url,omitempty"`
	ImageBase64      string   `json:"image_base64,omitempty"`
	ImageOnly        bool     `json:"image_only"`
	Actions          []string `json:"actions,omitempty"`
	Broadcast        string   `json:"broadcast,omitempty"`
	BroadcastTargets []string `json:"broadcast_targets,omitempty"`
}

func main() {
	dataDir := flag.String("data-dir", "", "运行数据目录（默认：可执行文件所在目录）")
	hostPID := flag.Int("host-pid", 0, "宿主插件进程 PID；进程退出时 worker 自行退出（看门狗）")
	adminHost := flag.String("admin-host", "127.0.0.1", "管理后台监听地址（默认 127.0.0.1；可设 0.0.0.0 或指定 IP/主机名）")
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
	// 内核初始化（建库、迁移、种子、管理后台）放后台执行：
	// ping 立即可达，首次冷启动不再拖垮插件握手；msg 在就绪前立即降级返回。
	initDone := make(chan error, 1)
	go func() {
		if err := initRuntime(absDir, *adminHost); err != nil {
			writeRuntimeLog("worker初始化失败", err.Error())
			initDone <- err
			return
		}
		writeRuntimeLog("worker就绪", fmt.Sprintf("data_dir=%s version=%s", absDir, appinfo.Version))
		initDone <- nil
	}()
	// stdout 只用于协议输出；运行日志走 stderr 与 runtime.log，绝不污染协议流。
	if err := runStdioLoop(os.Stdin, os.Stdout, initDone); err != nil {
		writeRuntimeLog("worker主循环退出", err.Error())
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// initRuntime 初始化数据目录、数据库、游戏引擎与管理后台（幂等）。
func initRuntime(dataDir, adminHost string) error {
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
	adminURL, err := startAdminServer(store, dataDir, adminHost)
	if err != nil {
		// 管理后台起不来不阻塞游戏主链路，记录后继续。
		writeRuntimeLog("管理后台启动失败", err.Error())
		return nil
	}
	writeRuntimeLog("管理后台就绪", adminURL)
	return nil
}

// processInbound 执行一条完整消息，保持游戏内核原有的命令语义。
func processInbound(message InboundMessage) (OutboundReply, error) {
	message.Text = normalizeMessage(message.Text)
	if message.Type != "msg" {
		return OutboundReply{Type: "error", Error: "消息类型必须为 msg"}, nil
	}
	if strings.TrimSpace(message.SenderID) == "" {
		return OutboundReply{Type: "reply", Handled: false}, nil
	}
	groupID := strings.TrimSpace(message.GroupID)
	if message.IsPrivate || groupID == "" {
		groupID = "私信"
	}
	scope := "group"
	if groupID == "私信" {
		scope = "private"
	}
	if !acceptMessageOnce(scope, groupID, message.SenderID, message.MessageID, message.Text) {
		return OutboundReply{Type: "reply", Handled: false}, nil
	}

	runtimeState.RLock()
	game := runtimeState.game
	runtimeState.RUnlock()
	if game == nil {
		return OutboundReply{}, fmt.Errorf("游戏内核尚未初始化")
	}
	utility := utilityResult(groupID, message.SenderID, message.Text)
	if utility != nil {
		return gameResultReply(*utility)
	}
	if gmCommand, ok := service.ParseGMCommand(message.Text); ok {
		if groupID != "私信" {
			allowed, status, err := game.GroupAccessAllowed(groupID)
			if err != nil {
				return OutboundReply{}, err
			}
			if !allowed && gmCommand.Name != "群审核" {
				blocked := game.GroupAccessBlockedResult(groupID, status)
				return gameResultReply(blocked)
			}
			if allowed {
				rememberKnownGroup(groupID)
			}
		}
		result, handled, err := game.ExecuteGM(message.SenderID, gmCommand)
		if err != nil {
			return gameResultReply(service.GameResult{Title: "神令未成", Content: err.Error()})
		}
		if !handled {
			return OutboundReply{Type: "reply", Handled: false}, nil
		}
		return gameResultReply(result)
	}
	command, ok := parseInboundCommand(game, message.SenderID, message.Text)
	if !ok {
		return OutboundReply{Type: "reply", Handled: false}, nil
	}
	if groupID != "私信" {
		allowed, status, err := game.GroupAccessAllowed(groupID)
		if err != nil {
			return OutboundReply{}, err
		}
		if !allowed && command.Spec.ID != 1191 {
			blocked := game.GroupAccessBlockedResult(groupID, status)
			return gameResultReply(blocked)
		}
		if allowed {
			rememberKnownGroup(groupID)
		}
	}
	result, handled, err := game.Execute(groupID, message.SenderID, command)
	if err != nil {
		return OutboundReply{}, err
	}
	if !handled {
		return OutboundReply{Type: "reply", Handled: false}, nil
	}
	return gameResultReply(result)
}

func parseInboundCommand(game *service.Game, accountID, text string) (handler.ParsedCommand, bool) {
	command, ok := handler.ParseCommand(text)
	if ok {
		return command, true
	}
	return game.ResolveShortcut(accountID, text)
}

func gameResultReply(result service.GameResult) (OutboundReply, error) {
	payload := GamePayload{
		Title: result.Title, Content: result.Content, Markdown: result.Markdown(),
		MarkdownContent: result.MarkdownContent, TextFallback: result.Text(),
		ImageURL: result.ImageURL, ImageOnly: result.ImageOnly, Actions: result.Actions,
		Broadcast: result.BroadcastContent,
	}
	if strings.TrimSpace(result.BroadcastContent) != "" {
		payload.BroadcastTargets = knownGroups()
	}
	if result.ImageOnly && strings.TrimSpace(result.ImageURL) != "" {
		image, err := os.ReadFile(result.ImageURL)
		_ = os.Remove(result.ImageURL)
		if err != nil {
			return OutboundReply{}, fmt.Errorf("读取状态图: %w", err)
		}
		payload.ImageBase64 = base64.StdEncoding.EncodeToString(image)
		// ImageOnly 的本地临时路径只供 worker 读取，不跨进程暴露给插件。
		payload.ImageURL = ""
	}
	return OutboundReply{Type: "reply", Handled: true, Result: &payload}, nil
}

// runStdioLoop 是协议 v0 主循环：逐行读 JSON、应答、直到 EOF 或 shutdown。
// initDone 携带后台内核初始化结果；未就绪时 ping 正常应答，msg 立即降级返回。
func runStdioLoop(input io.Reader, output io.Writer, initDone <-chan error) error {
	initResolved := false
	initErr := error(nil)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), stdioReadLimit)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !initResolved {
			select {
			case initErr = <-initDone:
				initResolved = true
			default:
			}
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
			kernelReady := initResolved && initErr == nil
			kernelError := ""
			if initErr != nil {
				kernelError = initErr.Error()
			}
			reply := struct {
				Type             string `json:"type"`
				ProtocolVersion  int    `json:"protocol_version"`
				Version          string `json:"version"`
				PluginName       string `json:"plugin_name"`
				AdminURL         string `json:"admin_url,omitempty"`
				KernelReady      bool   `json:"kernel_ready"`
				InitError        string `json:"init_error,omitempty"`
			}{Type: "pong", ProtocolVersion: protocolVersion, Version: appinfo.Version, PluginName: appinfo.PluginName, AdminURL: currentAdminURL(), KernelReady: kernelReady, InitError: kernelError}
			if err := encoder.Encode(reply); err != nil {
				return err
			}
		case "shutdown":
			encoder.Encode(OutboundReply{Type: "bye"})
			return nil
		case "msg":
			var message InboundMessage
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				if err := encoder.Encode(OutboundReply{Type: "error", Error: "无法解析消息: " + err.Error()}); err != nil {
					return err
				}
				continue
			}
			switch {
			case !initResolved:
				// 内核仍在初始化：立即降级返回，绝不阻塞插件侧的请求超时。
				if err := encoder.Encode(OutboundReply{Type: "reply", Handled: false, Error: "游戏内核初始化中，请稍后再试"}); err != nil {
					return err
				}
				continue
			case initErr != nil:
				if err := encoder.Encode(OutboundReply{Type: "reply", Error: "游戏内核初始化失败，请查看 runtime.log: " + initErr.Error()}); err != nil {
					return err
				}
				continue
			}
			reply, err := processInbound(message)
			if err != nil {
				reply = OutboundReply{Type: "reply", Error: err.Error()}
			}
			if err := encoder.Encode(reply); err != nil {
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
