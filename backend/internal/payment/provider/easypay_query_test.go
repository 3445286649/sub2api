package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayQueryOrderStatusMapping(t *testing.T) {
	t.Parallel()

	const orderID = "order-123"
	tests := []struct {
		name        string
		body        string
		wantStatus  string
		wantTradeNo string
		wantAmount  float64
		wantError   bool
		httpStatus  int
	}{
		{
			name:        "top level trade success is paid",
			body:        `{"code":1,"trade_status":"TRADE_SUCCESS","status":0,"money":"12.34","trade_no":"gateway-123"}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "gateway-123",
			wantAmount:  12.34,
		},
		{
			name:        "waiting trade status with paid numeric status stays pending",
			body:        `{"code":1,"trade_status":"WAITING","status":1,"money":"12.34","trade_no":"gateway-123"}`,
			wantStatus:  payment.ProviderStatusPending,
			wantTradeNo: "gateway-123",
			wantAmount:  12.34,
		},
		{
			name:       "empty trade status with paid numeric status stays pending",
			body:       `{"code":1,"trade_status":"","status":1,"money":"12.34"}`,
			wantStatus: payment.ProviderStatusPending,
			wantAmount: 12.34,
		},
		{
			name:        "nested data trade success is paid",
			body:        `{"code":1,"data":{"trade_status":"TRADE_SUCCESS","status":0,"money":"9.99","trade_no":"data-456"}}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "data-456",
			wantAmount:  9.99,
		},
		{
			name:        "legacy numeric paid status remains compatible",
			body:        `{"code":1,"status":1,"money":"3.21","trade_no":"legacy-platform-123"}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "legacy-platform-123",
			wantAmount:  3.21,
		},
		{
			name:       "legacy numeric non paid status is pending",
			body:       `{"code":1,"status":0,"money":"3.21"}`,
			wantStatus: payment.ProviderStatusPending,
			wantAmount: 3.21,
		},
		{
			name:      "query failure is rejected",
			body:      `{"code":0,"msg":"order not found"}`,
			wantError: true,
		},
		{
			name:      "query failure cannot claim paid",
			body:      `{"code":0,"status":1,"money":"3.21","trade_no":"gateway-123"}`,
			wantError: true,
		},
		{
			name:      "missing result code is rejected",
			body:      `{}`,
			wantError: true,
		},
		{
			name:      "paid without platform trade no is rejected",
			body:      `{"code":1,"trade_status":"TRADE_SUCCESS","money":"3.21"}`,
			wantError: true,
		},
		{
			name:      "paid with blank platform trade no is rejected",
			body:      `{"code":1,"status":1,"money":"3.21","trade_no":" "}`,
			wantError: true,
		},
		{
			name:        "empty top-level trade no uses real nested platform trade no",
			body:        `{"code":1,"status":1,"money":"3.21","trade_no":"","data":{"trade_no":"nested-platform-123"}}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "nested-platform-123",
			wantAmount:  3.21,
		},
		{
			name:      "paid invalid money is rejected",
			body:      `{"code":1,"status":1,"money":"invalid","trade_no":"gateway-123"}`,
			wantError: true,
		},
		{
			name:      "paid NaN money is rejected",
			body:      `{"code":1,"status":1,"money":"NaN","trade_no":"gateway-123"}`,
			wantError: true,
		},
		{
			name:      "paid infinity money is rejected",
			body:      `{"code":1,"status":1,"money":"+Inf","trade_no":"gateway-123"}`,
			wantError: true,
		},
		{
			name:       "HTTP failure cannot claim paid",
			body:       `{"code":1,"status":1,"money":"3.21","trade_no":"gateway-123"}`,
			httpStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotForm url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
				}
				if r.URL.Path != "/api.php" {
					t.Errorf("path = %q, want /api.php", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				gotForm = make(url.Values, len(r.PostForm))
				for key, values := range r.PostForm {
					gotForm[key] = append([]string(nil), values...)
				}
				w.Header().Set("Content-Type", "application/json")
				if tt.httpStatus != 0 {
					w.WriteHeader(tt.httpStatus)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			provider := newTestEasyPay(t, server.URL)
			resp, err := provider.QueryOrder(context.Background(), orderID)
			if tt.wantError {
				if err == nil {
					t.Fatalf("QueryOrder accepted invalid response: %+v", resp)
				}
				return
			}
			if err != nil {
				t.Fatalf("QueryOrder returned error: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q (response=%+v)", resp.Status, tt.wantStatus, resp)
			}
			if resp.TradeNo != tt.wantTradeNo {
				t.Fatalf("trade_no = %q, want %q", resp.TradeNo, tt.wantTradeNo)
			}
			if resp.Amount != tt.wantAmount {
				t.Fatalf("amount = %v, want %v", resp.Amount, tt.wantAmount)
			}
			for key, want := range map[string]string{
				"act":          "order",
				"pid":          "pid-1",
				"key":          "pkey-1",
				"out_trade_no": orderID,
			} {
				if got := gotForm.Get(key); got != want {
					t.Fatalf("form[%s] = %q, want %q (form=%v)", key, got, want, gotForm)
				}
			}
		})
	}
}
