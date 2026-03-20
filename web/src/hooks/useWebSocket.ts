import { useEffect, useRef, useCallback } from 'react'
import type { WsFrame } from '../types'

interface Options {
  token: string
  onFrame: (frame: WsFrame) => void
}

export function useWebSocket({ token, onFrame }: Options) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<number>(0)
  const onFrameRef = useRef(onFrame)
  onFrameRef.current = onFrame

  const connect = useCallback(() => {
    if (!token) return

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const host  = window.location.host
    const url   = `${proto}://${host}/ws?token=${token}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      reconnectRef.current = 0
      console.log('[ws] connected')
    }

    ws.onmessage = (ev) => {
      try {
        const frame: WsFrame = JSON.parse(ev.data)
        if (frame.type === 'message') {
          onFrameRef.current(frame)
        }
      } catch {
        console.warn('[ws] unhandled message', ev.data)
      }
    }

    ws.onerror = (e) => console.error('[ws] error', e)

    ws.onclose = () => {
      console.log('[ws] closed; reconnecting…')
      const delay = Math.min(500 * 2 ** reconnectRef.current, 30_000)
      reconnectRef.current++
      setTimeout(connect, delay)
    }
  }, [token])

  useEffect(() => {
    connect()
    return () => {
      wsRef.current?.close()
    }
  }, [connect])
}
