export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

const AUTH_TOKEN_KEY = "xianchen-admin-token"

export function getStoredToken(): string {
  return localStorage.getItem(AUTH_TOKEN_KEY) ?? ""
}

export function storeToken(token: string) {
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

export function clearStoredToken() {
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getStoredToken()
  let res: Response
  try {
    res = await fetch(path, {
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: "Bearer " + token } : {}),
        ...(init?.headers ?? {}),
      },
      ...init,
    })
  } catch {
    throw new ApiError(0, "网络错误，无法连接管理后台")
  }
  if (!res.ok) {
    if (res.status === 401) {
      // 会话失效：清空本地令牌并通知 AuthGate 呈现登录页
      clearStoredToken()
      window.dispatchEvent(new Event("xianchen:unauthorized"))
    }
    let message = `请求失败（${res.status}）`
    try {
      const body = await res.json()
      message = body?.error ?? body?.message ?? message
    } catch {
      // 非 JSON 错误体，保留默认消息
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!text) return undefined as T
  try {
    return JSON.parse(text) as T
  } catch {
    return text as unknown as T
  }
}

export interface ResourceRecord {
  id: number
  [key: string]: unknown
}

export interface ListResponse<T = ResourceRecord> {
  items?: T[]
  data?: T[]
}

export const api = {
  async list<T = ResourceRecord>(resource: string): Promise<T[]> {
    const raw = await request<unknown>(`/api/${resource}`)
    if (Array.isArray(raw)) return raw as T[]
    const obj = raw as ListResponse<T>
    return obj.items ?? obj.data ?? []
  },
  get: <T = ResourceRecord>(resource: string, id: number | string) =>
    request<T>(`/api/${resource}/${id}`),
  create: <T = ResourceRecord>(resource: string, body: unknown) =>
    request<T>(`/api/${resource}`, { method: "POST", body: JSON.stringify(body) }),
  update: <T = ResourceRecord>(resource: string, id: number | string, body: unknown) =>
    request<T>(`/api/${resource}/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  remove: (resource: string, id: number | string) =>
    request<void>(`/api/${resource}/${id}`, { method: "DELETE" }),
  getJson: <T>(path: string) => request<T>(path),
  postJson: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) }),
  /** 系统设置：按前缀拉取（返回完整数组） */
  configList: <T = ResourceRecord>(prefix: string) =>
    request<T[]>(`/api/config?prefix=${encodeURIComponent(prefix)}`),
  /** 系统设置：按 key 保存（FirstOrCreate，可建可改） */
  configSave: (key: string, body: { value: unknown; value_type?: string; description?: string }) =>
    request<ResourceRecord>(`/api/config/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  /** 运行监控汇总 */
  monitor: <T = Record<string, unknown>>() => request<T>("/api/monitor"),
}
