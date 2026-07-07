import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { viteSingleFile } from 'vite-plugin-singlefile'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'

// https://vite.dev/config/
export default defineConfig({
  server: {
    allowedHosts: ['.cohub.run'],
  },
  plugins: [
    viteSingleFile(),
    TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
    react(),
  ],
})
