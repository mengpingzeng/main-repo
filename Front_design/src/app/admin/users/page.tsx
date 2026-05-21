"use client"

import { useEffect, useState, useCallback } from "react"
import { useAutoMessage } from "@/hooks/useAutoMessage"
import { Button } from "@/components/ui/button"
import { Input, Label } from "@/components/ui/input"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { fetchUsers, createUser, updateUser, deleteUser } from "@/lib/api"
import { getAuthUser } from "@/lib/auth"
import type { AdminUserInfo } from "@/types"
import { formatDate } from "@/lib/utils"
import {
  Plus, Loader2, AlertCircle, CheckCircle, Trash2, Edit3, Eye, EyeOff, Users,
} from "lucide-react"

export default function AdminUsersPage() {
  const [allUsers, setAllUsers] = useState<AdminUserInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const { message, setMessage } = useAutoMessage()
  const [page, setPageState] = useState(1)

  const setPage = (p: number) => {
    setPageState(p)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }
  const pageSize = 5

  const total = allUsers.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const users = allUsers.slice((page - 1) * pageSize, page * pageSize)

  const [showCreate, setShowCreate] = useState(false)
  const [showEdit, setShowEdit] = useState<AdminUserInfo | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AdminUserInfo | null>(null)

  const currentUser = getAuthUser()

  const loadUsers = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const result = await fetchUsers()
      setAllUsers(result.users)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadUsers() }, [loadUsers])

  return (
    <div className="w-full max-w-[1080px]">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-lg font-semibold text-foreground">
          用户管理
        </h1>
        <Button size="lg" onClick={() => setShowCreate(true)}>
          <Plus size={16} /> 新增用户
        </Button>
      </div>

      {message && (
        <div className={`mb-4 p-3 rounded-lg text-sm flex items-center gap-2 ${
          message.type === "success"
            ? "bg-[#22c55e]/8 border border-[#22c55e]/20 text-[#22c55e]"
            : "bg-destructive/8 border border-destructive/20 text-destructive"
        }`}>
          {message.type === "success" ? <CheckCircle size={14} /> : <AlertCircle size={14} />}
          {message.text}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-6 h-6 animate-spin text-primary" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-20">
          <AlertCircle className="w-12 h-12 mb-4 text-destructive opacity-40" />
          <p className="text-sm text-muted-foreground mb-1">{error}</p>
          <Button variant="ghost" size="sm" onClick={loadUsers}>重试</Button>
        </div>
      ) : (
        <div className="bg-white rounded-lg border border-[#e5e6eb] overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-4 text-xs font-medium text-muted-foreground">用户名</th>
                  <th className="text-left py-3 px-4 text-xs font-medium text-muted-foreground">角色</th>
                  <th className="text-center py-3 px-4 text-xs font-medium text-muted-foreground">绑定账号</th>
                  <th className="text-center py-3 px-4 text-xs font-medium text-muted-foreground">任务数</th>
                  <th className="text-left py-3 px-4 text-xs font-medium text-muted-foreground">创建时间</th>
                  <th className="text-right py-3 px-4 text-xs font-medium text-muted-foreground">操作</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => {
                  const isSelf = u.uid === currentUser?.uid
                  const canDelete = u.accountCount === 0 && !isSelf
                  return (
                    <tr key={u.uid} className="border-b border-border last:border-b-0 hover:bg-muted/50 transition-colors">
                      <td className="py-3 px-4">
                        <span className="text-sm font-medium text-foreground">{u.username}</span>
                        {isSelf && <span className="text-xs text-muted-foreground ml-2">(我)</span>}
                      </td>
                      <td className="py-3 px-4">
                        <span className={`text-xs rounded font-medium ${
                          u.role === "admin"
                            ? "text-primary"
                            : "text-[#3b82f6]"
                        }`}>
                          {u.role === "admin" ? "管理员" : "用户"}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-center">
                        <span className="text-sm text-foreground">{u.accountCount}</span>
                      </td>
                      <td className="py-3 px-4 text-center">
                        <span className="text-sm text-foreground">{u.taskCount}</span>
                      </td>
                      <td className="py-3 px-4">
                        <span className="text-sm text-muted-foreground">{formatDate(u.createdAt)}</span>
                      </td>
                      <td className="py-3 px-4">
                        <div className="flex items-center justify-end gap-1.5">
                          <Button
                            variant="ghost" size="sm"
                            onClick={() => setShowEdit(u)}
                            className="px-0 w-7 text-muted-foreground hover:text-foreground"
                          >
                            <Edit3 size={14} />
                          </Button>
                          <Button
                            variant="ghost" size="sm"
                            onClick={() => setDeleteTarget(u)}
                            disabled={!canDelete}
                            className={canDelete ? "px-0 w-7 text-destructive hover:text-destructive hover:bg-destructive/8" : "px-0 w-7 opacity-30"}
                            title={!canDelete ? (isSelf ? "不能删除自己" : "请先解绑账号") : "删除"}
                          >
                            <Trash2 size={14} />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {total > pageSize && (
        <div className="flex items-center justify-between mt-4 text-sm">
          <span className="text-[#86909c]">共 {total} 条</span>
          <div className="flex items-center gap-2">
            <Button
              variant="outline" size="sm"
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              上一页
            </Button>
            <span className="text-[#86909c] px-2">
              {page} / {totalPages}
            </span>
            <Button
              variant="outline" size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
            >
              下一页
            </Button>
          </div>
        </div>
      )}

      <CreateUserModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={() => {
          setShowCreate(false)
          setPage(1)
          loadUsers()
          setMessage({ type: "success", text: "用户创建成功" })
        }}
      />

      {showEdit && (
        <EditUserModal
          user={showEdit}
          open={!!showEdit}
          onClose={() => setShowEdit(null)}
          onUpdated={() => {
            setShowEdit(null)
            loadUsers()
            setMessage({ type: "success", text: "用户信息已更新" })
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
          setPage(newPage)
          loadUsers()
          setMessage({ type: "success", text: "用户已删除" })
        }}
      />
    </div>
  )
}

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
        <DialogHeader>
          <DialogTitle>新增用户</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <Label>用户名</Label>
            <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="请输入用户名" />
          </div>
          <div>
            <Label>密码</Label>
            <div className="relative">
              <Input
                type={showPwd ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="至少 8 位"
              />
              <button type="button" onClick={() => setShowPwd(!showPwd)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
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
        <DialogHeader>
          <DialogTitle>编辑用户 - {user.username}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleUpdate} className="space-y-4">
          <div>
            <Label>新密码（留空则不修改）</Label>
            <div className="relative">
              <Input
                type={showPwd ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="至少 8 位，留空则不修改"
              />
              <button type="button" onClick={() => setShowPwd(!showPwd)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
                {showPwd ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </div>
          </div>
          <div>
            <Label>角色 {isSelf && <span className="text-xs text-primary ml-1">（不能修改自己的角色）</span>}</Label>
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

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>确认删除</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">
          确定要删除用户 <span className="text-foreground font-medium">{user?.username}</span> 吗？此操作不可撤销。
        </p>
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
