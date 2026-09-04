import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RESOURCE_MAP } from "@/lib/resources/registry"
import { api, type ResourceRecord } from "@/lib/api"
import { ResourceTable } from "@/components/resource/ResourceTable"
import { ReadonlyStats } from "@/components/resource/ReadonlyStats"
import { ResourceForm } from "@/components/resource/ResourceForm"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Card, CardContent } from "@/components/ui/card"
import type { ResourceDef } from "@/lib/resources/types"

function apiPath(def: ResourceDef | undefined): string {
  return def?.endpoint ?? "/api/"
}

function prefixOf(endpoint: string | undefined): string {
  const q = endpoint?.split("?")[1] ?? ""
  return new URLSearchParams(q).get("prefix") ?? ""
}

export default function ResourcePage() {
  const { key = "" } = useParams()
  const def = RESOURCE_MAP.get(key)
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ResourceRecord | null>(null)
  const [deleting, setDeleting] = useState<ResourceRecord | null>(null)

  // 切换资源时清理上一页遗留的弹窗状态
  useEffect(() => {
    setFormOpen(false)
    setEditing(null)
    setDeleting(null)
  }, [key])

  const { data, isLoading, isError } = useQuery({
    queryKey: ["resource", key],
    queryFn: async () => {
      if (def?.monitorField) {
        const res = await api.monitor<Record<string, Record<string, unknown>>>()
        return [res[def.monitorField] ?? {}] as ResourceRecord[]
      }
      if (def?.configMode) return api.configList<ResourceRecord>(prefixOf(def.endpoint))
      return api.list<ResourceRecord>(def?.endpoint ?? key)
    },
    enabled: Boolean(def),
  })

  const removeMutation = useMutation({
    mutationFn: (target: ResourceRecord) => api.remove(apiPath(def), String(target.id)),
    onSuccess: () => {
      toast.success("删除成功")
      queryClient.invalidateQueries({ queryKey: ["resource", key] })
      setDeleting(null)
    },
    onError: (e: Error) => toast.error("删除失败", { description: e.message }),
  })

  const batchDelete = useMutation({
    mutationFn: async (targets: ResourceRecord[]) => {
      for (const t of targets) await api.remove(apiPath(def), String(t.id))
      return targets.length
    },
    onSuccess: (n) => {
      toast.success(`已删除 ${n} 条`)
      queryClient.invalidateQueries({ queryKey: ["resource", key] })
    },
    onError: (e: Error) => toast.error("批量删除失败", { description: e.message }),
  })

  const batchToggle = useMutation({
    mutationFn: async ({ targets, enabled }: { targets: ResourceRecord[]; enabled: boolean }) => {
      let n = 0
      for (const t of targets) {
        try {
          await api.update(apiPath(def), String(t.id), { enabled })
          n++
        } catch {
          /* 单条失败不中断 */
        }
      }
      return n
    },
    onSuccess: (n) => {
      toast.success(`已更新 ${n} 条`)
      queryClient.invalidateQueries({ queryKey: ["resource", key] })
    },
    onError: (e: Error) => toast.error("批量更新失败", { description: e.message }),
  })

  if (!def) {
    return (
      <Card>
        <CardContent className="p-8 text-center text-sm text-muted-foreground">未知页面：{key}</CardContent>
      </Card>
    )
  }

  if (def.readonly || def.monitorField) {
    return <ReadonlyStats title={def.title} records={data ?? []} isLoading={isLoading} isError={isError} />
  }

  // 系统设置行没有数字 id：用 key 当主键，且不支持删除/批量启停
  const records = def.configMode
    ? (data ?? []).map((r) => ({ ...r, id: (r.key ?? "") as unknown as number }))
    : data ?? []
  const deletable = !def.configMode

  return (
    <div className="space-y-4">
      <ResourceTable
        def={def}
        records={records}
        isLoading={isLoading}
        isError={isError}
        onCreate={() => { setEditing(null); setFormOpen(true) }}
        onEdit={(r) => { setEditing(r); setFormOpen(true) }}
        onDelete={deletable ? (r) => setDeleting(r) : undefined}
        onBatchDelete={deletable ? (rs) => batchDelete.mutate(rs) : undefined}
        onBatchToggle={deletable ? (rs, enabled) => batchToggle.mutate({ targets: rs, enabled }) : undefined}
      />
      <ResourceForm def={def} open={formOpen} onOpenChange={setFormOpen} record={editing} />
      <AlertDialog open={Boolean(deleting)} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除？</AlertDialogTitle>
            <AlertDialogDescription>
              将删除 {def.title}「{deleting ? String(deleting.name ?? deleting.code ?? deleting.id) : ""}」，此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              onClick={() => deleting && removeMutation.mutate(deleting)}
            >
              {removeMutation.isPending ? "删除中…" : "删除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
