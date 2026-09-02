import { Route, Routes } from "react-router-dom"
import { Toaster } from "sonner"
import AppLayout from "@/components/layout/AppLayout"
import Dashboard from "@/pages/Dashboard"
import ResourcePage from "@/pages/ResourcePage"
import NotFound from "@/pages/NotFound"

export default function App() {
  return (
    <>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/r/:key" element={<ResourcePage />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
      <Toaster position="top-center" richColors closeButton />
    </>
  )
}
