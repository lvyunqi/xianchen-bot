import {
  Activity, BookOpen, Boxes, Brain, Coins, Database, FlaskConical, Gamepad2,
  Gauge, Gem, Globe2, Heart, Home, Images, LayoutDashboard, ListTree,
  Mail, Map as MapIcon, Package, ScrollText, Settings, Shield, ShieldAlert, ShoppingBag,
  Sparkles, Star, Store, Sword, Timer, Ticket, Users, Wand2, Webhook, Zap,
  type LucideIcon,
} from "lucide-react"
import type { ResourceDef } from "./types"
import { FIELDS_BY_RESOURCE } from "./legacy-fields"

interface PageMeta {
  key: string
  title: string
  description: string
  group: string
  icon: LucideIcon
  readonly?: boolean
  endpoint?: string
  configMode?: boolean
  monitorField?: "server" | "online" | "requests" | "performance"
}

const PAGES: PageMeta[] = [
  { key: "dashboard", title: "仪表盘", description: "玩家、活跃、仙侣、内容与运行趋势", group: "数据总览", icon: LayoutDashboard, readonly: true },
  { key: "config", title: "系统参数", description: "修炼、渡劫、体力与全局参数", group: "核心数据", icon: Settings },
  { key: "features", title: "功能开关", description: "开启或关闭任意游戏模块", group: "核心数据", icon: Zap, endpoint: "/api/config?prefix=feature.", configMode: true },
  { key: "constants", title: "游戏常量", description: "等级、转世、背包等全局常量", group: "核心数据", icon: Database, endpoint: "/api/config?prefix=constant.", configMode: true },
  { key: "cooldowns", title: "冷却时间", description: "各操作冷却时间配置", group: "核心数据", icon: Timer, endpoint: "/api/config?prefix=cooldown.", configMode: true },
  { key: "realms", title: "境界配置", description: "境界需求、属性成长与寿元", group: "核心数据", icon: Sparkles },
  { key: "spiritual_roots", title: "灵根图鉴", description: "一千种灵根、稀有权重与完整加成", group: "核心数据", icon: Wand2, endpoint: "/api/spiritual-roots" },
  { key: "items", title: "物品数据", description: "物品效果、价值、交易与堆叠", group: "内容配置", icon: Package },
  { key: "events", title: "事件数据", description: "随机事件、条件、概率与奖励", group: "内容配置", icon: Activity },
  { key: "tasks", title: "任务数据", description: "日常、悬赏、宗门与成就任务", group: "内容配置", icon: ListTree },
  { key: "skills", title: "功法数据", description: "功法类型、效果、升级与条件", group: "内容配置", icon: ScrollText },
  { key: "pets", title: "灵兽数据", description: "灵兽成长、忠诚与进化关系", group: "内容配置", icon: Heart },
  { key: "dungeons", title: "副本数据", description: "副本难度、战力、体力与奖励池", group: "内容配置", icon: Sword },
  { key: "recipes", title: "丹方数据", description: "丹方材料、产物和成功率", group: "内容配置", icon: FlaskConical },
  { key: "artifacts", title: "器谱数据", description: "法宝材料、属性和强化上限", group: "内容配置", icon: Gem },
  { key: "synthesis_recipes", title: "合成配方", description: "材料合成、产物、成功率与前置条件", group: "内容配置", icon: Boxes, endpoint: "/api/synthesis-recipes" },
  { key: "locations", title: "地图数据", description: "地点、区域、通行路线、境界条件与体力消耗", group: "内容配置", icon: MapIcon },
  { key: "world_leylines", title: "修仙界灵脉", description: "一千条地域灵脉、阶级、前置与独立加成", group: "内容配置", icon: Globe2, endpoint: "/api/world-leylines" },
  { key: "titles", title: "称号数据", description: "称号条件、类型与属性加成", group: "运营数据", icon: Star },
  { key: "activities", title: "活动数据", description: "活动时间、效果参数与状态", group: "运营数据", icon: Activity },
  { key: "mails", title: "邮件数据", description: "邮件正文、奖励、对象与发送", group: "运营数据", icon: Mail },
  { key: "checkin", title: "签到配置", description: "七日签到奖励与特殊奖励", group: "运营数据", icon: Coins },
  { key: "shop", title: "商店数据", description: "商品、价格、货币与上架状态", group: "运营数据", icon: Store },
  { key: "cdks", title: "兑换码数据", description: "兑换奖励、次数、期限与状态", group: "运营数据", icon: Ticket },
  { key: "notices", title: "公告数据", description: "公告正文、类型、置顶与发布", group: "运营数据", icon: Images },
  { key: "players", title: "玩家数据", description: "查询、编辑、物品、封禁和删除", group: "动态数据", icon: Users },
  { key: "couples", title: "仙侣数据", description: "仙侣关系、道缘与强制操作", group: "动态数据", icon: Heart },
  { key: "reviews", title: "内容与玩家反馈", description: "审核道号与社交内容，处理玩家反馈", group: "内容审核", icon: Shield },
  { key: "sensitive_words", title: "敏感词管理", description: "增删改敏感词和替换规则", group: "内容审核", icon: ShieldAlert, endpoint: "/api/sensitive-words" },
  { key: "formations", title: "阵法管理", description: "阵法配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Gamepad2 },
  { key: "talismans", title: "符箓管理", description: "符箓配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Gamepad2 },
  { key: "puppets_config", title: "傀儡管理", description: "傀儡配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Gamepad2, endpoint: "/api/puppets-config" },
  { key: "secret_conflicts", title: "秘境争夺", description: "秘境配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Gamepad2, endpoint: "/api/secret-conflicts" },
  { key: "inheritances", title: "传承管理", description: "传承配置、材料、效果、条件和等级数据", group: "全新玩法", icon: BookOpen },
  { key: "dao_insights", title: "悟道管理", description: "悟道配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Brain, endpoint: "/api/dao-insights" },
  { key: "battlefields", title: "仙魔战场", description: "战场配置、材料、效果、条件和等级数据", group: "全新玩法", icon: Sword },
  { key: "root_evolutions", title: "灵根进化", description: "灵根进化配置、材料、效果和条件", group: "全新玩法", icon: Wand2, endpoint: "/api/root-evolutions" },
  { key: "inner_demons", title: "渡劫心魔", description: "心魔配置、材料、效果和条件", group: "全新玩法", icon: Sparkles, endpoint: "/api/inner-demons" },
  { key: "couple_skills", title: "合体技管理", description: "合体技配置、材料、效果和条件", group: "全新玩法", icon: Heart, endpoint: "/api/couple-skills" },
  { key: "immortal_herbs", title: "仙药培育", description: "仙药配置、材料、效果和条件", group: "全新玩法", icon: FlaskConical, endpoint: "/api/immortal-herbs" },
  { key: "artifact_refinements", title: "法宝炼化", description: "炼化配置、材料、效果和条件", group: "全新玩法", icon: Gem, endpoint: "/api/artifact-refinements" },
  { key: "destiny_deductions", title: "天机推演", description: "推演配置、材料、效果和条件", group: "全新玩法", icon: Brain, endpoint: "/api/destiny-deductions" },
  { key: "leylines", title: "天地灵脉", description: "灵脉配置、材料、效果和条件", group: "全新玩法", icon: Globe2 },
  { key: "sect_wars", title: "宗门战争", description: "宗门战争配置、材料、效果和条件", group: "全新玩法", icon: Sword, endpoint: "/api/sect-wars" },
  { key: "immortal_encounters", title: "仙缘奇遇", description: "奇遇配置、材料、效果和条件", group: "全新玩法", icon: Sparkles, endpoint: "/api/immortal-encounters" },
  { key: "star_realms", title: "宇宙星河", description: "星河配置、材料、效果和条件", group: "全新玩法", icon: Star, endpoint: "/api/star-realms" },
  { key: "menus", title: "菜单管理", description: "后台导航与群内游戏菜单，保存立即生效", group: "系统管理", icon: ListTree },
  { key: "status_display", title: "状态显示", description: "切换状态指令使用纯图片模式或完整文字模式", group: "系统管理", icon: Images, endpoint: "/api/config?prefix=status.", configMode: true },
  { key: "owner_settings", title: "主人设置", description: "设置唯一主人QQ开放平台用户ID", group: "系统管理", icon: Settings, endpoint: "/api/config?prefix=owner.", configMode: true },
  { key: "managers", title: "管理设置", description: "管理员ID、名称、权限范围和启用状态", group: "系统管理", icon: Shield },
  { key: "server_status", title: "服务器状态", description: "CPU、内存、磁盘和运行时间", group: "系统监控", icon: Gauge, readonly: true, monitorField: "server" },
  { key: "online_monitor", title: "在线监控", description: "最近五分钟实时活跃人数", group: "系统监控", icon: Activity, readonly: true, monitorField: "online" },
  { key: "request_stats", title: "请求统计", description: "后台 API 请求和错误统计", group: "系统监控", icon: Webhook, readonly: true, monitorField: "requests" },
  { key: "slow_queries", title: "慢查询", description: "数据库慢 SQL 记录", group: "系统监控", icon: Timer, readonly: true, endpoint: "/api/slow-queries" },
  { key: "performance", title: "性能监控", description: "请求耗时和数据库连接状态", group: "系统监控", icon: Gauge, readonly: true, monitorField: "performance" },
  { key: "alerts", title: "告警配置", description: "CPU、内存、磁盘和延迟告警阈值", group: "系统监控", icon: ShieldAlert, endpoint: "/api/config?prefix=alert.", configMode: true },
]

export const RESOURCES: ResourceDef[] = PAGES.map((p) => ({
  key: p.key,
  title: p.title,
  description: p.description,
  group: p.group,
  icon: p.icon,
  readonly: p.readonly,
  endpoint: p.endpoint,
  configMode: p.configMode,
  monitorField: p.monitorField,
  fields: FIELDS_BY_RESOURCE[p.key],
}))

export const RESOURCE_MAP = new Map(RESOURCES.map((r) => [r.key, r]))

export function resourcesByGroup(): Array<[string, ResourceDef[]]> {
  const groups = new Map<string, ResourceDef[]>()
  for (const r of RESOURCES) {
    const list = groups.get(r.group) ?? []
    list.push(r)
    groups.set(r.group, list)
  }
  return [...groups.entries()]
}
