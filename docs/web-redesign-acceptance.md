# Web 管理端重构 · 验收记录

对照 `docs/web-redesign-plan.md` 的分期目标，逐项核对（更新于 2026-09-02，CI 1056 后）。

## 分期完成情况

| 分期 | 内容 | 状态 | 证据 |
|---|---|---|---|
| M1 | 脚手架 / 暗色主题 / 响应式布局 / 资源注册表 / 流水线接入 | 完成 | CI run 1033 全绿 |
| M2 | 字段控件（关联选择器、JSON 构建器）+ 通用 CRUD 引擎 + 标杆资源 | 完成 | CI run 1039 全绿 |
| M3 | 58 页资源字段全量迁移（57 资源 514 字段） | 完成 | CI run 1044 全绿 |
| M4 | 图表视图 / 批量操作 / 图片预览 / 空态引导 | 完成 | CI run 1049 全绿 |
| M5 | 大表虚拟滚动 / 移动端横向滚动 | 完成 | CI run 1052 全绿 |
| 主题返工 | 默认亮色 / 主色去紫改青碧 / 文案去 AI 腔 | 完成 | CI run 1056 全绿 |

## 验证方式

- 每次提交，Gitea 与 GitHub 两侧 CI 均执行：`npm install → npm run build（tsc -b 类型检查 + vite 构建）→ go vet → go test → go build`。go build 成功即证明 `go:embed web/admin` 能吃到前端产物，"构建由流水线完成、产物内嵌 Go 二进制"链路成立。
- 本地 dev（vite 5188 + mock 中间件）人工核对：布局、主题切换、页面过渡、表格搜索/排序/分页/多选、表单控件、虚拟滚动（1000 行 mock）、批量操作、空态。

## 发布前置（等待所有者授权，未执行）

1. `plugin/Cargo.toml` 与 `plugin/src/lib.rs` 版本号同步（现有 verify job 校验两者 + tag 一致）
2. 打 tag（`v0.1.1` 起）触发双平台 Release 流水线，产出内嵌新管理端的插件包
3. 下载 Release 产物回填 `marketplace/plugins/xianchen/versions/<ver>.toml` 的 sha256 / size_bytes
4. 如需更新商店投稿，另走 QimenBot 仓库 PR 流程

## 已知限制

- 监控类只读页以汇总图表呈现（后端暂无时间序列接口，历史趋势图留待后端支持后追加）
- 图片字段为 URL + 预览；直传需要后端提供上传接口
- 批量操作为逐条串行调用，超大批量时依赖后端未来批量端点
