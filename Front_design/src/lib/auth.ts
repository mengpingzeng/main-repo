"use client"

const JWT_KEY = "bff_jwt_token"
const USER_KEY = "bff_user"
const API_BASE = process.env.NEXT_PUBLIC_API_BASE || ""

export interface AuthUser {
  uid: string
  username: string
  role: "admin" | "user"
  token: string
}

let memoryUser: AuthUser | null = null

export function setAuthUser(user: AuthUser) {
  memoryUser = user
  try {
    localStorage.setItem(JWT_KEY, user.token)
    localStorage.setItem(USER_KEY, JSON.stringify({ uid: user.uid, username: user.username, role: user.role }))
  } catch { /* ignore */ }
}

export function getAuthUser(): AuthUser | null {
  if (memoryUser) return memoryUser
  try {
    const token = localStorage.getItem(JWT_KEY)
    const userStr = localStorage.getItem(USER_KEY)
    if (token && userStr) {
      const user = JSON.parse(userStr) as Omit<AuthUser, "token">
      memoryUser = { ...user, token }
      return memoryUser
    }
  } catch { /* ignore */ }
  return null
}

export function getToken(): string | null {
  if (memoryUser) return memoryUser.token
  try {
    const stored = localStorage.getItem(JWT_KEY)
    if (stored) return stored
  } catch { /* ignore */ }
  return null
}

export function isAuthenticated(): boolean {
  return getToken() !== null
}

export function isAdmin(): boolean {
  return getAuthUser()?.role === "admin"
}

export function clearAuth() {
  memoryUser = null
  try {
    localStorage.removeItem(JWT_KEY)
    localStorage.removeItem(USER_KEY)
  } catch { /* ignore */ }
}

export async function login(username: string, password: string): Promise<AuthUser> {
  const resp = await fetch(`${API_BASE}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  })
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error(body.message || body.errorMessage || "登录失败")
  }
  const data = await resp.json()
  const user: AuthUser = {
    uid: data.uid,
    username: data.username,
    role: data.role,
    token: data.token,
  }
  setAuthUser(user)
  return user
}

export function logout() {
  clearAuth()
  if (typeof window !== "undefined") {
    window.location.href = "/login"
  }
}

export async function fetchCurrentUser(): Promise<{ uid: string; username: string; role: string } | null> {
  const token = getToken()
  if (!token) return null
  try {
    const resp = await fetch(`${API_BASE}/api/auth/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!resp.ok) return null
    const data = await resp.json()
    return data.data || null
  } catch {
    return null
  }
}
