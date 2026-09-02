import type { FieldDef } from "./types"

/** M2 标杆资源字段（M3 铺开其余资源）；关联选择器与 JSON 构建器在此声明 */
export const FIELD_OVERRIDES: Record<string, FieldDef[]> = {
  items: [
    { key: "code", label: "物品编码", type: "system" },
    { key: "name", label: "物品名称", type: "text", placeholder: "如：回春丹" },
    { key: "category_name", label: "类别", type: "select", options: ["丹药", "材料", "灵草", "法宝", "礼包", "任务"] },
    { key: "rarity_name", label: "品阶", type: "select", options: ["凡品", "灵品", "仙品", "神品"] },
    { key: "effect_type", label: "效果类型", type: "select", options: ["回复", "增益", "攻击", "特殊"] },
    { key: "effect_value", label: "效果数值", type: "number" },
    { key: "base_value", label: "基础价值", type: "number" },
    { key: "stackable", label: "可堆叠", type: "bool" },
    { key: "tradeable", label: "可交易", type: "bool" },
    { key: "enabled", label: "启用", type: "bool" },
    { key: "effect_desc", label: "效果说明", type: "textarea", placeholder: "玩家在物品详情看到的话术" },
    { key: "unlock_conditions", label: "解锁条件", type: "json", jsonKind: "conditions", hideInTable: true },
  ],
  realms: [
    { key: "name", label: "境界名称", type: "text", placeholder: "如：金丹期" },
    { key: "sequence", label: "顺序", type: "system" },
    { key: "required_cultivation", label: "所需修为", type: "number" },
    { key: "base_health", label: "基础气血", type: "number" },
    { key: "base_attack", label: "基础攻击", type: "number" },
    { key: "base_defense", label: "基础防御", type: "number" },
    { key: "tribulation_base_rate", label: "渡劫基础成功率", type: "number", placeholder: "0-1 小数" },
    { key: "lifespan", label: "寿元（年）", type: "number" },
    { key: "description", label: "描述", type: "textarea" },
  ],
  players: [
    { key: "account_id", label: "平台用户ID", type: "system" },
    { key: "dao_name", label: "道号", type: "text" },
    { key: "realm_name", label: "境界", type: "select", options: ["练气一层", "练气三层", "筑基一层", "金丹二层", "元婴一期", "化神期", "炼虚期", "合体期", "大乘期", "渡劫期"] },
    { key: "cultivation", label: "修为", type: "number" },
    { key: "spirit_stones", label: "灵石", type: "number" },
    { key: "sect_name", label: "宗门", type: "text" },
    { key: "banned", label: "封禁", type: "bool" },
    { key: "notes", label: "管理备注", type: "textarea", hideInTable: true },
  ],
}
