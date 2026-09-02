import { useParams } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"
import { RESOURCE_MAP } from "@/lib/resources/registry"
import { api, type ResourceRecord } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

export default function ResourcePage() {
  const { key = "" } = useParams()
  const def = RESOURCE_MAP.get(key)
  const { data, isLoading, isError } = useQuery({
    queryKey: ["resource", key],
    queryFn: () => api.list<ResourceRecord>(key),
    enabled: Boolean(def) && !def?.readonly,
  })

  if (!def) {
    return (
      <Card>
        <CardContent className="p-8 text-center text-sm text-muted-foreground">未知页面：{key}</CardContent>
      </Card>
    )
  }

  if (def.readonly) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-2 p-10 text-center">
          <def.icon className="h-8 w-8 text-muted-foreground" />
          <div className="font-medium">{def.title}</div>
          <div className="text-sm text-muted-foreground">监控页将在 M4 阶段提供图表视图</div>
        </CardContent>
      </Card>
    )
  }

  const records = data ?? []
  const columns = records.length
    ? Object.keys(records[0]).filter((k) => k !== "id").slice(0, 8)
    : []

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-xl font-semibold tracking-tight">{def.title}</h1>
        <Badge variant="secondary">{records.length} 条</Badge>
      </div>
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : isError ? (
            <div className="p-8 text-center text-sm text-muted-foreground">
              数据加载失败，请确认管理后台 API 可访问
            </div>
          ) : records.length === 0 ? (
            <div className="p-10 text-center text-sm text-muted-foreground">暂无数据</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  {columns.map((c) => (
                    <TableHead key={c}>{c}</TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {records.slice(0, 100).map((row, i) => (
                  <TableRow key={row.id ?? i}>
                    {columns.map((c) => (
                      <TableCell key={c} className="max-w-56 truncate text-muted-foreground">
                        {String(row[c] ?? "-")}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <p className="text-xs text-muted-foreground">
        通用 CRUD 引擎（M2）将带来：搜索、筛选、排序、关联选择器、JSON 构建器与完整编辑。
      </p>
    </div>
  )
}
