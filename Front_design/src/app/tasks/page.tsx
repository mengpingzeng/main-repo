"use client"

import { useEffect, useState, useCallback, useRef } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { TaskSummary } from "@/types"
import { fetchTasks } from "@/lib/api"
import { formatDate } from "@/lib/utils"
import { FileText, Plus, Search, Loader2, AlertCircle } from "lucide-react"
import Link from "next/link"

export default function TaskListPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [tasks, setTasks] = useState<TaskSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const pageSize = 14
  const fetchedRef = useRef(false)

  const page = Number(searchParams.get("page")) || 1
  const search = searchParams.get("q") || ""
  const [searchInput, setSearchInput] = useState(search)

  const updateURL = (p: number, q: string) => {
    const params = new URLSearchParams()
    if (p > 1) params.set("page", String(p))
    if (q) params.set("q", q)
    const qs = params.toString()
    router.replace(qs ? `?${qs}` : "/tasks", { scroll: false })
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  const loadTasks = useCallback(async () => {
    if (fetchedRef.current) return
    fetchedRef.current = true
    setLoading(true)
    setError("")
    try {
      const list = await fetchTasks()
      setTasks(list)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadTasks() }, [loadTasks])

  useEffect(() => {
    history.scrollRestoration = "manual"
  }, [])

  useEffect(() => {
    if (!loading && tasks.length > 0) {
      const savedY = sessionStorage.getItem("taskList_scrollY")
      if (savedY) {
        window.scrollTo({ top: Number(savedY) })
        sessionStorage.removeItem("taskList_scrollY")
      }
    }
  }, [loading, tasks])

  const filtered = tasks.filter((t) =>
    !search ||
    t.topic.toLowerCase().includes(search.toLowerCase()) ||
    (t.novel_name && t.novel_name.toLowerCase().includes(search.toLowerCase())) ||
    t.task_id.includes(search)
  )

  const total = filtered.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const pagedTasks = filtered.slice((page - 1) * pageSize, page * pageSize)

  const platformLabel = (p: string) => ({ xhs: "小红书", wechat: "公众号", fanqie: "番茄小说", yuewen: "阅文", zhulang: "逐浪网" }[p] || p)

  const skillLabel = (s: string) => ({
    xhs_grass_v1: "小红书种草", wechat_deep_v1: "公众号深度长文",
    wechat_short_v1: "公众号短图文", yuewen_chapter_v1: "阅文章节", fanqie_short_v1: "番茄短篇",
    general_fallback_v1: "通用兜底", novel_continuation_ai: "小说续写",
    "my-novel-writer": "小说写手",
  }[s] || s)

  const modelLabel = (m: string) => {
    const name = m.replace(/^(team-deepseek|deepseek)\//, "")
    return ({ "deepseek-chat": "DeepSeek Chat", "deepseek-reasoner": "DeepSeek Reasoner", "hy3-preview": "混元 3" }[name] || name)
  }

  return (
    <div className="w-full max-w-[1080px]">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-lg font-semibold text-foreground">
          任务列表
        </h1>
        <Link href="/tasks/new">
          <Button size="lg"><Plus size={16} /> 新建任务</Button>
        </Link>
      </div>

      <div className="mb-6">
        <div className="relative">
          <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") updateURL(1, searchInput) }}
            placeholder="搜索主题或任务ID..."
            className="pl-9"
          />
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-6 h-6 animate-spin text-primary" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-20">
          <AlertCircle className="w-12 h-12 mb-4 text-destructive opacity-40" />
          <p className="text-sm text-muted-foreground mb-1">{error}</p>
          <Button variant="ghost" size="sm" onClick={loadTasks}>重试</Button>
        </div>
      ) : tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20">
          <div className="w-40 h-32 mb-5 rounded-lg border border-[#e5e6eb] bg-[#f7f8fa] flex items-center justify-center">
            <FileText className="w-12 h-12 text-[#c9cdd4]" />
          </div>
          <p className="text-sm text-muted-foreground mb-5">还没有创作任务</p>
          <Link href="/tasks/new">
            <Button size="lg"><Plus size={16} /> 新建任务</Button>
          </Link>
        </div>
      ) : (
        <>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {pagedTasks.map((task) => {
            const hasMeta = task.platform || task.skill_id || task.model || task.draft_version > 0
            return (
              <Link
                key={task.task_id}
                href={`/tasks/${task.task_id}${task.active_session_id ? `?sid=${task.active_session_id}` : ""}`}
                className="h-full"
                onClick={() => sessionStorage.setItem("taskList_scrollY", String(window.scrollY))}
              >
                <div className="bg-white rounded-lg border border-[#e5e6eb] hover:border-[#b0adff] hover:bg-[#f8f7ff] transition-colors cursor-pointer p-5 h-full flex flex-col">
                    <h3 className="text-[15px] font-semibold text-foreground leading-snug line-clamp-2">
                      {task.novel_name || task.topic}
                    </h3>

                    <div className="mt-auto pt-4">
                      {hasMeta && (
                        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[13px]">
                          {task.platform && <span><span className="text-[#86909c]">发布平台</span><span className="text-[#1d2129] ml-1">{platformLabel(task.platform)}</span></span>}
                          {task.skill_id && <span><span className="text-[#86909c]">写作风格</span><span className="text-[#1d2129] ml-1">{skillLabel(task.skill_id)}</span></span>}
                          {task.model && <span><span className="text-[#86909c]">AI 模型</span><span className="text-[#1d2129] ml-1">{modelLabel(task.model)}</span></span>}
                          {task.draft_version > 0 && <span className="text-[#4e5969]">草稿 v{task.draft_version}</span>}
                        </div>
                      )}

                      <div className="text-xs mt-2.5">
                        <span className="text-[#86909c]">创建时间</span><span className="text-[#4e5969] ml-1">{formatDate(task.created_at)}</span>
                      </div>
                    </div>
                  </div>
              </Link>
          )})}
        </div>
        {total > pageSize && (
          <div className="flex items-center justify-between mt-4 text-sm">
            <span className="text-[#86909c]">共 {total} 条</span>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => updateURL(page - 1, search)}>上一页</Button>
              <span className="text-[#86909c] px-2">{page} / {totalPages}</span>
              <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => updateURL(page + 1, search)}>下一页</Button>
            </div>
          </div>
        )}
        </>
      )}
    </div>
  )
}
