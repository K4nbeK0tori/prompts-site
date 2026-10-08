import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 构建产物直接落到 ../web，由 Go 的 //go:embed 打进单二进制。
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../web',
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
