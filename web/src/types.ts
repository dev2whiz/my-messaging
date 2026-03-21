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

export interface WsFrame {
  type: 'message' | 'ping' | 'error'
  payload: Message
}
