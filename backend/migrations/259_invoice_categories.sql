-- 发票类目：管理员配置用户申请普票时可选的开票类目。
-- 申请表只新增可空列，历史申请不补造类目；没配置类目时现有流程不受影响。
CREATE TABLE IF NOT EXISTS invoice_categories (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS invoice_categories_name_key ON invoice_categories (name);

COMMENT ON TABLE invoice_categories IS '管理员配置的开票类目，用户申请发票时下拉选择';
COMMENT ON COLUMN invoice_categories.description IS '展示给用户的类目说明，可为空';
COMMENT ON COLUMN invoice_categories.enabled IS '停用后用户不再能选，已提交的申请不受影响';

-- 已被申请引用的类目不能删除（RESTRICT），只能停用。
ALTER TABLE invoice_requests
    ADD COLUMN IF NOT EXISTS category_id BIGINT REFERENCES invoice_categories (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS category_name VARCHAR(100);

CREATE INDEX IF NOT EXISTS invoice_requests_category_id_idx ON invoice_requests (category_id) WHERE category_id IS NOT NULL;

COMMENT ON COLUMN invoice_requests.category_id IS '申请时选择的开票类目，历史申请为空';
COMMENT ON COLUMN invoice_requests.category_name IS '提交时的类目名称快照，类目改名不影响已提交的申请';
