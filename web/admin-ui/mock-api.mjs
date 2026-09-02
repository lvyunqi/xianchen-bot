// 仅 vite dev（apply: "serve"）生效的 mock 中间件：本地无 Go 后端时预览用。
// 产物构建（build）完全不受影响。
function genRecords(prefix, fields, count) {
  const rows = []
  for (let i = 1; i <= count; i++) {
    const row = { id: i }
    for (const [key, gen] of Object.entries(fields)) row[key] = gen(i)
    rows.push(row)
  }
  return rows
}

const pick = (list, i) => list[i % list.length]
const names = {
  items: ["回春丹", "筑基丹", "灵石", "聚灵草", "破境丹", "洗髓丹", "飞剑符", "灵兽粮", "天罡石", "玄冰髓", "紫金葫芦", "星尘砂"],
  realms: ["练气", "筑基", "金丹", "元婴", "化神", "炼虚", "合体", "大乘", "渡劫"],
  players: ["云无心", "月见栀", "青山客", "白鹭洲", "风清扬", "洛神赋", "夜未央", "千里江山", "一叶知秋", "紫气东来", "江晚吟", "岁寒松"],
}

const datasets = {
  dashboard: [
    { metric: "注册玩家", value: "1,286" },
    { metric: "今日活跃", value: "342" },
    { metric: "在线仙侣", value: "87" },
    { metric: "待审核内容", value: "12" },
    { metric: "今日修炼人次", value: "5,204" },
    { metric: "Boss 讨伐", value: "46" },
    { metric: "邮件待发", value: "3" },
    { metric: "兑换码核销", value: "129" },
  ],
}

function recordFor(resource, i) {
  switch (resource) {
    case "monitor":
      return [{ metric: "最近5分钟活跃", value: "342" }, { metric: "CPU", value: "23%" }, { metric: "内存", value: "61%" }]
    case "spiritual_roots":
    case "world_leylines": {
      const rows = []
      for (let i = 1; i <= 1000; i++) {
        rows.push({
          id: i,
          code: "SR" + String(i).padStart(4, "0"),
          name: "玄阴灵根·第" + i + "品",
          element: pick(["金", "木", "水", "火", "土", "雷", "冰", "风"], i),
          grade: pick(["下品", "中品", "上品", "极品"], i),
          base_quality: 30 + (i % 70),
          cultivation_bonus: 1 + (i % 9) * 0.1,
          rarity_weight: 1000 - i,
          enabled: i % 97 !== 0,
        })
      }
      return rows
    }
    case "realms":
      return genRecords("realm", {
        name: (i) => pick(names.realms, i - 1) + "期",
        sequence: (i) => i,
        required_cultivation: (i) => i * i * 500,
        base_health: (i) => 100 + i * 50,
        base_attack: (i) => 20 + i * 8,
        tribulation_base_rate: (i) => 0.9 - i * 0.05,
        description: (i) => "第" + i + "大境",
      }, 9)
    case "items":
      return genRecords("item", {
        code: (i) => "ITM" + String(i).padStart(4, "0"),
        name: (i) => pick(names.items, i - 1),
        category_name: (i) => pick(["丹药", "材料", "灵草", "法宝", "礼包"], i),
        rarity_name: (i) => pick(["凡品", "灵品", "仙品", "神品"], i),
        effect_value: (i) => i * 30,
        base_value: (i) => i * 120,
        stackable: (i) => i % 2 === 0,
        enabled: () => true,
      }, 12)
    case "players":
      return genRecords("player", {
        account_id: (i) => "OPENID" + String(i).padStart(6, "0"),
        dao_name: (i) => pick(names.players, i - 1),
        realm_name: (i) => pick(names.realms, i - 1) + "三层",
        cultivation: (i) => i * 4210,
        combat_power: (i) => i * 8800,
        spirit_stones: (i) => i * 640,
        sect_name: (i) => pick(["太虚剑宗", "丹霞谷", "万宝阁", "御兽山庄"], i),
        banned: (i) => i === 7,
      }, 12)
    default: {
      const en = resource.replace(/_/g, "-")
      return genRecords(en, {
        code: (i) => resource.toUpperCase().slice(0, 3) + String(i).padStart(3, "0"),
        name: (i) => resource + " 条目 " + i,
        type: (i) => pick(["攻击", "防御", "辅助", "成长"], i),
        level: (i) => (i % 9) + 1,
        sort_order: (i) => i,
        status: (i) => pick(["启用", "停用"], i),
        enabled: () => true,
      }, 8)
    }
  }
}

export default function mockApi() {
  return {
    name: "xianchen-mock-api",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = (req.url ?? "").split("?")[0]
        if (!url.startsWith("/api/")) return next()
        const resource = url.replace(/^\/api\//, "").replace(/\/$/, "")
        let body
        if (req.method === "GET") {
          body = datasets[resource] ?? recordFor(resource)
          res.setHeader("Content-Type", "application/json; charset=utf-8")
          res.end(JSON.stringify(body))
          return
        }
        // 写操作：回显成功
        res.setHeader("Content-Type", "application/json; charset=utf-8")
        res.end(JSON.stringify({ ok: true }))
      })
    },
  }
}
