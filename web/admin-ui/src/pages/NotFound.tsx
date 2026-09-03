import { Link } from "react-router-dom"
import { Compass } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

export default function NotFound() {
  return (
    <Card className="mx-auto max-w-md border-dashed">
      <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-accent text-accent-foreground">
          <Compass className="h-5 w-5" />
        </div>
        <div className="tnum text-4xl font-semibold tracking-tight text-muted-foreground/40">404</div>
        <p className="text-sm text-muted-foreground">此路不通——页面不存在或已随版本整理移除。</p>
        <Button asChild variant="outline" size="sm" className="mt-1">
          <Link to="/">返回总览</Link>
        </Button>
      </CardContent>
    </Card>
  )
}
