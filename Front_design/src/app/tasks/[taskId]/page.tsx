"use client"

import { useEffect, useState, useRef, useCallback } from "react"
import { useParams, useSearchParams, useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  sendMessage, publishTask, getDraft, fetchTask, fetchAccounts,
  deleteTask, fetchTaskSessions, createSession, closeSession, fetchTaskMessages,
} from "@/lib/api"
import { connectChatTaskWS, connectTaskWS, type WSController } from "@/lib/ws"
import type { SessionMessage, WSEvent } from "@/types"
import { Send, Loader2, CheckCircle, AlertCircle, ArrowLeft, Trash2, Plus } from "lucide-react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { cn } from "@/lib/utils"
import { resolveTaskListReturnUrl } from "@/lib/task-navigation"

type ChapterTab = { sessionId: string; label: string; index: number; fallbackDraft?: string; skillId?: string; model?: string; hasContent: boolean; chapterTitle?: string }

// 从 episode decisions 中提取 "## Latest Draft" 段落内容
function extractLatestDraft(decisions: string): string {
  const match = decisions.match(/## Latest Draft\s*\n+([\s\S]*?)(\n## |\n#[^#]|$)/)
  return match ? match[1].trim() : ""
}

export default function SessionPage() {
  const { taskId } = useParams<{ taskId: string }>()
  const searchParams = useSearchParams()
  const router = useRouter()
  const sessionIdFromQuery = searchParams.get("sid") || ""

  const [sessionId, setSessionId] = useState(sessionIdFromQuery)
  const [taskMessages, setTaskMessages] = useState<SessionMessage[]>([])
  const [draftVersion, setDraftVersion] = useState(0)
  const [input, setInput] = useState("")
  const [streaming, setStreaming] = useState(false)
  const [streamingText, setStreamingText] = useState("")
  const [publishState, setPublishState] = useState("")
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
  const [publishChapterId, setPublishChapterId] = useState("")
  const [publishedCount, setPublishedCount] = useState(0)

  const [showNewChapterModal, setShowNewChapterModal] = useState(false)
  const [newChapterVolume, setNewChapterVolume] = useState("第一卷")
  const [newChapterTitle, setNewChapterTitle] = useState("")
  const [creatingChapter, setCreatingChapter] = useState(false)

  const [tabsOverflow, setTabsOverflow] = useState(false)
  const [toolCallActive, setToolCallActive] = useState(false)
  const [wsReconnecting, setWsReconnecting] = useState(false)
  const [wsReconnectAttempt, setWsReconnectAttempt] = useState(0)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WSController | null>(null)
  const publishWsRef = useRef<WebSocket | null>(null)
  const msgCounterRef = useRef(0)
  const publishStateRef = useRef("")
  const taskResolvedRef = useRef(false)
  const tabsRef = useRef<HTMLDivElement>(null)
  // 用 ref 避免 WS 闭包中拿到过期值
  const activeChapterRef = useRef("")
  const sessionIdRef = useRef(sessionIdFromQuery)

  useEffect(() => { activeChapterRef.current = activeChapter }, [activeChapter])
  useEffect(() => { sessionIdRef.current = sessionId }, [sessionId])

  const nextMsgId = useCallback(() => {
    msgCounterRef.current += 1
    return `${sessionId}:${msgCounterRef.current}`
  }, [sessionId])

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [taskMessages, streamingText])

  // 检测章节 tab 是否溢出，控制"新建章节"按钮位置
  useEffect(() => {
    const el = tabsRef.current
    if (!el) return
    const check = () => setTabsOverflow(el.scrollWidth > el.clientWidth + 1)
    check()
    const ro = new ResizeObserver(check)
    ro.observe(el)
    return () => ro.disconnect()
  }, [chapters])

  const refreshChapterDraft = useCallback((sid: string) => {
    getDraft(sid).then((d) => {
      if (d.draft && activeChapterRef.current === sid) setChapterDraft(d.draft)
      if (d.chapter_title) setChapterTitle(d.chapter_title)
      setDraftVersion(d.draft_version || 0)
      setChapters(prev => prev.map(c => c.sessionId === sid ? {
        ...c,
        chapterTitle: d.chapter_title || c.chapterTitle,
        hasContent: c.hasContent || !!d.draft,
      } : c))
    }).catch(() => {})
  }, [])

  const startWS = useCallback(async () => {
    if (!taskId) return
    wsRef.current?.close()
    const ws = connectChatTaskWS(
      taskId,
      (event: WSEvent) => {
        const eventSessionId = event.session_id || sessionIdRef.current
        switch (event.type) {
          case "token":
            setStreamingText((prev) => prev + (event.text || ""))
            break
          case "draft_updated":
            setDraftVersion(event.draft_version || 0)
            refreshChapterDraft(eventSessionId)
            break
          case "novel_name":
            if (event.novel_name && !novelNameLocked) setNovelName(event.novel_name)
            break
          case "tool_call":
            // AI 正在调用工具（如联网搜索、代码执行等）
            setToolCallActive(true)
            break
          case "step_finish":
            // 一个推理步骤完成，清除工具调用状态
            setToolCallActive(false)
            break
          case "done":
            setStreamingText((prev) => {
              if (prev) {
                const mid = `${taskId}:assistant:${++msgCounterRef.current}`
                setTaskMessages((msgs) => [...msgs, {
                  id: mid, role: "assistant", text: prev,
                  timestamp: new Date().toISOString(), draft_version: event.draft_version,
                }])
              }
              return ""
            })
            refreshChapterDraft(eventSessionId)
            setToolCallActive(false)
            setStreaming(false)
            setWsReconnecting(false)
            break
          case "error":
            toast.error(event.message || event.error || event.reason || "发生错误")
            setStreaming(false)
            setToolCallActive(false)
            setWsReconnecting(false)
            break
          case "episode_created":
            if (event.next_session_id) setSessionId(event.next_session_id)
            break
          case "session_interrupted":
            toast.error("服务暂时中断，请重试")
            setStreaming(false)
            setToolCallActive(false)
            break
        }
      },
      () => toast.error("WebSocket 连接错误"),
      undefined,
      (attempt) => {
        setWsReconnecting(true)
        setWsReconnectAttempt(attempt)
      },
      () => { setWsReconnecting(false) },
    )
    wsRef.current = ws
  }, [taskId, refreshChapterDraft])

  // 加载 sessions 列表作为章节 tab
  const loadChapters = useCallback(async (tid: string) => {
    try {
      const resp = await fetchTaskSessions(tid)
      const sorted = [...(resp.sessions || [])].sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      )
      const tabs: ChapterTab[] = sorted.map((s, i) => {
        let fallbackDraft = ""
        if (s.episodes && s.episodes.length > 0) {
          const lastEp = s.episodes[s.episodes.length - 1]
          fallbackDraft = extractLatestDraft(lastEp.decisions || "")
        }
        const hasContent = fallbackDraft.length > 0 || (s.draft_version > 0)
        return {
          sessionId: s.session_id, label: `第 ${i + 1} 章`, index: i + 1,
          fallbackDraft, skillId: s.skill_id, model: s.model, hasContent,
        }
      })
      setChapters(tabs)
      // 异步补全每章的 chapter_title 与 hasContent（运行中的 session draft_version 可能为 0 但文件已有内容）
      tabs.forEach((tab) => {
        getDraft(tab.sessionId).then((d) => {
          setChapters(prev => prev.map(c => {
            if (c.sessionId !== tab.sessionId) return c
            return {
              ...c,
              chapterTitle: d.chapter_title || c.chapterTitle,
              hasContent: c.hasContent || !!(d.draft),
            }
          }))
        }).catch(() => {})
      })
      return tabs
    } catch { return [] }
  }, [])

  useEffect(() => {
    if (!taskId || taskResolvedRef.current) return
    taskResolvedRef.current = true
    const resolve = async () => {
      try {
        const found = await fetchTask(taskId)
        if (found) {
          // 不用 active_session_id 覆盖 URL 里的 sid，避免 WS 被重启
          // sessionId 由 URL param 或下方 loadChapters 决定
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
            setPublishedCount(found.published_chapter_count)
          }
        }
      } catch {}
      // 加载章节列表，默认选中最后一章（当前章）
      const tabs = await loadChapters(taskId)
      if (tabs.length > 0) {
        const last = tabs[tabs.length - 1]
        setActiveChapter(last.sessionId)
        // URL 没有 sid 参数时（从任务列表进入），用最后一章的 sessionId 建立 WS
        if (!sessionIdFromQuery) {
          setSessionId(last.sessionId)
        }
        setChapterDraftLoading(true)
        try {
          const d = await getDraft(last.sessionId)
          setChapterDraft(d.draft || last.fallbackDraft || "")
        } catch {
          setChapterDraft(last.fallbackDraft || "")
        } finally {
          setChapterDraftLoading(false)
        }
      }
    }
    resolve()
  }, [taskId, loadChapters])

  useEffect(() => {
    if (!taskId) return
    fetchTaskMessages(taskId)
      .then((resp) => setTaskMessages(resp.messages || []))
      .catch(() => {})
  }, [taskId])

  // 任务详情页只建立一条 task 级 WebSocket
  useEffect(() => {
    if (!taskId) return
    startWS()
    return () => {
      wsRef.current?.close()
    }
  }, [taskId, startWS])

  // 点击章节 tab 加载对应草稿
  const handleSelectChapter = async (sid: string) => {
    if (sid === activeChapter) return
    setActiveChapter(sid)
    setChapterDraftLoading(true)
    setChapterDraft("")
    try {
      const d = await getDraft(sid)
      setChapterDraft(d.draft || "")
      if (d.chapter_title) setChapterTitle(d.chapter_title)
      if (!d.draft) {
        const tab = chapters.find(c => c.sessionId === sid)
        setChapterDraft(tab?.fallbackDraft || "")
      }
    } catch {
      const tab = chapters.find(c => c.sessionId === sid)
      setChapterDraft(tab?.fallbackDraft || "")
    } finally {
      setChapterDraftLoading(false)
    }
  }

  const handleSend = async () => {
    if (!input.trim() || !sessionId) return
    const mid = nextMsgId()
    const text = input.trim()
    setTaskMessages((prev) => [...prev, { id: mid, role: "user", text, timestamp: new Date().toISOString() }])
    setInput("")
    setStreaming(true)
    try {
      await sendMessage(sessionId, text, draftVersion)
    } catch (err) {
      setTaskMessages((prev) => prev.filter(msg => msg.id !== mid))
      toast.error(err instanceof Error ? err.message : "发送失败")
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
      const publishSid = publishChapterId || sessionId
      const publishChapter = chapters.find(c => c.sessionId === publishSid)
      const publishChNum = publishChapter ? publishChapter.index : chapterNumber
      const publishChTitle = publishChapter?.chapterTitle || chapterTitle
      const result = await publishTask(taskId, {
        draft_version: draftVersion, sessionId: publishSid, platform, accounts: accountsForPublish,
        skillId: "", topic, novelName, title: publishChTitle, volumeName, chapterNumber: publishChNum,
      })
      if (result.status === "done") {
        setPublishState("done"); publishStateRef.current = "done"
        if (novelName) setNovelNameLocked(true)
      } else if (result.status === "done_partial") {
        setPublishState("partial"); publishStateRef.current = "partial"
        const failed = (result.results || []).filter((r: any) => r.status !== "ok")
        toast.error("部分账号发布失败: " + failed.map((r: any) => `${r.platform}:${r.errorCode || "unknown"}`).join(", "))
      } else {
        setPublishState("error"); publishStateRef.current = "error"
        toast.error("发布未完全成功")
      }
    } catch (err) {
      setPublishState("error"); publishStateRef.current = "error"
      toast.error(err instanceof Error ? err.message : "发布失败")
    }
    setShowPublishModal(false)
    setTimeout(() => { if (publishStateRef.current === "publishing") { setPublishState("done"); publishStateRef.current = "done" } }, 120000)
  }

  const handleDelete = async () => {
    if (!taskId) return
    setDeleting(true)
    try {
      await deleteTask(taskId)
      router.replace(resolveTaskListReturnUrl(searchParams))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除失败")
      setDeleting(false)
      setShowDeleteModal(false)
    }
  }

  const handleNewChapter = async () => {
    if (!taskId || creatingChapter) return
    setCreatingChapter(true)
    try {
      // 创建新章节前先关闭当前活跃 session，释放任务锁
      if (sessionId) {
        try { await closeSession(sessionId) } catch { /* session 可能已关闭，忽略 */ }
      }
      const lastChapter = chapters[chapters.length - 1]
      const sessionPayload = {
        task_id: taskId,
        skillId: lastChapter?.skillId || "general_fallback_v1",
        model: lastChapter?.model,
        topic,
        platform,
        accountId: lockedAccount?.account_id,
        novel_name: novelName || undefined,
        chapter_number: chapters.length + 1,
      }

      let result
      try {
        result = await createSession(sessionPayload)
      } catch (err: unknown) {
        // 409：任务仍有活跃 session，提取其 id，关掉后重试一次
        if (err instanceof Error) {
          const match = err.message.match(/existing_session_id[":\s]+([a-z0-9]+)/)
          if (match?.[1]) {
            try { await closeSession(match[1]) } catch { /* ignore */ }
            result = await createSession(sessionPayload)
          } else {
            throw err
          }
        } else {
          throw err
        }
      }
      setShowNewChapterModal(false)
      setNewChapterTitle("")
      // 重新加载章节列表，切换到新章节
      const newTabs = await loadChapters(taskId)
      const newSessionId = result.session_id
      setActiveChapter(newSessionId)
      setChapterDraft("")
      setSessionId(newSessionId)
      setDraftVersion(0)
      // 为新章节设置标题（如有）
      if (newChapterTitle.trim()) setChapterTitle(newChapterTitle.trim())
      const newIdx = newTabs.findIndex(t => t.sessionId === newSessionId)
      if (newIdx >= 0) setChapterNumber(newIdx + 1)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建章节失败")
    } finally {
      setCreatingChapter(false)
    }
  }
  const returnToTaskList = useCallback(() => {
    router.push(resolveTaskListReturnUrl(searchParams))
  }, [router, searchParams])

  const keyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() }
  }

  return (
    <div className="h-screen flex flex-col overflow-hidden bg-slate-50">

      {/* ── 顶部 Header ── */}
      <header className="h-14 bg-white border-b border-slate-200 flex items-center justify-between px-4 shrink-0 z-20 shadow-sm">
        <div className="flex items-center gap-3 min-w-0">
          <button
            onClick={returnToTaskList}
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
            onClick={() => {
              // 默认选中第一个有内容且未发布的章节
              const firstPending = chapters.find(c => c.hasContent)
              setPublishChapterId(firstPending?.sessionId || activeChapter)
              setShowPublishModal(true)
            }}
            disabled={!chapters.some(ch => ch.hasContent)}
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
        <section className="w-[40%] min-w-[360px] max-w-[600px] border-r border-slate-200 bg-slate-50/50 flex flex-col">
          <div className="flex-1 overflow-y-auto p-4 space-y-5 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
            {taskMessages.map((msg) => (
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
                    : toolCallActive
                      ? <span className="flex items-center gap-2 text-slate-400"><Loader2 className="w-3 h-3 animate-spin" />AI 正在处理...</span>
                      : <span className="flex items-center gap-2 text-slate-400"><Loader2 className="w-3 h-3 animate-spin" />AI 正在思考...</span>
                  }
                </div>
              </div>
            )}
            {wsReconnecting && !streaming && (
              <div className="flex justify-center">
                <span className="text-xs text-orange-500 bg-orange-50 border border-orange-200 px-3 py-1 rounded-full flex items-center gap-1.5">
                  <Loader2 className="w-3 h-3 animate-spin" />
                  连接中断，正在重连（第 {wsReconnectAttempt} 次）...
                </span>
              </div>
            )}
            <div ref={chatEndRef} />
          </div>

          <div className="p-4 bg-white border-t border-slate-200 shrink-0">
            <div className="flex gap-2 items-stretch">
              <textarea
                rows={2}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={keyDown}
                placeholder="给 AI 发送指令... (Enter 发送，Shift+Enter 换行)"
                className="flex-1 bg-slate-50 border border-slate-200 rounded-xl p-3 text-sm focus:outline-none focus:ring-2 focus:ring-orange-400 resize-none leading-relaxed"
              />
              <button
                onClick={handleSend}
                disabled={!input.trim() || streaming}
                className="w-10 rounded-xl bg-gradient-to-br from-orange-500 to-red-500 text-white hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition-all shadow-sm shrink-0 flex items-center justify-center"
              >
                {streaming ? <Loader2 size={18} className="animate-spin" /> : <Send size={18} />}
              </button>
            </div>
          </div>
        </section>

        {/* 右侧：草稿预览区 */}
        <section className="w-[60%] bg-white flex flex-col min-w-0">

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
              {/* 未溢出时，新建章节紧跟最后一个 tab */}
              {!tabsOverflow && (
                <button
                  onClick={() => setShowNewChapterModal(true)}
                  className="shrink-0 flex items-center gap-1 px-3 pb-2.5 pt-1 text-sm font-medium text-slate-400 hover:text-orange-500 border-b-2 border-transparent transition-colors whitespace-nowrap"
                >
                  <Plus size={13} />新建章节
                </button>
              )}
            </div>
            {/* 溢出时，新建章节固定在右侧 */}
            {tabsOverflow && (
              <button
                onClick={() => setShowNewChapterModal(true)}
                className="shrink-0 flex items-center gap-1 px-3 pb-2.5 pt-1 text-sm font-medium text-slate-400 hover:text-orange-500 border-b-2 border-transparent transition-colors whitespace-nowrap bg-slate-50/50 border-l border-slate-100"
              >
                <Plus size={13} />新建章节
              </button>
            )}
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
                <p className="text-slate-300 text-sm">暂无内容</p>
              </div>
            )}
          </div>
        </section>
      </main>

      {/* ── 新建章节 Modal ── */}
      <Dialog open={showNewChapterModal} onOpenChange={(v) => { setShowNewChapterModal(v); if (!v) setNewChapterTitle("") }}>
        <DialogContent className="max-w-md p-0 gap-0 overflow-hidden rounded-2xl" onPointerDownOutside={(e) => e.preventDefault()}>
          <DialogTitle className="sr-only">新建章节</DialogTitle>
          {/* Header */}
          <div className="px-6 py-4 border-b border-slate-200 flex justify-between items-center">
            <h2 className="text-lg font-bold text-slate-900">新建章节</h2>
          </div>
          {/* Body */}
          <div className="p-6 space-y-5">
            {/* 选择分卷 */}
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-2">选择分卷</label>
              <SelectRadix value={newChapterVolume} onValueChange={setNewChapterVolume} className="h-11 px-4 text-sm">
                <SelectItem value="第一卷">第一卷</SelectItem>
              </SelectRadix>
            </div>
            {/* 章节序号与名称 */}
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-2">
                章节序号与名称
                <span className="ml-1.5 text-slate-400 font-normal">（选填）</span>
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  readOnly
                  value={`第 ${chapters.length + 1} 章`}
                  className="w-24 px-3 py-2.5 border border-slate-200 rounded-lg text-sm text-center text-slate-700 bg-slate-50 focus:outline-none focus:ring-2 focus:ring-orange-500"
                />
                <input
                  type="text"
                  value={newChapterTitle}
                  onChange={(e) => setNewChapterTitle(e.target.value)}
                  placeholder="可留空，AI 将自动生成..."
                  className="flex-1 px-3 py-2.5 border border-slate-200 rounded-lg text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-orange-500"
                />
              </div>
            </div>
          </div>
          {/* Footer */}
          <div className="px-6 py-4 border-t border-slate-200 bg-slate-50 flex justify-end gap-3">
            <button
              onClick={() => { setShowNewChapterModal(false); setNewChapterTitle("") }}
              className="px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-200 rounded-lg transition-colors"
            >
              取消
            </button>
            <button
              onClick={handleNewChapter}
              disabled={creatingChapter}
              className="px-6 py-2 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-lg hover:opacity-90 shadow-sm disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {creatingChapter
                ? <span className="flex items-center gap-1.5"><Loader2 size={13} className="animate-spin" />创建中...</span>
                : "创建并开始写作"}
            </button>
          </div>
        </DialogContent>
      </Dialog>

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
        <DialogContent className="max-w-xl p-0 gap-0 overflow-hidden rounded-2xl" onPointerDownOutside={(e) => e.preventDefault()}>
          <DialogTitle className="sr-only">确认发布内容</DialogTitle>
          {/* Header */}
          <div className="px-6 py-4 border-b border-slate-200 flex justify-between items-center">
            <h2 className="text-xl font-bold text-slate-900">确认发布内容</h2>
          </div>

          {/* Body */}
          <div className="p-6">
            <h3 className="text-sm font-semibold text-slate-700 mb-3">选择要发布的单章节</h3>
            <div className="border border-slate-200 rounded-lg divide-y divide-slate-100">
              {chapters.filter(ch => ch.hasContent).map((ch) => {
                const isPublished = ch.index <= publishedCount
                const isSelected = publishChapterId === ch.sessionId
                return (
                  <label
                    key={ch.sessionId}
                    className={cn(
                      "flex items-center px-4 py-3 transition-colors select-none",
                      isPublished ? "bg-slate-50 cursor-not-allowed" :
                        isSelected ? "hover:bg-orange-50/50 cursor-pointer" : "hover:bg-orange-50/50 cursor-pointer"
                    )}
                  >
                    <input
                      type="radio"
                      name="pub_chap"
                      disabled={isPublished}
                      checked={isSelected}
                      onChange={() => { if (!isPublished) setPublishChapterId(ch.sessionId) }}
                      className="h-4 w-4 accent-orange-500 border-slate-300 shrink-0"
                    />
                    <span className={cn(
                      "ml-3 text-sm flex-1 leading-snug",
                      isPublished ? "text-slate-400" : "text-slate-800"
                    )}>
                      <span className="font-normal mr-1">{volumeName}</span>
                      <span className="font-normal">{ch.label}</span>
                      {ch.chapterTitle && (
                        <span className="font-normal ml-1">：{ch.chapterTitle}</span>
                      )}
                    </span>
                    {isPublished ? (
                      <span className="text-xs text-emerald-600 font-medium px-2 py-0.5 bg-emerald-50 rounded">已发布</span>
                    ) : (
                      <span className="text-xs text-orange-600 font-medium px-2 py-0.5 bg-orange-50 rounded">
                        {isSelected ? "待发布 (当前)" : "未发布"}
                      </span>
                    )}
                  </label>
                )
              })}
              {chapters.filter(ch => ch.hasContent).length === 0 && (
                <div className="px-4 py-6 text-center text-sm text-slate-400">暂无可发布章节</div>
              )}
            </div>
            <p className="text-xs text-slate-500 mt-3 flex items-center gap-1">
              <svg className="w-4 h-4 text-slate-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              注意：每次发布仅针对单一章节。发布顺序严格按照章节生成顺序进行。
            </p>
          </div>

          {/* Footer */}
          <div className="px-6 py-4 border-t border-slate-200 bg-slate-50 flex justify-end gap-3">
            <button
              onClick={() => setShowPublishModal(false)}
              className="px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-200 rounded-lg transition-colors"
            >
              取消
            </button>
            <button
              onClick={handlePublish}
              disabled={!publishChapterId || publishState === "publishing"}
              className="px-6 py-2 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-lg hover:opacity-90 shadow-sm disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {publishState === "publishing"
                ? <span className="flex items-center gap-1.5"><Loader2 size={13} className="animate-spin" />发布中...</span>
                : "立即发布"}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
