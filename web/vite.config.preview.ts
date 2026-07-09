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
  base: process.env.PREVIEW_BASE ?? "/enbu/main/web/",
  build: {
    outDir: "dist-preview",
  },
});
