const BASE = '/api'

function getToken(): string {
  try {
    const raw = localStorage.getItem('auth')
    if (!raw) return ''
    return JSON.parse(raw)?.state?.token ?? ''
  } catch {
    return ''
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken()
  const res = await fetch(BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  register: (username: string, email: string, password: string) =>
    request<{ token: string; user: import('../types').User }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    }),

  login: (email: string, password: string) =>
    request<{ token: string; user: import('../types').User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  logout: () =>
    request<{ message: string }>('/auth/logout', { method: 'POST' }),

  me: () => request<import('../types').User>('/auth/me'),

  listUsers: () => request<import('../types').User[]>('/users'),

  sendMessage: (recipient_id: string, body: string) =>
    request<import('../types').Message>('/messages', {
      method: 'POST',
      body: JSON.stringify({ recipient_id, body }),
    }),

  listMessages: (conversationId: string, cursor?: string) =>
    request<import('../types').Message[]>(
      `/conversations/${conversationId}/messages${cursor ? `?cursor=${cursor}` : ''}`
    ),

  getDirectConversation: (partnerId: string) =>
    request<import('../types').Conversation>(`/conversations/direct/${partnerId}`),
}
