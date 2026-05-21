"use client"

import { useEffect, useState, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { fetchDashboard } from "@/lib/api"
import type { DashboardQueryResponse } from "@/types"
import { formatNumber } from "@/lib/utils"
import { Eye, Heart, MessageCircle, Share2, Loader2, FileText, RefreshCw, AlertCircle } from "lucide-react"

export default function DashboardPage() {
  const [data, setData] = useState<DashboardQueryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [platform, setPlatform] = useState("")
  const [timeRange, setTimeRange] = useState("7d")
  const [page, setPageState] = useState(1)

  const setPage = (p: number) => {
    setPageState(p)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }
  const pageSize = 5

  const loadDashboard = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const req: { platforms?: string[]; startTime?: string; endTime?: string } = {}
      if (platform) req.platforms = [platform]
      if (timeRange) {
        const now = new Date()
        const start = new Date(now)
        if (timeRange === "24h") start.setHours(start.getHours() - 24)
        else if (timeRange === "7d") start.setDate(start.getDate() - 7)
        else start.setDate(start.getDate() - 30)
        req.startTime = start.toISOString()
        req.endTime = now.toISOString()
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

  const allItems = data?.items || []
  const total = allItems.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const pagedItems = allItems.slice((page - 1) * pageSize, page * pageSize)

  const summary = data?.summary
  const stats = [
    { label: "发布帖子总数", value: summary?.totalPosts ?? null, color: "#4d9fff" },
    { label: "总阅读数", value: summary?.totalViews ?? null, color: "#f5a623" },
    { label: "点赞总数", value: summary?.totalLikes ?? null, color: "#f54b4b" },
  ]

  const platformLabel = (p: string) => ({ xhs: "小红书", wechat: "公众号", fanqie: "番茄小说", yuewen: "阅文", zhulang: "逐浪网" }[p] || p)

  return (
    <div>
      {/* Title */}
      <div className="mb-5">
        <h1 className="text-lg font-semibold text-foreground">
          数据看板
        </h1>
      </div>

      <div className="grid grid-cols-3 gap-4 mb-6">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className="bg-[#eff4ff] rounded-xl p-5 flex items-center justify-between min-h-[110px]"
          >
            <div>
              <p className="text-sm text-[#86909c] mb-2">{stat.label}</p>
              <p className="text-[32px] font-semibold text-foreground leading-tight">
                {stat.value !== null && stat.value !== undefined ? formatNumber(stat.value) : "-"}
              </p>
              <p className="text-xs text-[#c9cdd4] mt-1">同比昨日 -</p>
            </div>
            <div className="w-[60px] h-[60px] rounded-xl flex items-center justify-center opacity-70"
              style={{ background: `${stat.color}15` }}>
              <FileText size={32} style={{ color: stat.color }} />
            </div>
          </div>
        ))}
      </div>

      {/* Filters bar */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <SelectRadix value={platform || "all"} onValueChange={(v) => setPlatform(v === "all" ? "" : v)} className="w-[160px]">
            <SelectItem value="all">全部账号</SelectItem>
            <SelectItem value="fanqie">番茄小说</SelectItem>
            <SelectItem value="xhs">小红书</SelectItem>
            <SelectItem value="wechat">公众号</SelectItem>
            <SelectItem value="yuewen">阅文</SelectItem>
            <SelectItem value="zhulang">逐浪网</SelectItem>
          </SelectRadix>
          <SelectRadix value={timeRange} onValueChange={setTimeRange} className="w-28">
            <SelectItem value="24h">昨天</SelectItem>
            <SelectItem value="7d">近7天</SelectItem>
            <SelectItem value="30d">近30天</SelectItem>
          </SelectRadix>
          <Button variant="ghost" size="sm" onClick={loadDashboard}>
            <RefreshCw size={14} className={loading ? "animate-spin" : ""} />
          </Button>
        </div>

      </div>

      {/* Content */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-6 h-6 animate-spin text-primary" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-20">
          <AlertCircle className="w-12 h-12 mb-4 text-destructive opacity-40" />
          <p className="text-sm text-muted-foreground mb-1">{error}</p>
          <Button variant="ghost" size="sm" onClick={loadDashboard}>重试</Button>
        </div>
      ) : !data?.items?.length ? (
          <div className="flex flex-col items-center justify-center py-20">
            <div className="w-32 h-32 mb-5 rounded-lg border border-[#e5e6eb] bg-[#f7f8fa] flex items-center justify-center">
              <FileText className="w-12 h-12 text-[#c9cdd4]" />
          </div>
          <p className="text-sm text-[#86909c] mb-5">暂无发布数据，完成一次发布后即可在此查看</p>
          <Button size="lg" onClick={() => window.location.href = "/tasks"}>去发布</Button>
        </div>
      ) : (
        <>
        <div className="bg-white rounded-lg border border-[#e5e6eb] overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-5 text-xs text-muted-foreground font-medium">账号名称</th>
                  <th className="text-left py-3 px-5 text-xs text-muted-foreground font-medium">小说名称</th>
                  <th className="text-left py-3 px-5 text-xs text-muted-foreground font-medium">平台</th>
                  <th className="text-right py-3 px-5 text-xs text-muted-foreground font-medium">阅读</th>
                  <th className="text-right py-3 px-5 text-xs text-muted-foreground font-medium">点赞</th>
                  <th className="text-right py-3 px-5 text-xs text-muted-foreground font-medium">评论</th>
                  <th className="text-right py-3 px-5 text-xs text-muted-foreground font-medium">分享</th>
                </tr>
              </thead>
              <tbody>
                {pagedItems.map((item, i) => (
                  <tr key={item.postId || i} className="border-b border-border/50 hover:bg-muted/50 transition-colors">
                    <td className="py-3 px-5 text-foreground">{item.loginName}</td>
                    <td className="py-3 px-5 text-foreground">{item.novelName}</td>
                    <td className="py-3 px-5 text-foreground">{platformLabel(item.platform)}</td>
                    <td className="py-3 px-5 text-right text-foreground">{formatNumber(item.views)}</td>
                    <td className="py-3 px-5 text-right text-foreground">{formatNumber(item.likes)}</td>
                    <td className="py-3 px-5 text-right text-foreground">{formatNumber(item.comments)}</td>
                    <td className="py-3 px-5 text-right text-foreground">{formatNumber(item.shares)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        {total > pageSize && (
          <div className="flex items-center justify-between mt-4 text-sm">
            <span className="text-[#86909c]">共 {total} 条</span>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>上一页</Button>
              <span className="text-[#86909c] px-2">{page} / {totalPages}</span>
              <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>下一页</Button>
            </div>
          </div>
        )}
        </>
      )}
    </div>
  )
}
