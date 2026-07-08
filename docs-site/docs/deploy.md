# 部署说明

推荐先把文档站部署成独立域名，例如：

```text
docs.subapi.loucer.cn
```

这样不会影响主站，也方便后续单独更新教程。

## 构建

在 `docs-site` 目录执行：

```bash
npm install
npm run build
```

构建产物：

```text
docs/.vitepress/dist
```

## Nginx 独立站点示例

```nginx
server {
    listen 80;
    server_name docs.subapi.loucer.cn;

    root /opt/subapi-docs/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

## 主站子路径示例

如果要挂到 `https://subapi.loucer.cn/docs/`，需要同步调整 VitePress `base` 配置：

```ts
export default defineConfig({
  base: '/docs/'
})
```

独立域名部署时不需要设置 `base`。
