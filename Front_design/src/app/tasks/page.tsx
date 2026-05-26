"use client"

import { useEffect, useState, useCallback } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import type { TaskSummary, AccountSummary } from "@/types"
import { fetchTasks, fetchAccounts, fetchModels } from "@/lib/api"
import { formatModelName, buildModelNameMap } from "@/lib/model-label"
import { formatRelativeTime } from "@/lib/utils"
import { buildTaskDetailHref } from "@/lib/task-navigation"
import { FileText, Plus, Search, Loader2, AlertCircle, Layers } from "lucide-react"
import Link from "next/link"
import { cn } from "@/lib/utils"

const PLATFORM_LABELS: Record<string, { label: string; color: string }> = {
  fanqie:  { label: "番茄小说", color: "text-red-600 bg-red-50 border-red-100" },
  zhulang: { label: "逐浪网",   color: "text-blue-600 bg-blue-50 border-blue-100" },
  xhs:     { label: "小红书",   color: "text-rose-600 bg-rose-50 border-rose-100" },
  wechat:  { label: "公众号",   color: "text-green-600 bg-green-50 border-green-100" },
  yuewen:  { label: "阅文",     color: "text-purple-600 bg-purple-50 border-purple-100" },
}

function platformBadge(p: string) {
  const conf = PLATFORM_LABELS[p] ?? { label: p, color: "text-slate-600 bg-slate-100 border-slate-200" }
  return (
    <span className={cn("px-2.5 py-1 text-[10px] font-bold tracking-wide border rounded-md", conf.color)}>
      {conf.label}
    </span>
  )
}

const skillLabel = (s: string) => ({
  xhs_grass_v1: "小红书种草", wechat_deep_v1: "公众号深度长文",
  wechat_short_v1: "公众号短图文", yuewen_chapter_v1: "阅文章节",
  fanqie_short_v1: "番茄短篇", general_fallback_v1: "通用兜底",
  novel_continuation_ai: "小说续写", "my-novel-writer": "小说写手",
}[s] || s)

export default function TaskListPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [tasks, setTasks] = useState<TaskSummary[]>([])
  const [total, setTotal] = useState(0)
  const [accountMap, setAccountMap] = useState<Record<string, AccountSummary>>({})
  const [modelNameMap, setModelNameMap] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const pageSize = 12

  const page = Number(searchParams.get("page")) || 1
  const search = searchParams.get("q") || ""
  const [searchInput, setSearchInput] = useState(search)
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const updateURL = (p: number, q: string) => {
    const params = new URLSearchParams()
    if (p > 1) params.set("page", String(p))
    if (q) params.set("q", q)
    const qs = params.toString()
    router.replace(qs ? `?${qs}` : "/tasks", { scroll: false })
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  const loadTasks = useCallback(async (targetPage = page, q = search) => {
    setLoading(true)
    setError("")
    try {
      const [result, accounts, models] = await Promise.all([
        fetchTasks(targetPage, pageSize, q),
        fetchAccounts(),
        fetchModels(),
      ])
      setTasks(result.tasks)
      setTotal(result.total)
      setModelNameMap(buildModelNameMap(models))
      const map: Record<string, AccountSummary> = {}
      accounts.forEach(a => { map[a.account_id] = a })
      setAccountMap(map)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, search])

  useEffect(() => { loadTasks(page, search) }, [page, search, loadTasks])
  useEffect(() => { setSearchInput(search) }, [search])
  useEffect(() => { history.scrollRestoration = "manual" }, [])

  return (
    <div className="max-w-7xl mx-auto px-6 pt-8">
      <div className="flex items-end justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-slate-900 tracking-tight">任务列表</h1>
          <p className="text-slate-500 mt-1 text-sm">管理您所有的内容创作与发布任务</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="relative flex items-center gap-2">
            <div className="relative">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <Input
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
                onKeyDown={e => { if (e.key === "Enter") updateURL(1, searchInput.trim()) }}
                placeholder="搜索小说名..."
                className="pl-9 w-60 h-9 text-sm"
              />
            </div>
            <Button variant="outline" size="sm" className="h-9" onClick={() => updateURL(1, searchInput.trim())}>
              搜索
            </Button>
          </div>
          <Link href="/tasks/new">
            <button className="px-4 py-2 bg-slate-900 text-white text-sm font-medium rounded-lg hover:bg-slate-800 shadow-sm flex items-center gap-1.5 transition-colors">
              <Plus size={15} />新建创作
            </button>
          </Link>
        </div>
      </div>

      {/* 内容区 */}
      {loading ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="w-6 h-6 animate-spin text-orange-500" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-24 gap-3">
          <AlertCircle className="w-12 h-12 text-red-300" />
          <p className="text-sm text-slate-500">{error}</p>
          <button onClick={() => loadTasks(page, search)} className="text-sm text-orange-500 hover:underline">重试</button>
        </div>
      ) : total === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 gap-4">
          <div className="w-16 h-16 rounded-2xl bg-slate-100 flex items-center justify-center">
            <FileText className="w-8 h-8 text-slate-300" />
          </div>
          <p className="text-sm text-slate-500">{search ? "没有匹配的任务" : "还没有创作任务"}</p>
          {!search && (
            <Link href="/tasks/new">
              <button className="px-5 py-2.5 bg-slate-900 text-white text-sm font-medium rounded-lg hover:bg-slate-800 flex items-center gap-1.5 transition-colors shadow-sm">
                <Plus size={15} />新建创作
              </button>
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
            {tasks.map(task => (
              <Link
                key={task.task_id}
                href={buildTaskDetailHref(task.task_id, {
                  sid: task.active_session_id,
                  from: "list",
                  listPage: page,
                  listQ: search,
                })}
                className="flex flex-col"
              >
                <div className="bg-white rounded-2xl border border-slate-200 shadow-sm hover:shadow-md transition-shadow cursor-pointer p-5 flex-1 flex flex-col">

                  {/* 平台徽标 + 状态 */}
                  <div className="flex items-start justify-between gap-2 mb-4">
                    <div className="flex gap-2">
                      {task.platform ? platformBadge(task.platform) : (
                        <span className="px-2.5 py-1 text-[10px] font-bold tracking-wide text-slate-500 bg-slate-100 border border-slate-200 rounded-md">未设平台</span>
                      )}
                    </div>
                    <span className="flex items-center text-xs font-medium text-orange-600 bg-orange-50 px-2 py-1 rounded-md flex-shrink-0">
                      <span className="w-1.5 h-1.5 rounded-full bg-orange-500 mr-1.5 animate-pulse" />AI 生成中
                    </span>
                  </div>

                  {/* 标题 */}
                  <h3 className="text-[17px] font-bold text-slate-900 leading-snug line-clamp-1 mb-2">
                    {task.novel_name || task.topic}
                  </h3>

                  {/* 描述 */}
                  <p className="text-sm text-slate-500 line-clamp-1 mb-4 leading-relaxed">
                    {task.volume_name && (task.chapter_number ?? 0) > 0 && task.title
                      ? `${task.volume_name}第${task.chapter_number}章：${task.title}`
                      : task.topic}
                  </p>

                  {/* 模型 & 技能标签 */}
                  <div className="flex flex-wrap gap-1.5 mb-4">
                    {task.model && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 text-slate-600 border border-slate-200">
                        模型: {formatModelName(task.model, modelNameMap)}
                      </span>
                    )}
                    {task.skill_id && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 text-slate-600 border border-slate-200">
                        Skill: {skillLabel(task.skill_id)}
                      </span>
                    )}
                    {task.draft_version > 0 && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium bg-orange-50 text-orange-600 border border-orange-100">
                        v{task.draft_version}
                      </span>
                    )}
                  </div>

                  {/* 底部 */}
                  <div className="mt-auto pt-4 border-t border-slate-100">
                    <div className="flex items-center justify-between mb-1">
                      {task.published_chapter_count != null && task.published_chapter_count > 0 ? (
                        <span className="text-xs font-medium text-slate-500 flex items-center gap-1">
                          <Layers size={13} className="text-slate-400" />
                          已发布: {task.published_chapter_count} 章
                        </span>
                      ) : (
                        <span className="text-xs text-slate-400">暂无发布章节</span>
                      )}
                      {task.account_id && accountMap[task.account_id] && (
                        <span className="text-[11px] font-medium text-orange-500 truncate max-w-[130px]">
                          {accountMap[task.account_id].masked_display}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center justify-between text-[11px] text-slate-400 mt-1">
                      <span>更新于 {formatRelativeTime(task.last_active_at || task.created_at)}</span>
                      <span>创建于 {formatRelativeTime(task.created_at)}</span>
                    </div>
                  </div>
                </div>
              </Link>
            ))}
          </div>

          {/* 分页 */}
          {total > pageSize && (
            <div className="flex items-center justify-between mt-8 text-sm">
              <span className="text-slate-500">共 {total} 条</span>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => updateURL(page - 1, search)}>上一页</Button>
                <span className="text-slate-500 px-2">{page} / {totalPages}</span>
                <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => updateURL(page + 1, search)}>下一页</Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
