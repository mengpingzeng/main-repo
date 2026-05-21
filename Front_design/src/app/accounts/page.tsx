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
import { formatDate } from "@/lib/utils"
import {
  Shield, Plus, Loader2, CheckCircle, AlertCircle, Wand2, XCircle, HelpCircle, RefreshCw, ExternalLink,
} from "lucide-react"

const PLATFORM_LABELS: Record<string, string> = {
  fanqie: "番茄小说",
  xhs: "小红书",
  wechat: "微信公众号",
  yuewen: "阅文",
  zhulang: "逐浪网",
}

// 扩展消息类型
type CaptureStatus = 'idle' | 'running' | 'done' | 'error'

/** UI 展示状态：checking 是前端独有的"进行中"状态，其余来自缓存 */
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
  // 注入状态：account_id → 'injecting' | 'done' | 'error'
  const [injectStatusMap, setInjectStatusMap] = useState<Record<string, 'injecting' | 'done' | 'error'>>({})
  const [page, setPageState] = useState(1)

  // Cookie 健康状态 map：account_id → DisplayStatus
  const [cookieStatusMap, setCookieStatusMap] = useState<Record<string, DisplayStatus>>({})
  // 自动获取 Cookie 相关状态
  const [captureStatus, setCaptureStatus] = useState<CaptureStatus>('idle')
  const [captureMessage, setCaptureMessage] = useState("")
  // 区分自动抓取结果回填到哪个弹窗（用 ref 避免 stale closure）
  const captureContextRef = useRef<'bind' | 'relogin'>('bind')
  const captureTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const setPage = (p: number) => {
    setPageState(p)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }
  const pageSize = 5

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

  /**
   * 初始化 Cookie 状态显示策略（缓存优先 + 乐观估算 + 按需检测）
   *
   * 逻辑：
   *  1. 先从 localStorage 批量读取缓存，有效缓存直接上屏，避免页面闪烁"检测中"
   *  2. 无缓存的账号 → 用 updated_at 做乐观估算立即显示，同时后台触发真实检测
   *  3. 有缓存但已过期（> 1h）→ 保留旧状态先显示，后台静默刷新，结果返回后覆盖
   *  4. 有缓存且新鲜 → 直接用缓存，不发任何请求
   */
  const initCookieStatus = useCallback((accs: AccountSummary[]) => {
    if (accs.length === 0) return

    const cached = getBulkCached(accs.map(a => a.account_id))
    const initial: Record<string, DisplayStatus> = {}
    const needsCheck: AccountSummary[] = []

    for (const acc of accs) {
      const entry = cached[acc.account_id]

      if (entry && !isCacheStale(entry)) {
        // 缓存新鲜：直接用，不发请求
        initial[acc.account_id] = entry.status
      } else if (entry && isCacheStale(entry)) {
        // 缓存过期：先用旧值展示，排入后台刷新队列
        initial[acc.account_id] = entry.status
        needsCheck.push(acc)
      } else {
        // 无缓存：用 updated_at 乐观估算作为初始值，排入检测队列
        initial[acc.account_id] = heuristicStatus(acc.updated_at)
        needsCheck.push(acc)
      }
    }

    setCookieStatusMap(initial)

    if (needsCheck.length > 0) {
      runHealthChecks(needsCheck)
    }
  }, [])

  /**
   * 对需要检测的账号并行发起后端探测。
   * 使用 Promise.allSettled 保证单个失败不影响其他。
   * 结果写入 localStorage 缓存并更新 UI 状态。
   */
  const runHealthChecks = useCallback(async (accs: AccountSummary[], markChecking = false) => {
    if (markChecking) {
      setCookieStatusMap(prev => {
        const next = { ...prev }
        for (const acc of accs) next[acc.account_id] = 'checking'
        return next
      })
    }

    const results = await Promise.allSettled(
      accs.map(acc => checkCookieHealth(acc.account_id))
    )

    setCookieStatusMap(prev => {
      const next = { ...prev }
      results.forEach((result, i) => {
        const id = accs[i].account_id
        const status: CookieHealthStatus =
          result.status === 'fulfilled'
            ? (result.value.valid ? 'valid' : 'expired')
            : 'unknown'
        next[id] = status
        // 写入 localStorage 缓存
        setCachedEntry(id, { status, checkedAt: Date.now(), source: 'backend' })
      })
      return next
    })
  }, [])

  /** 手动刷新单个账号的 Cookie 状态（强制跳过缓存） */
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
  useEffect(() => { setPage(1) }, [accountFilter])

  // 监听扩展通过 content.js 回传的消息
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.source !== window) return
      if (!event.data || typeof event.data !== 'object') return

      const { type, status, message: msg, cookieStr, username, cookieCount } = event.data

      // 收到任何回应，清除「未检测到扩展」的超时计时器
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
          setCaptureMessage(
            `已获取 ${cookieCount} 条 Cookie${username ? `，账号：${username}` : '，请填写显示名'}`
          )
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

        case 'FANQIE_INJECT_STATUS':
          // 注入进度消息，暂时只更新提示（通过 injectStatusMap 跟踪按钮状态）
          break
        case 'FANQIE_INJECT_DONE':
          // 注入完成，清除所有 injecting 状态
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

  // 弹窗关闭 / 平台切换时重置抓取状态
  useEffect(() => {
    if (!showBindModal) resetCaptureState()
  }, [showBindModal])

  useEffect(() => {
    if (!reLoginTarget) {
      setReLoginCredentials("")
      resetCaptureState()
    }
  }, [reLoginTarget])

  useEffect(() => {
    resetCaptureState()
  }, [platform])

  function resetCaptureState() {
    setCaptureStatus('idle')
    setCaptureMessage('')
    if (captureTimeoutRef.current) {
      clearTimeout(captureTimeoutRef.current)
      captureTimeoutRef.current = null
    }
  }

  // 点击「自动获取 Cookie」
  const handleAutoCapture = (context: 'bind' | 'relogin' = 'bind') => {
    captureContextRef.current = context
    setCaptureStatus('running')
    setCaptureMessage('正在连接扩展...')

    // 5 秒内没有收到扩展任何响应，提示安装扩展
    captureTimeoutRef.current = setTimeout(() => {
      setCaptureStatus('error')
      setCaptureMessage('未检测到「番茄账号管家」扩展，请安装并启用后重试')
    }, 5000)

    window.postMessage({ type: 'FANQIE_CAPTURE_START' }, '*')
  }

  const filteredAccounts = accountFilter ? accounts.filter((a) => a.platform === accountFilter) : accounts
  const total = filteredAccounts.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const pagedAccounts = filteredAccounts.slice((page - 1) * pageSize, page * pageSize)

  /** 渲染 Cookie 状态徽标 */
  function CookieStatusBadge({ accountId }: { accountId: string }) {
    const status = cookieStatusMap[accountId]
    const entry = getCachedEntry(accountId)
    const checkedText = entry
      ? `上次检测：${formatDate(new Date(entry.checkedAt).toISOString())}${entry.source === 'heuristic' ? '（估算）' : ''}`
      : ''

    const badge = (() => {
      if (status === 'checking') {
        return (
          <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
            <Loader2 size={11} className="animate-spin" /> 检测中
          </span>
        )
      }
      if (status === 'valid') {
        return (
          <span className="inline-flex items-center gap-1 text-xs font-medium text-[#22c55e]">
            <CheckCircle size={11} /> 正常
          </span>
        )
      }
      if (status === 'expired') {
        return (
          <span className="inline-flex items-center gap-1 text-xs font-medium text-destructive">
            <XCircle size={11} /> 已过期
          </span>
        )
      }
      return (
        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
          <HelpCircle size={11} /> 未知
        </span>
      )
    })()

    return (
      <span title={checkedText || undefined}>
        {badge}
      </span>
    )
  }

  const handleBind = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!credentials.trim()) return
    setBinding(true)
    setMessage(null)
    try {
      const resp = await bindAccount(platform, credentials.trim(), displayName.trim() || undefined)
      setMessage({ type: "success", text: resp.is_new_binding ? "绑定成功" : "凭证已更新" })
      // 凭证变更，旧的状态缓存不再有效
      invalidateCache(resp.account_id)
      setCredentials("")
      setDisplayName("")
      setShowBindModal(false)
      loadAccounts()
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "绑定失败" })
    } finally {
      setBinding(false)
    }
  }

  /** 从 Vault 取出 Cookie 并通过扩展注入浏览器，打开番茄写作者中心 */
  const handleOpenFanqie = async (acc: AccountSummary) => {
    setInjectStatusMap(prev => ({ ...prev, [acc.account_id]: 'injecting' }))
    try {
      const resp = await fetchAccountCredential(acc.account_id)
      window.postMessage({ type: 'FANQIE_INJECT_COOKIES', cookieStr: resp.credentials }, '*')
      // 10 秒超时兜底：若扩展未回传 DONE/ERROR，自动清除加载状态
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
      setTimeout(() => setInjectStatusMap(prev => {
        const next = { ...prev }
        delete next[acc.account_id]
        return next
      }), 3000)
    }
  }

  const handleReLogin = async (e: React.FormEvent) => {    e.preventDefault()
    if (!reLoginTarget || !reLoginCredentials.trim()) return
    setReLoginBinding(true)
    setMessage(null)
    try {
      const resp = await bindAccount(reLoginTarget.platform, reLoginCredentials.trim(), reLoginTarget.masked_display, reLoginTarget.account_id)
      invalidateCache(resp.account_id)
      setMessage({ type: "success", text: "Cookie 已更新，登录状态已恢复" })
      setReLoginTarget(null)
      loadAccounts()
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "更新失败，请重试" })
    } finally {
      setReLoginBinding(false)
    }
  }

  const handleUnbind = async (accountId: string) => {    try {
      await unbindAccount(accountId)
      invalidateCache(accountId)
      setMessage({ type: "success", text: "已解绑" })
      loadAccounts()
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "解绑失败" })
    }
  }

  return (
    <div className="w-full max-w-[1080px]">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-lg font-semibold text-foreground">账号配置</h1>
        <Button size="lg" onClick={() => setShowBindModal(true)}>
          <Plus size={16} /> 绑定账号
        </Button>
      </div>

      <div className="flex items-center gap-3 mb-4">
        <SelectRadix value={accountFilter || "all"} onValueChange={(v) => setAccountFilter(v === "all" ? "" : v)} className="w-[160px]">
          <SelectItem value="all">全部平台</SelectItem>
          <SelectItem value="fanqie">番茄小说</SelectItem>
          <SelectItem value="xhs">小红书</SelectItem>
          <SelectItem value="wechat">微信公众号</SelectItem>
          <SelectItem value="yuewen">阅文</SelectItem>
          <SelectItem value="zhulang">逐浪网</SelectItem>
        </SelectRadix>
      </div>

      {message && (
        <div className={`mb-4 p-3 rounded-lg text-sm flex items-center gap-2 ${
          message.type === "success"
            ? "bg-[#22c55e]/8 border border-[#22c55e]/20 text-[#22c55e]"
            : "bg-destructive/8 border border-destructive/20 text-destructive"
        }`}>
          {message.type === "success" ? <CheckCircle size={14} /> : <AlertCircle size={14} />}
          {message.text}
        </div>
      )}

      {/* ── 绑定账号弹窗 ── */}
      <Dialog open={showBindModal} onOpenChange={setShowBindModal}>
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>绑定新账号</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleBind} className="space-y-4">
            <div>
              <Label>平台</Label>
              <SelectRadix value={platform} onValueChange={setPlatform}>
                <SelectItem value="fanqie">番茄小说</SelectItem>
                <SelectItem value="xhs">小红书</SelectItem>
                <SelectItem value="wechat">微信公众号</SelectItem>
                <SelectItem value="yuewen">阅文</SelectItem>
                <SelectItem value="zhulang">逐浪网</SelectItem>
              </SelectRadix>
            </div>

            <div>
              <Label>显示名</Label>
              <Input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="平台账号名称"
              />
              <p className="text-xs text-muted-foreground mt-1">在控制台中显示的账号名称，方便识别</p>
            </div>

            <div>
              {/* Cookie 标签行：左边 Label，右边自动获取按钮（仅番茄小说显示） */}
              <div className="flex items-center justify-between mb-1">
                <Label>平台凭证 (Cookie)</Label>
                {platform === "fanqie" && (
                  <button
                    type="button"
                    onClick={() => handleAutoCapture('bind')}
                    disabled={captureStatus === 'running'}
                    className="inline-flex items-center gap-1.5 text-xs text-primary hover:text-primary/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    {captureStatus === 'running'
                      ? <Loader2 size={12} className="animate-spin" />
                      : <Wand2 size={12} />
                    }
                    自动获取 Cookie
                  </button>
                )}
              </div>

              {/* 番茄小说：显示抓取状态提示 */}
              {platform === "fanqie" && captureStatus !== 'idle' && (
                <div className={`flex items-center gap-1.5 text-xs mb-2 ${
                  captureStatus === 'error'
                    ? 'text-destructive'
                    : captureStatus === 'done'
                    ? 'text-[#22c55e]'
                    : 'text-muted-foreground'
                }`}>
                  {captureStatus === 'running' && <Loader2 size={11} className="animate-spin flex-shrink-0" />}
                  {captureStatus === 'done' && <CheckCircle size={11} className="flex-shrink-0" />}
                  {captureStatus === 'error' && <AlertCircle size={11} className="flex-shrink-0" />}
                  <span>{captureMessage}</span>
                </div>
              )}
              {/* 登录等待阶段的手动兜底按钮 */}
              {platform === "fanqie" && captureStatus === 'running' && captureMessage.includes('验证码登录') && (
                <button
                  type="button"
                  onClick={() => window.postMessage({ type: 'FANQIE_MANUAL_CAPTURE' }, '*')}
                  className="text-xs text-primary/70 hover:text-primary underline mb-2 block"
                >
                  已完成登录但页面未跳转？点此手动获取
                </button>
              )}

              <Textarea
                value={credentials}
                onChange={(e) => setCredentials(e.target.value)}
                placeholder="粘贴完整的 Cookie 字符串，或点击上方「自动获取 Cookie」..."
                rows={4}
              />
              <p className="text-xs text-muted-foreground mt-1">凭证将被加密存储，不会明文记录</p>
            </div>

            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setShowBindModal(false)}>取消</Button>
              <Button type="submit" disabled={binding}>
                {binding ? <><Loader2 className="w-4 h-4 animate-spin" />绑定中...</> : "确认绑定"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* ── 账号列表 ── */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-6 h-6 animate-spin text-primary" />
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-20">
          <AlertCircle className="w-12 h-12 mb-4 text-destructive opacity-40" />
          <p className="text-sm text-muted-foreground mb-1">{error}</p>
          <Button variant="ghost" size="sm" onClick={loadAccounts}>重试</Button>
        </div>
      ) : accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20">
          <div className="w-32 h-32 mb-5 rounded-lg border border-[#e5e6eb] bg-[#f7f8fa] flex items-center justify-center">
            <Shield className="w-12 h-12 text-[#c9cdd4]" />
          </div>
          <p className="text-sm text-muted-foreground mb-5">还没有绑定的账号</p>
          <Button size="lg" onClick={() => setShowBindModal(true)}><Plus size={16} /> 绑定账号</Button>
        </div>
      ) : pagedAccounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20">
          <p className="text-sm text-muted-foreground">该平台下暂无账号</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg border border-[#e5e6eb] overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full table-fixed">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-5 text-xs font-medium text-muted-foreground uppercase tracking-wider w-1/5">平台</th>
                  <th className="text-left py-3 px-5 text-xs font-medium text-muted-foreground uppercase tracking-wider w-1/5">账号名</th>
                  <th className="text-left py-3 px-5 text-xs font-medium text-muted-foreground uppercase tracking-wider w-1/5">登录状态</th>
                  <th className="text-left py-3 px-5 text-xs font-medium text-muted-foreground uppercase tracking-wider w-1/5">绑定时间</th>
                  <th className="py-3 px-5 text-xs font-medium text-muted-foreground uppercase tracking-wider w-1/5">
                    <div className="flex justify-end">操作</div>
                  </th>
                </tr>
              </thead>
              <tbody>
                {pagedAccounts.map((acc) => (
                  <tr key={acc.account_id} className="border-b border-border last:border-b-0 hover:bg-muted/50 transition-colors">
                    <td className="py-4 px-5">
                      <span className="text-sm font-medium text-foreground">
                        {PLATFORM_LABELS[acc.platform] || acc.platform}
                      </span>
                    </td>
                    <td className="py-4 px-5">
                      <span className="text-sm text-foreground">{acc.masked_display}</span>
                    </td>
                    <td className="py-4 px-5">
                      <div className="flex items-center gap-2 whitespace-nowrap">
                        <CookieStatusBadge accountId={acc.account_id} />
                        <button
                          title="重新检测"
                          onClick={() => recheckOne(acc.account_id)}
                          disabled={cookieStatusMap[acc.account_id] === 'checking'}
                          className="text-muted-foreground hover:text-foreground disabled:opacity-30 transition-colors"
                        >
                          <RefreshCw size={11} />
                        </button>
                      </div>
                    </td>
                    <td className="py-4 px-5">
                      <span className="text-sm text-muted-foreground">
                        {acc.bound_at ? formatDate(acc.bound_at) : "—"}
                      </span>
                    </td>
                    <td className="py-4 px-5">
                      <div className="flex justify-end items-center gap-0.5 whitespace-nowrap">
                        {cookieStatusMap[acc.account_id] === 'valid' && (
                          <Button
                            variant="ghost" size="sm"
                            onClick={() => handleOpenFanqie(acc)}
                            disabled={!!injectStatusMap[acc.account_id]}
                            className="text-primary hover:text-primary hover:bg-primary/8"
                          >
                            {injectStatusMap[acc.account_id] === 'injecting'
                              ? <><Loader2 size={12} className="animate-spin" />打开中...</>
                              : <><ExternalLink size={12} />打开番茄</>
                            }
                          </Button>
                        )}
                        {cookieStatusMap[acc.account_id] === 'expired' && (
                          <Button
                            variant="ghost" size="sm"
                            onClick={() => setReLoginTarget(acc)}
                            className="text-amber-500 hover:text-amber-600 hover:bg-amber-500/8"
                          >
                            重新登录
                          </Button>
                        )}
                        <Button
                          variant="ghost" size="sm"
                          onClick={() => setUnbindTarget(acc)}
                          className="text-destructive hover:text-destructive hover:bg-destructive/8 pr-0"
                        >
                          解绑
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {total > pageSize && (
        <div className="flex items-center justify-between mt-4 text-sm">
          <span className="text-[#86909c]">共 {total} 条</span>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              上一页
            </Button>
            <span className="text-[#86909c] px-2">{page} / {totalPages}</span>
            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              下一页
            </Button>
          </div>
        </div>
      )}

      {/* ── 重新登录弹窗 ── */}
      <Dialog open={!!reLoginTarget} onOpenChange={(open) => !open && setReLoginTarget(null)}>
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>重新登录</DialogTitle>
          </DialogHeader>

          {/* 账号信息提示 */}
          <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-50 border border-amber-100">
            <AlertCircle size={15} className="text-amber-500 flex-shrink-0 mt-0.5" />
            <div className="text-sm">
              <span className="font-medium text-foreground">
                {PLATFORM_LABELS[reLoginTarget?.platform || ""] || reLoginTarget?.platform}
              </span>
              <span className="text-muted-foreground"> · {reLoginTarget?.masked_display}</span>
              <p className="text-xs text-muted-foreground mt-0.5">当前 Cookie 已失效，请重新获取以恢复正常使用</p>
            </div>
          </div>

          <form onSubmit={handleReLogin} className="space-y-4">
            <div>
              <div className="flex items-center justify-between mb-1">
                <Label>平台凭证 (Cookie)</Label>
                {reLoginTarget?.platform === "fanqie" && (
                  <button
                    type="button"
                    onClick={() => handleAutoCapture('relogin')}
                    disabled={captureStatus === 'running'}
                    className="inline-flex items-center gap-1.5 text-xs text-primary hover:text-primary/80 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    {captureStatus === 'running'
                      ? <Loader2 size={12} className="animate-spin" />
                      : <Wand2 size={12} />
                    }
                    自动获取 Cookie
                  </button>
                )}
              </div>

              {reLoginTarget?.platform === "fanqie" && captureStatus !== 'idle' && (
                <div className={`flex items-center gap-1.5 text-xs mb-2 ${
                  captureStatus === 'error'
                    ? 'text-destructive'
                    : captureStatus === 'done'
                    ? 'text-[#22c55e]'
                    : 'text-muted-foreground'
                }`}>
                  {captureStatus === 'running' && <Loader2 size={11} className="animate-spin flex-shrink-0" />}
                  {captureStatus === 'done' && <CheckCircle size={11} className="flex-shrink-0" />}
                  {captureStatus === 'error' && <AlertCircle size={11} className="flex-shrink-0" />}
                  <span>{captureMessage}</span>
                </div>
              )}
              {/* 登录等待阶段的手动兜底按钮 */}
              {reLoginTarget?.platform === "fanqie" && captureStatus === 'running' && captureMessage.includes('验证码登录') && (
                <button
                  type="button"
                  onClick={() => window.postMessage({ type: 'FANQIE_MANUAL_CAPTURE' }, '*')}
                  className="text-xs text-primary/70 hover:text-primary underline mb-2 block"
                >
                  已完成登录但页面未跳转？点此手动获取
                </button>
              )}

              <Textarea
                value={reLoginCredentials}
                onChange={(e) => setReLoginCredentials(e.target.value)}
                placeholder="粘贴新的 Cookie 字符串，或点击上方「自动获取 Cookie」..."
                rows={4}
              />
              <p className="text-xs text-muted-foreground mt-1">凭证将被加密存储，仅用于替换当前失效的 Cookie</p>
            </div>

            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setReLoginTarget(null)}>取消</Button>
              <Button type="submit" disabled={reLoginBinding || !reLoginCredentials.trim()}>
                {reLoginBinding ? <><Loader2 className="w-4 h-4 animate-spin" />更新中...</> : "确认更新"}
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
          <p className="text-sm text-muted-foreground">
            确定要解绑账号 <span className="text-foreground font-medium">
              {PLATFORM_LABELS[unbindTarget?.platform || ""] || unbindTarget?.platform} · {unbindTarget?.masked_display}
            </span> 吗？
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setUnbindTarget(null)}>取消</Button>
            <Button variant="destructive" onClick={async () => {
              if (!unbindTarget) return
              await handleUnbind(unbindTarget.account_id)
              if (pagedAccounts.length === 1 && page > 1) setPage(page - 1)
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
