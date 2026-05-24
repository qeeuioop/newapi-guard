import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  base: "/guard/static/",
  plugins: [vue()],
  build: {
    outDir: "../web",
    emptyOutDir: true,
    chunkSizeWarningLimit: 700
  }
});
