# Web 管理端重构规划（shadcn/ui 版）

> 状态：规划待确认。确认后按分期动工；所有构建由流水线完成，本地不构建。

## 一、现状分析

现有 `web/admin` 是免构建单页：`index.html(93 行) + css(15KB) + app.js(62KB/746 行)`，由 `go:embed web/admin` 打进二进制，`handler.NewAdminMux` 提供静态文件与 `/api/*`。

`app.js` 已经是 schema 驱动的 CRUD：`fallbackPages` 声明 58 个页面（7 组），`schemas` 声明字段（key/label/type），通用引擎渲染列表与表单，JSON 类字段有 structured 编辑。思路是对的，问题在实现层：

### 痛点清单（按用户使用路径）

1. **关联字段全靠手填**（最痛）
   - `shop.item_id`、`synthesis_recipes.output_item_id`、`world_leylines.required_item`：手填物品 ID/名称
   - `events.drop_pool_id`：手填掉落池 ID
   - `couples.player_a_id/player_b_id`、`mails.target_id`：手填玩家 ID
   - `locations.neighbors_json`：手写 JSON 数组表达相邻地点
   - `skills.realm_required`、`minimum_realm_sequence`：手填境界顺序数字
   - `pets.evolution_target`：手填目标灵兽名
   - `world_leylines.required_root_element`：手填本源属性文本
2. **JSON 字段仍要手写结构**：`materials_json`（材料×数量）、`reward_json`（奖励×概率）、`condition_json`、`boss_reward_json` 等二十余个字段是 textarea 手写 JSON，错一个引号就存坏
3. **表格能力弱**：无全局搜索/列筛选/排序组合，千行级表（灵根 1000、灵脉 1000、玩家）全量渲染，找数据靠肉眼
4. **交互原始**：原生 input/select/checkbox、无日历选择器、无暗色模式、图标是单个汉字、无任何过渡动画、窄屏不可用
5. **反馈缺失**：无 toast/骨架屏/乐观更新，保存成败只能靠刷新验证

### 不变的资产

- 后端 `/api/<resource>` REST 面（38 个资源端点）与 `admin_resources_test` 契约
- `go:embed web/admin` 的嵌入路径与 `NewAdminMux` 的静态服务（Go 侧零改动）
- `web/admin/assets/logo.png` 与 uploads 图片目录

## 二、新前端设计

### 技术栈

| 层 | 选型 | 理由 |
|---|---|---|
| 构建 | Vite 5 + TypeScript | 秒级 HMR；产物落 `web/admin` 保持 embed 不变 |
| UI | React 18 + Tailwind CSS + shadcn/ui（Radix + CVA） | 用户指定；组件源码进仓库可深度定制 |
| 数据 | TanStack Query | 缓存/失效/乐观更新，CRUD 后列表自动刷新 |
| 表格 | TanStack Table + TanStack Virtual | 排序/筛选/列控制 + 千行虚拟滚动 |
| 表单 | react-hook-form + zod | 类型安全校验，schema 与 zod 同源生成 |
| 动画 | framer-motion + tailwindcss-animate | 用户要求"丝滑"：布局动画/页面过渡/spring 弹层 |
| 图表 | recharts | 仪表盘与监控 |
| 反馈 | sonner（toast）+ Skeleton | 保存反馈与加载态 |

### 目录结构

```
web/admin-ui/
├── src/
│   ├── lib/api.ts            # 类型化 fetch 封装 + 错误处理
│   ├── lib/resources/        # 资源注册表：58 个页面 = 1 份声明
│   │   ├── types.ts          # ResourceDef{ key,title,group,columns,form,relations }
│   │   ├── content.ts        # 物品/事件/任务/功法…
│   │   ├── gameplay.ts       # 17 个全新玩法（共享 gameplayFields）
│   │   ├── operations.ts     # 活动/邮件/商店/兑换码…
│   │   └── system.ts
│   ├── components/
│   │   ├── resource/         # 通用 CRUD 引擎：ResourceTable / ResourceForm / ResourceDetail
│   │   ├── fields/           # 字段控件：TextField / SelectField / RelationCombobox /
│   │   │                     #   JsonField(materials|rewards|conditions) / DatePicker / SwitchField
│   │   └── layout/           # Sidebar / Topbar / ThemeToggle / PageTransition
│   ├── pages/                # 特殊页：Dashboard / Monitor / Players / Menus / Reviews
│   └── main.tsx
├── vite.config.ts            # outDir: ../admin, emptyOutDir 保留 assets/logo.png
└── package.json
```

### 字段控件映射（便捷化核心）

| 现状 | 新控件 | 覆盖字段 |
|---|---|---|
| 手填物品 ID/名称 | RelationCombobox：搜索 + 显示名称 + 存 ID，单选/多选+数量 | item_id, output_item_id, required_item, evolution_target, target_id, player_*_id |
| 手写 materials/reward JSON | **材料构建器**：物品 Combobox + 数量步进器 + 行增删 + 概率输入，输出规范化 JSON | materials_json, reward_json, *_reward_json, set_bonus_json |
| 手写条件 JSON | 键值条件构建器（字段下拉 + 运算符 + 值） | condition_json, prerequisite_json, objective_json |
| 手写相邻地点 | 地点多选 Combobox（按区域分组） | neighbors_json |
| 手填境界顺序 | 境界下拉（来自 /api/realms，显示名+顺序） | realm_required, minimum_realm_*, required_root_element |
| 原生日期 | shadcn Popover+Calendar | starts_at, ends_at, expires_at, reviewed_at |
| 原生 checkbox | Switch + 语义色 | enabled/published/pinned/banned… |
| 枚举 select | shadcn Select（保留现有枚举清单） | category_name, rarity_name, effect_type… |

### 表格体验

- 顶部：全局搜索（防抖）+ 常用筛选片（枚举字段自动生成）+ 列设置
- 行：排序、虚拟滚动（>200 行启用）、行点击开右侧详情抽屉（编辑/删除/复制编码）
- 批量：勾选批量删除/批量启停，危险操作 AlertDialog 确认
- 空态/加载态：Skeleton 骨架 + 插图空态

### 动画与美感

- 主题：默认暗色"紫金水墨"（violet 主色 + amber 点缀），一键切换浅色；CSS 变量走 shadcn 标准 token
- 页面过渡：路由切换 200ms fade+8px slide；列表行 stagger 30ms 级联
- 弹层：Sheet/Dialog spring（stiffness 300, damping 30）；数字卡片 CountUp
- 骨架屏优先，杜绝白屏与布局跳动
- 字体 Inter + 系统中文栈；卡片圆角 xl + 细边框 + 悬浮微升

### 自适应

- ≥1024px：固定侧栏（可折叠成图标栏，动画过渡）
- 768–1024px：图标侧栏
- <768px：顶栏汉堡 + Sheet 抽屉侧栏；表格自动切"卡片列表"（显示 3 关键列 + 展开全部），表单改全宽单列
- 监控图表在小屏折叠为纵向堆叠

## 三、打包构建一体（流水线）

- 产物直接构建进二进制：`vite build` 输出到 `web/admin`（覆盖 index.html/assets/css/js），`go:embed web/admin` 与 `NewAdminMux` 零改动
- `web/admin-ui/node_modules` 与 `web/admin` 产物均不进仓库（.gitignore 白名单保留 logo.png）
- CI/Release 流水线在 `go build` 前插入步骤：Node 20 → npm ci → npm run build（linux 容器与 windows job 都加，因为两边都要 embed）；CI 增加"前端构建产物存在性校验"
- 开发期本地预览：`npm run dev` 代理 /api 到运行中的 admin 端口（本地不起 Go，纯联调服务器时用线上地址）

## 四、实施分期

| 期 | 内容 | 验收 |
|---|---|---|
| M1 | 脚手架 + 布局 + 主题 + API 层 + 资源注册表机制 | 布局/暗色/路由跑通，dashboard 只读 |
| M2 | 通用 CRUD 引擎 + 字段控件全家桶 + 3 个标杆资源（items、realms、players） | 表格/表单/关联选择/JSON 构建器全流程 |
| M3 | 58 页全量铺开（声明式注册，gameplay 17 页共享模板）+ menus/reviews/mails 特殊交互 | 全部资源可增删改查 |
| M4 | Dashboard/Monitor 图表 + 上传图片 + 批量操作 + 空态打磨 | 运营闭环 |
| M5 | 动画打磨 + 移动端适配 + 虚拟滚动压测 + Lighthouse | 丝滑验收 |

每期合入 develop，CI 绿后再进下一期；前端构建接入流水线从 M1 开始（保证后续每次 tag 出包都内嵌最新 UI）。

## 五、风险与后续

- 后端列表接口无分页/搜索参数：前端先做客户端分页与虚拟滚动；玩家表数据量增长后，后端补 `?page/keyword`（P2，接口兼容）
- 无登录鉴权：维持现状（默认 127.0.0.1 + 网络隔离），预留登录页与 token 机制接口位（P2）
- structured JSON 的历史脏数据：编辑器需容错解析（失败降级为源码模式并提示）
