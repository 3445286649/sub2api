# Grok 审核拒绝费

渠道 → 编辑 → 基础设置 → **Grok 内容审核拒绝费**。默认关闭。

示例配置（渠道 `features_config`）：

```json
{
  "grok_rejection_billing": {
    "enabled": true,
    "source_id": "supe-ai",
    "allowed_origins": ["https://supeai.cc"],
    "amount_mode": "upstream",
    "fixed_amount_usd": ""
  }
}
```

`source_id` 是永久的上游收费编号命名空间。同一供应商的多个账号/渠道必须使用相同值，不能随渠道改名、密钥轮换而改变。可信地址严格匹配 HTTPS origin，不含路径；不自动信任子域名或重定向目标。

`amount_mode=upstream` 按每条 `charged_amount` 扣款；`fixed` 使用字符串形式的 `fixed_amount_usd` 覆盖每条用户扣款金额。两种模式都必须有编号、有效正数金额和 `billing_status=charged`，都不加倍率，不对汇总字段收费。支持最多 8 位小数；拒绝无法被现有账务金额字段无损表示的值。上游实际金额和用户金额分别保存，首次有效凭据冻结用户价格，改配置不会重算旧账。

拒绝费独立扣钱包，订阅用户也不抵扣套餐。可信渠道的新 POST 请求需要正钱包余额；请求已发出后的费用可以记为欠费。成功结算同步清理余额及 API Key 认证/限额缓存。拒绝费不参与消费返利。

覆盖 OpenAI 兼容 Responses、Chat Completions、Messages 转换、图片及 Grok/OpenAI 视频 HTTP 路径。JSON 和 SSE 在错误映射、ping 过滤、协议转换前检查。只读取协议根字段及 `response`/`error` 包装，不遍历模型生成文本。`normal_billing=replaced` 仅跳过当前上游尝试的普通费用；视频任务的替代状态持久化，下一次独立正常请求不继承。

`grok_rejection_inbox` 是持久化接收队列：确认凭据的结算失败每 30 秒由恢复器检查并重试，单条失败至少退避 1 分钟。恢复器只重试本地结算，不重发上游生成请求。`grok_rejection_receipts` 的主键是 `(source_id,billing_request_id)`；凭据、余额/额度、使用记录在同一事务提交。此表及 `grok_rejection_tasks` 不随用量清理、Redis 过期或账号删除清理。

同号同用户同上游金额不重扣；同号金额/归属冲突进入 `conflict`。`unconfirmed`、缺失/非法凭据、拒绝无凭据及中断响应保存在 `review`，不猜金额扣款。视频查询必须有已持久化的创建归属；启用前创建的历史任务缺少绑定时需要核对迁移，不能默认归属于查询者。

管理员可按状态查账（只用于内部排查，不对普通用户暴露他人记录）：

```sql
SELECT state, COUNT(*) FROM grok_rejection_inbox GROUP BY state;
SELECT event_key, state, attempts, created_at,
       payload->>'source_id' AS source_id,
       payload->'receipt' AS receipt,
       payload->'usage'->>'UserID' AS user_id
FROM grok_rejection_inbox
WHERE state IN ('pending','review','conflict')
ORDER BY created_at DESC LIMIT 100;
```

如果数据库在接收凭据时已经不可用，响应处理报错并记录 `grok_rejection_inbox_failed` 及最小财务凭据，需依据该日志与上游账单核对；不把未持久化的凭据当作已结算。如果上游扣款后凭据完全未送达，必须依赖上游账单/补发，不能凭网络错误推定金额。公开 `https://supeai.cc/` 在本次核查返回 403，尚无真实收费 JSON/SSE 样例，模拟按需求给定协议构造。

模拟与事务验证：

```sh
cd backend
go test ./internal/service -run '^TestGrokRejection' -count=1
go test -tags integration ./internal/repository -run 'TestGrokRejection|TestUsageBillingRepositoryApply' -count=1 -v
```

数据库测试必须有 Docker，检查输出中确实执行测试，不能把 harness 的 `skipping integration tests` 当成通过。覆盖并发重复、跨来源/跨用户、固定金额、未确认升级、使用记录清理、事务回滚、恢复及视频归属。

发布：仅新增迁移 `257_grok_rejection_billing.sql`；先备份，关闭开关部署候选实例并健康检查，确认实际 Supe AI 账号所用分组/渠道后启用，再切流。保留旧镜像及 nginx 配置。回退前关闭开关、排空请求并核对 pending；旧版不支持拒绝费，不能在回退后继续将受影响流量发送上游却声称仍会代扣。永远保留永久凭据表，不通过删表或回滚整库撤销发布。
