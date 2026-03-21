import { useEffect, useRef, useCallback, useState } from 'react'
import type { WsFrame } from '../types'

export type WsConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'offline'

interface Options {
  token: string
  onFrame: (frame: WsFrame) => void
  onConnected?: () => void
}

export function useWebSocket({ token, onFrame, onConnected }: Options) {
  const wsRef = useRef<WebSocket | null>(null)
  const shouldReconnectRef = useRef<boolean>(true)
  const reconnectTimerRef = useRef<number | null>(null)
  const reconnectRef = useRef<number>(0)
  const [connectionState, setConnectionState] = useState<WsConnectionState>('offline')
  const onFrameRef = useRef(onFrame)
  const onConnectedRef = useRef(onConnected)
  onFrameRef.current = onFrame
  onConnectedRef.current = onConnected

  const connect = useCallback(() => {
    if (!token || !shouldReconnectRef.current) {
      setConnectionState('offline')
      return
    }

    setConnectionState(reconnectRef.current > 0 ? 'reconnecting' : 'connecting')

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const host  = window.location.host
    const url   = `${proto}://${host}/ws?token=${token}`

    wsRef.current?.close()
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      reconnectRef.current = 0
      setConnectionState('connected')
      console.log('[ws] connected')
      onConnectedRef.current?.()
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
      if (!shouldReconnectRef.current) return
      setConnectionState('reconnecting')
      console.log('[ws] closed; reconnecting…')
      const jitter = Math.floor(Math.random() * 300)
      const delay = Math.min(500 * 2 ** reconnectRef.current + jitter, 30_000)
      reconnectRef.current++
      reconnectTimerRef.current = window.setTimeout(connect, delay)
    }
  }, [token])

  useEffect(() => {
    shouldReconnectRef.current = true
    connect()

    const handleOnline = () => {
      if (!wsRef.current || wsRef.current.readyState === WebSocket.CLOSED) {
        connect()
      }
    }
    window.addEventListener('online', handleOnline)

    return () => {
      shouldReconnectRef.current = false
      if (reconnectTimerRef.current != null) {
        window.clearTimeout(reconnectTimerRef.current)
      }
      window.removeEventListener('online', handleOnline)
      wsRef.current?.close()
      setConnectionState('offline')
    }
  }, [connect])

  return connectionState
}
