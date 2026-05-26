"use client"

import { useEffect, useState, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { toast } from "@/components/ui/toast"
import { Input, Label } from "@/components/ui/input"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { fetchUsers, createUser, updateUser, deleteUser } from "@/lib/api"
import { getAuthUser } from "@/lib/auth"
import type { AdminUserInfo } from "@/types"
import { formatRelativeTime, formatDate } from "@/lib/utils"
import { Loader2, AlertCircle, Eye, EyeOff } from "lucide-react"

function formatLastLogin(v?: string) {
  if (!v) return <span className="text-slate-300">从未登录</span>
  return <span title={formatDate(v)}>{formatRelativeTime(v)}</span>
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUserInfo[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [page, setPageState] = useState(1)

  const setPage = (p: number) => {
    setPageState(p)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }
  const pageSize = 5
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const [showCreate, setShowCreate] = useState(false)
  const [showEdit, setShowEdit] = useState<AdminUserInfo | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AdminUserInfo | null>(null)
  const currentUser = getAuthUser()

  const loadUsers = useCallback(async (targetPage = page) => {
    setLoading(true)
    setError("")
    try {
      const result = await fetchUsers(targetPage, pageSize)
      setUsers(result.users)
      setTotal(result.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [page, pageSize])

  useEffect(() => { loadUsers(page) }, [page, loadUsers])

  const getInitials = (name: string) => name.slice(0, 2).toUpperCase()

  return (
    <div className="max-w-7xl mx-auto px-6 pt-6">

      {/* 页头 */}
      <div className="flex justify-between items-end mb-8">
        <div>
          <h1 className="text-3xl font-bold text-slate-900 tracking-tight">用户权限管理</h1>
          <p className="text-slate-500 mt-1">管理系统内的操作员账号及系统角色分配</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-slate-900 text-white text-sm font-medium rounded-lg hover:bg-slate-800 shadow-sm transition-colors"
        >
          + 邀请新用户
        </button>
      </div>

      {/* 内容区 */}
      {loading ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="w-6 h-6 animate-spin text-orange-500" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-24">
          <AlertCircle className="w-12 h-12 mb-4 text-slate-300" />
          <p className="text-sm text-slate-500 mb-3">{error}</p>
          <Button variant="ghost" size="sm" onClick={() => loadUsers(page)}>重试</Button>
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-2xl shadow-sm overflow-hidden">
          <table className="w-full text-left text-sm table-fixed">
            <colgroup>
              <col className="w-[22%]" />
              <col className="w-[10%]" />
              <col className="w-[18%]" />
              <col className="w-[18%]" />
              <col className="w-[12%]" />
              <col className="w-[20%]" />
            </colgroup>
            <thead className="bg-slate-50 text-slate-500 border-b border-slate-200">
              <tr>
                <th className="px-6 py-4 font-medium">用户档案</th>
                <th className="px-6 py-4 font-medium">系统角色</th>
                <th className="px-6 py-4 font-medium text-center">绑定账号</th>
                <th className="px-6 py-4 font-medium text-center">任务数</th>
                <th className="px-6 py-4 font-medium">最后登录</th>
                <th className="px-6 py-4 font-medium text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-slate-700">
              {users.map((u) => {
                const isSelf = u.uid === currentUser?.uid
                const canDelete = !isSelf
                const isAdminRole = u.role === "admin"
                return (
                  <tr key={u.uid} className="hover:bg-slate-50 transition-colors">

                    {/* 用户档案 */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0 ${
                          isSelf || isAdminRole
                            ? "bg-gradient-to-tr from-orange-400 to-red-500 text-white"
                            : "bg-slate-200 text-slate-600"
                        }`}>
                          {getInitials(u.username)}
                        </div>
                        <div className="min-w-0">
                          <p className="font-medium text-slate-900 line-clamp-2 break-all leading-snug">
                            {u.username}
                            {isSelf && <span className="text-slate-400 font-normal ml-1">（您）</span>}
                          </p>
                        </div>
                      </div>
                    </td>

                    {/* 系统角色 */}
                    <td className="px-6 py-4">
                      <span className={`px-2 py-1 text-xs font-medium rounded border ${
                        isAdminRole
                          ? "text-purple-700 bg-purple-50 border-purple-100"
                          : "text-slate-600 bg-slate-100 border-slate-200"
                      }`}>
                        {isAdminRole ? "管理员" : "用户"}
                      </span>
                    </td>

                    {/* 绑定账号 */}
                    <td className="px-6 py-4 text-center">{u.accountCount}</td>

                    {/* 任务数 */}
                    <td className="px-6 py-4 text-center">{u.taskCount}</td>

                    {/* 最后登录 */}
                    <td className="px-6 py-4 text-slate-500">{formatLastLogin(u.lastLoginAt)}</td>

                    {/* 操作 */}
                    <td className="px-6 py-4 text-right">
                      {isSelf ? (
                        <span className="text-slate-300">无法修改自身</span>
                      ) : (
                        <div className="flex items-center justify-end gap-4">
                          <button
                            onClick={() => setShowEdit(u)}
                            className="text-orange-600 hover:text-orange-700 font-medium transition-colors"
                          >
                            编辑
                          </button>
                          <button
                            onClick={() => setDeleteTarget(u)}
                            disabled={!canDelete}
                            title={isSelf ? "不能删除当前登录账号" : "删除用户"}
                            className={canDelete
                              ? "text-red-500 hover:text-red-700 font-medium transition-colors"
                              : "text-slate-300 font-medium cursor-not-allowed"
                            }
                          >
                            删除
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* 分页 */}
      {total > pageSize && (
        <div className="flex items-center justify-between mt-6 text-sm text-slate-500">
          <span>共 {total} 位用户</span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="px-3 py-1.5 border border-slate-200 bg-white text-slate-600 rounded-lg text-sm font-medium hover:bg-slate-50 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              上一页
            </button>
            <span className="px-2 text-slate-400">{page} / {totalPages}</span>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="px-3 py-1.5 border border-slate-200 bg-white text-slate-600 rounded-lg text-sm font-medium hover:bg-slate-50 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              下一页
            </button>
          </div>
        </div>
      )}

      {/* 弹窗 */}
      <CreateUserModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={() => {
          setShowCreate(false)
          setPage(1)
          loadUsers(1)
          toast.success("用户创建成功")
        }}
      />
      {showEdit && (
        <EditUserModal
          user={showEdit}
          open={!!showEdit}
          onClose={() => setShowEdit(null)}
          onUpdated={() => {
            setShowEdit(null)
            loadUsers(page)
            toast.success("用户信息已更新")
          }}
        />
      )}
      <DeleteConfirmModal
        user={deleteTarget}
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onDeleted={() => {
          setDeleteTarget(null)
          const newPage = users.length === 1 && page > 1 ? page - 1 : page
          if (newPage !== page) setPage(newPage)
          else loadUsers(page)
          toast.success("用户已删除")
        }}
      />
    </div>
  )
}

/* ── 新建用户弹窗 ── */
function CreateUserModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [showPwd, setShowPwd] = useState(false)
  const [role, setRole] = useState<"user" | "admin">("user")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    if (password.length < 8) { setError("密码至少需要 8 位"); return }
    setLoading(true)
    try {
      await createUser({ username: username.trim(), password, role })
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader><DialogTitle>新增用户</DialogTitle></DialogHeader>
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <Label>用户名</Label>
            <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="请输入用户名" />
          </div>
          <div>
            <Label>密码</Label>
            <div className="relative">
              <Input type={showPwd ? "text" : "password"} value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少 8 位" />
              <button type="button" onClick={() => setShowPwd(!showPwd)} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
                {showPwd ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </div>
          </div>
          <div>
            <Label>角色</Label>
            <SelectRadix value={role} onValueChange={(v) => setRole(v as "user" | "admin")}>
              <SelectItem value="user">用户</SelectItem>
              <SelectItem value="admin">管理员</SelectItem>
            </SelectRadix>
          </div>
          {error && <div className="text-xs text-destructive bg-destructive/8 rounded-lg px-3 py-2">{error}</div>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
            <Button type="submit" disabled={loading}>
              {loading ? <><Loader2 size={14} className="animate-spin" />创建中...</> : "确认"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/* ── 编辑用户弹窗 ── */
function EditUserModal({ user, open, onClose, onUpdated }: { user: AdminUserInfo; open: boolean; onClose: () => void; onUpdated: () => void }) {
  const [password, setPassword] = useState("")
  const [showPwd, setShowPwd] = useState(false)
  const [role, setRole] = useState<"user" | "admin">(user.role)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const currentUser = getAuthUser()
  const isSelf = user.uid === currentUser?.uid

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    if (password && password.length < 8) { setError("密码至少需要 8 位"); return }
    setLoading(true)
    try {
      const body: { password?: string; role?: "user" | "admin" } = {}
      if (password) body.password = password
      if (role !== user.role) body.role = role
      await updateUser(user.uid, body)
      onUpdated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "修改失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader><DialogTitle>编辑用户 · {user.username}</DialogTitle></DialogHeader>
        <form onSubmit={handleUpdate} className="space-y-4">
          <div>
            <Label>新密码（留空则不修改）</Label>
            <div className="relative">
              <Input type={showPwd ? "text" : "password"} value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少 8 位，留空则不修改" />
              <button type="button" onClick={() => setShowPwd(!showPwd)} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
                {showPwd ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </div>
          </div>
          <div>
            <Label>角色 {isSelf && <span className="text-xs text-orange-500 ml-1">（不能修改自己的角色）</span>}</Label>
            <SelectRadix value={role} onValueChange={(v) => setRole(v as "user" | "admin")} disabled={isSelf}>
              <SelectItem value="user">用户</SelectItem>
              <SelectItem value="admin">管理员</SelectItem>
            </SelectRadix>
          </div>
          {error && <div className="text-xs text-destructive bg-destructive/8 rounded-lg px-3 py-2">{error}</div>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
            <Button type="submit" disabled={loading}>
              {loading ? <><Loader2 size={14} className="animate-spin" />保存中...</> : "保存"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/* ── 删除确认弹窗 ── */
function DeleteConfirmModal({ user, open, onClose, onDeleted }: { user: AdminUserInfo | null; open: boolean; onClose: () => void; onDeleted: () => void }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const handleDelete = async () => {
    if (!user) return
    setLoading(true)
    setError("")
    try {
      await deleteUser(user.uid)
      onDeleted()
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除失败")
    } finally {
      setLoading(false)
    }
  }

  const hasAccounts = (user?.accountCount ?? 0) > 0

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader><DialogTitle>确认删除</DialogTitle></DialogHeader>
        {hasAccounts ? (
          <div className="text-sm text-slate-500 space-y-2">
            <p>
              用户 <span className="text-slate-900 font-medium">{user?.username}</span> 仍绑定{" "}
              <span className="text-slate-900 font-medium">{user?.accountCount}</span> 个发布账号。
            </p>
            <p>确认删除后，这些账号绑定将一并解除，且无法恢复。是否继续？</p>
          </div>
        ) : (
          <p className="text-sm text-slate-500">
            确定要删除用户 <span className="text-slate-900 font-medium">{user?.username}</span> 吗？此操作不可撤销。
          </p>
        )}
        {error && <div className="text-xs text-destructive bg-destructive/8 rounded-lg px-3 py-2">{error}</div>}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button variant="destructive" onClick={handleDelete} disabled={loading}>
            {loading ? <><Loader2 size={14} className="animate-spin" />删除中...</> : "确认删除"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
