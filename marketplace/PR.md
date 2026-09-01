# 商城投稿 PR 说明：xianchen（仙尘修仙文字游戏）

> 投稿方式：把 marketplace/plugins/xianchen/ 目录复制到 lvyunqi/QimenBot 的 main 分支同名路径，提交 PR。
> 一个 PR 只做一件事：首次收录 xianchen 0.1.0。
> 提交前必须把本文件末尾两条校验命令的真实输出粘贴进去。

## 插件功能、命令和事件

- 修仙文字 RPG：角色创建、修炼打坐、境界突破、功法神通、宗门与职事、灵田种植、炼丹、坊市、拍卖行、Boss 攻略、奇遇事件、称号与排行榜。
- 玩家在群聊/单聊内以自然语言命令交互（仙尘状态、修炼、突破、坊市等），主人神令支持后台管理。
- 仅处理 message 事件（官方 QQ 群消息与单聊消息），通过 pre_handle 拦截进入游戏队列。
- 回复走 BotApi send_*_rich（reply + rich-message）；定时/事件广播走 ProactiveBotApi（proactive）。

## 仓库、数字 ID、许可证和 Release

- 仓库：<https://github.com/lvyunqi/xianchen-bot>（公开）
- repository_id：1353790320
- 许可证：GPL-3.0-only（仓库根目录 LICENSE，GNU GPL v3 全文 + 双作者版权声明）
- Release：<https://github.com/lvyunqi/xianchen-bot/releases/tag/v0.1.0>
- 构建流水线：GitHub Actions（.github/workflows/release.yml），tag 一致性校验（tag = Cargo.toml = 动态描述符）→ Debian 11 容器构建 → 产物重命名 + SHA256 → attest-build-provenance → GitHub Release

## QimenBot、动态 API 和驱动场景

- 宿主要求：QimenBot >= 0.1.18, < 0.2.0
- 动态描述符：api = "0.6"，plugin_id = "xianchen"，version = "0.1.0"
- 驱动：qq-official；场景：private（C2C 单聊，实机验收）、group-at（官方 QQ 群消息；实机在 GROUP_MESSAGE_CREATE 普通群消息场景验收，兼容 GROUP_AT_MESSAGE_CREATE）；事件：message；出站：reply、proactive、rich-message
- 富媒体：官方 QQ Markdown（xiaowu 卡片），README 提供无 Markdown 权限时关闭 qq_official_markdown 走纯文本；Base64 图片已在官方 QQ 群/C2C 实机验证

## API 0.6 在线配置

- config_schema + config_ui 已编入动态库，管理面板显示表单。
- 密钥类字段无（不含任何凭据字段）；配置项：worker 进程参数、admin.listen_host/listen_port、messages.qq_official_markdown。
- 生效模式：部分配置经 #[config_change] 热生效（worker/admin/markdown 开关），其余需重启插件。

## target、glibc、大小和 SHA256

| target | 资产 | SHA256 | 大小 |
|---|---|---|---|
| x86_64-pc-windows-msvc | qimen_dynamic_plugin_xianchen-x86_64-pc-windows-msvc.dll | a2ba100ce7692c2c472378998fcfc989d502719cff62db8b656841664dd8c624 | 26502144 |
| x86_64-unknown-linux-gnu | libqimen_dynamic_plugin_xianchen-x86_64-unknown-linux-gnu.so | fa06a7f9939bf436a02dcb1242bb1c8714534800564e5c34fb22b75d0f721f23 | 25327184 |

- Linux 在 rust:1.89-bullseye（Debian 11 / glibc 2.31）容器构建，min_glibc = "2.31"。
- 两个资产均有 GitHub Artifact Attestation（run 33536041091）。

## 文件、数据库、网络、后台线程、Webhook

- 文件：宿主 data_dir 下建立 xianchen 子目录；首次启动释放嵌入式 Go worker（xianchen-worker 可执行文件）。
- 数据库：SQLite（纯 Go 驱动，无 CGO），system.schema_version = 2026.07.24.258.02。
- 网络：管理后台 HTTP（默认 127.0.0.1:8088-8098，可配置）；worker 与内核经 stdio JSON 通信（本机管道，不跨网络）。除此之外无外部网络请求（消息经宿主 API 发送）。
- 后台线程：Go worker 独立进程（内嵌释放）；admin HTTP server（goroutine）；广播/扫描定时任务。
- Webhook：无。

## 数据迁移和回滚限制

- 版本升级执行增量迁移（玩家数据保留）；rollback_safe = true（二进制可替换，未做破坏性字段删除）。
- data_schema_version = 258，与内核 2026.07.24.258.02 对应。

## 校验命令真实结果

【投稿前必须运行并粘贴】

```
cargo run --locked -p qimen-plugin-marketplace --bin qimen-marketplace-index -- --check marketplace
cargo run --locked -p qimen-plugin-marketplace --bin qimen-marketplace-index -- --check --verify-github marketplace
```

真实输出：

（在此粘贴两条命令的输出）
