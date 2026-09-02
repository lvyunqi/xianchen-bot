import { useMemo } from "react"
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"
import { motion } from "framer-motion"
import { Activity } from "lucide-react"
import type { ResourceRecord } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

const CHART_COLORS = ["hsl(262 83% 70%)", "hsl(43 92% 58%)", "hsl(199 89% 60%)", "hsl(160 84% 45%)", "hsl(340 75% 65%)", "hsl(27 87% 62%)"]

interface Props {
  title: string
  records: ResourceRecord[]
  isLoading: boolean
  isError: boolean
}

function parseValue(v: unknown): number | null {
  if (typeof v === "number") return Number.isFinite(v) ? v : null
  if (typeof v !== "string") return null
  const m = v.replace(/,/g, "").match(/^-?\d+(?:\.\d+)?/)
  if (!m) return null
  const n = Number(m[0])
  return Number.isFinite(n) ? n : null
}

/** 只读统计页：数值指标画横向条形图，其余以卡片呈现 */
export function ReadonlyStats({ title, records, isLoading, isError }: Props) {
  const { numeric, cards } = useMemo(() => {
    const numeric: Array<{ name: string; value: number }> = []
    const cards: Array<{ metric: string; value: string }> = []
    for (const r of records) {
      const metric = String(r.metric ?? r.key ?? "-")
      const value = r.value ?? r.count ?? "-"
      const num = parseValue(value)
      if (num !== null && Math.abs(num) > 0) numeric.push({ name: metric, value: num })
      else cards.push({ metric, value: String(value) })
    }
    return { numeric: numeric.slice(0, 12), cards: cards.slice(0, 12) }
  }, [records])

  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2">
        {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-48 rounded-xl" />)}
      </div>
    )
  }

  if (isError || records.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-2 p-12 text-center">
          <Activity className="h-8 w-8 text-muted-foreground/60" />
          <div className="text-sm text-muted-foreground">{isError ? "数据加载失败，请确认管理后台 API 可访问" : "暂无数据"}</div>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      {numeric.length >= 2 && (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.3 }}>
          <Card>
            <CardHeader className="pb-0">
              <CardTitle className="text-sm font-medium text-muted-foreground">{title} · 数值分布</CardTitle>
            </CardHeader>
            <CardContent className="h-72 pt-4">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={numeric} layout="vertical" margin={{ left: 8, right: 24 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(243 12% 20%)" horizontal={false} />
                  <XAxis type="number" tick={{ fontSize: 11, fill: "hsl(243 8% 60%)" }} axisLine={false} tickLine={false} />
                  <YAxis
                    type="category" dataKey="name" width={120}
                    tick={{ fontSize: 11, fill: "hsl(243 8% 72%)" }} axisLine={false} tickLine={false}
                  />
                  <Tooltip
                    cursor={{ fill: "hsl(243 20% 14%)" }}
                    contentStyle={{ background: "hsl(243 18% 9%)", border: "1px solid hsl(243 12% 20%)", borderRadius: 8, fontSize: 12 }}
                    labelStyle={{ color: "hsl(243 8% 72%)" }}
                  />
                  <Bar dataKey="value" radius={[0, 4, 4, 0]} barSize={16}>
                    {numeric.map((_, i) => (
                      <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>
        </motion.div>
      )}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {(numeric.length >= 2 ? cards : [...cards, ...numeric.map((n) => ({ metric: n.name, value: String(n.value) }))]).map((c, i) => (
          <motion.div
            key={c.metric}
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, delay: i * 0.05 }}
          >
            <Card className="transition-shadow hover:shadow-md">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{c.metric}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-semibold tracking-tight">{c.value}</div>
              </CardContent>
            </Card>
          </motion.div>
        ))}
      </div>
    </div>
  )
}
