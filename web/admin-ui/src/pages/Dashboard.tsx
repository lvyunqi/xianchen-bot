import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts"
import { CloudOff, Gem, HeartHandshake, Hourglass, Package, RefreshCw, ScrollText, Sparkles, TrendingUp, Users } from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

interface DashboardData {
  metrics: Record<string, number>
  trend: { day: string; count: number }[]
  recent: { id: number; gm_name: string; action: string; target_type: string; target_id: string; created_at: string }[]
}

const METRIC_LABELS: Record<string, { label: string; hint: string; icon: typeof Users }> = {
  players: { label: "注册玩家", hint: "玩家表总人数", icon: Users },
  active_today: { label: "今日活跃", hint: "今天有过游戏操作的玩家", icon: Sparkles },
  couples: { label: "仙侣结缘", hint: "已缔结的仙侣关系", icon: HeartHandshake },
  pending_reviews: { label: "待审核", hint: "排队中的道号与内容审核", icon: Hourglass },
  items: { label: "物品种类", hint: "物品目录条目", icon: Package },
}

const METRIC_ORDER = ["players", "active_today", "couples", "pending_reviews", "items"]

function MetricCard({ metric, value, index }: { metric: string; value: number; index: number }) {
  const meta = METRIC_LABELS[metric] ?? { label: metric, hint: "", icon: Gem }
  const Icon = meta.icon
  return (
    <motion.div
      initial={{ opacity: 0, y: 14 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 26, delay: Math.min(index * 0.05, 0.4) }}
      title={meta.hint || undefined}
    >
      <Card className="glass glass-hover group relative h-full overflow-hidden rounded-2xl border-transparent">
        <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-primary/10 blur-2xl transition-opacity duration-300 group-hover:bg-gold/15" />
        <CardContent className="relative flex items-start justify-between gap-3 p-4">
          <div className="min-w-0">
            <div className="truncate text-xs font-medium text-muted-foreground">{meta.label}</div>
            <div className="tnum mt-1.5 font-mono text-[28px] font-semibold leading-none tracking-tight">
              {value.toLocaleString()}
            </div>
          </div>
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-gold/25 bg-gold/10 text-gold">
            <Icon className="h-4 w-4" />
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

function TrendChart({ trend }: { trend: { day: string; count: number }[] }) {
  return (
    <Card className="glass rounded-2xl border-transparent">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <TrendingUp className="h-4 w-4 text-primary" />
          近七日新增玩家
        </CardTitle>
        <CardDescription>按玩家创建日期统计</CardDescription>
      </CardHeader>
      <CardContent className="h-56 pt-2">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={trend} margin={{ top: 4, right: 8, left: -18, bottom: 0 }}>
            <defs>
              <linearGradient id="jadeFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.35} />
                <stop offset="100%" stopColor="hsl(var(--primary))" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
            <XAxis dataKey="day" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: "hsl(var(--muted-foreground))" }} />
            <YAxis allowDecimals={false} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: "hsl(var(--muted-foreground))" }} width={44} />
            <ChartTooltip
              contentStyle={{
                background: "hsl(var(--popover))",
                border: "1px solid hsl(var(--border))",
                borderRadius: 10,
                fontSize: 12,
                color: "hsl(var(--popover-foreground))",
              }}
              formatter={(v: number | string) => [v, "新增"]}
            />
            <Area type="monotone" dataKey="count" stroke="hsl(var(--primary))" strokeWidth={2} fill="url(#jadeFill)" />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

function RecentLogs({ logs }: { logs: DashboardData["recent"] }) {
  return (
    <Card className="glass rounded-2xl border-transparent">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <ScrollText className="h-4 w-4 text-gold" />
          最近操作
        </CardTitle>
        <CardDescription>管理端操作审计（最新 8 条）</CardDescription>
      </CardHeader>
      <CardContent>
        {logs.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">还没有操作记录。</p>
        ) : (
          <ul className="space-y-1">
            {logs.map((log, i) => (
              <li
                key={log.id ?? i}
                className="flex items-baseline gap-3 rounded-lg px-2 py-1.5 text-sm transition-colors hover:bg-muted/60"
              >
                <span className="tnum shrink-0 font-mono text-xs text-muted-foreground">
                  {String(log.created_at ?? "").slice(5, 16)}
                </span>
                <span className="shrink-0 font-medium">{log.gm_name || "-"}</span>
                <span className="shrink-0">{log.action ?? "-"}</span>
                <span className="min-w-0 truncate text-muted-foreground">
                  {log.target_type ? log.target_type + (log.target_id ? " #" + log.target_id : "") : ""}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

export default function Dashboard() {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["dashboard"],
    queryFn: () => api.getJson<DashboardData>("/api/dashboard"),
  })

  const metrics = data?.metrics ?? {}
  const metricKeys = [
    ...METRIC_ORDER.filter((k) => k in metrics),
    ...Object.keys(metrics).filter((k) => !(k in METRIC_LABELS)),
  ]

  return (
    <div className="space-y-5">
      <div className="flex items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">数据总览</h1>
          <p className="text-sm text-muted-foreground">玩家、活跃与运行概况</p>
        </div>
      </div>
      {isLoading ? (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-[88px] rounded-xl" />
            ))}
          </div>
          <Skeleton className="h-64 rounded-xl" />
        </div>
      ) : isError ? (
        <Card className="border-destructive/40">
          <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
            <CloudOff className="h-6 w-6 text-destructive" />
            <div className="text-sm font-medium">总览数据拉取失败</div>
            <p className="text-xs text-muted-foreground">网络或后端服务不可用，请稍后重试。</p>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              <RefreshCw className="h-3.5 w-3.5" />
              重试
            </Button>
          </CardContent>
        </Card>
      ) : !data ? (
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
            <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-accent text-accent-foreground">
              <Sparkles className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <div className="text-sm font-medium">暂无运营数据</div>
              <p className="max-w-sm text-xs leading-relaxed text-muted-foreground">
                总览接口暂未返回数据。确认 worker 已完成初始化后可重新拉取。
              </p>
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              <RefreshCw className="h-3.5 w-3.5" />
              重新拉取
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {metricKeys.length > 0 ? (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {metricKeys.slice(0, 8).map((key, i) => (
                <MetricCard key={key} metric={key} value={Number(metrics[key] ?? 0)} index={i} />
              ))}
            </div>
          ) : (
            <Card className="glass rounded-2xl border-transparent">
              <CardContent className="flex flex-col items-center gap-2 p-8 text-center">
                <Sparkles className="h-5 w-5 text-gold" />
                <div className="text-sm font-medium">暂无指标数据</div>
                <p className="text-xs text-muted-foreground">等第一批玩家进入仙途后，这里会亮起来。</p>
              </CardContent>
            </Card>
          )}
          <div className="grid gap-4 lg:grid-cols-2">
            <TrendChart trend={data.trend ?? []} />
            <RecentLogs logs={data.recent ?? []} />
          </div>
        </>
      )}
    </div>
  )
}
