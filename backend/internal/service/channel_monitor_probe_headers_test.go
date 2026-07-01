//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"
)

func TestVerifyChannelMonitorRequestAcceptsSignedPrivateRequest(t *testing.T) {
	now := time.Unix(1700000000, 0)
	signer := NewChannelMonitorSigner()
	h := http.Header{}
	signer.SignHeaders(h, []int64{774, 751}, now)

	got, ok := signer.VerifyRequest(h, "127.0.0.1:45678", now)
	if !ok {
		t.Fatal("expected signed local request to verify")
	}
	if len(got) != 2 || got[0] != 774 || got[1] != 751 {
		t.Fatalf("unexpected excluded accounts: %#v", got)
	}
}

func TestVerifyChannelMonitorRequestRejectsPublicRemoteAddr(t *testing.T) {
	now := time.Unix(1700000000, 0)
	signer := NewChannelMonitorSigner()
	h := http.Header{}
	signer.SignHeaders(h, []int64{774}, now)

	if _, ok := signer.VerifyRequest(h, "8.8.8.8:45678", now); ok {
		t.Fatal("expected public remote addr to be rejected")
	}
}

func TestVerifyChannelMonitorRequestRejectsTamperedExcludeList(t *testing.T) {
	now := time.Unix(1700000000, 0)
	signer := NewChannelMonitorSigner()
	h := http.Header{}
	signer.SignHeaders(h, []int64{774}, now)
	h.Set(ChannelMonitorHeaderExcludeAccounts, "751")

	if _, ok := signer.VerifyRequest(h, "127.0.0.1:45678", now); ok {
		t.Fatal("expected tampered exclude list to be rejected")
	}
}
