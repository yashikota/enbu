import { defineConfig } from "@pandacss/dev";
import { createPreset } from "@park-ui/panda-preset";
import blue from "@park-ui/panda-preset/colors/blue";
import slate from "@park-ui/panda-preset/colors/slate";

export default defineConfig({
  preflight: true,
  jsxFramework: "react",
  include: ["./src/**/*.{ts,tsx}"],
  exclude: [],
  outdir: "styled-system",

  presets: [
    createPreset({ accentColor: blue, grayColor: slate, radius: "sm" }),
  ],

  theme: {
    extend: {
      tokens: {
        fonts: {
          body: {
            value:
              'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
          },
          heading: {
            value:
              'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
          },
          mono: {
            value: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
          },
        },
        colors: {
          brand: {
            50: { value: "#eef4ff" },
            100: { value: "#dbe4ff" },
            200: { value: "#bfcfff" },
            300: { value: "#93acff" },
            400: { value: "#607dff" },
            500: { value: "#2563eb" },
            600: { value: "#1d53cc" },
            700: { value: "#1640a0" },
            800: { value: "#113177" },
            900: { value: "#0c2454" },
          },
          canvas: { value: "#ffffff" },
          surface: { value: "#ffffff" },
          surfaceMuted: { value: "#f8f9fa" },
          borderStrong: { value: "#b8c0cc" },
        },
        radii: {
          sm: { value: "4px" },
          md: { value: "6px" },
          lg: { value: "8px" },
          full: { value: "9999px" },
        },
        shadows: {
          app: { value: "0 24px 80px rgba(15, 23, 42, 0.12)" },
          dropdown: { value: "0 16px 40px rgba(31, 35, 40, 0.16)" },
        },
        fontSizes: {
          "2xs": { value: "11px" },
          xs: { value: "12px" },
          sm: { value: "13px" },
          md: { value: "14px" },
          lg: { value: "15px" },
          xl: { value: "16px" },
          "2xl": { value: "20px" },
          "3xl": { value: "28px" },
          "4xl": { value: "34px" },
        },
        fontWeights: {
          normal: { value: "400" },
          medium: { value: "500" },
          semibold: { value: "650" },
          bold: { value: "700" },
          extrabold: { value: "750" },
        },
        sizes: {
          "control-sm": { value: "30px" },
          control: { value: "38px" },
        },
        spacing: {
          "4.5": { value: "18px" },
        },
      },
      semanticTokens: {
        colors: {
          bg: {
            default: { value: "{colors.canvas}" },
            surface: { value: "{colors.surface}" },
            muted: { value: "{colors.surfaceMuted}" },
          },
          fg: {
            default: { value: "#1f2328" },
            muted: { value: "#667085" },
            subtle: { value: "#475467" },
          },
          border: {
            default: { value: "#d8dee4" },
            strong: { value: "{colors.borderStrong}" },
          },
          accent: {
            default: { value: "{colors.brand.500}" },
            fg: { value: "#fff" },
          },
        },
      },
    },
  },
});
