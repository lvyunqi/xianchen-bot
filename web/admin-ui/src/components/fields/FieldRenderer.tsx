import { useId } from "react"
import type { FieldDef } from "@/lib/resources/types"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { RelationCombobox } from "./RelationCombobox"
import { JsonBuilder } from "./JsonBuilder"
import { cn } from "@/lib/utils"

interface Props {
  field: FieldDef
  value: unknown
  onChange: (value: unknown) => void
  invalid?: boolean
}

function toDatetimeLocal(v: unknown): string {
  if (!v) return ""
  const s = String(v)
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(s)) return s.slice(0, 16)
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return ""
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 单字段渲染：按 FieldDef.type 分发到对应控件 */
export function FieldRenderer({ field, value, onChange, invalid }: Props) {
  const id = useId()
  const wrap = (node: React.ReactNode) => (
    <div className={cn("space-y-1.5", (field.type === "json" || field.type === "textarea") && "sm:col-span-2")}>
      <Label htmlFor={id} className={cn("text-xs", invalid && "text-destructive")}>
        {field.label}
        {field.type === "system" && <span className="ml-1.5 text-[10px] text-muted-foreground">系统生成，留空自动编号</span>}
      </Label>
      {node}
    </div>
  )

  switch (field.type) {
    case "number":
      return wrap(
        <Input
          id={id}
          type="number"
          step="any"
          value={value === null || value === undefined ? "" : String(value)}
          placeholder={field.placeholder}
          onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}
        />,
      )
    case "textarea":
      return wrap(
        <textarea
          id={id}
          className="flex min-h-[88px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          value={value === null || value === undefined ? "" : String(value)}
          onChange={(e) => onChange(e.target.value)}
        />,
      )
    case "bool":
      return wrap(
        <div className="flex h-9 items-center">
          <Switch checked={Boolean(value)} onCheckedChange={(v) => onChange(v)} />
        </div>,
      )
    case "select": {
      const NONE = "__none__"
      return wrap(
        <Select
          value={value === "" || value === null || value === undefined ? NONE : String(value)}
          onValueChange={(v) => onChange(v === NONE ? "" : v)}
        >
          <SelectTrigger id={id}>
            <SelectValue placeholder="选择…" />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((opt) => (
              <SelectItem key={opt || NONE} value={opt === "" ? NONE : opt}>{opt || "（空）"}</SelectItem>
            ))}
          </SelectContent>
        </Select>,
      )
    }
    case "datetime":
      return wrap(
        <Input
          id={id}
          type="datetime-local"
          value={toDatetimeLocal(value)}
          onChange={(e) => onChange(e.target.value)}
        />,
      )
    case "relation":
      return wrap(
        <RelationCombobox
          resource={field.relation?.resource ?? ""}
          labelKey={field.relation?.labelKey ?? "name"}
          value={value}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
        />,
      )
    case "json":
      return wrap(
        <JsonBuilder
          kind={field.jsonKind ?? "raw"}
          value={value}
          onChange={(json) => onChange(json)}
        />,
      )
    case "image":
      return wrap(
        <div className="space-y-2">
          <Input
            id={id}
            value={value === null || value === undefined ? "" : String(value)}
            placeholder="https://…/图片.png"
            onChange={(e) => onChange(e.target.value)}
          />
          {value ? (
            <img
              src={String(value)}
              alt="预览"
              className="h-16 w-16 rounded-lg border object-cover"
              onError={(e) => { (e.target as HTMLImageElement).style.opacity = "0.25" }}
              onLoad={(e) => { (e.target as HTMLImageElement).style.opacity = "1" }}
            />
          ) : null}
        </div>,
      )
    default:
      return wrap(
        <Input
          id={id}
          value={value === null || value === undefined ? "" : String(value)}
          placeholder={field.placeholder ?? (field.type === "system" ? "留空自动生成" : undefined)}
          onChange={(e) => onChange(e.target.value)}
        />,
      )
  }
}
