-- 在线开票：开票申请、申请与订单的占用关系、管理员手动排除的订单。
CREATE TABLE IF NOT EXISTS invoice_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    invoice_type VARCHAR(20) NOT NULL CHECK (invoice_type IN ('enterprise', 'individual')),
    title VARCHAR(200) NOT NULL,
    enterprise_tax_number VARCHAR(20),
    enterprise_address VARCHAR(255),
    enterprise_phone VARCHAR(50),
    enterprise_bank VARCHAR(100),
    enterprise_bank_account VARCHAR(32),
    individual_id_number_encrypted TEXT,
    individual_id_number_masked VARCHAR(32),
    total_amount DECIMAL(20,2) NOT NULL,
    order_count INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'rejected')),
    receiver_email VARCHAR(255) NOT NULL,
    invoice_number VARCHAR(64),
    invoice_file_url TEXT,
    email_sent_at TIMESTAMPTZ,
    admin_notes TEXT,
    reject_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoice_requests_user_created ON invoice_requests(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_status_created ON invoice_requests(status, created_at DESC);

CREATE TABLE IF NOT EXISTS invoice_request_orders (
    request_id BIGINT NOT NULL REFERENCES invoice_requests(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    invoiced_amount DECIMAL(20,2) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (request_id, order_id)
);

-- 一笔订单同一时间只能被一个未驳回的申请占用；并发提交靠这条约束兜底。
CREATE UNIQUE INDEX IF NOT EXISTS uq_invoice_request_orders_active_order
    ON invoice_request_orders(order_id) WHERE active;

CREATE TABLE IF NOT EXISTS invoice_order_exclusions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE invoice_order_exclusions IS '已在系统外开过票的订单，用户不能再次申请开票';
COMMENT ON COLUMN invoice_requests.individual_id_number_encrypted IS 'AES-GCM ciphertext; responses to users only expose the masked value';
