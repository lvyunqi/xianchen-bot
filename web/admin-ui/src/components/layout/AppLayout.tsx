import { useState } from "react"
import { NavLink, Outlet, useLocation } from "react-router-dom"
import { AnimatePresence, motion, useReducedMotion } from "framer-motion"
import { Command as CommandIcon, Database, LogOut, Menu, Moon, PanelLeftClose, PanelLeftOpen, RefreshCw, Sun, X } from "lucide-react"
import { useQueryClient } from "@tanstack/react-query"
import { resourcesByGroup, RESOURCE_MAP } from "@/lib/resources/registry"
import { clearStoredToken } from "@/lib/api"
import { useTheme } from "@/lib/theme"
import { cn } from "@/lib/utils"
import { CommandPalette } from "@/components/layout/CommandPalette"
import { Button } from "@/components/ui/button"
import { Kbd } from "@/components/ui/kbd"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"

function navLinkClass(isActive: boolean, compact?: boolean) {
  return cn(
    "group relative flex w-full min-w-0 cursor-pointer flex-row flex-nowrap items-center gap-2.5 rounded-md px-3 py-2 text-[13px] leading-5 text-muted-foreground transition-colors duration-150 hover:bg-accent hover:text-accent-foreground",
    isActive ? "bg-accent text-accent-foreground" : "",
    compact && "justify-center px-0",
  )
}

function NavList({ onNavigate, compact }: { onNavigate?: () => void; compact?: boolean }) {
  return (
    <nav className="scrollbar-thin relative z-10 flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-2 py-3">
      {resourcesByGroup().map(([group, items]) => (
        <div key={group}>
          {!compact && (
            <div className="px-2.5 pb-1 pt-1.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/60">
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
                            className="absolute inset-0 rounded-lg border border-gold/40 bg-primary/20"
                            transition={{ type: "spring", stiffness: 420, damping: 36 }}
                          />
                        )}
                        {isActive && !compact && (
                          <motion.span
                            layoutId="nav-marker"
                            className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-gold"
                            transition={{ type: "spring", stiffness: 420, damping: 36 }}
                          />
                        )}
                        <r.icon
                          className={cn(
                            "relative z-10 h-4 w-4 shrink-0 transition-colors duration-200",
                            isActive ? "text-gold" : "text-muted-foreground/75 group-hover:text-foreground",
                          )}
                        />
                        {!compact && (
                          <span className={cn("relative z-10 truncate", isActive && "font-medium")}>{r.title}</span>
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
          <div className="px-2.5 pb-1 pt-1.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/60">
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
                  className="absolute inset-0 rounded-lg border border-gold/40 bg-primary/20"
                  transition={{ type: "spring", stiffness: 420, damping: 36 }}
                />
              )}
              {isActive && !compact && (
                <motion.span
                  layoutId="nav-marker"
                  className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-gold"
                  transition={{ type: "spring", stiffness: 420, damping: 36 }}
                />
              )}
              <Database
                className={cn(
                  "relative z-10 h-4 w-4 shrink-0 transition-colors duration-200",
                  isActive ? "text-gold" : "text-muted-foreground/75 group-hover:text-foreground",
                )}
              />
              {!compact && (
                <span className={cn("relative z-10 truncate", isActive && "font-medium")}>数据运维</span>
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
    <div
      className={cn(
        "relative z-10 flex h-14 shrink-0 items-center gap-2.5 border-b border-border/40 px-3.5",
        compact && "justify-center px-0",
      )}
    >
      <img src="/admin/assets/logo.png" alt="仙尘" className="h-8 w-8 rounded-xl shadow-card" />
      {!compact && (
        <div className="min-w-0 leading-tight">
          <div className="truncate text-[13px] font-semibold tracking-wide">仙尘管理后台</div>
          <div className="text-gilded truncate text-[10px] font-medium tracking-[0.18em]">修仙界 · 运营台</div>
        </div>
      )}
    </div>
  )
}

export default function AppLayout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const { theme, toggle } = useTheme()
  const location = useLocation()
  const reduceMotion = useReducedMotion()
  const queryClient = useQueryClient()
  const current = RESOURCE_MAP.get(location.pathname.replace(/^\/r\//, "").replace(/^\//, "") || "dashboard")
  const refreshAll = () => queryClient.invalidateQueries()

  return (
    <TooltipProvider delayDuration={200}>
      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        onRefresh={() => queryClient.invalidateQueries()}
        onToggleTheme={toggle}
        theme={theme}
      />
      <div className="bg-aurora" aria-hidden />
      <div className="flex min-h-dvh">
        {/* 桌面侧栏：悬浮玻璃面板 */}
        <aside
          className={cn(
            "fixed inset-y-0 left-0 z-30 hidden flex-col border-r border-border bg-card md:flex",
            collapsed ? "w-14" : "w-60",
          )}
        >
          <Brand compact={collapsed} />
          <NavList compact={collapsed} />
          <div className="relative z-10 border-t border-border/40 p-2">
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
        <div
          className={cn(
            "flex min-w-0 flex-1 flex-col transition-[padding] duration-300 ease-in-out",
            collapsed ? "md:pl-14" : "md:pl-60",
          )}
        >
          <header className="sticky top-0 z-20 flex h-14 items-center gap-1.5 border-b border-border/40 bg-background/55 px-4 backdrop-blur-xl">
            <div className="md:hidden">
              <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger asChild>
                  <Button variant="ghost" size="icon">
                    <Menu className="h-5 w-5" />
                  </Button>
                </SheetTrigger>
                <SheetContent side="left" className="w-[min(18rem,calc(100vw-1rem))] bg-card p-0">
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
            <Button
              variant="outline"
              size="sm"
              className="glass hidden cursor-pointer gap-2 border-border/40 text-muted-foreground hover:text-foreground sm:flex"
              onClick={() => setPaletteOpen(true)}
            >
              <CommandIcon className="h-3.5 w-3.5" />
              <span className="text-xs">搜索</span>
              <Kbd>Ctrl K</Kbd>
            </Button>
            <Button variant="ghost" size="icon" className="cursor-pointer sm:hidden" onClick={() => setPaletteOpen(true)} title="搜索">
              <CommandIcon className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="cursor-pointer" onClick={() => queryClient.invalidateQueries()} title="刷新全部数据">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="cursor-pointer"
              title="退出登录"
              onClick={() => {
                clearStoredToken()
                window.location.reload()
              }}
            >
              <LogOut className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="cursor-pointer" onClick={toggle} title="切换主题">
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </header>

          <main className="min-w-0 flex-1 overflow-x-hidden p-4 md:px-6 md:py-5">
            <AnimatePresence mode="wait">
              <motion.div
                key={location.pathname}
                initial={reduceMotion ? false : { opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={reduceMotion ? undefined : { opacity: 0, y: -4 }}
                transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                className="mx-auto w-full max-w-[1400px]"
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
