import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatNumber(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1)}万`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

export function truncate(text: string, maxLen: number): string {
  return text.length > maxLen ? text.slice(0, maxLen) + "..." : text
}

/**
 * 将 ISO 时间字符串转为人性化相对描述：
 *   刚刚 / x分钟前 / 今天 HH:mm / 昨天 HH:mm / M月D日 HH:mm / YYYY年M月D日
 */
export function formatRelativeTime(dateStr: string): string {
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr

  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMin = Math.floor(diffMs / 60_000)
  const diffHour = Math.floor(diffMs / 3_600_000)

  if (diffMin < 1) return "刚刚"
  if (diffMin < 60) return `${diffMin}分钟前`

  const pad = (n: number) => String(n).padStart(2, "0")
  const hhmm = `${pad(d.getHours())}:${pad(d.getMinutes())}`

  // 判断是否同一天
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterdayStart = new Date(todayStart.getTime() - 86_400_000)
  const dStart = new Date(d.getFullYear(), d.getMonth(), d.getDate())

  if (dStart.getTime() === todayStart.getTime()) return `今天 ${hhmm}`
  if (dStart.getTime() === yesterdayStart.getTime()) return `昨天 ${hhmm}`

  // 同年只显示 M月D日
  if (d.getFullYear() === now.getFullYear()) {
    return `${d.getMonth() + 1}月${d.getDate()}日 ${hhmm}`
  }

  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`
}
