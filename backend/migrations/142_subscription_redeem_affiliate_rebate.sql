-- 订阅卡邀请返利：给订阅兑换码增加返利基数，并记录返利来源卡密。

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS affiliate_rebate_base_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

COMMENT ON COLUMN redeem_codes.affiliate_rebate_base_amount IS '订阅卡邀请返利基数；兑换成功后按该金额乘以返利比例发放站内余额返利，0 表示不返利';

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE SET NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_redeem_code_id IS '产生该返利流水的订阅兑换码；余额订单返利为 NULL';

CREATE UNIQUE INDEX IF NOT EXISTS idx_ual_redeem_rebate_once
    ON user_affiliate_ledger(source_redeem_code_id)
    WHERE action = 'accrue' AND source_redeem_code_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_affiliate_rebate_base
    ON redeem_codes(affiliate_rebate_base_amount)
    WHERE affiliate_rebate_base_amount > 0;
