/**
 * Cookie 健康状态的客户端缓存（localStorage）
 *
 * 设计目标：
 *  - 避免每次打开页面都发起后端探测请求
 *  - 缓存 TTL 默认 1 小时；手动刷新时强制跳过缓存
 *  - 绑定/解绑账号后主动失效对应条目
 */

export type CookieHealthStatus = 'valid' | 'expired' | 'unknown'

/** 缓存中每条记录的结构 */
export interface CookieHealthCacheEntry {
  status: CookieHealthStatus
  /** 检测时间（ms timestamp） */
  checkedAt: number
  /**
   * 检测来源：
   *  - 'backend'    后端接口检测
   *  - 'heuristic'  本地 updated_at 时间估算（不准确，仅作兜底）
   */
  source: 'backend' | 'heuristic'
}

const PREFIX = 'ck_health_'

/** 默认缓存有效期：1 小时 */
export const DEFAULT_TTL_MS = 60 * 60 * 1000

/** 读取某账号的缓存条目；不存在或解析失败返回 null */
export function getCachedEntry(accountId: string): CookieHealthCacheEntry | null {
  try {
    const raw = localStorage.getItem(PREFIX + accountId)
    if (!raw) return null
    return JSON.parse(raw) as CookieHealthCacheEntry
  } catch {
    return null
  }
}

/** 写入缓存 */
export function setCachedEntry(accountId: string, entry: CookieHealthCacheEntry): void {
  try {
    localStorage.setItem(PREFIX + accountId, JSON.stringify(entry))
  } catch {
    // 存储配额满时静默忽略
  }
}

/** 判断缓存条目是否已过期（超过 TTL） */
export function isCacheStale(entry: CookieHealthCacheEntry, ttlMs = DEFAULT_TTL_MS): boolean {
  return Date.now() - entry.checkedAt > ttlMs
}

/** 删除某账号的缓存（解绑或重新绑定时调用） */
export function invalidateCache(accountId: string): void {
  try {
    localStorage.removeItem(PREFIX + accountId)
  } catch {
    // ignore
  }
}

/** 批量读取，返回 accountId → entry 的 map（只包含有缓存的条目） */
export function getBulkCached(
  accountIds: string[]
): Record<string, CookieHealthCacheEntry> {
  const result: Record<string, CookieHealthCacheEntry> = {}
  for (const id of accountIds) {
    const entry = getCachedEntry(id)
    if (entry) result[id] = entry
  }
  return result
}

/**
 * 基于 updated_at 做快速本地估算。
 *
 * 规则（保守）：
 *  - 7 天内更新      → valid（刚绑定/更新，大概率有效）
 *  - 7–60 天         → unknown（不确定，等待真实检测结果覆盖）
 *  - 60 天以上       → expired（大多数平台 Cookie 已到期）
 */
export function heuristicStatus(updatedAt: string): CookieHealthStatus {
  const ageMs = Date.now() - new Date(updatedAt).getTime()
  const days = ageMs / (1000 * 60 * 60 * 24)
  if (days <= 7) return 'valid'
  if (days <= 60) return 'unknown'
  return 'expired'
}
