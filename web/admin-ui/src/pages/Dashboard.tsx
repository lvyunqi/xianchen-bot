import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"
import { CloudOff, RefreshCw, Sparkles } from "lucide-react"
import { api, type ResourceRecord } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

function StatCard({ metric, value, index }: { metric: string; value: string; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 14 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 26, delay: Math.min(index * 0.04, 0.4) }}
    >
      <Card className="group relative overflow-hidden transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift">
        {/* 顶部鎏金发丝线，hover 时点亮 */}
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-gold/50 to-transparent opacity-60 transition-opacity duration-200 group-hover:opacity-100" />
        <CardContent className="p-5">
          <div className="text-[13px] font-medium text-muted-foreground">{metric}</div>
          <div className="tnum mt-2 font-mono text-[26px] font-semibold leading-none tracking-tight text-foreground">
            {value}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

function DashboardFallback({ onRetry }: { onRetry: () => void }) {
  return (
    <Card className="border-dashed">
      <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-accent text-accent-foreground">
          <Sparkles className="h-5 w-5" />
        </div>
        <div className="space-y-1">
          <div className="text-sm font-medium">暂无运营数据</div>
          <p className="max-w-sm text-xs leading-relaxed text-muted-foreground">
            总览接口暂未返回数据（/api/dashboard）。确认 worker 已完成初始化后可重新拉取。
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw className="h-3.5 w-3.5" />
          重新拉取
        </Button>
      </CardContent>
    </Card>
  )
}

export default function Dashboard() {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["dashboard"],
    queryFn: () => api.list<ResourceRecord>("dashboard"),
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">数据总览</h1>
        <p className="text-sm text-muted-foreground">玩家、活跃与运行概况</p>
      </div>
      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-[88px] rounded-xl" />
          ))}
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
      ) : !data?.length ? (
        <DashboardFallback onRetry={() => refetch()} />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {data.map((row, i) => (
            <StatCard key={i} metric={String(row.metric ?? "-")} value={String(row.value ?? "-")} index={i} />
          ))}
        </div>
      )}
    </div>
  )
}
