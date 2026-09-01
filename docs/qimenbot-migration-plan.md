# 仙尘 → QimenBot 动态插件迁移方案

> 状态：已确认待实施 ｜ 制定日期：2026-09-01 ｜ 分支：develop
> 参考实现：[douluo-bot](https://github.com/lvyunqi/douluo-bot)（同类文字游戏插件）、[ai-news](https://github.com/lvyunqi/ai-news)（主动推送插件）

## 0. 已确认的决策

| 决策项 | 结论 |
|---|---|
| 插件类型 | QimenBot **动态插件**（Rust cdylib，API 0.6） |
| 宿主环境 | Windows x64 + Linux x64 GNU **双 target**（musl 不支持动态加载） |
| 接入协议 | QQ 官方机器人（qq-official），ID 全程为 openid 字符串 |
| 内核策略 | **保留 Go 内核 + Rust 插件桥接**（Rust 插件顶替原 C 壳角色） |
| 授权体系 | 移除（卡密/机器码/云吊销全部删除） |
| 数据 | 从零开始，无旧数据迁移 |
| 分支策略 | 开发一律在 develop；模块测试完毕（流水线绿）后合并 main |
| 构建 | 全部由 GitHub Actions 流水线完成，严禁本地构建 |

## 1. 现状架构（迁移基线）

```
Bee 宿主(32位)
  └─ 仙尘.dll（C 壳，bee_bridge.c 编译）
       ├─ RCDATA 嵌入 bee_go_worker.exe（SHA-256 校验后释放启动）
       └─ 匿名管道 stdin/stdout JSON 行 IPC ↔ Go worker 进程
            ├─ runWorker 单线程主循环：onGroupMessage/onPrivateMessage
            ├─ internal/* 游戏引擎（48,787 行，平台无关，GORM+SQLite）
            └─ 内嵌 admin 后台（go:embed web/admin，127.0.0.1:8088-8098）
```

**关键事实**（代码走读结论，详见项目记忆）：

- 仙尘本来就是"薄桥接壳 + 独立 Go worker 进程"架构——迁移 = 换壳，不动内核。
- 游戏引擎是"消息驱动的纯函数式"：无定时器（挂机/闭关/体力全懒结算），状态全在 SQLite，输出收敛为单一结构体 `GameResult{Title, Content, MarkdownContent, ImageURL, ImageOnly, Actions, BroadcastContent}`。
- 命令面：CommandTable 257 核心 + ~297 辅助/别名 ≈ 554 表项，无前缀首词精确匹配，同名词按参数形态择优，另有玩家自定义快捷键（ResolveShortcut）。
- 平台耦合几乎全部压缩在根包 main 的入口/出口两端：plugin_main.go（消息入口+发送出口+广播+头像）、bee_sdk.go（72 opcode Bee API）、license*.go（授权）、ipc.go/other/（C 壳协议）。internal/ 仅两处 QQ 专有字符串：mqqapi 内联指令链接、头像 URL 模板。
- 管理后台：/api/ 单 mux + ~37 资源通用 CRUD + Excel 导入导出，无鉴权（本地回环）。

## 2. 目标架构

```
QimenBot 宿主(qimenbotd, x64)
  └─ xianchen 动态插件（.dll / .so，Rust cdylib，API 0.6）
       ├─ rust-embed 嵌入 xianchen-worker（Go，按平台分别构建）
       ├─ init：释放 worker 到 data_dir（SHA-256 校验）→ spawn 子进程
       ├─ pre_handle 拦截器：统一命令分发（复用 Go ParseCommand 语义）
       ├─ 出站：rich_reply（markdown/keyboard 段）+ send_rich（广播）
       └─ shutdown：停止 + 杀 worker 进程 + join 线程
            ↕ stdin/stdout JSON 行（简化协议：MSG_IN / MSG_OUT）
            xianchen-worker（Go 进程，改造自现有 worker）
                 ├─ internal/* 原样保留（引擎零改动）
                 ├─ qimen_entry.go：新入口（替代 bee 事件映射）
                 └─ admin_server.go：内嵌后台原样保留（127.0.0.1:8088）
```

**职责切分**：

| 层 | 职责 |
|---|---|
| Rust 插件 | 消息进出 QimenBot 的全部边界：命令拦截分发、身份解析（qimen_context）、Markdown/Keyboard 渲染、状态图第二条媒体消息、主动广播、worker 进程生命周期、在线配置 |
| Go worker | 游戏引擎 + 管理后台；输入=结构化消息 JSON，输出=GameResult JSON（含广播） |

## 3. 关键设计映射

### 3.1 入站消息（命令入口）

**方案：拦截器统一分发**（douluo-bot 已验证该链路的完整形态）。

理由：仙尘的匹配语义（无前缀、全角空格归一、xx图鉴后缀通配、同名词按参数择优、玩家自定义快捷键、未入道静默）是完整业务语义，交给宿主通用匹配必然失真；拦截器拿 raw message_text 交给 Go 侧 `ParseCommand` 可 100% 保真。

- `#[pre_handle]` 收到消息 → 提取 `message_text`、`sender_id`（openid 字符串）、`group_id`、昵称、`qimen_context`（协议/account_id）→ 组装 MSG_IN JSON 行 → 写 worker stdin → 阻塞读 stdout（带超时，参考宿主 dynamic_plugin_timeout_secs=30 与原 C 壳 15s）。
- Go 侧返回 `{handled: bool, result: GameResult}`：handled=false → `InterceptorResponse::allow()` 放行（静默语义保真）；handled=true → 组装 rich_reply 回复并 `InterceptorResponse::block()`。
- 554 条命令不逐条 `#[command]` 注册（宿主 help 页不可见是接受的代价；游戏自带菜单/帮助命令，另生成命令清单文档）。
- 补完原设计：`GroupAccessAllowed`（群白名单，已实现未接线）在拦截器入口接上——非授权群直接放行。
- 兜底红线：拦截器回调必须快、失败 fail-open（宿主行为），不做唯一鉴权边界。

**@提及（结缘 @对方 等）**：CommandRequest.args 只含纯文本，@ 段不进参数。Rust 侧按 douluo-bot identity.rs 的 `resolve_target_mention` 模式从 raw_event_json 解析结构化 at 段，把被 @ 者的 openid 拼回参数文本（"结缘 <openid>"）传给 Go，保持原"@QQ"参数语义。

### 3.2 出站消息（GameResult → QimenBot）

| GameResult 字段 | QimenBot 实现 |
|---|---|
| MarkdownContent | `DynamicActionResponse::rich_reply` + `[{"type":"markdown","data":{"content":...}}]`（douluo-bot 模式） |
| Actions（内联指令） | **Keyboard 段**：`{"type":"keyboard","data":{"content":{"rows":[...]}}}`，≤8 按钮/条、每行 2 个、action_type=2 action_data=命令文本（douluo-bot command_keyboard 模式）；**移除 mqqapi://aio/inlinecmd 渲染** |
| Text()（回退） | 配置开关 `qq_official_markdown`（默认 true）决定 markdown/纯文本；Go 侧 Text() 保留 |
| ImageOnly/状态图 | **第二条独立媒体消息**：QQ 官方群/C2C Markdown(msg_type=2) 与本地媒体(msg_type=7) 互斥；主回复成功后（after_completion 钩子或 BotApi 实时发送）发 `image_base64` 段（Go 渲染的 JPEG ≤8MB） |
| ImageURL | 图片 URL 直接进 markdown 段或 image 段（image.*_url 后台配置已有） |
| BroadcastContent | `BotApi::for_account(account_id).send_rich("group", group_openid, "{}", segments)`（ai-news 模式）；已知群改用 Go 侧已有持久化 knownGroupIDs |

### 3.3 身份与账号分区

- `Player.AccountID` ← sender_id（openid 字符串，长度 ≤64 已够用）；**永不数字强转**。
- 多 Bot 隔离：Player 唯一索引从 AccountID 改为 (AccountID, BotAccount) 复合（从零开始，无迁移负担）；BotAccount 取 `qimen_context.account_id`，缺失时拒绝有状态命令（douluo-bot identity.rs 语义）。
- 私聊：保持现语义（Go 侧 groupID="私信"字面量），Rust 侧按 group_id 空判定。
- 头像：官机无 GetAvatar 等价 API。refreshPlayerAvatar 改为「后台可配默认头像 URL + 模板渲染」；玩家真实头像不可得是**已知功能降级**，状态图用默认头像/昵称文字替代。

### 3.4 简化后的 IPC 协议

保留 JSON 行 + base64 框架，删 72 opcode 的 api_call 回调（Go 不再直接调宿主）。协议收敛为两类：

```jsonc
// → worker（stdin）
{"type":"msg","group_id":"...","sender_id":"...","sender_name":"...","text":"状态","is_private":false,"account_id":"...","message_id":"..."}
// ← worker（stdout）
{"type":"reply","handled":true,"result":{"title":"...","content":"...","markdown":"...","image_base64":"...","image_only":true,"actions":["修炼","背包"],"broadcast":"..."}}
```

worker 启动行（ready/版本）、错误行同框架。GameResult 由 Go 侧新增 JSON 序列化（ImageOnly 时 image_base64 直接送 JPEG）。

### 3.5 在线配置（API 0.6）

`config.schema.json`（照 douluo-bot/ai-news 模式）：

| 配置节 | 字段（示例） |
|---|---|
| worker | embedded_release=true（固定）、spawn_timeout_secs、io_timeout_secs、data_subdir="xianchen" |
| messages | qq_official_markdown=true、text_fallback=true、keyboard_buttons_max=8 |
| broadcast | enabled=true、batch_interval_ms（限流退避） |
| status_image | enabled=true、mode=base64`（第二条媒体消息） |
| admin | go 侧沿用 system_settings；此处仅暴露 bind_port 提示 |

`config_apply="reload"`：shutdown 杀 worker + init 重启（`init` 可重复调用）。

## 4. Go 内核改造清单（internal/ 尽量零改动）

1. **新增** `qimen_entry.go`：stdin JSON 行 → 复用 onGroupMessage/onPrivateMessage 的内部调用链（去重→GM→ParseCommand→Shortcut→Execute）→ 回复/广播收敛输出 stdout。
2. **改造** `game.go` `Markdown()`：mqqapi 内联指令渲染删除（Actions 数组原样传出，由 Rust 渲染 keyboard）；其余保留。
3. **改造** `plugin_main.go` 出口函数剥离：sendGroupResponse/sendFriendResponse/broadcastToKnownGroups/GetAvatar 相关移入 qimen 模式或删除；knownGroups 接到 game_group_access 的持久化 knownGroupIDs。
4. **删除**：bee_sdk.go、ipc.go（保留 JSON 行工具）、other/（bee_bridge.c、BeePlugin.def、worker_runtime.go、buildmeta）、build-x86.ps1、build.bat、license.go、license_cloud.go、license_server.go、internal/licensing/、settings.go（授权设置窗）、plugin_main.go 中 bee 入口。**32 位构建链全部退役**（QimenBot 是 x64，CGO/386/Zig 约束消失，SQLite 直接用纯 Go 驱动）。
5. **调整** Player 唯一索引（3.3）；头像逻辑（3.3）。
6. **保留原样**：internal/service、model、storage、handler、admin_server.go、cmd/server、web/、全部 160 个测试。

## 5. Rust 插件设计（plugin/ 目录）

```
plugin/
├─ Cargo.toml          # cdylib；abi-stable-host-api=0.1.13 + qimen-dynamic-plugin-derive=0.1.13（同版本双包）
├─ build.rs            # 校验 worker 二进制存在（CI 先出 Go 产物再构建插件）
├─ config.schema.json / config.ui.json
├─ worker-bin/xianchen-worker{.exe,_linux}   # CI 生成后放入，rust-embed 嵌入
└─ src/
   ├─ lib.rs           # #[dynamic_plugin(api="0.6", config_apply="reload")] + init/shutdown/validate_config
   ├─ worker.rs        # 子进程管理：释放（SHA-256 校验）、spawn、stdin/stdout JSON 行、超时、杀进程
   ├─ dispatch.rs      # #[pre_handle] 统一分发：MSG_IN/MSG_OUT、群白名单、静默放行
   ├─ identity.rs      # qimen_context 解析、account_id 分区、@提及解析（照 douluo-bot）
   ├─ reply.rs         # GameResult → rich_reply（markdown/keyboard/文本）渲染
   ├─ media.rs         # 状态图第二条媒体消息（after_completion 队列 + BotApi）
   ├─ broadcast.rs     # BroadcastContent → BotApi::for_account().send_rich 循环（限流退避）
   └─ config.rs        # TOML/JSON 配置解析与校验
```

生命周期红线（技能规范）：回调同步 FFI 不 async；`#[shutdown]` 停线程 + join + 杀 worker；热重载安全（dll 卸载前子进程必须已退出，Windows 上尤其重要）。

## 6. CI/CD（GitHub Actions，全部构建在流水线）

```
ci.yml（push/PR，develop 与 main 都跑）
├─ go-test       # go vet + go test ./...（160 个测试守门）
├─ rust-check    # cargo fmt/clippy/test（--locked，含 host contract 测试：bind_host_api_v1 伪造宿主验发送契约）
└─ build-matrix  # windows-x64 + linux-gnu-x64（Debian 11 容器固定 glibc 2.31）
                   job 内顺序：先构建 Go worker（纯 Go 无 CGO，GOOS 按矩阵）
                   → 拷入 plugin/worker-bin/ → cargo build --release --target
                   → 产物证据（SHA256/size/min_glibc，照 ai-news ci-evidence 模式）→ upload-artifact

release.yml（tag v*.*.* 触发，照 douluo-bot 模式）
├─ verify   # 版本一致性（Cargo.toml = 插件声明 = tag）、api="0.6" 断言
├─ build    # 同 ci build-matrix
└─ publish  # 双 target 资产 + .sha256 → GitHub Release（商城投稿为可选后续，需公开源码）
```

模块合并纪律：模块在 develop 上开发 → 流水线全绿 → 合并 main（--no-ff，保留模块边界）→ main 的流水线再验证一次。

## 7. 风险与缓解

| 风险 | 等级 | 缓解 |
|---|---|---|
| mqqapi 内联蓝字不可用 | 高 | 用 keyboard 按钮替代（douluo-bot 实证 ≤8 按钮）；正文保留完整文字，不依赖蓝字 |
| Markdown 与本地媒体互斥 | 高 | 状态图固定走第二条独立媒体消息（douluo-bot qq_media 模式） |
| 官机主动消息额度/限流 | 中 | 广播默认关闭可配置；批量退避（batch_interval_ms）；失败重试限次 |
| 拦截器超时熔断 fail-open | 中 | worker IO 超时 < 宿主超时；超时消息降级回复"稍后再试"而非挂起 |
| 554 命令宿主不可见 | 低 | 游戏自带菜单命令；生成命令清单文档（scripts/gen-command-docs.go） |
| 热重载 DLL 卸载崩溃 | 中 | shutdown：先停拦截路由 → 杀 worker → join 线程 → 释放 Host API；跨平台进程树清理测试 |
| Go worker 单线程串行吞吐 | 低 | 与原架构一致（Bee 也是串行），不是回归；后续可在 Rust 侧加消息队列 |
| 官机无头像 API | 低 | 默认头像模板 + 昵称；已知降级 |
| 双平台产物一致性 | 低 | CI 按 target 各自嵌入对应 worker；SHA-256 证据留档 |

## 8. 实施阶段（每阶段以流水线绿为完成标准）

| 阶段 | 内容 | 验证 |
|---|---|---|
| **P0 骨架** | 删 32 位链与授权；Go worker 入口抽象（Host 接口）；plugin/ Rust 骨架（douluo-bot 模板起步）；CI 骨架（go-test + rust-check） | CI 绿；go test 全过；插件空壳可被宿主加载（人工冒烟） |
| **P1 桥接打通** | worker.rs 子进程管理（嵌入+释放+spawn）；JSON 行协议对齐；拦截器 echo 一条命令（"状态"）走通；rich_reply markdown 发送 | 流水线构建双 target 产物；官机群冒烟：发"状态"收到 markdown 回复 |
| **P2 全量命令面** | 拦截器统一分发接 Go ParseCommand；身份分区；@提及解析；静默规则；keyboard Actions；状态图第二条媒体消息 | 官机冒烟：入道/菜单/修炼/探索等核心命令全通；未入道静默正确 |
| **P3 广播与配置** | broadcast.rs + knownGroups 持久化；config.schema.json 在线配置面板；admin 后台联通 | 面板改配置生效（reload）；渡劫广播到多群冒烟 |
| **P4 发布** | release.yml 双 target 资产 + SHA256；命令清单文档；部署文档（qimenbotd plugins/bin 放置 + 管理页加载） | tag 发布出资产；干净宿主上完整部署演练 |

## 9. 开放问题（不阻塞 P0）

1. QimenBot 宿主实际部署位置/版本（需 v0.1.18+ 才支持 API 0.6）——P1 冒烟前确认。
2. 广播限流参数需官机实测标定。
3. 商城发布（需公开源码，当前为私有镜像仓库）——默认不做。
4. admin 后台鉴权加固（douluo-bot 有 session 模式可抄）——后续模块。
