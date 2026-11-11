import {fileURLToPath} from "node:url";

import react from "@vitejs/plugin-react";
import {defineConfig} from "vite";

// The dev server proxies API and WebSocket traffic to the local backend (deploy/local/config.yaml).
const backend = "http://localhost:8080";

export default defineConfig(() => {
  return {
    plugins: [react()],
    resolve: {
      alias: {"@": fileURLToPath(new URL("./frontend", import.meta.url))},
    },
    publicDir: "frontend/public",
    build: {outDir: "dist"},
    server: {
      host: "0.0.0.0",
      port: 3000,
      proxy: {
        "^/api(?:/|$)": {target: backend, changeOrigin: true},
        "^/ws(?:/|$)": {target: backend, ws: true},
      },
    },
  };
});
