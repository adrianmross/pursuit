import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { AuthProvider, useAuth } from "@/lib/auth"
import { ThemeProvider } from "@/lib/theme"
import { AppShell } from "@/components/AppShell"
import { SignIn } from "@/components/SignIn"
import { PipelinePage } from "@/pages/PipelinePage"
import { QueuePage } from "@/pages/QueuePage"
import { DetailPage } from "@/pages/DetailPage"

function Gate() {
  const { me, loading } = useAuth()
  if (loading) return null
  if (!me) return <SignIn />
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<PipelinePage />} />
          <Route path="queue" element={<QueuePage />} />
          <Route path="app/:id" element={<DetailPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <Gate />
      </AuthProvider>
    </ThemeProvider>
  )
}

export default App
