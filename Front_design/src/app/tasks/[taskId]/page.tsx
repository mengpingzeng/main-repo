"use client"

import { useEffect, useState, useRef, useCallback } from "react"
import { useParams, useSearchParams, useRouter } from "next/navigation"
import {
  sendMessage, publishTask, getDraft, fetchTasks, fetchAccounts,
  deleteTask, fetchTaskSessions,
} from "@/lib/api"
import { connectSessionWS, connectTaskWS } from "@/lib/ws"
import type { SessionMessage, WSEvent } from "@/types"
import { Send, Loader2, CheckCircle, AlertCircle, ArrowLeft, Trash2, X, Plus } from "lucide-react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

const PLATFORM_LABELS: Record<string, string> = {
  fanqie: "番茄小说", zhulang: "逐浪网", xhs: "小红书", wechat: "公众号", yuewen: "阅文",
}

type ChapterTab = { sessionId: string; label: string; index: number }

export default function SessionPage() {
  const { taskId } = useParams<{ taskId: string }>()
  const searchParams = useSearchParams()
  const router = useRouter()
  const sessionIdFromQuery = searchParams.get("sid") || ""

  const [sessionId, setSessionId] = useState(sessionIdFromQuery)
  const [messages, setMessages] = useState<SessionMessage[]>([])
  const [draft, setDraft] = useState("")
  const [draftVersion, setDraftVersion] = useState(0)
  const [input, setInput] = useState("")
  const [streaming, setStreaming] = useState(false)
  const [streamingText, setStreamingText] = useState("")
  const [status, setStatus] = useState("")
  const [publishState, setPublishState] = useState("")
  const [error, setError] = useState("")
  const [topic, setTopic] = useState("")
  const [platform, setPlatform] = useState("")
  const [novelName, setNovelName] = useState("")
  const [novelNameLocked, setNovelNameLocked] = useState(false)
  const [chapterTitle, setChapterTitle] = useState("")
  const [volumeName, setVolumeName] = useState("第一卷")
  const [chapterNumber, setChapterNumber] = useState(1)
  const [lockedAccount, setLockedAccount] = useState<{ account_id: string; masked_display: string } | null>(null)

  const [chapters, setChapters] = useState<ChapterTab[]>([])
  const [activeChapter, setActiveChapter] = useState<string>("")
  const [chapterDraft, setChapterDraft] = useState("")
  const [chapterDraftLoading, setChapterDraftLoading] = useState(false)

  const [showPublishModal, setShowPublishModal] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const publishWsRef = useRef<WebSocket | null>(null)
  const msgCounterRef = useRef(0)
  const publishStateRef = useRef("")
  const taskResolvedRef = useRef(false)
  const autoSentRef = useRef(false)
  const tabsRef = useRef<HTMLDivElement>(null)

  const nextMsgId = useCallback(() => {
    msgCounterRef.current += 1
    return `${sessionId}:${msgCounterRef.current}`
  }, [sessionId])

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messages, streamingText])

  const startWS = useCallback(async () => {
    if (!sessionId) return
    wsRef.current?.close()
    const ws = connectSessionWS(
      sessionId,
      (event: WSEvent) => {
        switch (event.type) {
          case "token":
            setStreamingText((prev) => prev + (event.text || ""))
            break
          case "draft_updated":
            setDraftVersion(event.draft_version || 0)
            getDraft(sessionId).then((d) => { if (d.draft) setDraft(d.draft) }).catch(() => {})
            break
          case "novel_name":
            if (event.novel_name && !novelNameLocked) setNovelName(event.novel_name)
            break
          case "done":
            setStreamingText((prev) => {
              if (prev) {
                const mid = nextMsgId()
                setMessages((msgs) => [...msgs, {
                  id: mid, role: "assistant", text: prev,
                  timestamp: new Date().toISOString(), draft_version: event.draft_version,
                }])
              }
              return ""
            })
            setStreaming(false)
            setStatus("WARM")
            break
          case "error":
            setError(event.message || "发生错误")
            setStreaming(false)
            break
          case "episode_created":
            if (event.next_session_id) setSessionId(event.next_session_id)
            break
          case "session_interrupted":
            setError("服务暂时中断，请重试")
            setStreaming(false)
            break
        }
      },
      () => setError("WebSocket 连接错误"),
      () => setStatus("ARCHIVED")
    )
    wsRef.current = ws
  }, [sessionId])

  // 加载 sessions 列表作为章节 tab
  const loadChapters = useCallback(async (tid: string) => {
    try {
      const resp = await fetchTaskSessions(tid)
      const sorted = [...(resp.sessions || [])].sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      )
      const tabs: ChapterTab[] = sorted.map((s, i) => ({
        sessionId: s.session_id,
        label: `第 ${i + 1} 章`,
        index: i + 1,
      }))
      setChapters(tabs)
      return tabs
    } catch { return [] }
  }, [])

  useEffect(() => {
    if (!taskId || taskResolvedRef.current) return
    taskResolvedRef.current = true
    const resolve = async () => {
      try {
        const tasks = await fetchTasks()
        const found = tasks.find((t) => t.task_id === taskId)
        if (found) {
          if (found.active_session_id) setSessionId(found.active_session_id)
          if (found.topic) setTopic(found.topic)
          if (found.platform) setPlatform(found.platform)
          if (found.account_id) {
            const accs = await fetchAccounts(found.platform)
            const acc = accs.find((a) => a.account_id === found.account_id)
            if (acc) setLockedAccount(acc)
          }
          if (found.novel_name) { setNovelName(found.novel_name); setNovelNameLocked(true) }
          if (typeof found.published_chapter_count === "number") {
            setChapterNumber(found.published_chapter_count + 1)
          }
        }
      } catch {}
      // 加载章节列表，默认选中最后一章（当前章）
      const tabs = await loadChapters(taskId)
      if (tabs.length > 0) {
        const last = tabs[tabs.length - 1]
        setActiveChapter(last.sessionId)
      }
    }
    resolve()
  }, [taskId, loadChapters])

  useEffect(() => {
    if (!sessionId) return
    const init = async () => {
      let hasExistingDraft = false
      try {
        const d = await getDraft(sessionId)
        if (d.draft) {
          setDraft(d.draft)
          setChapterDraft(d.draft)
          setDraftVersion(d.draft_version || 0)
          setStatus("WARM")
          hasExistingDraft = true
        }
      } catch {}
      await startWS()
      if (!autoSentRef.current && !hasExistingDraft && topic) {
        autoSentRef.current = true
        try { await sendMessage(sessionId, topic, 0); setStreaming(true) } catch {}
      }
    }
    init()
    return () => { wsRef.current?.close() }
  }, [startWS, sessionId])

  // 点击章节 tab 加载对应草稿
  const handleSelectChapter = async (sid: string) => {
    if (sid === activeChapter) return
    setActiveChapter(sid)
    setChapterDraftLoading(true)
    setChapterDraft("")
    try {
      const d = await getDraft(sid)
      setChapterDraft(d.draft || "")
    } catch {
      setChapterDraft("")
    } finally {
      setChapterDraftLoading(false)
    }
  }

  // 当前章节 draft 随 draft state 同步（当前活跃 session）
  useEffect(() => {
    if (activeChapter === sessionId) setChapterDraft(draft)
  }, [draft, activeChapter, sessionId])

  const handleSend = async () => {
    if (!input.trim() || !sessionId) return
    const mid = nextMsgId()
    setMessages((prev) => [...prev, { id: mid, role: "user", text: input.trim(), timestamp: new Date().toISOString() }])
    setInput("")
    setStreaming(true)
    setError("")
    try {
      await sendMessage(sessionId, input.trim(), draftVersion)
    } catch (err) {
      setError(err instanceof Error ? err.message : "发送失败")
      setStreaming(false)
    }
  }

  const handlePublish = async () => {
    if (!taskId || !sessionId) return
    setPublishState("publishing")
    publishStateRef.current = "publishing"
    publishWsRef.current?.close()
    const ws = connectTaskWS(taskId,
      (event: WSEvent) => {
        const stage = (event as any).stage || event.type
        const wsStatus = (event as any).status
        if (stage === "done") { setPublishState(wsStatus === "success" ? "done" : "error"); publishStateRef.current = wsStatus === "success" ? "done" : "error" }
        else if (stage === "done_partial") { setPublishState("partial"); publishStateRef.current = "partial" }
        else if (stage === "error" || wsStatus === "error") { setPublishState("error"); publishStateRef.current = "error" }
      },
      () => { setPublishState("error"); publishStateRef.current = "error" },
      () => { if (publishStateRef.current === "publishing") { setPublishState("done"); publishStateRef.current = "done" } }
    )
    publishWsRef.current = ws
    try {
      const accountsForPublish = lockedAccount ? [lockedAccount.account_id] : []
      const result = await publishTask(taskId, {
        draft_version: draftVersion, sessionId, platform, accounts: accountsForPublish,
        skillId: "", topic, novelName, title: chapterTitle, volumeName, chapterNumber,
      })
      if (result.status === "done") {
        setPublishState("done"); publishStateRef.current = "done"; setError("")
        if (novelName) setNovelNameLocked(true)
      } else if (result.status === "done_partial") {
        setPublishState("partial"); publishStateRef.current = "partial"
        const failed = (result.results || []).filter((r: any) => r.status !== "ok")
        setError("部分账号发布失败: " + failed.map((r: any) => `${r.platform}:${r.errorCode || "unknown"}`).join(", "))
      } else {
        setPublishState("error"); publishStateRef.current = "error"
        setError("发布未完全成功")
      }
    } catch (err) {
      setPublishState("error"); publishStateRef.current = "error"
      setError(err instanceof Error ? err.message : "发布失败")
    }
    setShowPublishModal(false)
    setTimeout(() => { if (publishStateRef.current === "publishing") { setPublishState("done"); publishStateRef.current = "done" } }, 120000)
  }

  const handleDelete = async () => {
    if (!taskId) return
    setDeleting(true)
    try {
      await deleteTask(taskId)
      router.replace("/tasks")
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除失败")
      setDeleting(false)
      setShowDeleteModal(false)
    }
  }

  const keyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() }
  }

  return (
    <div className="h-screen flex flex-col overflow-hidden bg-slate-50">

      {/* ── 顶部 Header ── */}
      <header className="h-14 bg-white border-b border-slate-200 flex items-center justify-between px-4 shrink-0 z-20 shadow-sm">
        <div className="flex items-center gap-3 min-w-0">
          <button
            onClick={() => router.back()}
            className="p-2 text-slate-400 hover:text-slate-700 transition-colors rounded-lg hover:bg-slate-50 shrink-0"
          >
            <ArrowLeft size={18} />
          </button>
          <div className="h-4 w-px bg-slate-200 shrink-0" />
          <h1 className="text-sm font-semibold text-slate-700 truncate">
            {novelName || "创作工作台"}
          </h1>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => setShowDeleteModal(true)}
            className="px-3 py-1.5 text-sm font-medium text-slate-500 bg-white border border-slate-200 rounded-md hover:bg-red-50 hover:text-red-500 hover:border-red-200 transition-colors flex items-center gap-1.5 shadow-sm"
          >
            <Trash2 size={14} />
            删除
          </button>
          <button
            onClick={() => setShowPublishModal(true)}
            disabled={!draft}
            className="px-4 py-1.5 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-md hover:opacity-90 shadow-sm flex items-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          >
            {publishState === "publishing"
              ? <><Loader2 size={14} className="animate-spin" />发布中...</>
              : publishState === "done"
                ? <><CheckCircle size={14} />已发布</>
                : publishState === "error"
                  ? <><AlertCircle size={14} />发布失败</>
                  : <><Send size={14} />一键发布</>}
          </button>
        </div>
      </header>

      {/* ── 主内容区 ── */}
      <main className="flex-1 flex overflow-hidden">

        {/* 左侧：对话区 */}
        <section className="w-2/5 min-w-[340px] max-w-[560px] border-r border-slate-200 bg-slate-50/50 flex flex-col">
          <div className="flex-1 overflow-y-auto p-4 space-y-5 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
            {messages.map((msg) => (
              <div key={msg.id} className={cn("flex", msg.role === "user" ? "justify-end" : "justify-start items-start gap-3")}>
                {msg.role === "assistant" && (
                  <div className="w-8 h-8 rounded-full bg-orange-100 flex items-center justify-center shrink-0 text-xs font-bold text-orange-600">AI</div>
                )}
                <div className={cn(
                  "max-w-[85%] px-4 py-3 text-sm shadow-sm leading-relaxed",
                  msg.role === "user"
                    ? "bg-slate-900 text-white rounded-[20px_20px_4px_20px]"
                    : "bg-white border border-slate-200 text-slate-700 rounded-[20px_20px_20px_4px]"
                )}>
                  <p className="whitespace-pre-wrap">{msg.text}</p>
                </div>
              </div>
            ))}
            {streaming && (
              <div className="flex justify-start items-start gap-3">
                <div className="w-8 h-8 rounded-full bg-orange-100 flex items-center justify-center shrink-0 text-xs font-bold text-orange-600">AI</div>
                <div className="max-w-[85%] px-4 py-3 bg-white border border-slate-200 text-slate-700 rounded-[20px_20px_20px_4px] text-sm shadow-sm leading-relaxed">
                  {streamingText
                    ? <span className="whitespace-pre-wrap">{streamingText}<span className="inline-block w-0.5 h-4 bg-orange-500 animate-pulse ml-0.5 align-middle" /></span>
                    : <span className="flex items-center gap-2 text-slate-400"><Loader2 className="w-3 h-3 animate-spin" />AI 正在思考...</span>
                  }
                </div>
              </div>
            )}
            <div ref={chatEndRef} />
          </div>

          {error && (
            <div className="mx-4 mb-2 p-2.5 rounded-lg bg-red-50 border border-red-100 text-red-600 text-xs flex items-center gap-2">
              <AlertCircle size={13} className="shrink-0" /> {error}
              <button className="ml-auto text-red-400 hover:text-red-600" onClick={() => setError("")}><X size={13} /></button>
            </div>
          )}

          <div className="p-4 bg-white border-t border-slate-200 shrink-0">
            <div className="flex gap-2 items-end">
              <textarea
                rows={3}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={keyDown}
                placeholder="给 AI 发送指令... (Enter 发送，Shift+Enter 换行)"
                className="flex-1 bg-slate-50 border border-slate-200 rounded-xl p-3 text-sm focus:outline-none focus:ring-2 focus:ring-orange-400 resize-none leading-relaxed"
              />
              <button
                onClick={handleSend}
                disabled={!input.trim() || streaming}
                className="p-2.5 rounded-xl bg-gradient-to-br from-orange-500 to-red-500 text-white hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition-all shadow-sm shrink-0"
              >
                {streaming ? <Loader2 size={16} className="animate-spin" /> : <Send size={16} />}
              </button>
            </div>
          </div>
        </section>

        {/* 右侧：草稿预览区 */}
        <section className="flex-1 bg-white flex flex-col min-w-0">

          {/* 章节 Tab 栏 */}
          <div className="h-12 border-b border-slate-200 flex items-end shrink-0 bg-slate-50/50 relative">
            {/* 可横向滚动的章节列表 */}
            <div
              ref={tabsRef}
              className="flex-1 flex items-end overflow-x-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none] px-2"
            >
              {chapters.map((ch) => (
                <button
                  key={ch.sessionId}
                  onClick={() => handleSelectChapter(ch.sessionId)}
                  className={cn(
                    "shrink-0 px-3 pb-2.5 pt-1 text-sm border-b-2 transition-colors whitespace-nowrap",
                    activeChapter === ch.sessionId
                      ? "border-orange-500 text-orange-600 font-semibold"
                      : "border-transparent text-slate-500 hover:text-slate-800 font-medium"
                  )}
                >
                  {ch.label}
                </button>
              ))}
            </div>

            {/* 新建章节 — 固定在右侧 */}
            <button className="shrink-0 flex items-center gap-1 px-3 pb-2.5 pt-1 text-sm font-medium text-slate-400 hover:text-orange-500 border-b-2 border-transparent transition-colors whitespace-nowrap bg-slate-50/50 border-l border-slate-100">
              <Plus size={13} />新建章节
            </button>
          </div>

          {/* 草稿内容 */}
          <div className="flex-1 overflow-y-auto px-12 py-10 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
            {chapterDraftLoading ? (
              <div className="max-w-2xl mx-auto text-center pt-24">
                <Loader2 className="w-6 h-6 text-slate-300 animate-spin mx-auto mb-3" />
                <p className="text-slate-400 text-sm">加载章节内容...</p>
              </div>
            ) : chapterDraft ? (
              <div className="max-w-2xl mx-auto">
                <pre className="text-[15px] text-slate-700 whitespace-pre-wrap font-sans leading-[1.85] tracking-wide">{chapterDraft}</pre>
              </div>
            ) : (
              <div className="max-w-2xl mx-auto text-center pt-24">
                <div className="w-16 h-16 rounded-full bg-slate-100 flex items-center justify-center mx-auto mb-4">
                  <Loader2 className="w-6 h-6 text-slate-300 animate-spin" />
                </div>
                <p className="text-slate-400 text-sm">AI 正在创作中，稿件生成后将自动展示...</p>
              </div>
            )}
          </div>
        </section>
      </main>

      {/* ── 删除确认 Modal ── */}
      <Dialog open={showDeleteModal} onOpenChange={setShowDeleteModal}>
        <DialogContent className="max-w-sm" onPointerDownOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle className="text-center">删除小说</DialogTitle>
          </DialogHeader>
          <div className="py-3 text-center space-y-2">
            <div className="w-12 h-12 rounded-full bg-red-50 flex items-center justify-center mx-auto">
              <Trash2 size={20} className="text-red-500" />
            </div>
            <p className="text-sm text-slate-700 font-medium">确认删除《{novelName || "该小说"}》？</p>
            <p className="text-xs text-slate-400">删除后所有内容将无法找回，请谨慎操作。</p>
          </div>
          <DialogFooter>
            <button
              onClick={() => setShowDeleteModal(false)}
              className="px-4 py-2 text-sm text-slate-500 hover:text-slate-700 transition-colors"
            >
              取消
            </button>
            <button
              onClick={handleDelete}
              disabled={deleting}
              className="px-6 py-2 text-sm font-medium text-white bg-red-500 hover:bg-red-600 rounded-lg disabled:opacity-50 transition-all"
            >
              {deleting ? <span className="flex items-center gap-1.5"><Loader2 size={13} className="animate-spin" />删除中...</span> : "确认删除"}
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── 发布 Modal ── */}
      <Dialog open={showPublishModal} onOpenChange={setShowPublishModal}>
        <DialogContent className="max-w-lg" onPointerDownOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>确认发布内容</DialogTitle>
          </DialogHeader>
          <div className="space-y-5 py-2">
            <div>
              <p className="text-sm font-semibold text-slate-700 mb-3">1. 章节信息</p>
              <div className="space-y-3">
                <div>
                  <label className="text-xs text-slate-500 mb-1 block">小说名称</label>
                  {novelNameLocked
                    ? <p className="text-sm text-slate-800 px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg">{novelName || "（未设置）"}</p>
                    : <Input value={novelName} onChange={(e) => setNovelName(e.target.value)} placeholder="请输入小说名称" className="h-9 text-sm" />
                  }
                </div>
                <div className="flex gap-2">
                  <div className="flex-1">
                    <label className="text-xs text-slate-500 mb-1 block">分卷</label>
                    <Input value={volumeName} onChange={(e) => setVolumeName(e.target.value)} placeholder="如：第一卷" className="h-9 text-sm" />
                  </div>
                  <div className="w-24">
                    <label className="text-xs text-slate-500 mb-1 block">章节号</label>
                    <Input type="number" value={chapterNumber} onChange={(e) => setChapterNumber(Number(e.target.value))} className="h-9 text-sm" />
                  </div>
                </div>
                <div>
                  <label className="text-xs text-slate-500 mb-1 block">章节标题 <span className="text-slate-400">（选填）</span></label>
                  <Input value={chapterTitle} onChange={(e) => setChapterTitle(e.target.value)} placeholder="可留空，AI 自动生成..." className="h-9 text-sm" />
                </div>
              </div>
            </div>
            <div>
              <p className="text-sm font-semibold text-slate-700 mb-3">2. 目标平台确认</p>
              <div className="flex items-center gap-3">
                {platform && (
                  <span className={cn(
                    "px-2.5 py-1 text-xs font-bold border rounded-md",
                    platform === "fanqie" ? "text-red-600 bg-red-50 border-red-100" :
                    platform === "zhulang" ? "text-blue-600 bg-blue-50 border-blue-100" :
                    "text-slate-600 bg-slate-50 border-slate-200"
                  )}>{PLATFORM_LABELS[platform] || platform}</span>
                )}
                {lockedAccount && <span className="text-xs text-slate-500">{lockedAccount.masked_display}</span>}
              </div>
            </div>
          </div>
          <DialogFooter>
            <button onClick={() => setShowPublishModal(false)} className="px-4 py-2 text-sm text-slate-500 hover:text-slate-700 transition-colors">取消</button>
            <button
              onClick={handlePublish}
              disabled={!novelName || publishState === "publishing"}
              className="px-6 py-2 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-lg hover:opacity-90 shadow-sm disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {publishState === "publishing" ? <span className="flex items-center gap-1.5"><Loader2 size={13} className="animate-spin" />发布中...</span> : "确认，立即发布"}
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
