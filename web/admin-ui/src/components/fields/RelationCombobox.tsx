import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Check, ChevronsUpDown } from "lucide-react"
import { api, type ResourceRecord } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"

interface Props {
  resource: string
  labelKey: string
  /** 保存时取记录的哪个字段，默认 code */
  saveKey?: string
  value: unknown
  onChange: (value: string | number) => void
  placeholder?: string
}

/** 关联选择器：搜索远端资源，展示名称，保存 ID/编码 */
export function RelationCombobox({ resource, labelKey, saveKey, value, onChange, placeholder }: Props) {
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState("")
  const { data } = useQuery({
    queryKey: ["relation", resource],
    queryFn: () => api.list<ResourceRecord>(resource),
    enabled: open,
    staleTime: 60_000,
  })

  const records = useMemo(() => data ?? [], [data])
  const selected = records.find((r) => String(r.id) === String(value) || r.code === value)
  const label = selected ? String(selected[labelKey] ?? selected.code ?? selected.id) : value ? String(value) : ""
  const filtered = useMemo(
    () =>
      records.filter((r) => {
        if (!keyword) return true
        const text = String(r[labelKey] ?? "") + String(r.code ?? "") + String(r.name ?? "")
        return text.toLowerCase().includes(keyword.toLowerCase())
      }),
    [records, keyword, labelKey],
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn("w-full justify-between font-normal", !value && "text-muted-foreground")}
        >
          <span className="truncate">{label || placeholder || "选择…"}</span>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[320px] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput placeholder="搜索…" value={keyword} onValueChange={setKeyword} />
          <CommandList>
            <CommandEmpty>无匹配项</CommandEmpty>
            <CommandGroup>
              {filtered.slice(0, 50).map((r) => (
                <CommandItem
                  key={String(r.id)}
                  value={String(r.id)}
                  onSelect={() => {
                    const saved = r[saveKey ?? "code"] ?? (r.id as string | number)
                    onChange(saved as string | number)
                    setOpen(false)
                  }}
                >
                  <Check className={cn("mr-2 h-4 w-4", String(r.id) === String(value) || r.code === value ? "opacity-100" : "opacity-0")} />
                  <span className="flex-1 truncate">{String(r[labelKey] ?? r.code ?? r.id)}</span>
                  <span className="text-xs text-muted-foreground">{String(r.code ?? r.id)}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
