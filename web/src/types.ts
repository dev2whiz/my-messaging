export interface User {
  id: string
  username: string
  email: string
  created_at: string
}

export interface Conversation {
  id: string
  name?: string
  isGroup: boolean
  createdBy: string
  createdAt: string
}

export interface Message {
  id: string
  conversation_id: string
  sender_id: string
  sender_username: string
  body: string
  sent_at: string
  read_at?: string
}

export interface TypingPayload {
  sender_id: string
  recipient_id: string
  is_typing: boolean
}

export interface PresencePayload {
  user_id: string
  is_online: boolean
}

export type WsFrame =
  | {
      type: 'message'
      payload: Message
    }
  | {
      type: 'typing'
      payload: TypingPayload
    }
  | {
      type: 'presence'
      payload: PresencePayload
    }
  | {
      type: 'ping' | 'error'
      payload?: unknown
    }

export interface OutboundTypingFrame {
  type: 'typing'
  payload: {
    recipient_id: string
    is_typing: boolean
  }
}
