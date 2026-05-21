"use client"

import { useEffect, useState, useRef } from "react"
import { usePathname, useRouter } from "next/navigation"
import { isAuthenticated, getToken, clearAuth } from "@/lib/auth"

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || ""

async function verifyToken(): Promise<boolean> {
  const token = getToken()
  if (!token) return false
  try {
    const resp = await fetch(`${API_BASE}/api/auth/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!resp.ok) {
      clearAuth()
      return false
    }
    return true
  } catch {
    clearAuth()
    return false
  }
}

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const [ready, setReady] = useState(false)
  const redirectingRef = useRef(false)

  useEffect(() => {
    if (pathname === "/login") {
      if (isAuthenticated()) {
        verifyToken().then((valid) => {
          if (valid) {
            if (!redirectingRef.current) {
              redirectingRef.current = true
              router.replace("/tasks/new")
            }
          } else {
            setReady(true)
          }
        })
      } else {
        setReady(true)
      }
      return
    }

    if (!isAuthenticated()) {
      if (!redirectingRef.current) {
        redirectingRef.current = true
        router.replace("/login")
      }
      return
    }

    verifyToken().then((valid) => {
      if (valid) {
        setReady(true)
      } else {
        if (!redirectingRef.current) {
          redirectingRef.current = true
          router.replace("/login")
        }
      }
    })
  }, [pathname, router])

  if (pathname === "/login") {
    return ready ? <>{children}</> : null
  }

  if (!ready) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <div className="text-muted-foreground text-sm">加载中...</div>
      </div>
    )
  }

  return <>{children}</>
}
