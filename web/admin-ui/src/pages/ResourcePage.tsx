import { useState } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RESOURCE_MAP } from "@/lib/resources/registry"
import { api, type ResourceRecord } from "@/lib/api"
import { ResourceTable } from "@/components/resource/ResourceTable"
import { ResourceForm } from "@/components/resource/ResourceForm"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Card, CardContent } from "@/components/ui/card"

export default function ResourcePage() {
  const { key = "" } = useParams()
  const def = RESOURCE_MAP.get(key)
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ResourceRecord | null>(null)
  const [deleting, setDeleting] = useState<ResourceRecord | null>(null)

  const { data, isLoading, isError } = useQuery({
    queryKey: ["resource", key],
    queryFn: () => api.list<ResourceRecord>(key),
    enabled: Boolean(def) && !def?.readonly,
  })

  const removeMutation = useMutation({
    mutationFn: (target: ResourceRecord) => api.remove(key, target.id as number),
    onSuccess: () => {
      toast.success("删除成功")
      queryClient.invalidateQueries({ queryKey: ["resource", key] })
      setDeleting(null)
    },
    onError: (e: Error) => toast.error("删除失败", { description: e.message }),
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

  return (
    <div className="space-y-4">
      <ResourceTable
        def={def}
        records={data ?? []}
        isLoading={isLoading}
        isError={isError}
        onCreate={() => { setEditing(null); setFormOpen(true) }}
        onEdit={(r) => { setEditing(r); setFormOpen(true) }}
        onDelete={(r) => setDeleting(r)}
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
