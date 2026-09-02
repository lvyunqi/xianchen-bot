import { useEffect, useMemo, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import type { ResourceDef } from "@/lib/resources/types"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { FieldRenderer } from "@/components/fields/FieldRenderer"

interface Props {
  def: ResourceDef
  open: boolean
  onOpenChange: (open: boolean) => void
  /** null = 新建 */
  record: Record<string, unknown> | null
}

export function ResourceForm({ def, open, onOpenChange, record }: Props) {
  const queryClient = useQueryClient()
  const fields = def.fields ?? []
  const initial = useMemo<Record<string, unknown>>(() => {
    const base: Record<string, unknown> = {}
    for (const f of fields) base[f.key] = record?.[f.key] ?? (f.type === "bool" ? false : f.type === "json" ? "[]" : "")
    return base
  }, [record, fields])
  const [values, setValues] = useState<Record<string, unknown>>(initial)
  const [errors, setErrors] = useState<Record<string, string>>({})

  useEffect(() => {
    if (open) {
      setValues(initial)
      setErrors({})
    }
  }, [open, initial])

  const mutation = useMutation({
    mutationFn: async () => {
      const body: Record<string, unknown> = {}
      for (const f of fields) {
        let v = values[f.key]
        if (f.type === "system" && (v === "" || v === null || v === undefined)) continue
        if (f.type === "number" && (v === "" || v === null)) continue
        body[f.key] = v
      }
      if (record) return api.update(def.key, record.id as number, body)
      return api.create(def.key, body)
    },
    onSuccess: () => {
      toast.success(record ? "保存成功" : "创建成功")
      queryClient.invalidateQueries({ queryKey: ["resource", def.key] })
      onOpenChange(false)
    },
    onError: (e: Error) => toast.error("保存失败", { description: e.message }),
  })

  const submit = () => {
    const errs: Record<string, string> = {}
    for (const f of fields) {
      const v = values[f.key]
      if (f.type === "json" && typeof v === "string" && v.trim()) {
        try { JSON.parse(v) } catch { errs[f.key] = "JSON 语法错误" }
      }
    }
    setErrors(errs)
    if (Object.keys(errs).length === 0) mutation.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto scrollbar-thin">
        <DialogHeader>
          <DialogTitle>{record ? "编辑" : "新增"} · {def.title}</DialogTitle>
          <DialogDescription>{def.description}</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {fields.map((f) => (
            <FieldRenderer
              key={f.key}
              field={f}
              value={values[f.key]}
              invalid={Boolean(errors[f.key])}
              onChange={(v) => setValues((s) => ({ ...s, [f.key]: v }))}
            />
          ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button onClick={submit} disabled={mutation.isPending}>
            {mutation.isPending ? "保存中…" : "保存"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
