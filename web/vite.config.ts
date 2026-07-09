import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import { resolve } from "path";

export default defineConfig({
  plugins: [TanStackRouterVite(), react()],
  resolve: {
    alias: {
      "styled-system": resolve(__dirname, "styled-system"),
      "~/components": resolve(__dirname, "src/components"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:3939",
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "jsdom",
  },
});
