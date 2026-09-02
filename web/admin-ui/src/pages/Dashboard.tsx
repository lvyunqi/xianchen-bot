import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"
import { api, type ResourceRecord } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

function StatCard({ metric, value, index }: { metric: string; value: string; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: index * 0.05, ease: "easeOut" }}
    >
      <Card className="transition-shadow hover:shadow-md">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">{metric}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-semibold tracking-tight">{value}</div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

export default function Dashboard() {
  const { data, isLoading, isError } = useQuery({
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
            <Skeleton key={i} className="h-28 rounded-xl" />
          ))}
        </div>
      ) : isError || !data?.length ? (
        <Card>
          <CardContent className="p-8 text-center text-sm text-muted-foreground">
            暂无数据或后端未连接（/api/dashboard）
          </CardContent>
        </Card>
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
