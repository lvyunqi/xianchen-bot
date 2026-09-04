import type { LucideIcon } from "lucide-react"

export type FieldType =
  | "text"
  | "image"
  | "number"
  | "textarea"
  | "select"
  | "bool"
  | "datetime"
  | "json"
  | "relation"
  | "system"

export interface FieldDef {
  key: string
  label: string
  type: FieldType
  options?: string[]
  placeholder?: string
  /** relation: 从哪个资源拉选项 */
  relation?: { resource: string; labelKey: string }
  /** json: 结构化编辑器子类型 */
  jsonKind?: "materials" | "rewards" | "conditions" | "pairs" | "raw"
  /** 列表里默认隐藏的列 */
  hideInTable?: boolean
}

export interface ResourceDef {
  key: string
  title: string
  description: string
  group: string
  icon: LucideIcon
  fields?: FieldDef[]
  /** 只读监控页（无 CRUD） */
  readonly?: boolean
  /** 覆盖默认的 /api/{key} 请求路径（可带 query） */
  endpoint?: string
  /** 系统设置型资源：key/value 行，PUT /api/config/{key} 保存，不支持删除 */
  configMode?: boolean
  /** 从 /api/monitor 响应取哪一块渲染成单行只读表 */
  monitorField?: "server" | "online" | "requests" | "performance"
}

export const GROUP_ORDER = [
  "数据总览",
  "核心数据",
  "内容配置",
  "运营数据",
  "动态数据",
  "内容审核",
  "全新玩法",
  "系统管理",
  "系统监控",
] as const
