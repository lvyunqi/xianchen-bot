# 数据库瘦身与数据层解耦方案

> 背景：生产库启动即 47MB，公告表一千多条。本文基于对 internal/storage、internal/model、internal/service 的逐行走查（含写入路径与清理机制证据），给出根因分析与分阶段治理方案。

## 一、一句话根因

**目录种子 ×1000 的设计一次性灌入静态内容（公告/邮件/物品/地点各一千条）+ 流水表零清理（SocialMessage/BankTransaction/DungeonRun 只增不删）+ SQLite 无 VACUUM（删了也不缩文件）**。清理的执行机构（调度器）在 config 里预留了三个 cron 表达式，但从未实现。

## 二、47MB 解剖（按贡献排序）

### 1. 目录种子 ×1000 —— 47MB 的静态主体

- `contentSeedLimit() = 1000`（seed_hundred.go:344-349，仅测试降为 3）：每个目录条目自动生成一对 `catalog_notice_<n>` 公告（seed_hundred.go:581-587）+ `catalog_mail_<n>` 邮件（547-551），n=1..1000
- FirstOrCreate 按 code 去重只防重复、**不淘汰**：1000 条公告永久驻留；seed_hundred.go:747-750 又把全部 catalog_notice 归一化为"公告"类型 → 全部进入玩家公告板
- 同机制覆盖物品、地点、CDK、任务、副本、商店等目录 → 万行级静态行 + 大量 type:text 长文本列
- 触发时机：仅 schema_version 变更后的那次启动全量 seed（db.go:61-72 有 databaseReady 短路），日常启动只跑 hotfix —— 所以是"设计生成"，不是重复插入 bug

### 2. 流水表零清理 —— 运行期持续增长

| 表 | 写入点 | 增长特征 | 清理 |
|---|---|---|---|
| SocialMessage | 每条私信/情缘/守护/通知/奇遇各 INSERT（game_social_trade_task.go:75、game_couple.go:111,245,381、game_notifications.go:153、game_endgame.go:595） | **运行期增长冠军** | ❌ |
| BankTransaction | 存/取/借/还各 1 行（game_bank.go:148,188,238,287）、转账 2 行（game_money_transfer.go:58-61） | 永久累积 | ❌ |
| DungeonRun | 每次副本结算 1 行（game_endgame.go:140,173）、挂机批量（game_afk.go:330）；查询只用当日 run_date，历史行零价值 | 永久累积 | ❌ |
| PlayerTask | 每天每玩家 INSERT 新行（game_social_trade_task.go:645，按 AssignedDate） | O(玩家×天数) 无界 | **ResetDaily 已写好但全仓零调用（死代码）** |
| TradeRecord / ContentReview / Broadcast / GameLog / OperationLog / AdminMenuLog / SlowQueryLog | 各自动作逐条 | 慢速累积 | ❌ |
| RankEntry | 全删重灌 ≤400 行（rank_repo.go:14） | 有界 ✅ | —（但调度缺失未跑） |

### 3. SQLite 无空间回收

全仓无 VACUUM/autovacuum（仅测试演练用 VACUUM INTO）。删号级联（player_repo.go:79-155 硬删 18 表）与排行榜重灌即使发生，**文件体积也不缩小**。

### 4. 读放大（放大 47MB 的感知）

noticeBoard 对 type="公告" 的查询**无 LIMIT**（game_ranking_notice.go:554-562）：每次"公告"指令把 1000 条长文本全部拉进内存再虚拟分页。

### 5. 宽表与 JSON blob 列

Player 单表 60+ 列（派生值 CombatPower 等全部落库）；大量 `type:text` 的 JSON blob 列（ProgressJSON/RewardJSON/ConditionJSON/NPCJSON…）把结构化 schema 藏进 service 手搓拼接——既是行宽来源，也是 schema 不可维护的来源。

## 三、清理机制现状

- `TaskRepository.ResetDaily`（task_repo.go:24-31）：**已写好、零调用**
- 调度器：config.go:91-93 预留 daily_reset / ranking_refresh / backup 三个 cron，cmd/server/main.go（46 行）无任何调度器实现
- 过期字段（TradeListing/BarterRequest/PlayerValue/AccountMigrationCode 的 ExpiresAt）：只有字段、无扫描删除
- 删号级联、排行榜重灌：实现正确，仅缺触发

## 四、解耦现状

1. **service 直接持有 gorm**：`store.DB` 出现 1156 次 / 106 个文件（Top：game_catalog 59、game_social_trade_task 54、game_skill_pet 37）——事务、业务、SQL 三合一
2. **Repository 半成品**：storage 下 38 个 repo 文件只服务 admin CRUD（admin.go:21-25 组装，59 处引用），游戏路径全部绕过直查 DB，两种风格并存
3. model 本身干净（纯 GORM struct），但 JSON blob 列 + service 手搓序列化把真实 schema 藏了起来
4. 测试固化直查风格（game_test.go 56 处 store.DB 断言）

## 五、治理方案

### P1 止血：调度器 + 保留期 + 空间回收（1-2 天，不动架构）

1. 实现调度器（接入 config 已有的三个 cron），落地：每日 ResetDaily（激活死代码）、排行榜刷新、备份
2. 新增 `retention.go` 策略表驱动（表 → 保留天数 → 批量 DELETE LIMIT 5000 循环）：
   - SocialMessage 60 天；Broadcast 30 天；GameLog 30 天且每玩家留最近 200 条
   - BankTransaction 180 天（财务流水）；DungeonRun 只留 7 天（查询只用当日）；
   - ContentReview 180 天；SlowQueryLog 14 天；AdminMenuLog 90 天；OperationLog 365 天（审计）
   - TradeListing/BarterRequest/PlayerValue/AccountMigrationCode 按 ExpiresAt 扫描清理
3. SQLite 连接参数加 `_pragma=auto_vacuum(1)`，清理后 `PRAGMA incremental_vacuum`；管理端加"压缩数据库"按钮（手动 VACUUM）
4. noticeBoard 加 LIMIT 与类型过滤（公告板只取最近 N 条置顶+分页）
5. 管理端加"数据体积"面板（dbstat 按 top 表展示）

### P2 种子降容：目录内容出库（架构性瘦身，收益最大）

- 目录条目 ×1000 是示例性填充：降为真实运营规模（如 50-100）或改为**按需懒生成**（玩家查询时生成 + 缓存）
- 更彻底：静态配置编译进只读档案库 `config.db`（构建期从 JSON 生成、go:embed、ATTACH READONLY），主库只存动态状态；运营可编辑类（公告/菜单）留主库
- 预期：主库 47MB → 15MB 以内，schema 迁移与启动校验时间同步大减

### P3 深度解耦（杜绝屎山的结构改造）

目标分层：`service（领域用例，只依赖接口）→ repo（接口 + gorm 实现）→ storage（连接/迁移/保留期/档案库路由）`

1. **repo 接口收口**：从 1156 处直查按热度渐进替换（先 Player/Item/Log/Social 四个热域），service 注入窄接口——可 mock、Postgres/SQLite 双驱动差异收敛到实现层
2. **流水写入统一 + 批量缓冲**：全部走 LogRepository，内部 channel + 定时 flush（500 条/批），写放大立降
3. **JSON blob 治理**：高频查询字段（进度/条件的关键键）提为真实列 + 索引，blob 只存真正的自由文本
4. **Player 拆表**：PlayerCore（身份）/ PlayerProgress（成长）/ PlayerProfile（展示档案），派生值实时计算
5. **schema 治理**：AutoMigrate 收敛为显式迁移目录（migrations/NNN_xxx.sql），种子与迁移分离；repo 声明 RetentionPolicy 元数据，新增表天然带清理

## 六、落地顺序

| 阶段 | 内容 | 工作量 | 效果 |
|---|---|---|---|
| 1 | P1 调度器 + 保留期 + VACUUM + 公告板 LIMIT | 1-2 天 | 流水止涨、文件可回收、公告板体验修复 |
| 2 | P2 种子降容/出库 | 2-3 天 | 主库预计 <15MB |
| 3 | P3.1/2 repo 收口（四个热域）+ 流水批量缓冲 | 2-3 天 | 写放大下降、可测性建立 |
| 4 | P3.3-5 JSON 治理 / Player 拆表 / 迁移目录化 | 3-5 天 | 结构长期可维护 |

每阶段独立可发布可回滚；P2/P4 涉及 data_schema_version 升级，走 live_database_rehearsal 真实库演练流程。