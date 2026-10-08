import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The production build lands inside the Go tree so `go:embed` can pick it up.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: "../internal/ui/dist", emptyOutDir: true },
  server: {
    // `npm run dev` talks to a running `sentinel mcp --ui` / `sentinel ui`.
    proxy: {
      "/api": "http://127.0.0.1:8848",
      "/ws": { target: "ws://127.0.0.1:8848", ws: true },
    },
  },
});
