import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [tanstackRouter({ target: 'react' }), react(), tailwindcss()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '^/api(?:/|\\?|$)': {
        target: 'http://app:8080',
        ws: true,
        // Keep the browser's Host and Origin available for server-side checks.
        changeOrigin: false,
        rewriteWsOrigin: false,
      },
    },
    watch: {
      usePolling: true,
      interval: 500,
      ignored: ['**/node_modules/**', '**/dist/**'],
    },
  },
})
