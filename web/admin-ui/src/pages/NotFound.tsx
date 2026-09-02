import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"

export default function NotFound() {
  return (
    <div className="flex flex-col items-center gap-4 py-24 text-center">
      <div className="text-5xl font-semibold text-muted-foreground/40">404</div>
      <div className="text-sm text-muted-foreground">页面不存在或已移动</div>
      <Button asChild variant="outline">
        <Link to="/">返回总览</Link>
      </Button>
    </div>
  )
}
