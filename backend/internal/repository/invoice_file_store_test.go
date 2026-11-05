//go:build unit

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLocalInvoiceFileStoreRoundTrip(t *testing.T) {
	store := newLocalInvoiceFileStore(t.TempDir())
	ctx := context.Background()

	key, err := store.Save(ctx, []byte("%PDF-1.7 invoice"))
	require.NoError(t, err)
	require.Regexp(t, invoiceFileKeyRE, key)

	data, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte("%PDF-1.7 invoice"), data)

	// 写入走临时文件再改名，目录里不该留下半成品。
	entries, err := os.ReadDir(store.directory)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, store.Delete(ctx, key))
	require.NoError(t, store.Delete(ctx, key), "deleting twice is not an error")
	_, err = store.Read(ctx, key)
	require.Equal(t, service.ErrInvoiceFileNotFound, err)
}

func TestLocalInvoiceFileStoreUsesDistinctKeys(t *testing.T) {
	store := newLocalInvoiceFileStore(t.TempDir())
	first, err := store.Save(context.Background(), []byte("%PDF-1.7 a"))
	require.NoError(t, err)
	second, err := store.Save(context.Background(), []byte("%PDF-1.7 a"))
	require.NoError(t, err)
	require.NotEqual(t, first, second)
}

func TestLocalInvoiceFileStoreIgnoresKeysThatEscapeTheDirectory(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "secret.pdf")
	require.NoError(t, os.WriteFile(secret, []byte("private"), 0o600))
	store := newLocalInvoiceFileStore(filepath.Join(root, "data"))
	ctx := context.Background()

	for _, key := range []string{
		"../../secret.pdf",
		`..\..\secret.pdf`,
		"",
		".upload-123",
		"ABCDEF0123456789ABCDEF0123456789.pdf",
		"0123456789abcdef0123456789abcdef.exe",
	} {
		_, err := store.Read(ctx, key)
		require.Equal(t, service.ErrInvoiceFileNotFound, err, key)
		require.NoError(t, store.Delete(ctx, key), key)
	}

	_, err := os.Stat(secret)
	require.NoError(t, err, "a hostile key must never delete files outside the store")
}
