import { useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { Database, Moon, RefreshCw, Sun } from "lucide-react"
import {
  CommandDialog, CommandEmpty, CommandGroup, CommandInput,
  CommandItem, CommandList, CommandSeparator,
} from "@/components/ui/command"
import { resourcesByGroup } from "@/lib/resources/registry"

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onRefresh: () => void
  onToggleTheme: () => void
  theme: string
}

export function CommandPalette({ open, onOpenChange, onRefresh, onToggleTheme, theme }: Props) {
  const navigate = useNavigate()

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        onOpenChange(!open)
      }
    }
    document.addEventListener("keydown", down)
    return () => document.removeEventListener("keydown", down)
  }, [open, onOpenChange])

  const go = (path: string) => {
    onOpenChange(false)
    navigate(path)
  }

  const groups = resourcesByGroup()

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="搜索页面或执行动作…" />
      <CommandList className="max-h-[min(60vh,420px)]">
        <CommandEmpty>没有匹配的结果。</CommandEmpty>
        <CommandGroup heading="动作">
          <CommandItem onSelect={() => { onOpenChange(false); onRefresh() }}>
            <RefreshCw className="h-4 w-4" />刷新全部数据
            <span className="ml-auto text-xs text-muted-foreground">F5 数据</span>
          </CommandItem>
          <CommandItem onSelect={() => { onOpenChange(false); onToggleTheme() }}>
            {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            切换{theme === "dark" ? "浅色" : "深色"}主题
          </CommandItem>
          <CommandItem onSelect={() => go("/database")}>
            <Database className="h-4 w-4" />数据运维
          </CommandItem>
        </CommandGroup>
        {groups.map(([group, items], gi) => (
          <div key={group}>
            {gi === 0 && <CommandSeparator />}
            <CommandGroup heading={group}>
              {items.map((r) => (
                <CommandItem
                  key={r.key}
                  value={r.key + " " + r.title + " " + (r.description ?? "")}
                  onSelect={() => go(r.key === "dashboard" ? "/" : `/r/${r.key}`)}
                >
                  <r.icon className="h-4 w-4 text-muted-foreground" />
                  {r.title}
                  <span className="ml-auto max-w-40 truncate text-xs text-muted-foreground">{r.description}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </div>
        ))}
      </CommandList>
    </CommandDialog>
  )
}
