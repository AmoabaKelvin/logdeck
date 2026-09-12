/// <reference types="vitest/config" />
import tailwindcss from "@tailwindcss/vite";
import tanstackRouter from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { resolve } from "node:path";
import { defineConfig } from "vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    viteReact(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
  },
  server: {
    host: true, // Listen on all addresses, needed for Docker
    port: 5173,
    proxy: {
      // Proxy API requests to the backend during development
      "/api": {
        target: "http://localhost:8080",
        // The container terminal talks over a websocket on the same path, and
        // the server rejects an upgrade whose Origin host differs from the
        // request Host. Rewriting the origin would make every dev exec fail
        // that check, so leave both pointing at the dev server.
        changeOrigin: false,
        ws: true,
      },
    },
  },
});
