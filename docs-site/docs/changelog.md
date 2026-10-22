# 更新记录

## 2026-10-22

- 新增 [找到配置文件](/config-files)：Codex、Claude Code 配置目录在 Windows / macOS / Linux 上的打开方法。
- CCSwitch、Codex 的下载与配置步骤按系统分栏，下载入口改为按钮。
- Codex 教程补充「模型目录」：远程目录与本地文件 `codex-models.json` 的区别和下载方法。
- 常见报错排查按状态码（400 / 401 / 403 / 404 / 429 / 5xx）分组，补充超时和联系售后要带的信息。
- 快速开始、CCSwitch、Codex 页首增加步骤概览。
- 客户端地址统一改为 `codeplan.codes`，与站内「使用密钥」弹窗一致；`subapi.loucer.cn` 作为备用地址继续可用。
- 更正 CCSwitch 导入说明：一次导入只配置 Key 分组对应的那一类客户端。
- 移除 Claude Code 页画成 OpenAI 兼容方式的错误示意图，换成弹窗实际截图。
- 站内截图全部重拍为 CodePlan 当前界面；创建密钥补充「厂商」一步；充值商城改为卡密充值流程。
- 新增「不登录查 Key 用量」说明；站内功能补充用量查询、模型广场。
- 更正报错码：未带 Key 为 `API_KEY_REQUIRED`，余额不足为 `INSUFFICIENT_BALANCE`。

## 2026-09-28

- 文档站更名为 **CodePlan 使用教程**，配色与主站统一。
- 两个访问地址 `codeplan.codes` 与 `subapi.loucer.cn` 指向同一站点，账户通用。
- CCSwitch 导入后的配置名变为 **CodePlan**（此前为 `loucer_api`）。
- 修正 Claude Code 手动配置：改为站内生成的 `ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN`。
- 修正 Codex 手动配置：改为站内生成的 `config.toml`。
- 修正余额悬浮窗 MSI 安装包的 SHA-256 校验值。
- Codex 生图 Skill 改为「复制提示词给 Codex 自动安装」。
- 常见报错排查补充 `403`、`429`、`502`/`524`、超时与「能回复但没有使用记录」。

## 2026-07-14

- 新增余额悬浮窗 v0.1.0 使用教程和 Windows、macOS 下载入口。

## 2026-07-12

- 新增售后群一群、二群二维码。

## 2026-07-09

- 新增 Codex 生图 Skill 接入教程，补充中转站配置和 2K 生图示例。

## 2026-07-08

- 整理为 VitePress 静态帮助中心结构，适合部署到独立文档域名。
