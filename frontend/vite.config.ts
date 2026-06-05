import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { compression } from "vite-plugin-compression2";
import path from "path";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // Pre-compress JS/CSS/SVG/HTML so the Go backend can serve .br/.gz
    // when the client advertises Accept-Encoding: br|gzip. The Go side
    // needs a matching content-negotiation handler — until then the
    // artefacts simply sit alongside the originals.
    compression({
      algorithms: ["brotliCompress"],
      include: [/\.(js|mjs|css|html|svg|json)$/],
      threshold: 1024,
    }),
    compression({
      algorithms: ["gzip"],
      include: [/\.(js|mjs|css|html|svg|json)$/],
      threshold: 1024,
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        // Rolldown requires manualChunks as a function (not an object).
        // Split large vendor libs so the initial chunk stays small.
        manualChunks(id: string) {
          // recharts depends on the d3-* family; bundle them together
          // so the chart chunk is self-contained.
          if (id.includes("node_modules/recharts") || /node_modules\/d3-/.test(id)) {
            return "vendor-charts";
          }
          if (id.includes("node_modules/motion")) {
            return "vendor-motion";
          }
          if (
            id.includes("node_modules/react-aria-components") ||
            id.includes("node_modules/@react-stately") ||
            id.includes("node_modules/react-aria") ||
            id.includes("node_modules/@react-aria")
          ) {
            return "vendor-aria";
          }
          if (id.includes("node_modules/@tanstack/")) {
            return "vendor-query";
          }
          if (
            id.includes("node_modules/react-dom") ||
            id.includes("node_modules/react-router") ||
            id.includes("node_modules/react/")
          ) {
            return "vendor-react";
          }
        },
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": "http://127.0.0.1:8081",
      "/events": {
        target: "http://127.0.0.1:8081",
        ws: true,
      },
      "/health": "http://127.0.0.1:8081",
    },
  },
});
