package service

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// InvoiceFileMaxBytes 是单个发票文件的大小上限。
const InvoiceFileMaxBytes = 10 << 20

const invoiceFileNameMaxRunes = 100

var (
	ErrInvoiceFileRequired    = infraerrors.BadRequest("INVOICE_FILE_REQUIRED", "invoice file is required")
	ErrInvoiceFileTooLarge    = infraerrors.BadRequest("INVOICE_FILE_TOO_LARGE", "invoice file exceeds 10 MB")
	ErrInvoiceFileNotPDF      = infraerrors.BadRequest("INVOICE_FILE_NOT_PDF", "only PDF invoice files are supported")
	ErrInvoiceFileNotFound    = infraerrors.NotFound("INVOICE_FILE_NOT_FOUND", "invoice file not found")
	ErrInvoiceFileUnavailable = infraerrors.ServiceUnavailable("INVOICE_FILE_UNAVAILABLE", "invoice file storage is not available")
	ErrInvoiceFileStatus      = infraerrors.Conflict("INVOICE_FILE_STATUS", "invoice file can only be uploaded while the request is pending, processing or completed")
)

// InvoiceFileStore 保存发票 PDF 的字节。key 由存储自己生成，调用方只负责记录；
// 业务代码只依赖这个接口，将来换成 S3 不用改调用处。
type InvoiceFileStore interface {
	Save(ctx context.Context, data []byte) (key string, err error)
	// Read 在文件不存在时返回 ErrInvoiceFileNotFound。
	Read(ctx context.Context, key string) ([]byte, error)
	// Delete 对不存在的 key 不报错。
	Delete(ctx context.Context, key string) error
}

// InvoiceFileMeta 是写入数据库的上传文件元数据。
type InvoiceFileMeta struct {
	Name      string
	Key       string
	Size      int64
	UpdatedAt time.Time
}

// validateInvoicePDF 按文件头判断是不是 PDF，不信任扩展名和客户端给的类型。
func validateInvoicePDF(data []byte) error {
	switch {
	case len(data) == 0:
		return ErrInvoiceFileRequired
	case len(data) > InvoiceFileMaxBytes:
		return ErrInvoiceFileTooLarge
	case !bytes.HasPrefix(data, []byte("%PDF-")):
		return ErrInvoiceFileNotPDF
	}
	return nil
}

// sanitizeInvoiceFileName 只用于展示和下载时的文件名：去掉路径、控制字符和不适合放进头部的符号，
// 保证以 .pdf 结尾并限制长度。磁盘上的文件名与它无关。
func sanitizeInvoiceFileName(raw string, requestID int64) string {
	fallback := fmt.Sprintf("invoice-%d.pdf", requestID)
	name := path.Base(strings.ReplaceAll(raw, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`"<>:*?|`, r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return fallback
	}
	if !strings.EqualFold(path.Ext(name), ".pdf") {
		name += ".pdf"
	}
	base := strings.TrimSpace(name[:len(name)-len(".pdf")])
	if base == "" {
		return fallback
	}
	if utf8.RuneCountInString(base) > invoiceFileNameMaxRunes {
		base = string([]rune(base)[:invoiceFileNameMaxRunes])
	}
	return base + ".pdf"
}
