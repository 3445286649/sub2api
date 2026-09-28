# CodePlan 文档站

CodePlan 面向用户的静态帮助中心，基于 VitePress 构建。

线上地址：<https://docs.loucer.cn>

## 本地预览

```bash
npm install
npm run dev
```

## 构建

```bash
npm run build
```

构建产物位于 `docs/.vitepress/dist`。

## 目录说明

| 路径 | 用途 |
| --- | --- |
| `docs/*.md` | 文档正文 |
| `docs/.vitepress/config.ts` | 站点配置：导航、侧边栏、访问地址 |
| `docs/.vitepress/theme/` | 主题：配色与主站 `frontend/src/style.css` 的语义 token 保持一致 |
| `docs/public/` | 图片、Logo、下载文件 |

### 写作约定

- **对外品牌统一叫 CodePlan。** 两个访问地址 `codeplan.codes` 和 `subapi.loucer.cn` 指向同一个后端。
- **客户端里要填的地址写 `subapi.loucer.cn`**，与主站后台「API 地址」一致，因为站内「使用密钥」弹窗和「导入到 CCS」生成的配置用的就是它。后台改了这个地址，文档要同步改。
- **配置示例必须和站内「使用密钥」弹窗生成的内容一致**（`frontend/src/components/keys/UseKeyModal.vue`）。弹窗改了生成逻辑，对应教程也要改。
- CCSwitch 的完整步骤只在 `ccswitch.md` 写一次，其他页面链接过去；`/v1` 的填写规则只在 `api-key.md` 维护。
- 软件与群组的真实名称保持原样（如 `Sub2API Balance Float`、`loucer中转站一群`），否则用户搜不到。

## 部署

生产环境只发布 `docs/.vitepress/dist` 静态文件，**不要把 `npm run dev` 暴露到公网**。

VitePress 1.6.4 依赖 Vite dev server，`npm audit` 会提示开发服务器相关风险；该风险不影响已构建的静态产物。用 Nginx、Caddy 或对象存储托管构建结果即可。

### 独立域名（推荐）

```nginx
server {
    listen 80;
    server_name docs.loucer.cn;

    root /opt/codeplan-docs/dist;
    index index.html;

    location / {
        try_files $uri $uri.html $uri/ /404.html;
    }
}
```

站点开启了 `cleanUrls`，所以 `try_files` 要带上 `$uri.html`，否则 `/quick-start` 这类地址刷新后会 404。

### 挂在主站子路径

如果要挂到主站的 `/docs/` 下，需要在 `docs/.vitepress/config.ts` 里设置：

```ts
export default defineConfig({
  base: '/docs/',
})
```

独立域名部署时不需要设置 `base`。
