import { useCallback, useEffect, useState } from "react"
import { KeyRound, Loader2, ShieldCheck } from "lucide-react"
import { ApiError, api, clearStoredToken, getStoredToken, storeToken } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

interface VerifyResponse {
  enabled: boolean
}

type Phase = "checking" | "login" | "ready"

export default function AuthGate({ children }: { children: React.ReactNode }) {
  const [phase, setPhase] = useState<Phase>("checking")
  const [input, setInput] = useState("")
  const [error, setError] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const verify = useCallback(async (token: string): Promise<boolean> => {
    try {
      // verify 自身免鉴权；携带本地令牌以便校验既有会话
      const res = await fetch("/api/auth/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      })
      if (res.status === 401) return false
      const body = (await res.json()) as { data?: VerifyResponse }
      const enabled = body?.data?.enabled ?? false
      return !enabled || (token !== "" && res.ok)
    } catch {
      // 网络失败时若本地已有令牌，放行进入（页面内 API 会再次校验）
      return token !== ""
    }
  }, [])

  const check = useCallback(async () => {
    setPhase("checking")
    const token = getStoredToken()
    const ok = await verify(token)
    setPhase(ok ? "ready" : "login")
  }, [verify])

  useEffect(() => {
    void check()
  }, [check])

  useEffect(() => {
    const onUnauthorized = () => {
      clearStoredToken()
      setPhase("login")
    }
    window.addEventListener("xianchen:unauthorized", onUnauthorized)
    return () => window.removeEventListener("xianchen:unauthorized", onUnauthorized)
  }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const token = input.trim()
    if (!token) {
      setError("请输入访问令牌")
      return
    }
    setSubmitting(true)
    setError("")
    try {
      const res = await api.postJson<unknown>("/api/auth/verify", { token })
      const data = res as { data?: VerifyResponse }
      if (data?.data?.enabled === false) {
        // 服务端未启用鉴权，无需令牌
        storeToken("")
        setPhase("ready")
        return
      }
      if (res) {
        storeToken(token)
        setPhase("ready")
      }
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? "访问令牌不正确" : "验证失败，请稍后重试")
    } finally {
      setSubmitting(false)
    }
  }

  if (phase === "checking") {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          正在验证会话…
        </div>
      </div>
    )
  }

  if (phase === "login") {
    return (
      <div className="surface-grain flex min-h-dvh items-center justify-center p-6">
        <Card className="relative z-10 w-full max-w-sm overflow-hidden">
          <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-gold/60 to-transparent" />
          <CardContent className="p-8">
            <div className="mb-6 flex flex-col items-center gap-3 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-accent text-accent-foreground">
                <ShieldCheck className="h-6 w-6" />
              </div>
              <div>
                <div className="text-lg font-semibold">仙尘管理后台</div>
                <div className="text-gilded text-xs font-medium">修仙界 · 运营台</div>
              </div>
              <p className="text-xs leading-relaxed text-muted-foreground">
                此后台已启用访问令牌保护，请输入配置文件中设置的令牌登录。
              </p>
            </div>
            <form onSubmit={submit} className="space-y-3">
              <div className="relative">
                <KeyRound className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="password"
                  className="pl-8"
                  placeholder="访问令牌"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  autoFocus
                />
              </div>
              {error && <p className="text-xs text-destructive">{error}</p>}
              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting ? "验证中…" : "登录"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    )
  }

  return <>{children}</>
}
