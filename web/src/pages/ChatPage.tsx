import { useState, useEffect, useRef, useCallback, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { api } from '../api/client'
import { useAuthStore } from '../store/authStore'
import { useWebSocket } from '../hooks/useWebSocket'
import type { User, Message, WsFrame, TypingPayload, PresencePayload, OutboundTypingFrame } from '../types'

export default function ChatPage() {
  const navigate = useNavigate()
  const { token, user: me, clearAuth } = useAuthStore()
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [messages, setMessages] = useState<Map<string, Message[]>>(new Map())
  const [unreadByUser, setUnreadByUser] = useState<Map<string, number>>(new Map())
  const [typingByUser, setTypingByUser] = useState<Map<string, boolean>>(new Map())
  const [onlineByUser, setOnlineByUser] = useState<Map<string, boolean>>(new Map())
  const [convMap, setConvMap] = useState<Map<string, string>>(new Map()) // recipientId → conversationId
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const typingStopTimerRef = useRef<number | null>(null)
  const typingThrottleRef = useRef<number>(0)
  const isTypingSentRef = useRef<boolean>(false)
  const typingExpiryTimersRef = useRef<Map<string, number>>(new Map())

  // Scroll to bottom when new messages arrive for the selected conversation
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, selectedUser])

  // Load user list on mount
  useEffect(() => {
    if (!token) { navigate('/login'); return }
    api.listUsers()
      .then((all) => setUsers(all.filter((u) => u.id !== me?.id)))
      .catch(() => {})
  }, [token, me, navigate])

  // Handle incoming WS frames globally (messages for any conversation)
  const handleFrame = useCallback((frame: WsFrame) => {
    if (frame.type === 'presence') {
      const payload = frame.payload as PresencePayload
      if (!payload?.user_id) return
      setOnlineByUser((prev) => {
        const next = new Map(prev)
        if (payload.is_online) next.set(payload.user_id, true)
        else next.delete(payload.user_id)
        return next
      })
      return
    }

    if (frame.type === 'typing') {
      const payload = frame.payload as TypingPayload
      if (!payload?.sender_id || payload.sender_id === me?.id) return

      const senderId = payload.sender_id
      if (!payload.is_typing) {
        const existingTimer = typingExpiryTimersRef.current.get(senderId)
        if (existingTimer != null) {
          window.clearTimeout(existingTimer)
          typingExpiryTimersRef.current.delete(senderId)
        }
        setTypingByUser((prev) => {
          if (!prev.has(senderId)) return prev
          const next = new Map(prev)
          next.delete(senderId)
          return next
        })
        return
      }

      setTypingByUser((prev) => new Map(prev).set(senderId, true))
      const existingTimer = typingExpiryTimersRef.current.get(senderId)
      if (existingTimer != null) {
        window.clearTimeout(existingTimer)
      }
      const timerId = window.setTimeout(() => {
        setTypingByUser((prev) => {
          if (!prev.has(senderId)) return prev
          const next = new Map(prev)
          next.delete(senderId)
          return next
        })
        typingExpiryTimersRef.current.delete(senderId)
      }, 2800)
      typingExpiryTimersRef.current.set(senderId, timerId)
      return
    }

    if (frame.type !== 'message') return
    const msg = frame.payload

    setMessages((prev) => {
      const convId = msg.conversation_id
      const existing = prev.get(convId) ?? []
      // Deduplicate by id
      if (existing.some((m) => m.id === msg.id)) return prev
      return new Map(prev).set(convId, [...existing, msg])
    })

    const senderId = msg.sender_id
    if (senderId !== me?.id) {
      setTypingByUser((prev) => {
        if (!prev.has(senderId)) return prev
        const next = new Map(prev)
        next.delete(senderId)
        return next
      })
      const existingTimer = typingExpiryTimersRef.current.get(senderId)
      if (existingTimer != null) {
        window.clearTimeout(existingTimer)
        typingExpiryTimersRef.current.delete(senderId)
      }
      if (selectedUser?.id !== senderId) {
        setUnreadByUser((prev) => new Map(prev).set(senderId, (prev.get(senderId) ?? 0) + 1))
      }
    }

    // Update convMap so we know the conversationId for this recipient
    setConvMap((prev) => {
      if (senderId !== me?.id && !prev.has(senderId)) {
        return new Map(prev).set(senderId, msg.conversation_id)
      }
      return prev
    })
  }, [me, selectedUser])

  const syncSelectedConversation = useCallback(async () => {
    if (!selectedUser) return

    let conversationId = convMap.get(selectedUser.id)
    if (!conversationId) {
      try {
        const conv = await api.getDirectConversation(selectedUser.id)
        conversationId = conv.id
        setConvMap((prev) => new Map(prev).set(selectedUser.id, conv.id))
      } catch {
        return
      }
    }

    try {
      const hist = await api.listMessages(conversationId)
      const ordered = hist.reverse()
      setMessages((prev) => {
        const existing = prev.get(conversationId!) ?? []
        const byId = new Map(existing.map((m) => [m.id, m]))
        for (const msg of ordered) byId.set(msg.id, msg)
        const merged = Array.from(byId.values()).sort(
          (a, b) => new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime()
        )
        return new Map(prev).set(conversationId!, merged)
      })
    } catch {
      // Ignore sync errors; WS stream will continue retrying.
    }
  }, [selectedUser, convMap])

  const { connectionState, sendFrame } = useWebSocket({
    token: token ?? '',
    onFrame: handleFrame,
    onConnected: syncSelectedConversation,
  })

  // Reset presence map whenever the WS connection drops; server re-pushes on reconnect
  useEffect(() => {
    if (connectionState !== 'connected') setOnlineByUser(new Map())
  }, [connectionState])

  const sendTyping = useCallback((isTyping: boolean) => {
    if (!selectedUser) return

    const now = Date.now()
    if (isTyping) {
      if (isTypingSentRef.current && now - typingThrottleRef.current < 1200) return
      isTypingSentRef.current = true
      typingThrottleRef.current = now
    } else if (!isTypingSentRef.current) {
      return
    } else {
      isTypingSentRef.current = false
    }

    const frame: OutboundTypingFrame = {
      type: 'typing',
      payload: {
        recipient_id: selectedUser.id,
        is_typing: isTyping,
      },
    }
    sendFrame(frame)
  }, [selectedUser, sendFrame])

  // Load message history when selecting a user
  const selectUser = async (u: User) => {
    setSelectedUser(u)
    setUnreadByUser((prev) => {
      if (!prev.has(u.id)) return prev
      const next = new Map(prev)
      next.delete(u.id)
      return next
    })

    if (typingStopTimerRef.current != null) {
      window.clearTimeout(typingStopTimerRef.current)
      typingStopTimerRef.current = null
    }
    sendTyping(false)

    // Resolve the conversationId: use cached value or discover it from the server.
    // This lets history load on a fresh page without needing to send a message first.
    let convId = convMap.get(u.id)
    if (!convId) {
      try {
        const conv = await api.getDirectConversation(u.id)
        convId = conv.id
        setConvMap((prev) => new Map(prev).set(u.id, conv.id))
      } catch { /* no conversation exists yet — first time chatting */ }
    }

    if (convId && !(messages.get(convId)?.length)) {
      try {
        const hist = await api.listMessages(convId)
        setMessages((prev) => new Map(prev).set(convId!, hist.reverse()))
      } catch { /* history fetch failed */ }
    }
  }

  const send = async () => {
    if (!selectedUser || !body.trim() || sending) return
    setSending(true)
    if (typingStopTimerRef.current != null) {
      window.clearTimeout(typingStopTimerRef.current)
      typingStopTimerRef.current = null
    }
    sendTyping(false)
    try {
      const msg = await api.sendMessage(selectedUser.id, body.trim())
      setBody('')
      textareaRef.current?.focus()
      // Update convMap
      setConvMap((prev) => {
        const next = new Map(prev)
        next.set(selectedUser.id, msg.conversation_id)
        return next
      })
      // Append to local messages (WS echo will also come, deduplicated)
      setMessages((prev) => {
        const existing = prev.get(msg.conversation_id) ?? []
        if (existing.some((m) => m.id === msg.id)) return prev
        return new Map(prev).set(msg.conversation_id, [...existing, msg])
      })
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Send failed')
    } finally {
      setSending(false)
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  const handleBodyChange = (value: string) => {
    setBody(value)

    if (!selectedUser) return

    if (value.trim()) {
      sendTyping(true)
      if (typingStopTimerRef.current != null) {
        window.clearTimeout(typingStopTimerRef.current)
      }
      typingStopTimerRef.current = window.setTimeout(() => {
        sendTyping(false)
        typingStopTimerRef.current = null
      }, 1500)
    } else {
      if (typingStopTimerRef.current != null) {
        window.clearTimeout(typingStopTimerRef.current)
        typingStopTimerRef.current = null
      }
      sendTyping(false)
    }
  }

  useEffect(() => {
    return () => {
      if (typingStopTimerRef.current != null) {
        window.clearTimeout(typingStopTimerRef.current)
      }
      for (const timer of typingExpiryTimersRef.current.values()) {
        window.clearTimeout(timer)
      }
    }
  }, [])

  const logout = async () => {
    try { await api.logout() } catch { /* ignore */ }
    clearAuth()
    navigate('/login')
  }

  const convId = selectedUser ? convMap.get(selectedUser.id) : undefined
  const activeMessages = convId ? (messages.get(convId) ?? []) : []
  const charCount = [...body].length
  const charWarn = charCount > 3800

  const otherUserTyping = selectedUser ? typingByUser.get(selectedUser.id) === true : false
  const otherUserOnline = selectedUser ? onlineByUser.get(selectedUser.id) === true : false

  const initials = (name: string) => name.slice(0, 2).toUpperCase()

  return (
    <div className="app-shell">
      {/* ── Sidebar ─────────────────────────────────────────── */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-brand">
            <div className="sidebar-brand-icon">💬</div>
            MyMessaging
          </div>
        </div>

        <div className="sidebar-section-title">Direct Messages</div>

        <div className="sidebar-users">
          {users.length === 0 && (
            <p style={{ padding: '8px 10px', color: 'var(--text-muted)', fontSize: 13 }}>
              No other users yet
            </p>
          )}
          {users.map((u) => (
            <div
              key={u.id}
              id={`user-${u.id}`}
              className={`user-item ${selectedUser?.id === u.id ? 'active' : ''}`}
              onClick={() => selectUser(u)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => e.key === 'Enter' && selectUser(u)}
            >
              <div className="avatar">{initials(u.username)}</div>
              <span className="user-item-name">{u.username}</span>
              {(unreadByUser.get(u.id) ?? 0) > 0 && (
                <span className="user-unread-badge" aria-label={`${unreadByUser.get(u.id)} unread messages`}>
                  {(unreadByUser.get(u.id) ?? 0) > 99 ? '99+' : unreadByUser.get(u.id)}
                </span>
              )}
            </div>
          ))}
        </div>

        <div className="sidebar-footer">
          <div className="sidebar-me">
            <div className="avatar" style={{ width: 28, height: 28, fontSize: 11 }}>
              {me ? initials(me.username) : '?'}
            </div>
            <span className="sidebar-me-name">{me?.username}</span>
          </div>
          <button id="logout-btn" className="btn btn-ghost" style={{ fontSize: 13, padding: '6px 10px' }} onClick={logout}>
            Sign out
          </button>
        </div>
      </aside>

      {/* ── Chat area ────────────────────────────────────────── */}
      <main className="chat-area">
        {!selectedUser ? (
          <div className="chat-empty">
            <div className="chat-empty-icon">💬</div>
            <p>Select a user to start chatting</p>
          </div>
        ) : (
          <>
            <div className="chat-header">
              <div className="avatar">{initials(selectedUser.username)}</div>
              <div>
                <div className="chat-header-name">{selectedUser.username}</div>
                <div className="chat-header-meta">
                  <span className={`chat-header-status${otherUserOnline ? '' : ' offline'}`}>
                    {otherUserOnline ? 'online' : 'offline'}
                  </span>
                  {otherUserTyping && <span className="typing-indicator">typing...</span>}
                </div>
              </div>
            </div>

            <div className="chat-messages" id="chat-messages">
              {activeMessages.map((msg) => {
                const isSent = msg.sender_id === me?.id
                return (
                  <div key={msg.id} className={`msg-row ${isSent ? 'sent' : 'recv'}`}>
                    {!isSent && (
                      <div className="msg-sender">{msg.sender_username}</div>
                    )}
                    <div className="msg-bubble">{msg.body}</div>
                    <div className="msg-time">
                      {formatDistanceToNow(new Date(msg.sent_at), { addSuffix: true })}
                    </div>
                  </div>
                )
              })}
              <div ref={bottomRef} />
            </div>

            <div className="chat-input-bar">
              <div className="chat-input-wrap">
                <textarea
                  ref={textareaRef}
                  id="message-input"
                  className="chat-textarea"
                  placeholder={`Message ${selectedUser.username}… (Enter to send, Shift+Enter for newline)`}
                  value={body}
                  onChange={(e) => handleBodyChange(e.target.value)}
                  onKeyDown={handleKeyDown}
                  maxLength={4096}
                  rows={1}
                  style={{ height: 'auto' }}
                  onInput={(e) => {
                    const el = e.currentTarget
                    el.style.height = 'auto'
                    el.style.height = `${Math.min(el.scrollHeight, 140)}px`
                  }}
                />
                {charCount > 100 && (
                  <span className={`char-count ${charWarn ? 'warn' : ''}`}>
                    {charCount}/4096
                  </span>
                )}
              </div>
              <button
                id="send-btn"
                className="btn btn-icon send"
                onClick={send}
                disabled={sending || !body.trim()}
                title="Send message"
              >
                ➤
              </button>
            </div>
          </>
        )}
      </main>
    </div>
  )
}
