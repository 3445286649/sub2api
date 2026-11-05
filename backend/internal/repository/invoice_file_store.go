package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 磁盘上的文件名只能是随机串加 .pdf；读取和删除前先校验，key 里带不进路径。
var invoiceFileKeyRE = regexp.MustCompile(`^[a-f0-9]{32}\.pdf$`)

// localInvoiceFileStore 把发票存到 <data_dir>/invoice-files。目录不对外提供静态访问，
// 下载只能走带登录和归属校验的接口。
type localInvoiceFileStore struct{ directory string }

func NewInvoiceFileStore(cfg *config.Config) service.InvoiceFileStore {
	dataDir := ""
	if cfg != nil {
		dataDir = cfg.Pricing.DataDir
	}
	return newLocalInvoiceFileStore(dataDir)
}

func newLocalInvoiceFileStore(dataDir string) *localInvoiceFileStore {
	if dataDir == "" {
		dataDir = "data"
	}
	return &localInvoiceFileStore{directory: filepath.Join(dataDir, "invoice-files")}
}

// Save 先写临时文件再改名，读取方不会看到写了一半的文件。
func (s *localInvoiceFileStore) Save(_ context.Context, data []byte) (string, error) {
	if err := os.MkdirAll(s.directory, 0o750); err != nil {
		return "", err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	key := hex.EncodeToString(id[:]) + ".pdf"

	tmp, err := os.CreateTemp(s.directory, ".upload-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, filepath.Join(s.directory, key)); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return key, nil
}

func (s *localInvoiceFileStore) Read(_ context.Context, key string) ([]byte, error) {
	if !invoiceFileKeyRE.MatchString(key) {
		return nil, service.ErrInvoiceFileNotFound
	}
	data, err := os.ReadFile(filepath.Join(s.directory, key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, service.ErrInvoiceFileNotFound
		}
		return nil, err
	}
	return data, nil
}

func (s *localInvoiceFileStore) Delete(_ context.Context, key string) error {
	if !invoiceFileKeyRE.MatchString(key) {
		return nil
	}
	if err := os.Remove(filepath.Join(s.directory, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
