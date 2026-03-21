import { useState, useEffect, useRef, useCallback, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { api } from '../api/client'
import { useAuthStore } from '../store/authStore'
import { useWebSocket } from '../hooks/useWebSocket'
import type { User, Message, WsFrame } from '../types'

export default function ChatPage() {
  const navigate = useNavigate()
  const { token, user: me, clearAuth } = useAuthStore()
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [messages, setMessages] = useState<Map<string, Message[]>>(new Map())
  const [convMap, setConvMap] = useState<Map<string, string>>(new Map()) // recipientId → conversationId
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

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
    if (frame.type !== 'message') return
    const msg = frame.payload
    setMessages((prev) => {
      const convId = msg.conversation_id
      const existing = prev.get(convId) ?? []
      // Deduplicate by id
      if (existing.some((m) => m.id === msg.id)) return prev
      return new Map(prev).set(convId, [...existing, msg])
    })
    // Update convMap so we know the conversationId for this recipient
    setConvMap((prev) => {
      const senderId = msg.sender_id
      if (senderId !== me?.id && !prev.has(senderId)) {
        return new Map(prev).set(senderId, msg.conversation_id)
      }
      return prev
    })
  }, [me])

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

  const wsStatus = useWebSocket({ token: token ?? '', onFrame: handleFrame, onConnected: syncSelectedConversation })

  // Load message history when selecting a user
  const selectUser = async (u: User) => {
    setSelectedUser(u)

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

  const logout = async () => {
    try { await api.logout() } catch { /* ignore */ }
    clearAuth()
    navigate('/login')
  }

  const convId = selectedUser ? convMap.get(selectedUser.id) : undefined
  const activeMessages = convId ? (messages.get(convId) ?? []) : []
  const charCount = [...body].length
  const charWarn = charCount > 3800

  const wsStatusLabel: Record<typeof wsStatus, string> = {
    connected: 'Live',
    connecting: 'Connecting',
    reconnecting: 'Reconnecting',
    offline: 'Offline',
  }

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
                  <span className="chat-header-status">online</span>
                  <span className={`connection-badge ${wsStatus}`}>{wsStatusLabel[wsStatus]}</span>
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
                  onChange={(e) => setBody(e.target.value)}
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
