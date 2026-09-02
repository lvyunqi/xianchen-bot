import { useMemo, useState } from "react"
import { Plus, Trash2, Code2, ListTree } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RelationCombobox } from "./RelationCombobox"
import { cn } from "@/lib/utils"

type Row = Record<string, string | number>

interface Props {
  kind: "materials" | "rewards" | "conditions" | "pairs" | "raw"
  value: unknown
  onChange: (json: string) => void
}

/** JSON 字段：结构化行编辑为主，源码模式兜底（容错历史脏数据） */
export function JsonBuilder({ kind, value, onChange }: Props) {
  const [rawMode, setRawMode] = useState(false)
  const [rawText, setRawText] = useState("")

  const rows: Row[] = useMemo(() => {
    if (rawMode) return []
    try {
      const parsed = typeof value === "string" ? JSON.parse(value || "[]") : value
      if (Array.isArray(parsed)) return parsed as Row[]
      if (parsed && typeof parsed === "object") {
        return Object.entries(parsed).map(([k, v]) => ({ key: k, value: String(v) }))
      }
      return []
    } catch {
      return []
    }
  }, [value, rawMode])

  const emit = (next: Row[]) => onChange(JSON.stringify(next, null, 2))

  const updateRow = (i: number, field: string, v: string | number) => {
    const next = rows.map((r, idx) => (idx === i ? { ...r, [field]: v } : r))
    emit(next)
  }
  const addRow = () => {
    const base: Row =
      kind === "materials" ? { name: "", count: 1 }
      : kind === "rewards" ? { name: "", count: 1, probability: 0.5 }
      : kind === "conditions" ? { field: "", op: ">=", value: "" }
      : { key: "", value: "" }
    emit([...rows, base])
  }
  const removeRow = (i: number) => emit(rows.filter((_, idx) => idx !== i))

  const switchToRaw = () => {
    setRawText(JSON.stringify(rows, null, 2))
    setRawMode(true)
  }
  const applyRaw = () => {
    try {
      JSON.parse(rawText)
      onChange(rawText)
      setRawMode(false)
    } catch {
      alert("JSON 语法错误，请检查后再应用")
    }
  }

  const columns: string[] =
    kind === "materials" ? ["name", "count"]
    : kind === "rewards" ? ["name", "count", "probability"]
    : kind === "conditions" ? ["field", "op", "value"]
    : ["key", "value"]

  return (
    <div className="rounded-lg border bg-muted/30 p-3 space-y-2">
      <div className="flex items-center justify-between">
        <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          <ListTree className="h-3.5 w-3.5" /> 结构化编辑 · {rows.length} 项
        </span>
        <div className="flex gap-1">
          {rawMode ? (
            <Button type="button" size="sm" variant="secondary" onClick={applyRaw}>应用源码</Button>
          ) : (
            <Button type="button" size="sm" variant="ghost" onClick={switchToRaw}>
              <Code2 className="h-3.5 w-3.5" /> 源码
            </Button>
          )}
        </div>
      </div>

      {rawMode ? (
        <textarea
          className="min-h-[140px] w-full rounded-md border bg-background p-2 font-mono text-xs"
          value={rawText}
          onChange={(e) => setRawText(e.target.value)}
        />
      ) : (
        <div className="space-y-1.5">
          {rows.map((row, i) => (
            <div key={i} className="flex items-center gap-1.5">
              {columns.map((col) =>
                col === "name" ? (
                  <RelationCombobox
                    key={col}
                    resource="items"
                    labelKey="name"
                    value={row[col]}
                    onChange={(v) => updateRow(i, col, v)}
                    placeholder="物品"
                  />
                ) : (
                  <Input
                    key={col}
                    className="h-8 flex-1 text-xs"
                    value={String(row[col] ?? "")}
                    placeholder={col}
                    onChange={(e) => updateRow(i, col, e.target.value)}
                  />
                ),
              )}
              <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground" onClick={() => removeRow(i)}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
          <Button type="button" variant="outline" size="sm" className="w-full border-dashed" onClick={addRow}>
            <Plus className="h-3.5 w-3.5" /> 添加一项
          </Button>
        </div>
      )}
    </div>
  )
}
