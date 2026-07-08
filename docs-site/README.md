# subapi 文档站

这是给用户访问的 subapi 静态帮助中心，基于 VitePress 构建。

## 本地预览

```bash
npm install
npm run dev
```

## 构建

```bash
npm run build
```

构建产物位于：

```text
docs/.vitepress/dist
```

可部署到独立域名，例如 `docs.subapi.loucer.cn`，也可以由主站 Nginx 挂到 `/docs`。

## 部署安全说明

生产环境只发布 docs/.vitepress/dist 静态文件，不要把 npm run dev 暴露到公网。

当前 VitePress 最新版 1.6.4 依赖 Vite dev server，npm audit 会提示开发服务器相关风险；该风险不影响已构建的静态产物。公网部署时使用 Nginx/Caddy/对象存储托管构建结果即可。
