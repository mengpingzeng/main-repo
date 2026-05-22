"use client"

import { useEffect, useState, useCallback, useRef } from "react"
import { useAutoMessage } from "@/hooks/useAutoMessage"
import { Button } from "@/components/ui/button"
import { Label, Textarea, Input } from "@/components/ui/input"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { fetchAccounts, bindAccount, unbindAccount, checkCookieHealth, fetchAccountCredential } from "@/lib/api"
import {
  getCachedEntry,
  setCachedEntry,
  isCacheStale,
  invalidateCache,
  getBulkCached,
  heuristicStatus,
  type CookieHealthStatus,
} from "@/lib/cookie-health-cache"
import type { AccountSummary } from "@/types"
import { formatDate, formatRelativeTime } from "@/lib/utils"
import {
  Shield, Plus, Loader2, CheckCircle, AlertCircle, RefreshCw, ExternalLink, LogIn,
} from "lucide-react"

const PLATFORM_LABELS: Record<string, string> = {
  fanqie: "番茄小说",
  xhs: "小红书",
  wechat: "微信公众号",
  yuewen: "阅文",
  zhulang: "逐浪网",
}

const PLATFORM_ORDER = ["fanqie", "zhulang", "xhs", "wechat", "yuewen"]

/** 各平台卡片图标配色 */
const PLATFORM_ICON: Record<string, { bg: string; text: string; char: string }> = {
  fanqie: { bg: "bg-red-50",    text: "text-red-500",    char: "番" },
  zhulang: { bg: "bg-blue-50",  text: "text-blue-500",   char: "逐" },
  xhs:     { bg: "bg-rose-100", text: "text-rose-600",   char: "红" },
  wechat:  { bg: "bg-green-50", text: "text-green-600",  char: "微" },
  yuewen:  { bg: "bg-purple-50",text: "text-purple-500", char: "阅" },
}

type CaptureStatus = 'idle' | 'running' | 'done' | 'error'
type DisplayStatus = CookieHealthStatus | 'checking'

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<AccountSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [showBindModal, setShowBindModal] = useState(false)
  const [platform, setPlatform] = useState("fanqie")
  const [displayName, setDisplayName] = useState("")
  const [credentials, setCredentials] = useState("")
  const [binding, setBinding] = useState(false)
  const { message, setMessage } = useAutoMessage()
  const [accountFilter, setAccountFilter] = useState("")
  const [unbindTarget, setUnbindTarget] = useState<AccountSummary | null>(null)
  const [reLoginTarget, setReLoginTarget] = useState<AccountSummary | null>(null)
  const [reLoginCredentials, setReLoginCredentials] = useState("")
  const [reLoginBinding, setReLoginBinding] = useState(false)
  const [bindDialogError, setBindDialogError] = useState<string | null>(null)
  const [reLoginDialogError, setReLoginDialogError] = useState<string | null>(null)
  const [injectStatusMap, setInjectStatusMap] = useState<Record<string, 'injecting' | 'done' | 'error'>>({})

  const [cookieStatusMap, setCookieStatusMap] = useState<Record<string, DisplayStatus>>({})
  const [captureStatus, setCaptureStatus] = useState<CaptureStatus>('idle')
  const [captureMessage, setCaptureMessage] = useState("")
  const captureContextRef = useRef<'bind' | 'relogin'>('bind')
  const captureTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const loadAccounts = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const accs = await fetchAccounts()
      setAccounts(accs)
      initCookieStatus(accs)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [])

  const initCookieStatus = useCallback((accs: AccountSummary[]) => {
    if (accs.length === 0) return
    const cached = getBulkCached(accs.map(a => a.account_id))
    const initial: Record<string, DisplayStatus> = {}
    const needsCheck: AccountSummary[] = []
    for (const acc of accs) {
      const entry = cached[acc.account_id]
      if (entry && !isCacheStale(entry)) {
        initial[acc.account_id] = entry.status
      } else if (entry && isCacheStale(entry)) {
        initial[acc.account_id] = entry.status
        needsCheck.push(acc)
      } else {
        initial[acc.account_id] = heuristicStatus(acc.updated_at)
        needsCheck.push(acc)
      }
    }
    setCookieStatusMap(initial)
    if (needsCheck.length > 0) runHealthChecks(needsCheck)
  }, [])

  const runHealthChecks = useCallback(async (accs: AccountSummary[], markChecking = false) => {
    if (markChecking) {
      setCookieStatusMap(prev => {
        const next = { ...prev }
        for (const acc of accs) next[acc.account_id] = 'checking'
        return next
      })
    }
    const results = await Promise.allSettled(accs.map(acc => checkCookieHealth(acc.account_id)))
    setCookieStatusMap(prev => {
      const next = { ...prev }
      results.forEach((result, i) => {
        const id = accs[i].account_id
        const status: CookieHealthStatus =
          result.status === 'fulfilled' ? (result.value.valid ? 'valid' : 'expired') : 'unknown'
        next[id] = status
        setCachedEntry(id, { status, checkedAt: Date.now(), source: 'backend' })
      })
      return next
    })
  }, [])

  const recheckOne = useCallback(async (accountId: string) => {
    setCookieStatusMap(prev => ({ ...prev, [accountId]: 'checking' }))
    try {
      const res = await checkCookieHealth(accountId)
      const status: CookieHealthStatus = res.valid ? 'valid' : 'expired'
      setCookieStatusMap(prev => ({ ...prev, [accountId]: status }))
      setCachedEntry(accountId, { status, checkedAt: Date.now(), source: 'backend' })
    } catch {
      setCookieStatusMap(prev => ({ ...prev, [accountId]: 'unknown' }))
      setCachedEntry(accountId, { status: 'unknown', checkedAt: Date.now(), source: 'backend' })
    }
  }, [])

  useEffect(() => { loadAccounts() }, [loadAccounts])

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.source !== window) return
      if (!event.data || typeof event.data !== 'object') return
      const { type, status, message: msg, cookieStr, username, cookieCount } = event.data
      if (captureTimeoutRef.current) {
        clearTimeout(captureTimeoutRef.current)
        captureTimeoutRef.current = null
      }
      switch (type) {
        case 'FANQIE_CAPTURE_STATUS':
          setCaptureStatus(status === 'busy' ? 'error' : 'running')
          setCaptureMessage(msg || '')
          break
        case 'FANQIE_CAPTURE_RESULT':
          setCaptureStatus('done')
          setCaptureMessage(`已获取 ${cookieCount} 条 Cookie${username ? `，账号：${username}` : '，请填写显示名'}`)
          if (captureContextRef.current === 'relogin') {
            setReLoginCredentials(cookieStr || '')
          } else {
            setCredentials(cookieStr || '')
            if (username) setDisplayName(username)
          }
          break
        case 'FANQIE_CAPTURE_ERROR':
          setCaptureStatus('error')
          setCaptureMessage(msg || '抓取失败，请重试')
          break
        case 'FANQIE_INJECT_DONE':
          setInjectStatusMap({})
          break
        case 'FANQIE_INJECT_ERROR':
          setInjectStatusMap({})
          setMessage({ type: 'error', text: msg || 'Cookie 注入失败，请重试' })
          break
      }
    }
    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [])

  useEffect(() => {
    if (!showBindModal) {
      resetCaptureState()
      setPlatform("fanqie")
      setCredentials("")
      setDisplayName("")
      setBindDialogError(null)
    }
  }, [showBindModal])
  useEffect(() => { if (!reLoginTarget) { setReLoginCredentials(""); resetCaptureState() } }, [reLoginTarget])
  useEffect(() => { resetCaptureState() }, [platform])

  function resetCaptureState() {
    setCaptureStatus('idle')
    setCaptureMessage('')
    if (captureTimeoutRef.current) { clearTimeout(captureTimeoutRef.current); captureTimeoutRef.current = null }
  }

  const handleAutoCapture = (context: 'bind' | 'relogin' = 'bind') => {
    captureContextRef.current = context
    setCaptureStatus('running')
    setCaptureMessage('正在连接扩展...')
    captureTimeoutRef.current = setTimeout(() => {
      setCaptureStatus('error')
      setCaptureMessage('未检测到「番茄账号管家」扩展，请安装并启用后重试')
    }, 5000)
    window.postMessage({ type: 'FANQIE_CAPTURE_START', platform }, '*')
  }

  const handleBind = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!credentials.trim()) return
    setBinding(true)
    setBindDialogError(null)
    try {
      const resp = await bindAccount(platform, credentials.trim(), displayName.trim() || undefined)
      invalidateCache(resp.account_id)
      setCredentials("")
      setDisplayName("")
      setShowBindModal(false)
      setMessage({ type: "success", text: resp.is_new_binding ? "绑定成功" : "凭证已更新" })
      loadAccounts()
    } catch (err) {
      setBindDialogError(err instanceof Error ? err.message : "绑定失败")
    } finally {
      setBinding(false)
    }
  }

  const handleOpenFanqie = async (acc: AccountSummary) => {
    setInjectStatusMap(prev => ({ ...prev, [acc.account_id]: 'injecting' }))
    try {
      const resp = await fetchAccountCredential(acc.account_id)
      window.postMessage({ type: 'FANQIE_INJECT_COOKIES', cookieStr: resp.credentials, platform: acc.platform }, '*')
      setTimeout(() => {
        setInjectStatusMap(prev => {
          if (prev[acc.account_id] === 'injecting') {
            const next = { ...prev }
            delete next[acc.account_id]
            return next
          }
          return prev
        })
      }, 10000)
    } catch (err) {
      setInjectStatusMap(prev => ({ ...prev, [acc.account_id]: 'error' }))
      setMessage({ type: 'error', text: err instanceof Error ? err.message : '获取凭证失败，请重试' })
      setTimeout(() => setInjectStatusMap(prev => { const next = { ...prev }; delete next[acc.account_id]; return next }), 3000)
    }
  }

  const handleReLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!reLoginTarget || !reLoginCredentials.trim()) return
    setReLoginBinding(true)
    setReLoginDialogError(null)
    try {
      const resp = await bindAccount(reLoginTarget.platform, reLoginCredentials.trim(), reLoginTarget.masked_display, reLoginTarget.account_id)
      invalidateCache(resp.account_id)
      setReLoginTarget(null)
      setMessage({ type: "success", text: "Cookie 已更新，登录状态已恢复" })
      loadAccounts()
    } catch (err) {
      setReLoginDialogError(err instanceof Error ? err.message : "更新失败，请重试")
    } finally {
      setReLoginBinding(false)
    }
  }

  const handleUnbind = async (accountId: string) => {
    try {
      await unbindAccount(accountId)
      invalidateCache(accountId)
      setMessage({ type: "success", text: "已解绑" })
      loadAccounts()
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "解绑失败" })
    }
  }

  const filteredAccounts = accountFilter ? accounts.filter(a => a.platform === accountFilter) : accounts

  // 按平台分组
  const grouped = PLATFORM_ORDER.reduce<Record<string, AccountSummary[]>>((acc, p) => {
    const list = filteredAccounts.filter(a => a.platform === p)
    if (list.length > 0) acc[p] = list
    return acc
  }, {})
  // 有账号但平台不在预设列表中的也收录
  filteredAccounts.forEach(a => {
    if (!PLATFORM_ORDER.includes(a.platform) && !grouped[a.platform]) {
      grouped[a.platform] = filteredAccounts.filter(x => x.platform === a.platform)
    }
  })

  /** Cookie 状态徽标 */
  function StatusBadge({ accountId }: { accountId: string }) {
    const status = cookieStatusMap[accountId]
    const entry = getCachedEntry(accountId)
    const title = entry
      ? `上次检测：${formatDate(new Date(entry.checkedAt).toISOString())}${entry.source === 'heuristic' ? '（估算）' : ''}`
      : ''

    if (status === 'checking') {
      return (
        <span className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-slate-500 bg-slate-50 border border-slate-200 rounded-md" title={title}>
          <Loader2 size={10} className="animate-spin" /> 检测中
        </span>
      )
    }
    if (status === 'valid') {
      return (
        <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium text-emerald-600 bg-emerald-50 border border-emerald-100 rounded-md" title={title}>
          状态: 有效
        </span>
      )
    }
    if (status === 'expired') {
      return (
        <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium text-rose-600 bg-rose-50 border border-rose-200 rounded-md" title={title}>
          状态: 失效
        </span>
      )
    }
    return (
      <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium text-slate-400 bg-slate-50 border border-slate-200 rounded-md" title={title}>
        状态: 未知
      </span>
    )
  }

  return (
    <div className="max-w-7xl mx-auto px-6 pt-6">
      {/* ── 页面头部 ── */}
      <div className="flex justify-between items-end mb-8">
        <div>
          <h1 className="text-3xl font-bold text-slate-900 tracking-tight">发布账号配置</h1>
          <p className="text-slate-500 mt-1 text-sm">管理各平台用于分发的账号，通过 KMS 安全保管</p>
        </div>
        <button
          onClick={() => setShowBindModal(true)}
          className="px-4 py-2 bg-slate-900 text-white text-sm font-medium rounded-lg hover:bg-slate-800 transition-colors shadow-sm flex items-center gap-1.5"
        >
          <Plus size={15} />
          绑定新账号
        </button>
      </div>

      {/* ── 全局消息条 ── */}
      {message && (
        <div className={`mb-6 p-3 rounded-lg text-sm flex items-center gap-2 ${
          message.type === "success"
            ? "bg-emerald-50 border border-emerald-100 text-emerald-700"
            : "bg-red-50 border border-red-100 text-red-600"
        }`}>
          {message.type === "success" ? <CheckCircle size={14} /> : <AlertCircle size={14} />}
          {message.text}
        </div>
      )}

      {/* ── 平台过滤 Tab ── */}
      <div className="flex items-center gap-1 mb-6 bg-slate-100 rounded-lg p-1 w-fit">
        {[
          { value: "",        label: "全部平台" },
          { value: "fanqie",  label: "番茄小说" },
          { value: "zhulang", label: "逐浪网"   },
        ].map(tab => (
          <button
            key={tab.value}
            onClick={() => setAccountFilter(tab.value)}
            className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all ${
              accountFilter === tab.value
                ? "bg-white text-slate-900 shadow-sm"
                : "text-slate-500 hover:text-slate-700"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* ── 内容区 ── */}
      {loading ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="w-7 h-7 animate-spin text-orange-500" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-24">
          <AlertCircle className="w-12 h-12 mb-4 text-red-300" />
          <p className="text-sm text-slate-500 mb-3">{error}</p>
          <button onClick={loadAccounts} className="text-sm text-orange-600 hover:underline">重试</button>
        </div>
      ) : accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24">
          <div className="w-20 h-20 mb-5 rounded-2xl border border-slate-200 bg-slate-50 flex items-center justify-center">
            <Shield className="w-9 h-9 text-slate-300" />
          </div>
          <p className="text-base font-medium text-slate-400">还没有绑定的账号</p>
          <p className="text-sm text-slate-300 mt-1 mb-6">绑定后可在任务中选择对应平台账号分发内容</p>
          <button
            onClick={() => setShowBindModal(true)}
            className="px-4 py-2 bg-slate-900 text-white text-sm font-medium rounded-lg hover:bg-slate-800 transition-colors flex items-center gap-1.5"
          >
            <Plus size={15} /> 绑定新账号
          </button>
        </div>
      ) : filteredAccounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24">
          <p className="text-sm text-slate-400">该平台下暂无账号</p>
        </div>
      ) : (
        <div className="space-y-8">
          {Object.entries(grouped).map(([plt, accs]) => {
            const icon = PLATFORM_ICON[plt] ?? { bg: "bg-slate-100", text: "text-slate-500", char: plt[0]?.toUpperCase() ?? "?" }
            return (
              <div key={plt}>
                {/* 平台分组标题 */}
                <h3 className="text-sm font-bold text-slate-400 uppercase tracking-wider mb-4">
                  {PLATFORM_LABELS[plt] || plt}
                </h3>

                {/* 卡片网格 */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                  {accs.map(acc => {
                    const status = cookieStatusMap[acc.account_id]
                    const isExpired = status === 'expired'
                    const entry = getCachedEntry(acc.account_id)

                    return (
                      <div
                        key={acc.account_id}
                        className={`bg-white p-5 rounded-2xl border shadow-sm flex flex-col transition-colors ${
                          isExpired ? "border-rose-200 bg-rose-50/20" : "border-slate-200"
                        }`}
                      >
                        {/* 卡头：图标 + 状态徽标 */}
                        <div className="flex justify-between items-start mb-3">
                          <div className={`w-10 h-10 ${icon.bg} ${icon.text} rounded-lg flex items-center justify-center font-bold text-base flex-shrink-0`}>
                            {icon.char}
                          </div>
                          <StatusBadge accountId={acc.account_id} />
                        </div>

                        {/* 账号名 + 打开（同行） */}
                        <div className="flex items-center justify-between gap-3 mb-5">
                          <h4 className="text-base font-bold text-slate-900 leading-snug line-clamp-2 break-all min-w-0">
                            {acc.masked_display}
                          </h4>
                          {status === 'valid' && (
                            <button
                              onClick={() => handleOpenFanqie(acc)}
                              disabled={!!injectStatusMap[acc.account_id]}
                              className="flex-shrink-0 inline-flex items-center justify-center w-9 h-9 rounded-full bg-orange-500 hover:bg-orange-600 text-white disabled:opacity-50 transition-colors shadow-sm shadow-orange-200"
                            >
                              {injectStatusMap[acc.account_id] === 'injecting'
                                ? <Loader2 size={15} className="animate-spin" />
                                : <ExternalLink size={15} />
                              }
                            </button>
                          )}
                          {isExpired && (
                            <span className="flex-shrink-0 text-xs text-rose-400 pt-0.5"></span>
                          )}
                        </div>

                        {/* 卡底：上次检测 + 操作 */}
                        <div className={`mt-auto pt-4 border-t flex justify-between items-center ${isExpired ? "border-rose-100" : "border-slate-100"}`}>
                          {/* 上次检测时间 + 刷新 */}
                          <div className="flex items-center gap-1.5">
                            <span className={`text-xs ${isExpired ? "text-rose-300" : "text-slate-400"}`}>
                              {entry
                                ? `上次检测：${formatRelativeTime(new Date(entry.checkedAt).toISOString())}`
                                : "等待检测"
                              }
                            </span>
                            <button
                              title="重新检测"
                              onClick={() => recheckOne(acc.account_id)}
                              disabled={status === 'checking'}
                              className="text-slate-300 hover:text-slate-500 disabled:opacity-30 transition-colors"
                            >
                              <RefreshCw size={11} />
                            </button>
                          </div>

                          {/* 操作按钮组 */}
                          <div className="flex items-center gap-2">
                            {isExpired && (
                              <button
                                onClick={() => setReLoginTarget(acc)}
                                className="px-3 py-1.5 text-xs font-medium text-white bg-rose-500 hover:bg-rose-600 rounded-md transition-colors"
                              >
                                重新登录
                              </button>
                            )}
                            <button
                              onClick={() => setUnbindTarget(acc)}
                              className="text-xs font-medium text-red-400 hover:text-red-600 transition-colors"
                            >
                              解绑
                            </button>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* ── 绑定账号弹窗 ── */}
      <Dialog open={showBindModal} onOpenChange={(open) => { setShowBindModal(open); if (!open) setBindDialogError(null) }}>
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>绑定新账号</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleBind} className="space-y-5">

            {/* 平台选择 — 成功后锁定 */}
            <div>
              <p className="text-sm font-medium text-slate-700 mb-2.5">选择平台</p>
              <div className={`grid grid-cols-2 gap-3 ${captureStatus === 'done' ? "opacity-50 pointer-events-none" : ""}`}>
                {[
                  { value: "fanqie",  label: "番茄小说", bg: "bg-red-50",  text: "text-red-500",  char: "番" },
                  { value: "zhulang", label: "逐浪网",   bg: "bg-blue-50", text: "text-blue-500", char: "逐" },
                ].map(opt => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setPlatform(opt.value)}
                    className={`flex items-center gap-3 p-3 rounded-xl border-2 transition-all text-left ${
                      platform === opt.value
                        ? "border-orange-400 bg-orange-50"
                        : "border-slate-200 bg-white hover:border-slate-300"
                    }`}
                  >
                    <div className={`w-9 h-9 ${opt.bg} ${opt.text} rounded-lg flex items-center justify-center font-bold text-sm flex-shrink-0`}>
                      {opt.char}
                    </div>
                    <span className={`text-sm font-medium ${platform === opt.value ? "text-orange-700" : "text-slate-700"}`}>
                      {opt.label}
                    </span>
                    {platform === opt.value && (
                      <span className="ml-auto w-4 h-4 rounded-full bg-orange-500 flex items-center justify-center flex-shrink-0">
                        <svg width="8" height="8" viewBox="0 0 10 10" fill="none">
                          <path d="M2 5.5l2.5 2.5 4-4" stroke="white" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"/>
                        </svg>
                      </span>
                    )}
                  </button>
                ))}
              </div>
            </div>

            {/* 前往登录 / 已完成 */}
            <div>
              {captureStatus === 'done' ? (
                /* 成功态：显示已完成 + 重新获取链接 */
                <div className="flex items-center justify-between px-3 py-2.5 rounded-lg bg-emerald-50 border border-emerald-100">
                  <div className="flex items-center gap-2 text-sm font-medium text-emerald-600">
                    <CheckCircle size={15} />
                    登录完成，凭证已获取
                  </div>
                  <button
                    type="button"
                    onClick={() => { resetCaptureState(); setCredentials(""); setDisplayName("") }}
                    className="text-xs text-slate-400 hover:text-orange-500 underline underline-offset-2 transition-colors"
                  >
                    重新获取
                  </button>
                </div>
              ) : (
                /* 默认态 / 进行态 */
                <>
                  <button
                    type="button"
                    onClick={() => handleAutoCapture('bind')}
                    disabled={captureStatus === 'running'}
                    className="w-full flex items-center justify-center gap-2 py-2.5 rounded-lg border-2 border-dashed border-orange-300 bg-orange-50 text-orange-600 hover:bg-orange-100 hover:border-orange-400 disabled:opacity-60 disabled:cursor-not-allowed transition-all text-sm font-medium"
                  >
                    {captureStatus === 'running'
                      ? <><Loader2 size={14} className="animate-spin" />正在连接中...</>
                      : <><LogIn size={14} />前往登录</>
                    }
                  </button>

                  {/* 等待登录阶段：引导提示卡 */}
                  {captureStatus === 'running' && captureMessage.includes('登录') && (
                    <div className="mt-3 rounded-xl bg-slate-50 border border-slate-200 p-4">
                      <div className="flex gap-3">
                        <div className="flex flex-col items-center gap-1 pt-0.5">
                          <div className="w-5 h-5 rounded-full bg-orange-500 flex items-center justify-center flex-shrink-0">
                            <Loader2 size={11} className="text-white animate-spin" />
                          </div>
                          <div className="w-px flex-1 bg-slate-200 min-h-[24px]" />
                        </div>
                        <div className="pb-2">
                          <p className="text-sm font-medium text-slate-800 leading-snug">
                            请在弹出的窗口中完成登录
                          </p>
                          <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                            登录成功后，程序将自动识别并保存凭证，页面会自动关闭。<br />
                            <span className="text-orange-400 font-medium">请勿手动关闭登录窗口。</span>
                          </p>
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => window.postMessage({ type: 'FANQIE_MANUAL_CAPTURE', platform }, '*')}
                        className="mt-2 text-xs text-slate-400 hover:text-orange-500 underline underline-offset-2 transition-colors block"
                      >
                        已登录但窗口未关闭？点此手动触发
                      </button>
                    </div>
                  )}
                </>
              )}
            </div>

            {/* 抓取结果展示区 */}
            {captureStatus === 'done' && credentials && (
              <div className="rounded-xl bg-emerald-50 border border-emerald-100 p-4 space-y-2">
                <div className="flex items-center gap-2 mb-1">
                  <CheckCircle size={14} className="text-emerald-500 flex-shrink-0" />
                  <span className="text-sm font-medium text-emerald-700">获取成功</span>
                </div>
                {displayName && (
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-slate-400 w-16 flex-shrink-0">账号名</span>
                    <span className="text-sm text-slate-900 font-medium truncate">{displayName}</span>
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <span className="text-xs text-slate-400 w-16 flex-shrink-0">凭证</span>
                  <span className="text-sm text-slate-600">已加密保存，共 {credentials.split(';').filter(Boolean).length} 条</span>
                </div>
              </div>
            )}

            {captureStatus === 'error' && (
              <div className="flex items-start gap-2 rounded-xl bg-red-50 border border-red-100 p-3">
                <AlertCircle size={14} className="text-red-400 flex-shrink-0 mt-0.5" />
                <p className="text-xs text-red-600">{captureMessage || '获取失败，请重试'}</p>
              </div>
            )}

            <DialogFooter>
              {bindDialogError && (
                <p className="flex items-center gap-1.5 text-xs text-red-500 mr-auto">
                  <AlertCircle size={13} />{bindDialogError}
                </p>
              )}
              <Button type="button" variant="ghost" onClick={() => setShowBindModal(false)}>取消</Button>
              <Button type="submit" disabled={binding || !credentials.trim()}>
                {binding ? <><Loader2 className="w-4 h-4 animate-spin" />提交中...</> : "完成"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* ── 重新登录弹窗 ── */}
      <Dialog open={!!reLoginTarget} onOpenChange={(open) => { if (!open) { setReLoginTarget(null); setReLoginDialogError(null) } }}>
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>重新登录</DialogTitle>
          </DialogHeader>

          {/* 账号信息提示条 */}
          <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 border border-amber-100">
            <AlertCircle size={15} className="text-amber-500 flex-shrink-0 mt-0.5" />
            <div className="text-sm">
              <span className="font-medium text-slate-900">{PLATFORM_LABELS[reLoginTarget?.platform || ""] || reLoginTarget?.platform}</span>
              <span className="text-slate-500"> · {reLoginTarget?.masked_display}</span>
              <p className="text-xs text-slate-400 mt-0.5">登录态已失效，请重新登录以恢复正常使用</p>
            </div>
          </div>

          <form onSubmit={handleReLogin} className="space-y-4">

            {/* 前往登录 / 已完成 */}
            <div>
              {captureStatus === 'done' ? (
                <div className="flex items-center justify-between px-3 py-2.5 rounded-lg bg-emerald-50 border border-emerald-100">
                  <div className="flex items-center gap-2 text-sm font-medium text-emerald-600">
                    <CheckCircle size={15} />
                    登录完成，凭证已获取
                  </div>
                  <button
                    type="button"
                    onClick={() => { resetCaptureState(); setReLoginCredentials("") }}
                    className="text-xs text-slate-400 hover:text-orange-500 underline underline-offset-2 transition-colors"
                  >
                    重新获取
                  </button>
                </div>
              ) : (
                <>
                  <button
                    type="button"
                    onClick={() => handleAutoCapture('relogin')}
                    disabled={captureStatus === 'running'}
                    className="w-full flex items-center justify-center gap-2 py-2.5 rounded-lg border-2 border-dashed border-orange-300 bg-orange-50 text-orange-600 hover:bg-orange-100 hover:border-orange-400 disabled:opacity-60 disabled:cursor-not-allowed transition-all text-sm font-medium"
                  >
                    {captureStatus === 'running'
                      ? <><Loader2 size={14} className="animate-spin" />正在连接中...</>
                      : <><LogIn size={14} />前往登录</>
                    }
                  </button>

                  {/* 等待登录阶段：引导提示卡 */}
                  {captureStatus === 'running' && captureMessage.includes('登录') && (
                    <div className="mt-3 rounded-xl bg-slate-50 border border-slate-200 p-4">
                      <div className="flex gap-3">
                        <div className="flex flex-col items-center gap-1 pt-0.5">
                          <div className="w-5 h-5 rounded-full bg-orange-500 flex items-center justify-center flex-shrink-0">
                            <Loader2 size={11} className="text-white animate-spin" />
                          </div>
                          <div className="w-px flex-1 bg-slate-200 min-h-[24px]" />
                        </div>
                        <div className="pb-2">
                          <p className="text-sm font-medium text-slate-800 leading-snug">请在弹出的窗口中完成登录</p>
                          <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                            登录成功后，程序将自动识别并保存凭证，页面会自动关闭。<br />
                            <span className="text-orange-400 font-medium">请勿手动关闭登录窗口。</span>
                          </p>
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => window.postMessage({ type: 'FANQIE_MANUAL_CAPTURE', platform: reLoginTarget!.platform }, '*')}
                        className="mt-2 text-xs text-slate-400 hover:text-orange-500 underline underline-offset-2 transition-colors block"
                      >
                        已登录但窗口未关闭？点此手动触发
                      </button>
                    </div>
                  )}

                  {/* 失败提示 */}
                  {captureStatus === 'error' && (
                    <div className="flex items-start gap-2 mt-3 rounded-xl bg-red-50 border border-red-100 p-3">
                      <AlertCircle size={14} className="text-red-400 flex-shrink-0 mt-0.5" />
                      <p className="text-xs text-red-600">{captureMessage || '获取失败，请重试'}</p>
                    </div>
                  )}
                </>
              )}
            </div>

            {/* 成功后展示账号信息 */}
            {captureStatus === 'done' && reLoginCredentials && (
              <div className="rounded-xl bg-emerald-50 border border-emerald-100 p-4 space-y-2">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-slate-400 w-16 flex-shrink-0">账号名</span>
                  <span className="text-sm text-slate-900 font-medium truncate">{reLoginTarget?.masked_display}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-slate-400 w-16 flex-shrink-0">凭证</span>
                  <span className="text-sm text-slate-600">已加密保存，共 {reLoginCredentials.split(';').filter(Boolean).length} 条</span>
                </div>
              </div>
            )}

            <DialogFooter>
              {reLoginDialogError && (
                <p className="flex items-center gap-1.5 text-xs text-red-500 mr-auto">
                  <AlertCircle size={13} />{reLoginDialogError}
                </p>
              )}
              <Button type="button" variant="ghost" onClick={() => setReLoginTarget(null)}>取消</Button>
              <Button type="submit" disabled={reLoginBinding || !reLoginCredentials.trim()}>
                {reLoginBinding ? <><Loader2 className="w-4 h-4 animate-spin" />提交中...</> : "完成"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* ── 解绑确认弹窗 ── */}
      <Dialog open={!!unbindTarget} onOpenChange={(open) => !open && setUnbindTarget(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>确认解绑</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-slate-500">
            确定要解绑账号{" "}
            <span className="text-slate-900 font-medium">
              {PLATFORM_LABELS[unbindTarget?.platform || ""] || unbindTarget?.platform} · {unbindTarget?.masked_display}
            </span>{" "}
            吗？
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setUnbindTarget(null)}>取消</Button>
            <Button variant="destructive" onClick={async () => {
              if (!unbindTarget) return
              await handleUnbind(unbindTarget.account_id)
              setUnbindTarget(null)
            }}>
              确认解绑
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
