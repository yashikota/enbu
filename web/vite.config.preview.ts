import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

export default defineConfig({
  plugins: [TanStackRouterVite(), react()],
  base: process.env.PREVIEW_BASE ?? "/enbu/main/web/",
  build: {
    outDir: "dist-preview",
  },
});
