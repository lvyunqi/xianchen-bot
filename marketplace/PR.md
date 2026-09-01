# xianchen 0.1.0 首次收录

## 这是什么

仙尘，修仙题材的群聊文字游戏。玩家在群里发命令就能修炼、突破境界、加宗门、种灵田、炼丹、逛坊市和拍卖行，也有 Boss 和奇遇事件。之前斗罗大陆插件是我们自己群里一直在跑的，这是同一套玩法的修仙版。

游戏内核是 Go 写的独立进程，直接嵌在动态库里，插件第一次启动时自动释放到数据目录，用户不用额外装东西。

## 命令和事件

- 玩家命令是自然语言风格：仙尘状态、修炼、突破、坊市、拍卖行 等；主人另有神令做后台管理
- 只处理 message 事件（官方 QQ 的群聊和单聊），pre_handle 拦截进游戏队列
- 回复走 send_*_rich（reply + rich-message），定时广播走 ProactiveBotApi

## 仓库和许可

- 仓库：https://github.com/lvyunqi/xianchen-bot （repository_id 1353790320，公开）
- GPL-3.0-only。LICENSE 是 GNU v3 全文加版权头，suiyuan 是原作者，mryunqi 是维护者
- Release：https://github.com/lvyunqi/xianchen-bot/releases/tag/v0.1.0
- 构建在 GitHub Actions：tag、Cargo.toml、动态描述符三方一致性校验，Linux 在 Debian 11 容器里出包，产物重命名 + SHA256 + attest-build-provenance

## 兼容范围

QimenBot >= 0.1.18, < 0.2.0，动态 API 0.6。

驱动只声明了 qq-official：私聊（C2C）和群消息。我们的群一直在官方 QQ 上实机跑，群消息走 GROUP_MESSAGE_CREATE（也兼容 @ 消息），单聊同样测过。OneBot 11 没实测过，就不写了。

富媒体是官方 QQ 的 Markdown 卡片。没拿到 Markdown 权限的机器人把 qq_official_markdown 配成 false 就走纯文本。

## 在线配置

API 0.6 的 schema 编在动态库里，管理面板直接出表单：worker 进程参数、管理后台监听地址端口、官方 QQ Markdown 开关。没有密钥类字段。部分配置热生效，改不了的要重启插件。

## 产物

| target | 文件 | SHA256 | 大小 |
|---|---|---|---|
| x86_64-pc-windows-msvc | qimen_dynamic_plugin_xianchen-x86_64-pc-windows-msvc.dll | a2ba100ce7692c2c472378998fcfc989d502719cff62db8b656841664dd8c624 | 26502144 |
| x86_64-unknown-linux-gnu | libqimen_dynamic_plugin_xianchen-x86_64-unknown-linux-gnu.so | fa06a7f9939bf436a02dcb1242bb1c8714534800564e5c34fb22b75d0f721f23 | 25327184 |

Linux 包在 rust:1.89-bullseye（Debian 11，glibc 2.31）里构建，min_glibc = "2.31"。两个文件都有 build provenance attestation。

## 部署和资源

- 数据在宿主 data_dir 的 xianchen/ 子目录，SQLite，纯 Go 驱动无 CGO
- 管理后台 HTTP 默认只听 127.0.0.1（端口 8088 起可配置），和 worker 之间走本机 stdio
- 除了宿主的消息 API，没有别的对外网络请求；没有 Webhook
- 后台有 worker 进程、管理 server 和几个定时任务

## 数据迁移

升级走增量迁移，玩家数据保留。data_schema_version 258，没删过字段，rollback_safe = true。

## 校验输出

两条校验命令的真实输出在下方评论补充。
