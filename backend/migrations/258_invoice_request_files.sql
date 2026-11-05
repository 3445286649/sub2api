-- 发票文件：管理员上传的 PDF 存在服务端私有目录，数据库只记录元数据。
-- 旧的 invoice_file_url 保留，历史记录和手动贴外部链接的情况不受影响。
ALTER TABLE invoice_requests
    ADD COLUMN IF NOT EXISTS invoice_file_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS invoice_file_key VARCHAR(64),
    ADD COLUMN IF NOT EXISTS invoice_file_size BIGINT;

COMMENT ON COLUMN invoice_requests.invoice_file_name IS '上传的发票文件原名，仅用于展示和下载时的文件名';
COMMENT ON COLUMN invoice_requests.invoice_file_key IS '磁盘上的随机存储名，不对外暴露';
COMMENT ON COLUMN invoice_requests.invoice_file_size IS '上传的发票文件大小，单位字节';
