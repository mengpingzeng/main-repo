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
      router.replace("/tasks")
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 relative overflow-hidden">
      {/* 全局装饰光斑 */}
      <div className="absolute top-[-80px] left-[-80px] w-[500px] h-[500px] rounded-full bg-orange-500/15 blur-[120px] pointer-events-none" />
      <div className="absolute bottom-[-60px] right-[30%] w-[400px] h-[400px] rounded-full bg-red-500/10 blur-[120px] pointer-events-none" />
      <div className="absolute top-1/2 left-1/3 -translate-y-1/2 w-[600px] h-[600px] rounded-full bg-orange-600/6 blur-[140px] pointer-events-none" />

      {/* ── 左侧品牌区 ── */}
      <div className="hidden lg:flex lg:flex-1 relative flex-col p-12">
        {/* Logo */}
        <div className="relative z-10 flex items-center gap-4">
          <svg width="44" height="44" viewBox="0 0 40 40" fill="none">
            <path d="M20 4C14 4 10 9 10 15C10 19 13 22.5 17 24C15 27 12 28 12 28C14.5 30 18 31 20 31C28 31 32 24 32 15C32 9 26 4 20 4Z" fill="#F97316"/>
          </svg>
          <span className="text-white font-bold text-2xl tracking-tight">铸文坊</span>
        </div>

        {/* 主文案 — 垂直居中（稍微上移修正视觉偏差） */}
        <div className="relative z-10 flex-1 flex items-center -mt-16">
          <div>
            <h1 className="text-4xl font-bold text-white leading-tight mb-4 whitespace-nowrap">
              智叙千篇，<span className="bg-gradient-to-r from-orange-400 to-red-400 bg-clip-text text-transparent">匠造万象</span>
            </h1>
            <p className="text-slate-400 text-base leading-relaxed mb-8">
              AI 驱动的多平台内容创作与分发工作站，<br />让创作回归创意本身。
            </p>

            {/* 特性列表 */}
            <div className="space-y-3">
              {[
                "多平台一键分发，告别重复操作",
                "AI 全程辅助，从大纲到成稿",
                "实时数据看板，追踪内容表现",
              ].map((text) => (
                <div key={text} className="flex items-center gap-3">
                  <div className="w-5 h-5 rounded-full bg-gradient-to-br from-orange-500 to-red-500 flex items-center justify-center flex-shrink-0">
                    <svg width="10" height="10" viewBox="0 0 12 12" fill="none">
                      <path d="M2 6l3 3 5-5" stroke="white" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  </div>
                  <span className="text-slate-300 text-sm">{text}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* 底部版权 */}
        <div className="relative z-10">
          <p className="text-slate-600 text-xs">© 2026 铸文坊. All rights reserved.</p>
        </div>
      </div>

      {/* ── 右侧登录区（卡片浮于深色背景上） ── */}
      <div className="flex-1 flex items-center justify-center px-6 py-12">
        <div className="w-full max-w-sm bg-white/95 backdrop-blur-sm rounded-2xl shadow-2xl shadow-black/30 p-8">
          {/* 移动端 Logo（仅小屏显示） */}
          <div className="lg:hidden text-center mb-8">
            <div className="flex items-center justify-center gap-2 mb-1">
              <svg width="36" height="36" viewBox="0 0 40 40" fill="none">
                <path d="M20 4C14 4 10 9 10 15C10 19 13 22.5 17 24C15 27 12 28 12 28C14.5 30 18 31 20 31C28 31 32 24 32 15C32 9 26 4 20 4Z" fill="#F97316"/>
              </svg>
              <h1 className="text-xl font-bold text-slate-900">铸文坊</h1>
            </div>
          </div>

          <div className="mb-7">
            <h2 className="text-2xl font-bold text-slate-900">欢迎回来</h2>
            <p className="text-slate-500 text-sm mt-1">登录你的账号，继续你的创作之旅</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-slate-700 mb-1.5">
                用户名
              </label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="请输入用户名"
                autoFocus
                className="w-full h-10 px-3.5 bg-white border border-slate-200 rounded-lg text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-orange-400 focus:ring-2 focus:ring-orange-500/15 transition-colors text-sm shadow-sm"
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-slate-700 mb-1.5">
                密码
              </label>
              <div className="relative">
                <input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="请输入密码"
                  className="w-full h-10 px-3.5 pr-11 bg-white border border-slate-200 rounded-lg text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-orange-400 focus:ring-2 focus:ring-orange-500/15 transition-colors text-sm shadow-sm"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 transition-colors"
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            {error && (
              <div className="bg-red-50 border border-red-200 rounded-lg px-3.5 py-2.5 text-red-600 text-xs flex items-center gap-2">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 3a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 018 4zm0 7a1 1 0 110-2 1 1 0 010 2z"/>
                </svg>
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full h-10 bg-gradient-to-r from-orange-500 to-red-500 hover:from-orange-600 hover:to-red-600 active:from-orange-700 active:to-red-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-all text-sm flex items-center justify-center gap-2 shadow-md shadow-orange-500/20 mt-2"
            >
              {loading ? (
                <>
                  <Loader2 size={15} className="animate-spin" />
                  登录中...
                </>
              ) : (
                "登 录"
              )}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
