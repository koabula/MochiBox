import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import fs from 'fs'

const packageJson = JSON.parse(fs.readFileSync('./package.json', 'utf-8'))

// https://vitejs.dev/config/
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(packageJson.version)
  },
  plugins: [vue()],
  server: {
    port: 5173,
    strictPort: true, // Fail if port is busy
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  base: './', // For Electron relative paths
})
