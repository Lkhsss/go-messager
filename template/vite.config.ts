import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import vueDevTools from "vite-plugin-vue-devtools";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), vueDevTools(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  // 开发时把 /api 转发给 Go 后端，避免跨域
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    // 拆分后单个 chunk 应远小于该值，这里当门槛防止以后又合回一个大包
    chunkSizeWarningLimit: 400,
    rolldownOptions: {
      output: {
        // Vite 8 使用 Rolldown，按库拆分 vendor，配合路由懒加载
        codeSplitting: {
          groups: [
            // highlight.js 体积最大，且只有文章详情/编辑器预览会用到
            {
              name: "highlight",
              test: /node_modules[\\/]highlight\.js[\\/]/,
              priority: 30,
            },
            // emoji 短代码表列表页也要用，单独成块，避免顺带把 markdown 渲染链路拉下来
            {
              name: "emoji",
              test: /node_modules[\\/]markdown-it-emoji[\\/]lib[\\/]data[\\/]/,
              priority: 25,
            },
            // Markdown 渲染链路：markdown-it + markdown-it-emoji 插件 + DOMPurify 及其依赖
            {
              name: "markdown-vendor",
              test: /node_modules[\\/](markdown-it|markdown-it-emoji|dompurify|linkify-it|mdurl|uc\.micro|entities|punycode\.js)[\\/]/,
              priority: 20,
            },
            {
              name: "axios",
              test: /node_modules[\\/]axios[\\/]/,
              priority: 20,
            },
            {
              name: "vue-vendor",
              test: /node_modules[\\/](vue|@vue|vue-router|pinia)[\\/]/,
              priority: 10,
            },
          ],
        },
      },
    },
  },
});
