import { useEffect } from "react"
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom"
import { AuthProvider, useAuth } from "@/lib/auth"
import { ThemeProvider } from "@/lib/theme"
import { AppShell } from "@/components/AppShell"
import { Landing } from "@/components/Landing"
import { SignIn } from "@/components/SignIn"
import { TooltipProvider } from "@/components/ui/tooltip"
import { PipelinePage } from "@/pages/PipelinePage"
import { DetailPage } from "@/pages/DetailPage"
import { HomePage } from "@/pages/HomePage"
import { AuditPage } from "@/pages/AuditPage"

// Where to go after signing in. Same-origin paths only: "//evil.com" and
// "https://..." would turn ?next= into an open redirect.
function nextPath(search: string) {
  const next = new URLSearchParams(search).get("next") || "/"
  return next.startsWith("/") && !next.startsWith("//") ? next : "/"
}

function SignedIn() {
  const { search } = useLocation()
  const navigate = useNavigate()
  const next = nextPath(search)
  useEffect(() => {
    // /reference and /docs are served by the daemon, not the SPA router.
    if (next === "/reference" || next.startsWith("/docs")) window.location.assign(next)
    else navigate(next, { replace: true })
  }, [next, navigate])
  return null
}

function ToLogin() {
  const { pathname, search } = useLocation()
  return <Navigate to={"/login?next=" + encodeURIComponent(pathname + search)} replace />
}

function Gate() {
  const { me, loading } = useAuth()
  if (loading) return null
  return (
    <BrowserRouter>
      {me ? (
        <Routes>
          <Route path="login" element={<SignedIn />} />
          <Route element={<AppShell />}>
            <Route index element={<HomePage />} />
            <Route path="pipeline" element={<PipelinePage />} />
            <Route path="audit" element={<AuditPage />} />
            <Route path="app/:id" element={<DetailPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      ) : (
        <Routes>
          <Route index element={<Landing />} />
          <Route path="login" element={<SignIn />} />
          <Route path="*" element={<ToLogin />} />
        </Routes>
      )}
    </BrowserRouter>
  )
}

function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <TooltipProvider delayDuration={300}>
          <Gate />
        </TooltipProvider>
      </AuthProvider>
    </ThemeProvider>
  )
}

export default App
