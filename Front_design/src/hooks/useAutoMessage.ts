import { useState, useEffect, useRef, useCallback } from "react"

export interface AppMessage {
  type: "success" | "error"
  text: string
}

/**
 * 带自动消失的消息状态 hook。
 * @param duration 显示时长（毫秒），默认 4000ms
 */
export function useAutoMessage(duration = 4000) {
  const [message, setMessageRaw] = useState<AppMessage | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const setMessage = useCallback((msg: AppMessage | null) => {
    if (timerRef.current) clearTimeout(timerRef.current)
    setMessageRaw(msg)
    if (msg) {
      timerRef.current = setTimeout(() => setMessageRaw(null), duration)
    }
  }, [duration])

  // 组件卸载时清除计时器
  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current) }, [])

  return { message, setMessage }
}
