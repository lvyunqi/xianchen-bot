import { useMemo, useRef, useState } from "react"
import { Ban, CheckCircle2, Trash2 as TrashAll } from "lucide-react"
import { AnimatePresence, motion } from "framer-motion"
import { useVirtualizer } from "@tanstack/react-virtual"
import {
  type ColumnDef, type SortingState, flexRender,
  getCoreRowModel, getPaginationRowModel, getSortedRowModel, useReactTable,
} from "@tanstack/react-table"
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, Pencil, Plus, Search, Trash2 } from "lucide-react"
import { Checkbox } from "@/components/ui/checkbox"
import type { ResourceDef } from "@/lib/resources/types"
import type { ResourceRecord } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"

interface Props {
  def: ResourceDef
  records: ResourceRecord[]
  isLoading: boolean
  isError: boolean
  onCreate: () => void
  onEdit: (record: ResourceRecord) => void
  onDelete?: (record: ResourceRecord) => void
  onBatchDelete?: (records: ResourceRecord[]) => void
  onBatchToggle?: (records: ResourceRecord[], enabled: boolean) => void
}

function cellText(v: unknown): string {
  if (v === null || v === undefined || v === "") return "-"
  if (typeof v === "boolean") return v ? "是" : "否"
  const s = String(v)
  return s.length > 60 ? s.slice(0, 60) + "…" : s
}

export function ResourceTable({ def, records, isLoading, isError, onCreate, onEdit, onDelete, onBatchDelete, onBatchToggle }: Props) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [keyword, setKeyword] = useState("")
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const rowKey = (r: ResourceRecord) => String(r.id)
  const toggleRow = (r: ResourceRecord) => {
    setSelected((s) => {
      const next = new Set(s)
      const k = rowKey(r)
      if (next.has(k)) next.delete(k)
      else next.add(k)
      return next
    })
  }
  const toggleAllVisible = () => {
    setSelected((s) => {
      const visible = filteredRows.map(rowKey)
      const allIn = visible.every((k) => s.has(k))
      const next = new Set(s)
      for (const k of visible) {
        if (allIn) next.delete(k)
        else next.add(k)
      }
      return next
    })
  }
  const selectedRecords = () => records.filter((r) => selected.has(rowKey(r)))
  const clearSelection = () => setSelected(new Set())

  const fields = def.fields ?? []
  const filteredRows = useMemo(() => {
    if (!keyword) return records
    const kw = keyword.toLowerCase()
    return records.filter((r) => Object.values(r).some((v) => String(v ?? "").toLowerCase().includes(kw)))
  }, [records, keyword])

  const columns = useMemo<ColumnDef<ResourceRecord>[]>(() => {
    const cols: ColumnDef<ResourceRecord>[] = [
      {
        id: "select",
        header: () => (
          <Checkbox
            checked={filteredRows.length > 0 && filteredRows.every((r) => selected.has(rowKey(r)))}
            onCheckedChange={toggleAllVisible}
            aria-label="全选本页"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={selected.has(rowKey(row.original))}
            onCheckedChange={() => toggleRow(row.original)}
            aria-label="选择该行"
          />
        ),
        enableSorting: false,
      },
    ]
    cols.push(...fields
      .filter((f) => f.type !== "json" && f.type !== "textarea")
      .slice(0, 7)
      .map((f): ColumnDef<ResourceRecord> => ({
        accessorKey: f.key,
        header: f.label,
        cell: ({ row }) => {
          const v = row.original[f.key]
          if (typeof v === "boolean") {
            return <Badge variant={v ? "success" : "secondary"}>{v ? "是" : "否"}</Badge>
          }
          if (f.type === "select" && v) {
            const tone = /启用|已发布|开启/.test(String(v)) ? "success" : /停用|下架/.test(String(v)) ? "secondary" : "outline"
            return <Badge variant={tone as "success" | "secondary" | "outline"}>{String(v)}</Badge>
          }
          return <span className="text-muted-foreground">{cellText(v)}</span>
        },
      })))
    cols.push({
      id: "actions",
      header: "操作",
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={(e) => { e.stopPropagation(); onEdit(row.original) }}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          {onDelete && (
            <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive" onClick={(e) => { e.stopPropagation(); onDelete(row.original) }}>
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      ),
    })
    return cols
  }, [fields, onEdit, onDelete, selected, filteredRows])

  const table = useReactTable({
    data: filteredRows,
    columns,
    state: { sorting, pagination },
    onSortingChange: setSorting,
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  const virtualEnabled = filteredRows.length > 200
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const rowVirtualizer = useVirtualizer({
    count: filteredRows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 44,
    overscan: 12,
  })

  const rows = table.getRowModel().rows
  const pageCount = Math.max(1, table.getPageCount())
  const pageIndex = table.getState().pagination.pageIndex

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-52">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-8"
            placeholder={"搜索" + def.title + "…"}
            value={keyword}
            onChange={(e) => { setKeyword(e.target.value); setPagination((p) => ({ ...p, pageIndex: 0 })) }}
          />
        </div>
        <Button onClick={onCreate}>
          <Plus className="h-4 w-4" /> 新增
        </Button>
      </div>

      {selected.size > 0 && (onBatchDelete || onBatchToggle) && (
        <motion.div
          initial={{ opacity: 0, y: -6 }}
          animate={{ opacity: 1, y: 0 }}
          className="flex flex-wrap items-center gap-2 rounded-lg border bg-accent/40 px-3 py-2"
        >
          <span className="text-sm font-medium">已选 {selected.size} 条</span>
          {onBatchToggle && (
            <>
              <Button size="sm" variant="outline" onClick={() => { onBatchToggle(selectedRecords(), true); clearSelection() }}>
                <CheckCircle2 className="h-3.5 w-3.5" /> 批量启用
              </Button>
              <Button size="sm" variant="outline" onClick={() => { onBatchToggle(selectedRecords(), false); clearSelection() }}>
                <Ban className="h-3.5 w-3.5" /> 批量停用
              </Button>
            </>
          )}
          {onBatchDelete && (
            <Button size="sm" variant="destructive" onClick={() => { onBatchDelete(selectedRecords()); clearSelection() }}>
              <TrashAll className="h-3.5 w-3.5" /> 批量删除
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={clearSelection}>取消选择</Button>
        </motion.div>
      )}

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
            </div>
          ) : isError ? (
            <div className="p-10 text-center text-sm text-muted-foreground">数据加载失败，请确认管理后台 API 可访问</div>
          ) : filteredRows.length === 0 ? (
            <div className="flex flex-col items-center gap-3 p-12 text-center">
              {keyword ? (
                <span className="text-sm text-muted-foreground">没有匹配「{keyword}」的记录</span>
              ) : (
                <>
                  <span className="text-sm text-muted-foreground">暂无{def.title}数据</span>
                  <Button size="sm" variant="outline" onClick={onCreate}>
                    <Plus className="h-3.5 w-3.5" /> 新增
                  </Button>
                </>
              )}
            </div>
          ) : virtualEnabled ? (
            <div>
              {/* 虚拟模式：表头与数据行使用同一套 flex 列布局，保证对齐 */}
              <div className="sticky top-0 z-10 flex h-10 items-center border-b bg-card px-3 text-xs font-medium text-muted-foreground">
                {table.getHeaderGroups()[0]?.headers.map((h) => (
                  <div key={h.id} className={h.column.id === "select" ? "w-9 shrink-0" : "min-w-0 flex-1"}>
                    {h.column.getCanSort() ? (
                      <button className="inline-flex items-center gap-1 hover:text-foreground" onClick={h.column.getToggleSortingHandler()}>
                        {flexRender(h.column.columnDef.header, h.getContext())}
                        {h.column.getIsSorted() === "asc" && <ArrowUp className="h-3 w-3" />}
                        {h.column.getIsSorted() === "desc" && <ArrowDown className="h-3 w-3" />}
                      </button>
                    ) : (
                      flexRender(h.column.columnDef.header, h.getContext())
                    )}
                  </div>
                ))}
              </div>
              <div ref={scrollRef} className="max-h-[560px] overflow-y-auto scrollbar-thin">
                <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
                  {rowVirtualizer.getVirtualItems().map((vi) => {
                    const row = table.getPrePaginationRowModel().rows[vi.index]
                    if (!row) return null
                    return (
                      <div
                        key={row.id}
                        style={{
                          position: "absolute",
                          top: 0, left: 0, width: "100%",
                          transform: `translateY(${vi.start}px)`,
                          height: vi.size,
                        }}
                        className="flex items-center border-b px-3 text-sm transition-colors hover:bg-muted/50"
                        onDoubleClick={() => onEdit(row.original)}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <div key={cell.id} className={cell.column.id === "select" ? "w-9 shrink-0" : "min-w-0 flex-1 truncate pr-2"}>
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </div>
                        ))}
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          ) : (
            <div className="overflow-x-auto scrollbar-thin">
              <Table>
                <TableHeader>
                  {table.getHeaderGroups().map((hg) => (
                    <TableRow key={hg.id}>
                      {hg.headers.map((h) => (
                        <TableHead key={h.id}>
                          {h.column.getCanSort() ? (
                            <button
                              className="inline-flex items-center gap-1 hover:text-foreground"
                              onClick={h.column.getToggleSortingHandler()}
                            >
                              {flexRender(h.column.columnDef.header, h.getContext())}
                              {h.column.getIsSorted() === "asc" && <ArrowUp className="h-3 w-3" />}
                              {h.column.getIsSorted() === "desc" && <ArrowDown className="h-3 w-3" />}
                            </button>
                          ) : (
                            flexRender(h.column.columnDef.header, h.getContext())
                          )}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  <AnimatePresence initial={false}>
                    {rows.map((row, i) => (
                      <motion.tr
                        key={row.id}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.15, delay: Math.min(i * 0.02, 0.2) }}
                        className="border-b transition-colors hover:bg-muted/50"
                        onDoubleClick={() => onEdit(row.original)}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <TableCell key={cell.id}>
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </TableCell>
                        ))}
                      </motion.tr>
                    ))}
                  </AnimatePresence>
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {!virtualEnabled && filteredRows.length > 0 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>共 {filteredRows.length} 条{filteredRows.length > pagination.pageSize ? ` · 第 ${pageIndex + 1}/${pageCount} 页` : ""}</span>
          <div className="flex items-center gap-2">
            <select
              className="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground outline-none transition-colors hover:bg-accent/50"
              value={pagination.pageSize}
              onChange={(e) => setPagination({ pageIndex: 0, pageSize: Number(e.target.value) })}
              aria-label="每页条数"
            >
              {[10, 20, 50, 100].map((n) => (
                <option key={n} value={n}>每页 {n} 条</option>
              ))}
            </select>
            <div className="flex gap-1">
              <Button variant="outline" size="icon" className="h-8 w-8" disabled={pageIndex === 0} onClick={() => table.previousPage()}>
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button variant="outline" size="icon" className="h-8 w-8" disabled={pageIndex >= pageCount - 1} onClick={() => table.nextPage()}>
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
