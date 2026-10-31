import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In dev, Vite serves the app on :5173 and proxies the API (including the
// WebSocket) to the Go server on :8080. Keeping the original Host header means
// the server's same-origin WebSocket check passes without extra config.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": { target: "http://localhost:8080", ws: true },
    },
  },
});
