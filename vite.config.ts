import {fileURLToPath} from "node:url";

import react from "@vitejs/plugin-react";
import {defineConfig, loadEnv} from "vite";

export default defineConfig(({mode}) => {
  const env = loadEnv(mode, process.cwd(), "");
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
        "^/api(?:/|$)": {target: env.BACKEND_API_URL, changeOrigin: true},
        "^/ws(?:/|$)": {target: env.BACKEND_API_URL, ws: true},
      },
    },
  };
});
