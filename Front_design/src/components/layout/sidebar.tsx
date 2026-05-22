"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { cn } from "@/lib/utils"
import { getAuthUser, logout, isAdmin } from "@/lib/auth"
import { LogOut, AlertTriangle } from "lucide-react"
import { useEffect, useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

export function Sidebar() {
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const [showLogoutDialog, setShowLogoutDialog] = useState(false)
  const user = typeof window !== "undefined" ? getAuthUser() : null

  useEffect(() => { setMounted(true) }, [])

  const isActive = (href: string) => {
    if (href === "/tasks/new") return pathname === "/tasks/new"
    if (href === "/tasks") return pathname.startsWith("/tasks") && !pathname.startsWith("/tasks/new")
    return pathname.startsWith(href)
  }

  return (
    <>
      <aside className="fixed left-0 top-0 bottom-0 w-64 bg-white border-r border-slate-200 flex flex-col z-40">

        {/* Logo 区：h-16 固定高度 + 下边框，与 HTML 原型一致 */}
        <div className="h-16 flex items-center px-6 border-b border-slate-100 flex-shrink-0">
          {/* 火焰 SVG — 与 StoryforgeUI 原型完全相同的路径 */}
          <svg width="24" height="24" viewBox="0 0 40 40" fill="none" className="mr-3 flex-shrink-0">
            <path d="M20 4C14 4 10 9 10 15C10 19 13 22.5 17 24C15 27 12 28 12 28C14.5 30 18 31 20 31C28 31 32 24 32 15C32 9 26 4 20 4Z" fill="#F97316"/>
          </svg>
          <span className="font-bold text-lg tracking-tight text-slate-900">铸文坊</span>
        </div>

        {/* 主导航 — 无图标，纯文字，与 HTML 原型完全一致 */}
        <nav className="flex-1 px-4 py-6 space-y-1 overflow-y-auto">
          {/* 主要功能 */}
          {[
            { href: "/tasks",     label: "任务列表" },
            { href: "/tasks/new", label: "新建任务" },
            { href: "/dashboard", label: "数据看板" },
          ].map(({ href, label }) => (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center px-3 py-2.5 text-sm font-medium rounded-lg transition-colors",
                isActive(href)
                  ? "bg-orange-50 text-orange-700"
                  : "text-slate-600 hover:bg-slate-50"
              )}
            >
              {label}
            </Link>
          ))}

          {/* 系统设置 分隔标题 */}
          <div className="pt-4 pb-1">
            <div className="px-3 text-xs font-semibold text-slate-400 uppercase tracking-wider">
              系统设置
            </div>
          </div>

          {/* 账号配置 */}
          <Link
            href="/accounts"
            className={cn(
              "flex items-center px-3 py-2.5 text-sm font-medium rounded-lg transition-colors",
              isActive("/accounts")
                ? "bg-orange-50 text-orange-700"
                : "text-slate-600 hover:bg-slate-50"
            )}
          >
            账号配置
          </Link>

          {/* 用户管理（仅管理员可见） */}
          {mounted && isAdmin() && (
            <Link
              href="/admin/users"
              className={cn(
                "flex items-center px-3 py-2.5 text-sm font-medium rounded-lg transition-colors",
                isActive("/admin/users")
                  ? "bg-orange-50 text-orange-700"
                  : "text-slate-600 hover:bg-slate-50"
              )}
            >
              用户管理
              <span className={cn(
                "ml-auto py-0.5 px-1.5 rounded text-[10px] font-semibold",
                isActive("/admin/users")
                  ? "bg-orange-100 text-orange-700"
                  : "bg-slate-100 text-slate-500"
              )}>
                Admin
              </span>
            </Link>
          )}
        </nav>

        {/* 底部用户区（退出功能） */}
        {mounted && user && (
          <div className="border-t border-slate-100 px-4 py-4 flex-shrink-0">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-orange-400 to-red-500 flex items-center justify-center flex-shrink-0">
                <span className="text-white text-xs font-bold">
                  {user.username.slice(0, 2).toUpperCase()}
                </span>
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-slate-900 truncate">{user.username}</p>
                <p className="text-xs text-slate-400">
                  {user.role === "admin" ? "管理员" : "用户"}
                </p>
              </div>
              <button
                onClick={() => setShowLogoutDialog(true)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-red-500 hover:bg-red-50 transition-colors flex-shrink-0"
                title="退出登录"
              >
                <LogOut size={14} />
              </button>
            </div>
          </div>
        )}
      </aside>

      {/* 退出确认弹窗 */}
      <Dialog open={showLogoutDialog} onOpenChange={setShowLogoutDialog}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-red-50 mb-2">
              <AlertTriangle className="h-6 w-6 text-red-500" />
            </div>
            <DialogTitle className="text-center">确认退出</DialogTitle>
            <DialogDescription className="text-center pt-1">
              确定要退出登录吗？退出后需要重新登录才能访问。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="sm:justify-center gap-3 mt-2">
            <Button variant="outline" onClick={() => setShowLogoutDialog(false)}>取消</Button>
            <Button variant="default" onClick={() => { setShowLogoutDialog(false); logout() }}>
              确认退出
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
