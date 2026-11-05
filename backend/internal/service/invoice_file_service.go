package service

import (
	"context"
	"log/slog"
	"time"
)

// UploadFile 保存管理员上传的发票 PDF。重复上传会替换旧文件；旧文件在数据库更新成功之后才删，
// 中途失败时旧文件仍然可用，新写入的文件会被清掉。
func (s *InvoiceService) UploadFile(ctx context.Context, id int64, filename string, data []byte) (*InvoiceRequest, error) {
	if s.files == nil {
		return nil, ErrInvoiceFileUnavailable
	}
	if err := validateInvoicePDF(data); err != nil {
		return nil, err
	}
	// 先确认申请存在，避免为不存在的申请写盘。
	if _, err := s.repo.GetRequest(ctx, id); err != nil {
		return nil, err
	}

	key, err := s.files.Save(ctx, data)
	if err != nil {
		return nil, err
	}
	previous, err := s.repo.SetFile(ctx, id, InvoiceFileMeta{
		Name:      sanitizeInvoiceFileName(filename, id),
		Key:       key,
		Size:      int64(len(data)),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		if delErr := s.files.Delete(ctx, key); delErr != nil {
			slog.Warn("invoice file cleanup failed after rejected upload", "request_id", id, "error", delErr)
		}
		return nil, err
	}
	if previous != nil && *previous != key {
		if delErr := s.files.Delete(ctx, *previous); delErr != nil {
			slog.Warn("replaced invoice file could not be deleted", "request_id", id, "error", delErr)
		}
	}
	return s.GetForAdmin(ctx, id)
}

// DownloadFileForUser 只允许申请人下载自己已开具的发票。他人的申请与不存在的申请一样返回 not found，
// 不暴露它是否存在。
func (s *InvoiceService) DownloadFileForUser(ctx context.Context, userID, id int64) (string, []byte, error) {
	req, err := s.GetForUser(ctx, userID, id)
	if err != nil {
		return "", nil, err
	}
	if req.Status != InvoiceStatusCompleted {
		return "", nil, ErrInvoiceFileNotFound
	}
	return s.readInvoiceFile(ctx, req)
}

// DownloadFileForAdmin 管理员可以下载任何状态下已上传的文件，用于开票前核对。
func (s *InvoiceService) DownloadFileForAdmin(ctx context.Context, id int64) (string, []byte, error) {
	req, err := s.repo.GetRequest(ctx, id)
	if err != nil {
		return "", nil, err
	}
	return s.readInvoiceFile(ctx, req)
}

func (s *InvoiceService) readInvoiceFile(ctx context.Context, req *InvoiceRequest) (string, []byte, error) {
	if s.files == nil {
		return "", nil, ErrInvoiceFileUnavailable
	}
	if req.InvoiceFileKey == nil {
		return "", nil, ErrInvoiceFileNotFound
	}
	data, err := s.files.Read(ctx, *req.InvoiceFileKey)
	if err != nil {
		return "", nil, err
	}
	name := sanitizeInvoiceFileName("", req.ID)
	if req.InvoiceFileName != nil && *req.InvoiceFileName != "" {
		name = *req.InvoiceFileName
	}
	return name, data, nil
}
