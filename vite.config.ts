import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
    plugins: [tailwindcss(), react({
        babel: {
            plugins: ['babel-plugin-react-compiler']
        }
    })],
    optimizeDeps: {
        exclude: ['maplibre-gl']
    },
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
