import * as React from "react"
import { cn } from "@/lib/utils"

/** 键盘按键标识：命令面板快捷键提示等场景。 */
function Kbd({ className, ...props }: React.HTMLAttributes<HTMLElement>) {
  return (
    <kbd
      className={cn(
        "pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border border-border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground",
        className,
      )}
      {...props}
    />
  )
}

export { Kbd }
