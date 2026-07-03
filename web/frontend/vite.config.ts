import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

export default defineConfig({
  plugins: [TanStackRouterVite(), react()],
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:3939",
        changeOrigin: true,
      },
    },
  },
});
