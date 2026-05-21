"use client"

import { useEffect, useState, useRef, useCallback } from "react"
import { useParams, useSearchParams, useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Textarea, Input, Label } from "@/components/ui/input"
import { Card } from "@/components/ui/card"
import {
  sendMessage,
  publishTask,
  getDraft,
  fetchTasks,
  fetchAccounts,
} from "@/lib/api"
import { connectSessionWS, connectTaskWS } from "@/lib/ws"
import type { SessionMessage, WSEvent } from "@/types"
import {
  Send, Loader2, FileText, CheckCircle, AlertCircle, Sparkles, ArrowLeft,
} from "lucide-react"

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
  const [publishResults, setPublishResults] = useState<Array<{status:string;platform:string;accountId:string;postId?:string;errorCode?:string}>>([])
  const [topic, setTopic] = useState("")
  const [platform, setPlatform] = useState("")
  const [novelName, setNovelName] = useState("")
  const [novelNameLocked, setNovelNameLocked] = useState(false)
  const [chapterTitle, setChapterTitle] = useState("")
  const [volumeName, setVolumeName] = useState("第一卷")
  const [chapterNumber, setChapterNumber] = useState(1)
  const [lockedAccount, setLockedAccount] = useState<{account_id:string;masked_display:string} | null>(null)

  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const publishWsRef = useRef<WebSocket | null>(null)
  const msgCounterRef = useRef(0)
  const publishStateRef = useRef("")
  const taskResolvedRef = useRef(false)
  const autoSentRef = useRef(false)

  const platformLabel = (p: string) => ({ xhs: "小红书", wechat: "公众号", fanqie: "番茄小说", yuewen: "阅文", zhulang: "逐浪网" }[p] || p)

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
            getDraft(sessionId).then((d) => {
              if (d.draft) setDraft(d.draft)
            }).catch(() => {})
            break
          case "novel_name":
            if (event.novel_name && !novelNameLocked) {
              setNovelName(event.novel_name)
            }
            break
          case "done":
            setStreamingText((prev) => {
              if (prev) {
                const mid = nextMsgId()
                setMessages((msgs) => [...msgs, {
                  id: mid,
                  role: "assistant",
                  text: prev,
                  timestamp: new Date().toISOString(),
                  draft_version: event.draft_version,
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
          if (found.novel_name) {
            setNovelName(found.novel_name)
            setNovelNameLocked(true)
          }
          if (typeof found.published_chapter_count === "number") {
            setChapterNumber(found.published_chapter_count + 1)
          }
        }
      } catch {}
    }
    resolve()
  }, [taskId])

  useEffect(() => {
    if (!sessionId) return
    const init = async () => {
      let hasExistingDraft = false
      try {
        const d = await getDraft(sessionId)
        if (d.draft) {
          setDraft(d.draft)
          setDraftVersion(d.draft_version || 0)
          setStatus("WARM")
          hasExistingDraft = true
        }
      } catch {}
      await startWS()

      if (!autoSentRef.current && !hasExistingDraft && topic) {
        autoSentRef.current = true
        try {
          await sendMessage(sessionId, topic, 0)
          setStreaming(true)
        } catch {}
      }
    }
    init()
    return () => { wsRef.current?.close() }
  }, [startWS, sessionId])

  const handleSend = async () => {
    if (!input.trim() || !sessionId) return

    const mid = nextMsgId()
    setMessages((prev) => [...prev, {
      id: mid,
      role: "user",
      text: input.trim(),
      timestamp: new Date().toISOString(),
    }])
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
        if (stage === "done") {
          setPublishState(wsStatus === "success" ? "done" : "error")
          publishStateRef.current = wsStatus === "success" ? "done" : "error"
        } else if (stage === "done_partial") {
          setPublishState("partial")
          publishStateRef.current = "partial"
        } else if (stage === "error" || wsStatus === "error") {
          setPublishState("error")
          publishStateRef.current = "error"
        } else {
          setPublishState("publishing")
          publishStateRef.current = "publishing"
        }
      },
      () => { setPublishState("error"); publishStateRef.current = "error" },
      () => {
        if (publishStateRef.current === "publishing") {
          setPublishState("done")
          publishStateRef.current = "done"
        }
      }
    )
    publishWsRef.current = ws

    try {
      const accountsForPublish = lockedAccount ? [lockedAccount.account_id] : []
      const result = await publishTask(taskId, {
        draft_version: draftVersion, sessionId, platform, accounts: accountsForPublish, skillId: "", topic,
        novelName, title: chapterTitle, volumeName, chapterNumber,
      })
      if (result.status === "done") {
        setPublishState("done")
        publishStateRef.current = "done"
        if (novelName) setNovelNameLocked(true)
      } else if (result.status === "done_partial") {
        setPublishState("partial")
        publishStateRef.current = "partial"
        if (result.results) setPublishResults(result.results)
        const failed = (result.results || []).filter((r: any) => r.status !== "ok")
        const msgs = failed.map((r: any) => `${r.platform}:${r.errorCode || "unknown"}`).join(", ")
        setError("部分账号发布失败: " + (msgs || "unknown error"))
      } else {
        setPublishState("error")
        publishStateRef.current = "error"
        setError("发布未完全成功: " + JSON.stringify(result))
      }
    } catch (err) {
      setPublishState("error")
      publishStateRef.current = "error"
      setError(err instanceof Error ? err.message : "发布失败")
    }

    setTimeout(() => {
      if (publishStateRef.current === "publishing") {
        setPublishState("done")
        publishStateRef.current = "done"
      }
    }, 120000)
  }

  const keyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="flex h-[calc(100vh-7rem)] gap-6">
      <div className="flex-1 flex flex-col min-w-0">
        <div className="flex items-center justify-between pb-4 border-b border-border">
          <div className="flex items-center gap-4">
            <button onClick={() => router.back()} className="text-muted-foreground hover:text-foreground transition-colors">
              <ArrowLeft size={20} />
            </button>
            <div>
              <h1 className="text-base font-semibold text-foreground">会话视图</h1>
              <p className="text-xs text-muted-foreground mt-0.5">
                Task: {taskId?.slice(0, 16)}... · {status || "就绪"}
              </p>
            </div>
          </div>
        </div>

        <div className="flex-1 bg-white rounded-lg border border-[#e5e6eb] overflow-hidden min-h-0 flex flex-col">
          <div className="flex-1 overflow-y-auto py-6 px-4 space-y-4">
          {messages.length === 0 && !streaming && (
            <div className="flex flex-col items-center justify-center py-16">
              <Sparkles className="w-8 h-8 mb-4 text-primary/30" />
              {draft ? (
                <>
                  <p className="text-sm text-foreground font-medium">AI 已生成初稿</p>
                  <p className="text-xs text-muted-foreground mt-1">可在右侧查看稿子，或发送消息继续修改</p>
                </>
              ) : (
                <>
                  <p className="text-sm text-foreground font-medium">AI 正在准备创作，请稍候...</p>
                  <p className="text-xs text-muted-foreground mt-1">初始化完成后即可发送消息与 AI 对话</p>
                </>
              )}
            </div>
          )}

          {messages.map((msg) => (
            <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
              <div className={`max-w-[75%] px-4 py-2.5 rounded-xl text-[14px] leading-relaxed ${
                msg.role === "user"
                  ? "bg-primary text-primary-foreground rounded-br-md"
                  : "bg-[#f2f3f5] text-foreground rounded-bl-md"
              }`}>
                <p className="whitespace-pre-wrap">{msg.text}</p>
              </div>
            </div>
          ))}

          {streaming && (
            <div className="flex justify-start">
              <div className="max-w-[75%] px-4 py-2.5 rounded-xl bg-[#f2f3f5] text-foreground rounded-bl-md text-[14px] leading-relaxed">
                {streamingText ? (
                  <span>{streamingText}<span className="typing-cursor" /></span>
                ) : (
                  <span className="flex items-center gap-2 text-muted-foreground">
                    <Loader2 className="w-3 h-3 animate-spin" /> AI 正在思考...
                  </span>
                )}
              </div>
            </div>
          )}

          <div ref={chatEndRef} />
        </div>
        </div>

        {error && (
          <div className="mb-3 p-3 rounded-lg bg-destructive/8 border border-destructive/20 text-destructive text-sm flex items-center gap-2">
            <AlertCircle size={14} /> {error}
            <button className="ml-auto text-xs underline" onClick={() => setError("")}>关闭</button>
          </div>
        )}

        <div className="pt-3 border-t border-border">
          <div className="flex gap-3">
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={keyDown}
              placeholder="输入修改意见... (Shift+Enter 换行)"
              rows={2}
            />
            <Button onClick={handleSend} disabled={!input.trim()} className="self-end" size="md">
              <Send size={16} />
            </Button>
          </div>
        </div>
      </div>

      <div className="w-80 shrink-0 flex flex-col min-h-0">
        <Card className="flex-1 flex flex-col p-4 min-h-0">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <FileText size={16} /> 当前稿子
            </div>
            <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
              Draft v{draftVersion}
            </span>
          </div>

          <div className="flex-1 overflow-y-auto">
            {draft ? (
              <pre className="text-[13px] text-foreground whitespace-pre-wrap font-sans leading-relaxed">{draft}</pre>
            ) : (
              <p className="text-[13px] text-muted-foreground">等待 AI 生成稿子...</p>
            )}
          </div>

          <div className="pt-4 mt-4 border-t border-border space-y-2">
            <div className="space-y-2">
              <div>
                <Label className="text-xs">发布平台</Label>
                <p className="text-sm text-foreground mt-1 px-2 py-1.5 bg-muted rounded border border-[#e5e6eb]">
                  {platformLabel(platform)}
                </p>
              </div>
              <div>
                <Label className="text-xs">发布账号</Label>
                {lockedAccount ? (
                  <p className="text-sm text-foreground mt-1 px-2 py-1.5 bg-muted rounded border border-[#e5e6eb]">
                    {lockedAccount.masked_display}
                  </p>
                ) : (
                  <p className="text-xs text-muted-foreground mt-1">加载中...</p>
                )}
              </div>
              {novelNameLocked ? (
                <div>
                  <Label className="text-xs">书本名称</Label>
                  <p className="text-sm text-foreground mt-1 px-2 py-1.5 bg-muted rounded border border-[#e5e6eb]">
                    {novelName || "（未设置）"}
                  </p>
                </div>
              ) : (
                <Input value={novelName} onChange={(e) => setNovelName(e.target.value)}
                  placeholder="书本名称 (novelName)" className="text-xs" />
              )}
              <Input value={chapterTitle} onChange={(e) => setChapterTitle(e.target.value)}
                placeholder="章节标题 (title)" className="text-xs" />
              <div className="flex gap-2">
                <Input value={volumeName} onChange={(e) => setVolumeName(e.target.value)}
                  placeholder="卷名" className="text-xs flex-1" />
                <Input type="number" value={chapterNumber} onChange={(e) => setChapterNumber(Number(e.target.value))}
                  placeholder="章节号" className="text-xs w-20" />
              </div>
            </div>
            <Button onClick={handlePublish} className="w-full" size="md" disabled={!draft || publishState === "publishing" || !novelName}>
              {publishState === "done" ? <><CheckCircle size={16} />已发布</> :
               publishState === "partial" ? <><AlertCircle size={16} />部分失败</> :
               publishState === "publishing" ? <><Loader2 className="w-4 h-4 animate-spin" />发布中...</> :
               publishState === "error" ? <><AlertCircle size={16} />发布失败，重试</> :
                 <><Send size={16} />发布稿件</>}
            </Button>
            <p className="text-[11px] text-muted-foreground text-center">发布后将生成任务档案 MD</p>
          </div>
        </Card>
      </div>
    </div>
  )
}
