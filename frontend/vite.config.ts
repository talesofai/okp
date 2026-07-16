import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

// Cohub directory Works serve assets under a nested snapshot path.
// Relative base keeps JS/CSS/chunk URLs valid after publish.
export default defineConfig({
  base: "./",
  plugins: [
    TanStackRouterVite({ target: "react", autoCodeSplitting: true }),
    react(),
  ],
  build: {
    outDir: "dist",
    assetsDir: "assets",
    sourcemap: true,
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
    allowedHosts: [".cohub.run"],
  },
});
