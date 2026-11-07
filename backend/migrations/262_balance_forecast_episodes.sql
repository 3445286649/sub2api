-- 余额耗尽预测：记录每个用户的一轮预测告警。
-- 邮件去重由通知邮件服务的投递键负责（用本表 id 作 ReminderKey），
-- 本表只记录这一轮是否已开、是否已发、何时关闭。
-- 同一用户同一时间最多一轮 open；余额回升或充值后关闭，之后再次预测耗尽才开新一轮。
CREATE TABLE IF NOT EXISTS balance_forecast_episodes (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status                  VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    balance_at_open         DECIMAL(20, 10) NOT NULL DEFAULT 0,
    daily_burn_at_open      DECIMAL(20, 10) NOT NULL DEFAULT 0,
    hours_remaining_at_open DECIMAL(12, 2) NOT NULL DEFAULT 0,
    notified_at             TIMESTAMPTZ,
    opened_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at               TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_balance_forecast_episodes_open_user
    ON balance_forecast_episodes (user_id)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_balance_forecast_episodes_status_opened
    ON balance_forecast_episodes (status, opened_at);

COMMENT ON TABLE balance_forecast_episodes IS '余额耗尽预测的告警轮次，每个用户同时最多一条 open';
COMMENT ON COLUMN balance_forecast_episodes.hours_remaining_at_open IS '开轮时按近期消耗速度估算的剩余小时数';
COMMENT ON COLUMN balance_forecast_episodes.notified_at IS '预测邮件发出的时间，为空表示尚未发送或发送失败待重试';
