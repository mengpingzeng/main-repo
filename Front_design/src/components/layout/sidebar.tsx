"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { cn } from "@/lib/utils"
import { getAuthUser, logout, isAdmin } from "@/lib/auth"
import {
  PlusCircle,
  FileText,
  LayoutDashboard,
  Shield,
  Users,
  LogOut,
  User,
  AlertTriangle,
} from "lucide-react"
import { useEffect, useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

const BASE_ITEMS = [
  { href: "/tasks/new", label: "新建任务", icon: PlusCircle },
  { href: "/tasks", label: "任务列表", icon: FileText },
  { href: "/dashboard", label: "数据看板", icon: LayoutDashboard },
  { href: "/accounts", label: "账号配置", icon: Shield },
]

export function Sidebar() {
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const [showLogoutDialog, setShowLogoutDialog] = useState(false)
  const user = typeof window !== "undefined" ? getAuthUser() : null

  useEffect(() => {
    setMounted(true)
  }, [])

  const navItems = isAdmin()
    ? [...BASE_ITEMS, { href: "/admin/users", label: "用户管理", icon: Users }]
    : BASE_ITEMS

  return (
    <>
    <aside className="fixed left-0 top-0 bottom-0 w-[240px] bg-[#F4F2F6] border-r border-border flex flex-col z-40">
      {/* Logo */}
      <div className="px-4 pt-5 pb-1">
        <div className="flex items-center gap-2.5">
          <img src="/logo.png" alt="Logo" className="w-10 h-10" />
          <span className="font-semibold text-foreground text-[15px] tracking-tight">
            小龙虾创作
          </span>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-3 py-3 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive =
            item.href === "/tasks/new"
              ? pathname === "/tasks/new"
              : item.href === "/tasks"
                ? pathname.startsWith("/tasks") && !pathname.startsWith("/tasks/new")
                : pathname.startsWith(item.href)

          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 px-3 h-10 rounded-xl text-[14px] transition-all duration-150",
                isActive
                  ? "bg-white text-foreground font-semibold shadow-[0_2px_8px_rgba(0,0,0,0.08)]"
                  : "text-[#4e5969] hover:bg-[#eeeff2] hover:text-foreground"
              )}
            >
              <Icon size={18} className={isActive ? "text-[#635bff]" : "text-[#8c8c8c]"} />
              {item.label}
            </Link>
          )
        })}
      </nav>

      {/* User */}
      {mounted && user && (
        <div className="p-3">
          <div className="bg-white rounded-xl p-3 shadow-[0_2px_8px_rgba(0,0,0,0.08)]">
            <div className="flex items-center gap-2.5">
              <div className="w-9 h-9 rounded-full bg-primary/10 flex items-center justify-center">
                <User size={16} className="text-primary" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[13px] font-semibold text-foreground truncate">{user.username}</p>
                {user.role === "admin" && (
                  <p className="text-[11px] text-muted-foreground">管理员</p>
                )}
              </div>
              <button
                onClick={() => setShowLogoutDialog(true)}
                className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                title="退出登录"
              >
                <LogOut size={15} />
              </button>
            </div>
          </div>
        </div>
      )}
    </aside>

    <Dialog open={showLogoutDialog} onOpenChange={setShowLogoutDialog}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 mb-2">
            <AlertTriangle className="h-6 w-6 text-destructive" />
          </div>
          <DialogTitle className="text-center">确认退出</DialogTitle>
          <DialogDescription className="text-center pt-1">
            确定要退出登录吗？退出后需要重新登录才能访问。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="sm:justify-center gap-3 mt-2">
          <Button variant="outline" onClick={() => setShowLogoutDialog(false)}>
            取消
          </Button>
          <Button
              variant="default"
            onClick={() => {
              setShowLogoutDialog(false)
              logout()
            }}
          >
            确认退出
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    </>
  )
}
