import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/admin/',
  build: {
    // Never inline fonts as data: URIs; CSP font-src 'self' would block them.
    assetsInlineLimit: (file) => (/\.(woff2?|ttf|otf|eot)$/.test(file) ? false : undefined),
  },
})
