"use client"

import { useEffect, useState } from "react"
import { getAuthUser, isAuthenticated, logout } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { LogOut } from "lucide-react"

export function Header() {
  const [mounted, setMounted] = useState(false)
  const user = typeof window !== "undefined" ? getAuthUser() : null

  useEffect(() => {
    setMounted(true)
  }, [])

  if (!mounted || !user) return null

  return (
    <header className="h-16 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between px-8 sticky top-0 z-30">
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">
          {user.role === "admin" ? "管理员模式" : ""}
        </span>
      </div>
    </header>
  )
}
