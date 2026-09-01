# 仙尘 · 尘缘入道，一念登仙

修仙题材 QQ 群文字游戏，以 [QimenBot](https://github.com/lvyunqi/QimenBot) 动态插件形式发布：Rust 薄桥接层负责插件生命周期、消息拦截与 worker 进程管理，Go 内核承载全部游戏语义、SQLite 数据与管理后台。

- **玩法**：入道修炼、探索战斗、宗门经营、灵兽功法、副本活动，数百条无前缀指令、40 余类菜单
- **架构**：插件（`libqimen_dynamic_plugin_xianchen.so` / `.dll`）⇄ Go worker（JSON over stdio，协议 v1），惰性初始化，握手不阻塞
- **数据**：SQLite 本地库，自动建库与增量迁移，内置管理后台可视化维护全部游戏数据
- **消息**：QQ 官方原生 Markdown 优先，可一键回退纯文本

## 安装

1. 从 [Releases](https://gitea.acmecloud.cn/mryunqi/dengxian-bot/releases) 下载对应平台的压缩包（x86_64 Linux / Windows）。
2. 将 `libqimen_dynamic_plugin_xianchen.so`（Windows 为 `.dll`）放入 QimenBot 插件目录 `plugins/bin/`，重载插件。
3. 插件自动抽取 Go worker 到数据目录 `xianchen/bin/` 并完成首次建库（首次建库需要一些时间）；发送 `仙尘状态` 查看内核就绪情况与管理后台地址。

## 在线配置

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `worker.enabled` | `true` | 是否启用 Go 内核进程 |
| `worker.spawn_timeout_secs` | `20` | worker 启动握手超时（5-120 秒） |
| `worker.io_timeout_secs` | `25` | 单次协议交互超时（5-29 秒） |
| `worker.data_subdir` | `xianchen` | 数据目录名（宿主数据目录下的单级子目录） |
| `admin.listen_host` | `127.0.0.1` | 管理后台监听地址；可设 `0.0.0.0` 或指定 IP 供远程访问，公网暴露请注意安全；修改后重载插件生效 |
| `messages.qq_official_markdown` | `true` | 官方原生 Markdown 渲染；关闭则使用纯文本 |

管理后台端口在 8088-8098 间自动选择，地址见 `仙尘状态` 诊断或数据目录下的 `runtime.log`。

## 开始游戏

所有游戏指令均无前缀，常用入口：

```text
入道 道号        状态            菜单            修炼 / 出关
探索            地图            前往 东洲·云海剑台
首领            讨伐            挂机 猎妖       挂机结算
创功 剑道 名     我的创功        功法分享        物品 灵果
查询 1          寻缘            副本            宗门
活动菜单        活动总览        七日目标        新秀榜
体力            竞技段位        大世界          仙尘介绍
```

未入道玩家发送除 `入道`、`获取ID`、`获取群ID` 以外的游戏指令时，插件保持静默。发送 `系统` 打开系统菜单，`运行状态` 查看内核运行详情。

## 主人与神令

后台“系统参数”的 `owner.user_id` 设置主人开放平台用户 ID；“管理员设置”可增加管理 ID、角色和启用状态。群内神令无前缀，按护法、长老、宗主、仙尊、道祖五级授权，主人自动拥有道祖权限。未授权账号发送神令时保持静默，所有成功、失败及越权操作都会写入神令操作审计。

## 修炼规则

发送 `修炼` 只会开始计时，不会立即增加修为。默认闭关至少 5 分钟，单次最多结算 14 天；重复发送 `修炼` 只显示当前进度，发送 `出关` 才结算。最短/最长时间、基础收益、灵根倍率和仙侣倍率均可在后台修改。

体力基础上限 100、每分钟自动恢复 10 点；每提升一个大境界，上限永久增加 100 点、恢复速度增加 10 点（不设上限）。体力按在线或离线经过时间自动结算，发送 `体力` 查看详情。

## 活动中心

`活动菜单` 进入七日目标、境界冲刺、七日福利、开服密令、道友召集、新秀榜和庆典专属玩法；`活动总览` 分页显示进行中、即将开启、已结束、剩余时间与本人参与状态。奖励领取、密令兑换和邀请绑定均使用数据库事务，同一奖励不可重复领取。

## 数据后台

管理后台随 worker 自动启动，提供动态菜单、仪表盘、内容审核、系统监控和完整游戏数据管理；地图、竞技段位、符箓、秘境、仙魔战场、宗门战争、宇宙星河等模块均有独立页面，支持图片上传与 JSON / Excel 导入导出。

需要独立于插件单独运行后台时：

```powershell
go run ./cmd/server
```

## 玩家交流

点击链接加入群聊【斗罗大陆·S2公测一群】：[557647235](https://qun.qq.com/universal-share/share?ac=1&authKey=lDlAIR7KPU1Bz31kyFzgvI5lT111SPGu8rTx1fpM2a4oJgJ1IYRESmopYK7HwOi%2B&busi_data=eyJncm91cENvZGUiOiI1NTc2NDcyMzUiLCJ0b2tlbiI6IkRmVWpsYnlZK29rbUh4ZCttVVVwb1Z1akdocGJ1K2FEV1JIcUkxb1lvbDNkbFgyQ1ZYM3g1VXBnTVpQSlNJd2wiLCJ1aW4iOiI0MzQ2NTgxOTgifQ%3D%3D&data=i_oqUZAxPVfVrpnyoVhHAW14wb0i3EG_pYpEsqHg8_f4am4TCwrP11KD5JzJ_UIcIfDQIx65h_-NYKYRIrb6Hg&svctype=4&tempid=h5_group_info)

## 作者

随缘 · 夜空

## 许可证

本项目以 [GPL-3.0](LICENSE) 协议开源。原作者 随缘（suiyuan），维护者 夜空（mryunqi）。本程序为自由软件，你可以依据 GNU 通用公共许可证第 3 版重新发布或修改它。
