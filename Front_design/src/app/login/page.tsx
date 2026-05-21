"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { login } from "@/lib/auth"
import { Eye, EyeOff, Loader2 } from "lucide-react"

export default function LoginPage() {
  const router = useRouter()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (!username.trim()) {
      setError("请输入用户名")
      return
    }
    if (password.length < 8) {
      setError("密码至少需要 8 位")
      return
    }

    setLoading(true)
    try {
      await login(username.trim(), password)
      router.replace("/tasks/new")
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#f7f8fa]">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <img src="/logo.png" alt="Logo" className="w-16 h-16 rounded-xl mx-auto mb-4" />
          <h1 className="text-xl font-semibold text-foreground">小龙虾创作平台</h1>
          <p className="text-sm text-muted-foreground mt-1">AI 驱动的多平台内容创作</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-white border border-border rounded-xl p-6 shadow-[0_2px_8px_rgba(0,0,0,0.08)] space-y-4">
          <div>
            <label htmlFor="username" className="block text-sm text-foreground mb-1.5">
              用户名
            </label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入用户名"
              autoFocus
              className="w-full h-9 px-3 bg-white border border-input rounded-lg text-foreground placeholder:text-[#86909c] focus:outline-none focus:border-primary focus:ring-2 focus:ring-[rgba(99,91,255,0.15)] transition-colors text-sm"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm text-foreground mb-1.5">
              密码
            </label>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="请输入密码"
                className="w-full h-9 px-3 pr-10 bg-white border border-input rounded-lg text-foreground placeholder:text-[#86909c] focus:outline-none focus:border-primary focus:ring-2 focus:ring-[rgba(99,91,255,0.15)] transition-colors text-sm"
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              >
                {showPassword ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </div>
          </div>

          {error && (
            <div className="bg-destructive/8 border border-destructive/20 rounded-lg px-3 py-2 text-destructive text-xs">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full h-10 bg-primary hover:bg-primary-hover active:bg-primary-active disabled:opacity-50 text-white font-medium rounded-lg transition-colors text-sm flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 size={14} className="animate-spin" />
                登录中...
              </>
            ) : (
              "登录"
            )}
          </button>
        </form>
      </div>
    </div>
  )
}
