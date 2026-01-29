import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react({
    babel: {
        plugins: ['babel-plugin-react-compiler']
    }
  })],
  build: {
    assetsInlineLimit: 0
  },
  server: {
    host: true,
    watch: {
      usePolling: true,
      interval: 100
    }
  }
})
