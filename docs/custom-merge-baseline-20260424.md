# Sub2API 本地严格合并基线

更新时间：2026-07-08

## 结论

这份 `[D:\项目\subapi](D:\项目\subapi)` 当前应视为：

- 官方 `v0.1.117` 源码快照基础上整理出来的本地工作目录
- 已保留你现有的核心定制功能
- 已补上本地生图页图片预览失败的关键修复

注意：

- 官方快照目录 `[_tmp_sub2api_v0117](C:\Users\Loucer\Desktop\注册机ui设计\_tmp_sub2api_v0117)` 的 `[VERSION](C:\Users\Loucer\Desktop\注册机ui设计\_tmp_sub2api_v0117\backend\cmd\server\VERSION)` 仍然写的是 `0.1.116`
- 当前本地目录 `[VERSION](D:\项目\subapi\backend\cmd\server\VERSION)` 也仍然是 `0.1.116`
- 所以页面左上角版本角标现在显示 `v0.1.116`，这不是本次合并漏改，而是上游源码本身就没改这个文件

## 当前保留的定制功能

### 1. 顶栏发卡平台跳转

- 文件：[AppHeader.vue](D:\项目\subapi\frontend\src\components\layout\AppHeader.vue)
- 行为：右上角按钮跳转 `https://shop.loucer.cn/`

### 2. 订阅页续费 / 重置跳转

- 文件：[SubscriptionsView.vue](D:\项目\subapi\frontend\src\views\user\SubscriptionsView.vue)
- 行为：
  - 续费跳转 `https://shop.loucer.cn/`
  - OpenAI 订阅重置跳转 `https://shop.loucer.cn/categories/8`

### 3. 订阅额度刷新码

- 相关文件：
  - [subscription_quota_reset.go](D:\项目\subapi\backend\internal\service\subscription_quota_reset.go)
  - [redeem_service.go](D:\项目\subapi\backend\internal\service\redeem_service.go)
  - [admin/redeem_handler.go](D:\项目\subapi\backend\internal\handler\admin\redeem_handler.go)
- 当前规则：
  - `daily` 刷新时会把当天已用额度清零
  - 同时把本次已用的 `daily_usage_usd` 从 `weekly_usage_usd` 和 `monthly_usage_usd` 里扣回去
  - 不会出现负值

### 4. 订阅倍率按 ActualCost 扣减

- 核心文件：[gateway_service.go](D:\项目\subapi\backend\internal\service\gateway_service.go)
- 当前状态：
  - 订阅扣减链已经按 `ActualCost` 生效
  - 所以如果分组倍率是 `2.5`
  - 页面仍显示“每日 100 刀额度”
  - 但真实原始可用量约等于 `100 / 2.5 = 40`

### 5. 在线生图页

- 页面文件：[ImageStudioView.vue](D:\项目\subapi\frontend\src\views\user\ImageStudioView.vue)
- API 文件：[images.ts](D:\项目\subapi\frontend\src\api\images.ts)
- 后端图片链路文件：
  - [public_image_store.go](D:\项目\subapi\backend\internal\service\public_image_store.go)
  - [public_image.go](D:\项目\subapi\backend\internal\handler\public_image.go)
  - [openai_images.go](D:\项目\subapi\backend\internal\service\openai_images.go)
  - [openai_images_responses.go](D:\项目\subapi\backend\internal\service\openai_images_responses.go)

### 6. 独立帮助文档站

- 目录：[docs-site](D:\项目\subapi\docs-site)
- 技术栈：VitePress 静态文档站
- 当前公开域名：`https://docs.loucer.cn/`
- 线上部署方式：独立静态目录 `/opt/subapi-docs` + 独立 Nginx vhost `/etc/nginx/conf.d/docs.loucer.cn.conf`
- 与主站关系：不打进 `sub2api` 容器，不进入 `8080/8081` 蓝绿链路；主站蓝绿切换不会影响文档站
- 内容范围：
  - 新用户 3 分钟速通
  - API Key 创建与安全
  - CCSwitch 接入
  - Codex 接入
  - Claude Code 接入
  - VS Code 插件接入
  - 常见报错排查
  - 计费、余额与使用记录
- 关键接入教程已补充：
  - 导入到 CCSwitch
  - 点击启用 loucer_api
  - 刷新用量测试连通
  - 打开 Codex / Claude Code / VS Code 插件测试对话
  - 回到站内使用记录确认请求命中 subapi
- 截图资源位于：[docs-site/docs/public/images/subapi](D:\项目\subapi\docs-site\docs\public\images\subapi)
- 构建命令：`cd docs-site && npm run build`
- 构建产物：`docs-site/docs/.vitepress/dist`，只上传静态产物到服务器，不提交构建目录

### 7. 健康账号低频巡检分钟级配置

- 迁移文件：[167_healthy_probe_interval_minutes.sql](D:/项目/subapi/backend/migrations/167_healthy_probe_interval_minutes.sql)
- 后端字段：healthy_probe_interval_minutes；旧字段 healthy_probe_interval_hours 保留作为兼容回退
- 核心文件：account_health_repo.go、account_health.go、account_handler.go、AccountsView.vue
- 当前规则：
  - 健康态低频巡检优先使用分钟字段
  - 分钟字段为空时回退旧小时字段
  - 两者都为空时默认 360 分钟
  - 保存探活设置后会重新推进 next_probe_at
  - 关闭低频巡检时清空健康态旧 next_probe_at
- 前端行为：支持 1/5/15/30 分钟 和 1/3/6/12/24 小时，并支持自定义分钟
- 已验证：LAX 测试机迁移成功；设置 1 分钟后 next_probe_at 能推进到约 60 秒后；日本主服务器已通过蓝绿上线

### 8. 上游余额查询 new-api 适配

- 核心文件：[account_upstream_balance.go](D:/项目/subapi/backend/internal/service/account_upstream_balance.go)
- 适配目标：QuantumNous/new-api
- 新增候选接口：
  - {root}/dashboard/billing/subscription
  - {root}/dashboard/billing/usage
  - {apiRoot}/dashboard/billing/subscription
  - {apiRoot}/dashboard/billing/usage
- 查询规则：
  - 继续按 base_url 聚合，每个上游只选一个代表账号查询
  - 鉴权仍使用账号现有 api_key，请求头为 Authorization: Bearer key
  - 不需要网页登录态、cookie 或管理员 token
  - 组合查询 subscription.hard_limit_usd 和 usage.total_usage
  - 余额按 hard_limit_usd - total_usage / 100 计算
  - 单位标记为 upstream，表示按上游站点展示口径理解
- 安全边界：不存储 key，不存储完整响应体，错误信息继续脱敏和截断；查询失败只影响余额展示，不影响账号健康、调度、隔离状态
- 已验证：LAX mock new-api 返回 hard_limit_usd=100、total_usage=2500 时余额为 75；日本主服务器已随本次镜像蓝绿上线

### 9. 日本本地镜像蓝绿脚本

- 脚本文件：[japan-bluegreen-local-image.sh](D:\项目\subapi\deploy\japan-bluegreen-local-image.sh)
- 使用场景：本地构建镜像，上传到日本服务器，远端 docker load，然后使用服务器本地镜像 tag 做 8080/8081 蓝绿
- 关键行为：
  - 不执行 docker pull
  - 自动检测当前 nginx 活跃端口
  - 目标端口被运行容器占用时阻断
  - 自动备份 nginx 配置和容器 inspect
  - 显式挂载 /root/sub2api-deploy/data:/app/data
  - 切换前 smoke：/health、首页、/admin、无效登录
  - 切换后默认观察 60 秒
  - 切换后失败会自动恢复 nginx
  - 成功后停止旧容器但保留回滚点
- 默认域名验证可传：--public-health https://subapi.loucer.cn/health 和 --public-health https://jp.loucer.cn/health

## 这次新增确认的修复

### 生图成功但页面加载失败

根因不是上游没生成，而是嵌入式前端中间件把 `/p/img/...` 当成了 SPA 路由，直接返回了 `index.html`。

修复文件：

- [embed_on.go](D:\项目\subapi\backend\internal\web\embed_on.go)

修复内容：

- `shouldBypassEmbeddedFrontend()` 新增 `/p/` 旁路
- 这样 `/p/img/:image_id/:index` 会走后端真实图片路由，不再被首页吞掉

配套测试：

- [embed_test.go](D:\项目\subapi\backend\internal\web\embed_test.go)

已验证：

- `go test -tags embed ./internal/web/...`

## 当前已知事实

### 1. 路由层已修通

重建前：

- `GET /p/img/...` 被错误返回首页 HTML

重建后：

- `GET /p/img/...` 不再回首页 HTML
- 旧图片因为图片缓存是内存态，服务重建后返回 `404` 属于正常现象
- 新生成图片会重新进入这条正确链路

### 2. 本地真实生图链路仍然偏慢

这不是这次修出来的新问题，当前本地生图调用上游耗时依然很长，可能需要几十秒到几分钟。

所以：

- “页面按钮点了很久没出结果”
- 和“图片预览路径被首页吞掉”

这是两件不同的事。

本次已经修掉的是第二件。

## 后续推服务器前建议检查的最小清单

### 必查后端

- [embed_on.go](D:\项目\subapi\backend\internal\web\embed_on.go)
- [gateway_service.go](D:\项目\subapi\backend\internal\service\gateway_service.go)
- [subscription_quota_reset.go](D:\项目\subapi\backend\internal\service\subscription_quota_reset.go)
- [public_image.go](D:\项目\subapi\backend\internal\handler\public_image.go)
- [public_image_store.go](D:\项目\subapi\backend\internal\service\public_image_store.go)

### 必查前端

- [AppHeader.vue](D:\项目\subapi\frontend\src\components\layout\AppHeader.vue)
- [SubscriptionsView.vue](D:\项目\subapi\frontend\src\views\user\SubscriptionsView.vue)
- [ImageStudioView.vue](D:\项目\subapi\frontend\src\views\user\ImageStudioView.vue)

### 推送前验证

1. 订阅页续费按钮跳发卡网
2. OpenAI 订阅卡片的重置按钮跳分类 8
3. 刷新码兑换后只刷新额度，不加订阅天数
4. 生图接口返回后，结果图能直接显示
5. `/p/img/...` 返回真实图片或 `404`，不能再返回首页 HTML

## 备注

当前目录不是 git 仓库，所以这份“严格合并版”的真实基线，建议以后就以这个文档加当前目录文件为准，不要只靠页面版本号判断。
