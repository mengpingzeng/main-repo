"use client"

import { useEffect, useState, useCallback } from "react"
import { fetchDashboard } from "@/lib/api"
import type { DashboardQueryResponse } from "@/types"
import { formatNumber } from "@/lib/utils"
import { Loader2, AlertCircle } from "lucide-react"

const PLATFORM_LABEL: Record<string, string> = {
  fanqie: "番茄小说", zhulang: "逐浪网", xhs: "小红书", wechat: "公众号", yuewen: "阅文",
}
const platformLabel = (p: string) => PLATFORM_LABEL[p] || p

const TIME_TABS = [
  { value: "7d",  label: "近 7 天" },
  { value: "30d", label: "近 30 天" },
  { value: "all", label: "全部时间" },
]

const PLATFORM_TABS = [
  { value: "", label: "全部平台" },
  { value: "fanqie",  label: "番茄小说" },
  { value: "zhulang", label: "逐浪网" },
]

export default function DashboardPage() {
  const [data, setData] = useState<DashboardQueryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [platform, setPlatform] = useState("")
  const [timeRange, setTimeRange] = useState("30d")
  const [page, setPageState] = useState(1)

  const setPage = (p: number) => {
    setPageState(p)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }
  const pageSize = 10

  const loadDashboard = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const req: { platforms?: string[]; startTime?: string; endTime?: string } = {}
      if (platform) req.platforms = [platform]
      if (timeRange && timeRange !== "all") {
        const now = new Date()
        const start = new Date(now)
        if (timeRange === "7d") start.setDate(start.getDate() - 7)
        else start.setDate(start.getDate() - 30)
        req.startTime = start.toISOString()
        req.endTime   = now.toISOString()
      }
      const d = await fetchDashboard(req)
      setData(d)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [platform, timeRange])

  useEffect(() => { loadDashboard() }, [loadDashboard])
  useEffect(() => { setPage(1) }, [platform, timeRange])

  const allItems  = data?.items || []
  const total     = allItems.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const pagedItems = allItems.slice((page - 1) * pageSize, page * pageSize)
  const summary   = data?.summary

  const statCards = [
    { label: "总阅读量 / 曝光", value: summary?.totalViews    ?? null },
    { label: "总点赞 / 收藏",   value: summary?.totalLikes    ?? null },
    { label: "互动评论数",       value: summary?.totalComments ?? null },
    { label: "已发布篇数",       value: summary?.totalPosts    ?? null },
  ]

  return (
    <div className="max-w-7xl mx-auto px-6 pt-6">
      <header className="flex justify-between items-end mb-8">
        <div>
          <h1 className="text-3xl font-bold text-slate-900 tracking-tight">内容数据概览</h1>
          <p className="text-slate-500 mt-1">跨平台数据自动汇总分析 (30分钟刷新)</p>
        </div>

        {/* 时间 Tab — 与 HTML 原型完全一致 */}
        <div className="bg-white border border-slate-200 rounded-lg p-1 flex shadow-sm">
          {TIME_TABS.map(({ value, label }) => (
            <button
              key={value}
              onClick={() => setTimeRange(value)}
              className={`px-3 py-1 text-sm font-medium rounded transition-colors ${
                timeRange === value
                  ? "bg-slate-100 text-slate-800"
                  : "text-slate-500 hover:text-slate-800 hover:bg-slate-50"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </header>

      {/* ── 统计卡片 — 无图标，与 HTML 原型完全一致 ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {statCards.map((s) => (
          <div key={s.label} className="bg-white p-6 rounded-2xl border border-slate-200 shadow-sm relative overflow-hidden">
            <p className="text-sm font-medium text-slate-500 mb-1">{s.label}</p>
            <h3 className="text-3xl font-bold text-slate-900">
              {s.value !== null && s.value !== undefined ? formatNumber(s.value) : "—"}
            </h3>
          </div>
        ))}
      </div>

      {/* ── 平台筛选 ── */}
      <div className="flex items-center gap-1 mb-4 bg-slate-100 rounded-lg p-1 w-fit">
        {PLATFORM_TABS.map(({ value, label }) => (
          <button
            key={value}
            onClick={() => setPlatform(value)}
            className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all ${
              platform === value
                ? "bg-white text-slate-900 shadow-sm"
                : "text-slate-500 hover:text-slate-700"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* ── 内容区 ── */}
      {loading ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="w-6 h-6 animate-spin text-orange-500" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-24">
          <AlertCircle className="w-12 h-12 mb-4 text-slate-300" />
          <p className="text-sm text-slate-500 mb-3">{error}</p>
          <button
            onClick={loadDashboard}
            className="px-4 py-2 text-sm font-medium border border-slate-200 bg-white text-slate-600 rounded-lg hover:bg-slate-50 transition-colors"
          >
            重试
          </button>
        </div>
      ) : !allItems.length ? (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div className="w-20 h-20 rounded-2xl bg-slate-100 flex items-center justify-center mb-5">
            <svg width="36" height="36" viewBox="0 0 24 24" fill="none" className="text-slate-300">
              <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" strokeWidth="1.5"/>
              <path d="M3 9h18" stroke="currentColor" strokeWidth="1.5"/>
              <path d="M9 9v12" stroke="currentColor" strokeWidth="1.5"/>
              <path d="M8 13h3M8 16h5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
            </svg>
          </div>
          <p className="text-base font-medium text-slate-400">暂无数据</p>
          <p className="text-sm text-slate-300 mt-1">当前筛选条件下没有数据记录</p>
        </div>
      ) : (
        <>
          {/* ── 数据表格 — 与 HTML 原型对齐 ── */}
          <div className="bg-white border border-slate-200 rounded-2xl shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-200 flex items-center bg-slate-50/50">
              <h3 className="text-lg font-semibold text-slate-800">各平台数据明细</h3>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm table-fixed">
                <colgroup>
                  <col className="w-[28%]" />
                  <col className="w-[15%]" />
                  <col className="w-[12%]" />
                  <col className="w-[12%]" />
                  <col className="w-[10%]" />
                  <col className="w-[10%]" />
                  <col className="w-[10%]" />
                </colgroup>
                <thead className="bg-slate-50 text-slate-500 border-b border-slate-200">
                  <tr>
                    <th className="px-6 py-4 font-medium">作品名</th>
                    <th className="px-6 py-4 font-medium">账号名称</th>
                    <th className="px-6 py-4 font-medium">发布平台</th>
                    <th className="px-6 py-4 font-medium text-center">阅读 / 曝光</th>
                    <th className="px-6 py-4 font-medium text-center">点赞</th>
                    <th className="px-6 py-4 font-medium text-center">评论</th>
                    <th className="px-6 py-4 font-medium text-center">分享</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 text-slate-700">
                  {pagedItems.map((item, i) => (
                    <tr key={item.postId || i} className="hover:bg-slate-50 transition-colors">
                      <td className="px-6 py-4 font-medium text-slate-900">
                        <div className="line-clamp-2 whitespace-normal leading-snug">{item.novelName}</div>
                      </td>
                      <td className="px-6 py-4 text-slate-600 truncate">{item.loginName}</td>
                      <td className="px-6 py-4">{platformLabel(item.platform)}</td>
                      <td className="px-6 py-4 tabular-nums text-center">{formatNumber(item.views)}</td>
                      <td className="px-6 py-4 tabular-nums text-center">{formatNumber(item.likes)}</td>
                      <td className="px-6 py-4 tabular-nums text-center">{formatNumber(item.comments)}</td>
                      <td className="px-6 py-4 tabular-nums text-center">{formatNumber(item.shares)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* ── 分页 ── */}
          {total > pageSize && (
            <div className="flex items-center justify-between mt-6 text-sm text-slate-500">
              <span>共 {total} 条</span>
              <div className="flex items-center gap-2">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                  className="px-3 py-1.5 border border-slate-200 bg-white text-slate-600 rounded-lg font-medium hover:bg-slate-50 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  上一页
                </button>
                <span className="px-2 text-slate-400">{page} / {totalPages}</span>
                <button
                  disabled={page >= totalPages}
                  onClick={() => setPage(page + 1)}
                  className="px-3 py-1.5 border border-slate-200 bg-white text-slate-600 rounded-lg font-medium hover:bg-slate-50 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  下一页
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
