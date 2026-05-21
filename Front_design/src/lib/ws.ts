import { getToken } from "./auth"
import type { WSEvent } from "@/types"

const WS_BASE =
  process.env.NEXT_PUBLIC_WS_BASE || ""

export function connectSessionWS(
  sessionId: string,
  onEvent: (event: WSEvent) => void,
  onError?: (err: Event) => void,
  onClose?: () => void
): WebSocket {
  const token = getToken()
  const url = `${WS_BASE}/ws/session/${sessionId}?token=${token || ""}`
  const ws = new WebSocket(url)

  ws.onmessage = (msg) => {
    try {
      const event: WSEvent = JSON.parse(msg.data)
      onEvent(event)
    } catch {
      // ignore parse errors
    }
  }

  ws.onerror = (err) => {
    onError?.(err)
  }

  ws.onclose = () => {
    onClose?.()
  }

  return ws
}

export function connectTaskWS(
  taskId: string,
  onEvent: (event: WSEvent) => void,
  onError?: (err: Event) => void,
  onClose?: () => void
): WebSocket {
  const token = getToken()
  const url = `${WS_BASE}/ws/task/${taskId}?token=${token || ""}`
  const ws = new WebSocket(url)

  ws.onmessage = (msg) => {
    try {
      const event: WSEvent = JSON.parse(msg.data)
      onEvent(event)
    } catch {
      // ignore
    }
  }

  ws.onerror = (err) => {
    onError?.(err)
  }

  ws.onclose = () => {
    onClose?.()
  }

  return ws
}
