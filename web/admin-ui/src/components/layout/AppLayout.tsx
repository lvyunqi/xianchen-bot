import { useState } from "react"
import { NavLink, Outlet, useLocation } from "react-router-dom"
import { AnimatePresence, motion } from "framer-motion"
import { Database, Menu, Moon, PanelLeftClose, PanelLeftOpen, RefreshCw, Sun, X } from "lucide-react"
import { useQueryClient } from "@tanstack/react-query"
import { resourcesByGroup, RESOURCE_MAP } from "@/lib/resources/registry"
import { useTheme } from "@/lib/theme"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"

function navLinkClass(isActive: boolean, compact?: boolean) {
  return cn(
    "group relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-muted-foreground transition-all duration-200 hover:bg-accent/70 hover:text-accent-foreground active:scale-[0.98]",
    isActive ? "text-accent-foreground" : "",
    compact && "justify-center px-0",
  )
}

function NavList({ onNavigate, compact }: { onNavigate?: () => void; compact?: boolean }) {
  return (
    <nav className="relative z-10 flex-1 space-y-4 overflow-y-auto px-2 py-3 scrollbar-thin">
      {resourcesByGroup().map(([group, items]) => (
        <div key={group}>
          {!compact && (
            <div className="px-3 pb-1.5 pt-2 text-[11px] font-medium tracking-widest text-muted-foreground/80">
              {group}
            </div>
          )}
          <div className="space-y-0.5">
            {items.map((r) => (
              <Tooltip key={r.key} delayDuration={200}>
                <TooltipTrigger asChild>
                  <NavLink
                    to={r.key === "dashboard" ? "/" : `/r/${r.key}`}
                    onClick={onNavigate}
                    className={({ isActive }) => navLinkClass(isActive, compact)}
                  >
                    {({ isActive }) => (
                      <>
                        {isActive && (
                          <motion.span
                            layoutId="nav-active"
                            className="absolute inset-0 rounded-lg bg-accent"
                            transition={{ type: "spring", stiffness: 380, damping: 32 }}
                          />
                        )}
                        <r.icon
                          className={cn(
                            "relative z-10 h-4 w-4 shrink-0 transition-colors",
                            isActive ? "text-primary" : "text-muted-foreground/70 group-hover:text-foreground",
                          )}
                        />
                        {!compact && (
                          <span className={cn("relative z-10 truncate", isActive && "font-medium text-foreground")}>
                            {r.title}
                          </span>
                        )}
                      </>
                    )}
                  </NavLink>
                </TooltipTrigger>
                {compact && <TooltipContent side="right">{r.title}</TooltipContent>}
              </Tooltip>
            ))}
          </div>
        </div>
      ))}
      <div>
        {!compact && (
          <div className="px-3 pb-1.5 pt-2 text-[11px] font-medium tracking-widest text-muted-foreground/80">
            系统工具
          </div>
        )}
        <NavLink
          to="/database"
          onClick={onNavigate}
          className={({ isActive }) => navLinkClass(isActive, compact)}
        >
          {({ isActive }) => (
            <>
              {isActive && (
                <motion.span
                  layoutId="nav-active"
                  className="absolute inset-0 rounded-lg bg-accent"
                  transition={{ type: "spring", stiffness: 380, damping: 32 }}
                />
              )}
              <Database
                className={cn(
                  "relative z-10 h-4 w-4 shrink-0 transition-colors",
                  isActive ? "text-primary" : "text-muted-foreground/70 group-hover:text-foreground",
                )}
              />
              {!compact && (
                <span className={cn("relative z-10 truncate", isActive && "font-medium text-foreground")}>
                  数据运维
                </span>
              )}
            </>
          )}
        </NavLink>
      </div>
    </nav>
  )
}

function Brand({ compact }: { compact?: boolean }) {
  return (
    <div className={cn("relative z-10 flex h-14 items-center gap-2.5 border-b border-border/60 px-4", compact && "justify-center px-0")}>
      <img src="/admin/assets/logo.png" alt="仙尘" className="h-7 w-7 rounded-lg shadow-card" />
      {!compact && (
        <div className="leading-tight">
          <div className="text-sm font-semibold">仙尘管理后台</div>
          <div className="text-gilded text-[11px] font-medium">修仙界 · 运营台</div>
        </div>
      )}
    </div>
  )
}

export default function AppLayout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const { theme, toggle } = useTheme()
  const location = useLocation()
  const queryClient = useQueryClient()
  const current = RESOURCE_MAP.get(location.pathname.replace(/^\/r\//, "").replace(/^\//, "") || "dashboard")

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex min-h-dvh">
        {/* 桌面侧栏 */}
        <aside
          className={cn(
            "surface-grain fixed inset-y-0 left-0 z-30 hidden flex-col border-r border-border/60 bg-card transition-[width] duration-300 ease-in-out md:flex",
            collapsed ? "w-14" : "w-60",
          )}
        >
          <Brand compact={collapsed} />
          <NavList compact={collapsed} />
          <div className="relative z-10 border-t border-border/60 p-2">
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-center text-muted-foreground"
              onClick={() => setCollapsed((c) => !c)}
              title={collapsed ? "展开侧栏" : "收起侧栏"}
            >
              {collapsed ? <PanelLeftOpen className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
              {!collapsed && <span className="text-xs">收起</span>}
            </Button>
          </div>
        </aside>

        {/* 主区域 */}
        <div className={cn("flex min-w-0 flex-1 flex-col transition-[padding] duration-300 ease-in-out", collapsed ? "md:pl-14" : "md:pl-60")}>
          <header className="sticky top-0 z-20 flex h-14 items-center gap-2 border-b border-border/60 bg-background/75 px-4 backdrop-blur-md">
            <div className="md:hidden">
              <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger asChild>
                  <Button variant="ghost" size="icon">
                    <Menu className="h-5 w-5" />
                  </Button>
                </SheetTrigger>
                <SheetContent side="left" className="w-72 p-0">
                  <SheetTitle className="sr-only">导航菜单</SheetTitle>
                  <Brand />
                  <NavList onNavigate={() => setMobileOpen(false)} />
                </SheetContent>
              </Sheet>
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-semibold">{current?.title ?? "仙尘管理后台"}</div>
              {current && <div className="truncate text-xs text-muted-foreground">{current.description}</div>}
            </div>
            <Button variant="ghost" size="icon" onClick={() => queryClient.invalidateQueries()} title="刷新全部数据">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" onClick={toggle} title="切换主题">
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </header>

          <main className="flex-1 p-4 md:p-6">
            <AnimatePresence mode="wait">
              <motion.div
                key={location.pathname}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -6 }}
                transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
                className="mx-auto w-full max-w-7xl"
              >
                <Outlet />
              </motion.div>
            </AnimatePresence>
          </main>
        </div>
      </div>
    </TooltipProvider>
  )
}
