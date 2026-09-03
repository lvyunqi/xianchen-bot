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
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          // 只把 recharts 系叶子库独立成 chunk：Dashboard 主路径单向依赖它，
          // 无循环引用风险。react/vendor 混拆会产生跨 chunk TDZ（Cannot access before init）。
          if (!id.includes("node_modules")) return undefined
          if (id.includes("recharts") || id.includes("d3-") || id.includes("victory-vendor") || id.includes("lodash")) return "charts"
          return undefined
        },
      },
    },
  },
})
