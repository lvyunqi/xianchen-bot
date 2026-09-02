export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  })
  if (!res.ok) {
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
}
