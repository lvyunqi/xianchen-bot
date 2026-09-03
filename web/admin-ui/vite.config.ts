import path from "node:path"
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import mockApi from "./mock-api.mjs"

// 产物直接输出到 go:embed 的 web/admin 目录；logo 由 public/ 提供。
// base 必须指向 /admin/：Go 侧把 /admin 之外的一切 302 到后台入口，
// 若产物用根绝对路径 /assets/*，模块脚本会被 302 重定向成 text/html（Strict MIME 白屏）。
export default defineConfig({
  base: "/admin/",
  plugins: [react(), mockApi()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    port: 5188,
    proxy: {
      "/api": { target: "http://127.0.0.1:8088", changeOrigin: true },
    },
  },
  build: {
    outDir: "../admin",
    emptyOutDir: true,
    sourcemap: false,
    chunkSizeWarningLimit: 1200,
  },
})
