import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { motion } from "framer-motion"
import { HardDrive, Sparkles } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

interface TableStat {
  name: string
  kb: number
  rows: number
}

interface DatabaseStats {
  mode: string
  file_size_bytes: number
  dbstat_available: boolean
  tables: TableStat[] | null
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + " MB"
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + " KB"
  return bytes + " B"
}

export default function DatabaseOps() {
  const queryClient = useQueryClient()
  const { data, isLoading, isError } = useQuery({
    queryKey: ["db-stats"],
    queryFn: () => api.getJson<DatabaseStats>("/api/db-stats"),
  })

  const vacuum = useMutation({
    mutationFn: () => api.postJson("/api/db-vacuum"),
    onSuccess: () => {
      toast.success("压缩完成，空间已回收")
      void queryClient.invalidateQueries({ queryKey: ["db-stats"] })
    },
    onError: (e) => toast.error("压缩失败：" + e.message),
  })

  const retention = useMutation({
    mutationFn: () => api.postJson("/api/db-retention"),
    onSuccess: (res) => {
      const deleted = (res as { deleted?: Record<string, number> }).deleted ?? {}
      const total = Object.values(deleted).reduce((a, b) => a + b, 0)
      const detail = Object.entries(deleted).map(([k, v]) => `${k} ${v} 条`).join("、")
      toast.success(total > 0 ? `清理完成（${detail}）` : "没有超过保留期的数据")
      void queryClient.invalidateQueries({ queryKey: ["db-stats"] })
    },
    onError: (e) => toast.error("清理失败：" + e.message),
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">数据运维</h1>
        <p className="text-sm text-muted-foreground">数据体积、保留期清理与空间回收</p>
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        <StatCard title="存储引擎" value={isLoading ? "…" : data?.mode ?? "-"} index={0} />
        <StatCard title="数据库文件" value={isLoading ? "…" : formatSize(data?.file_size_bytes ?? 0)} index={1} />
        <StatCard title="细粒度统计" value={isLoading ? "…" : data?.dbstat_available ? "可用" : "当前内核不支持"} index={2} />
      </div>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base"><Sparkles className="h-4 w-4" />维护操作</CardTitle>
          <CardDescription>清理按每张流水表的保留期执行；压缩将空闲页归还操作系统，执行期间写入会短暂等待。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-3">
          <Button onClick={() => retention.mutate()} disabled={retention.isPending}>
            {retention.isPending ? "清理中…" : "执行保留期清理"}
          </Button>
          <Button variant="outline" onClick={() => vacuum.mutate()} disabled={vacuum.isPending}>
            {vacuum.isPending ? "压缩中…" : "压缩数据库（VACUUM）"}
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base"><HardDrive className="h-4 w-4" />体积排行</CardTitle>
          <CardDescription>按占用空间排序的前 20 张表</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
            </div>
          ) : isError || !data ? (
            <p className="text-sm text-muted-foreground">无法加载体积数据。</p>
          ) : !data.dbstat_available || !data.tables?.length ? (
            <p className="text-sm text-muted-foreground">当前 SQLite 内核未启用 dbstat 统计，仅提供文件总大小。</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>表</TableHead>
                  <TableHead className="text-right">占用</TableHead>
                  <TableHead className="text-right">行数</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.tables.map((t, i) => (
                  <motion.tr
                    key={t.name}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2, delay: i * 0.02 }}
                    className="border-b transition-colors hover:bg-muted/50"
                  >
                    <TableCell className="font-mono text-sm">{t.name}</TableCell>
                    <TableCell className="text-right tabular-nums">{t.kb} KB</TableCell>
                    <TableCell className="text-right tabular-nums text-muted-foreground">{t.rows.toLocaleString()}</TableCell>
                  </motion.tr>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function StatCard({ title, value, index }: { title: string; value: string; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: index * 0.05, ease: "easeOut" }}
    >
      <Card className="transition-shadow hover:shadow-md">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-semibold tracking-tight">{value}</div>
        </CardContent>
      </Card>
    </motion.div>
  )
}
