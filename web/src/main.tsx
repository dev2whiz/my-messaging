import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store/authStore'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import ChatPage from './pages/ChatPage'
import './index.css'

function App() {
  const token = useAuthStore((s) => s.token)

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={!token ? <LoginPage /> : <Navigate to="/chat" />} />
        <Route path="/register" element={!token ? <RegisterPage /> : <Navigate to="/chat" />} />
        <Route path="/chat" element={token ? <ChatPage /> : <Navigate to="/login" />} />
        <Route path="*" element={<Navigate to={token ? '/chat' : '/login'} />} />
      </Routes>
    </BrowserRouter>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
)
