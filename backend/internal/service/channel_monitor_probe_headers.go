package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	ChannelMonitorHeaderTimestamp        = "X-Sub2API-Monitor-Timestamp"
	ChannelMonitorHeaderSignature        = "X-Sub2API-Monitor-Signature"
	ChannelMonitorHeaderExcludeAccounts  = "X-Sub2API-Monitor-Exclude-Accounts"
	ChannelMonitorHeaderSelectedAccount  = "X-Sub2API-Monitor-Account-ID"
	channelMonitorSignatureMaxSkew       = 2 * time.Minute
	channelMonitorSignatureMessagePrefix = "channel-monitor"
)

type ChannelMonitorSigner struct {
	secret []byte
}

func NewChannelMonitorSigner() *ChannelMonitorSigner {
	secret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		panic("channel monitor signer random failed: " + err.Error())
	}
	return &ChannelMonitorSigner{secret: secret}
}

func (s *ChannelMonitorSigner) SignHeaders(headers http.Header, excludedAccountIDs []int64, now time.Time) {
	if s == nil || len(s.secret) == 0 || headers == nil {
		return
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	excluded := formatMonitorAccountIDs(excludedAccountIDs)
	headers.Set(ChannelMonitorHeaderTimestamp, ts)
	headers.Set(ChannelMonitorHeaderSignature, s.signature(ts, excluded))
	if excluded != "" {
		headers.Set(ChannelMonitorHeaderExcludeAccounts, excluded)
	} else {
		headers.Del(ChannelMonitorHeaderExcludeAccounts)
	}
}

func (s *ChannelMonitorSigner) SignHeaderMap(headers map[string]string, excludedAccountIDs []int64, now time.Time) {
	if s == nil || len(s.secret) == 0 || headers == nil {
		return
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	excluded := formatMonitorAccountIDs(excludedAccountIDs)
	headers[ChannelMonitorHeaderTimestamp] = ts
	headers[ChannelMonitorHeaderSignature] = s.signature(ts, excluded)
	if excluded != "" {
		headers[ChannelMonitorHeaderExcludeAccounts] = excluded
	} else {
		delete(headers, ChannelMonitorHeaderExcludeAccounts)
	}
}

func (s *ChannelMonitorSigner) VerifyRequest(headers http.Header, remoteAddr string, now time.Time) ([]int64, bool) {
	if !isLocalMonitorRemoteAddr(remoteAddr) {
		return nil, false
	}
	if s == nil || len(s.secret) == 0 || headers == nil {
		return nil, false
	}
	ts := strings.TrimSpace(headers.Get(ChannelMonitorHeaderTimestamp))
	sig := strings.TrimSpace(headers.Get(ChannelMonitorHeaderSignature))
	if ts == "" || sig == "" {
		return nil, false
	}
	parsedTS, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, false
	}
	reqTime := time.Unix(parsedTS, 0)
	if now.Sub(reqTime) > channelMonitorSignatureMaxSkew || reqTime.Sub(now) > channelMonitorSignatureMaxSkew {
		return nil, false
	}
	excludedRaw := strings.TrimSpace(headers.Get(ChannelMonitorHeaderExcludeAccounts))
	want := s.signature(ts, excludedRaw)
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil, false
	}
	return parseMonitorAccountIDs(excludedRaw), true
}

func (s *ChannelMonitorSigner) signature(ts, excluded string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(channelMonitorSignatureMessagePrefix))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(excluded))
	return hex.EncodeToString(mac.Sum(nil))
}

func formatMonitorAccountIDs(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

func parseMonitorAccountIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func isLocalMonitorRemoteAddr(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func selectedMonitorAccountID(headers http.Header) int64 {
	if headers == nil {
		return 0
	}
	id, err := strconv.ParseInt(strings.TrimSpace(headers.Get(ChannelMonitorHeaderSelectedAccount)), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func monitorProbeLogFields(excluded []int64) string {
	if len(excluded) == 0 {
		return ""
	}
	return fmt.Sprintf("excluded_accounts=%s", formatMonitorAccountIDs(excluded))
}
