import { getToken } from "./auth"
import type {
  Model,
  Skill,
  AccountSummary,
  AccountListResponse,
  BindRequest,
  BindResponse,
  UnbindRequest,
  UnbindResponse,
  CookieHealthResponse,
  AccountCredentialResponse,
  TaskCreateInput,
  TaskCreateResponse,
  TaskSummary,
  TaskListResponse,
  SessionCreateInput,
  SessionCreateResponse,
  Session,
  Draft,
  SendMessageResponse,
  PublishInput,
  DashboardQueryRequest,
  DashboardQueryResponse,
  AdminUserInfo,
  AdminUserListResponse,
  CreateUserRequest,
  CreateUserResponse,
  UpdateUserRequest,
  UpdateUserResponse,
} from "@/types"

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || ""

function authHeaders(): Record<string, string> {
  const token = getToken()
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (token) headers["Authorization"] = `Bearer ${token}`
  return headers
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { headers: authHeaders() })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.message || body.errorMessage || `HTTP ${res.status}`)
  }
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}))
    throw new Error(errBody.message || errBody.errorMessage || `HTTP ${res.status}`)
  }
  return res.json()
}

// ===== 资源 =====

export async function fetchModels(): Promise<Model[]> {
  const data = await get<{ count: number; models: Model[] }>("/api/models")
  return data.models || []
}

export async function fetchSkills(): Promise<Skill[]> {
  const data = await get<{ skills: Skill[]; total: number }>("/api/skill/list")
  return data.skills || []
}

// ===== 账号 =====

export async function fetchAccounts(platform = ""): Promise<AccountSummary[]> {
  const params = new URLSearchParams()
  if (platform) params.set("platform", platform)
  const qs = params.toString()
  const resp = await get<AccountListResponse>(`/api/account/list${qs ? "?" + qs : ""}`)
  return resp.accounts || []
}

export async function bindAccount(
  platform: string,
  credentialsPlaintext: string,
  maskedDisplay?: string,
  accountId?: string
): Promise<BindResponse> {
  return post<BindResponse>("/api/account/bind", {
    platform,
    credentials_plaintext: credentialsPlaintext,
    masked_display: maskedDisplay || undefined,
    account_id: accountId || undefined,
    caller: "bff",
  })
}

export async function unbindAccount(accountId: string): Promise<UnbindResponse> {
  return post<UnbindResponse>("/api/account/unbind", {
    account_id: accountId,
    caller: "bff",
  })
}

/**
 * 检测账号 Cookie 是否仍有效。
 * 后端路由：GET /api/account/health/:account_id
 * 后端逻辑：用存储的 cookie 向对应平台发一次轻量探测请求（如获取用户信息），
 * 成功则返回 valid=true，失败（401/403/解析失败）则返回 valid=false。
 * 若后端尚未实现此路由，前端会捕获错误并将该账号标记为 'unknown'。
 */
export async function checkCookieHealth(accountId: string): Promise<CookieHealthResponse> {
  return get<CookieHealthResponse>(`/api/account/health/${accountId}`)
}

/** 用户自取账号凭证明文（仅用于 Cookie 注入回浏览器），凭证不应落 localStorage */
export async function fetchAccountCredential(accountId: string): Promise<AccountCredentialResponse> {
  return get<AccountCredentialResponse>(`/api/account/credential/${accountId}`)
}

// ===== 任务 =====

export async function createTask(input: TaskCreateInput): Promise<TaskCreateResponse> {
  return post<TaskCreateResponse>("/api/task/create", input)
}

export async function fetchTasks(): Promise<TaskSummary[]> {
  const resp = await get<TaskListResponse>("/api/task/list")
  return resp.tasks || []
}

// ===== 会话 =====

export async function createSession(input: SessionCreateInput): Promise<SessionCreateResponse> {
  return post<SessionCreateResponse>("/api/session/create", input)
}

export async function getSession(sessionId: string): Promise<Session> {
  return get<Session>(`/api/session/${sessionId}/status`)
}

export async function getDraft(sessionId: string): Promise<Draft> {
  return get<Draft>(`/api/session/${sessionId}/draft`)
}

export async function sendMessage(
  sessionId: string,
  text: string,
  draftVersion?: number
): Promise<SendMessageResponse> {
  return post<SendMessageResponse>(`/api/session/${sessionId}/message`, {
    text,
    draft_version: draftVersion,
  })
}

export async function closeSession(sessionId: string): Promise<{ episode_id: string }> {
  return post<{ episode_id: string }>(`/api/session/${sessionId}/close`, {})
}

// ===== 发布 =====

export async function publishTask(taskId: string, body: PublishInput): Promise<{ status: string; taskId: string; results?: Array<{ status: string; platform: string; accountId: string; postId?: string; errorCode?: string }> }> {
  return post<any>(`/api/task/${taskId}/publish`, body)
}

// ===== 看板 =====

function buildQuery(params: Record<string, unknown>): string {
  const parts = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue
    if (Array.isArray(value)) {
      for (const item of value) {
        parts.append(key, String(item))
      }
    } else {
      parts.append(key, String(value))
    }
  }
  const qs = parts.toString()
  return qs ? `?${qs}` : ""
}

export async function fetchDashboard(req: DashboardQueryRequest = {}): Promise<DashboardQueryResponse> {
  return get<DashboardQueryResponse>(`/api/dashboard/query${buildQuery(req as unknown as Record<string, unknown>)}`)
}

// ===== 用户管理（管理员） =====

export async function fetchUsers(page = 1, size = 20): Promise<{ users: AdminUserInfo[]; total: number }> {
  const resp = await get<AdminUserListResponse>(`/api/admin/users?page=${page}&size=${size}`)
  return { users: resp.users || [], total: resp.total || 0 }
}

export async function createUser(req: CreateUserRequest): Promise<CreateUserResponse> {
  return post<CreateUserResponse>("/api/admin/users", req)
}

export async function updateUser(uid: string, req: UpdateUserRequest): Promise<UpdateUserResponse> {
  return put<UpdateUserResponse>(`/api/admin/users/${uid}`, req)
}

export async function deleteUser(uid: string): Promise<{ deleted: boolean }> {
  return del<{ deleted: boolean }>(`/api/admin/users/${uid}`)
}

async function put<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}))
    throw new Error(errBody.message || errBody.errorMessage || `HTTP ${res.status}`)
  }
  return res.json()
}

async function del<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "DELETE",
    headers: authHeaders(),
  })
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}))
    throw new Error(errBody.message || errBody.errorMessage || `HTTP ${res.status}`)
  }
  return res.json()
}
