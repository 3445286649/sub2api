package authguard

import (
	"context"
	"testing"
)

func TestIPKeyIPv4AndMappedShareKey(t *testing.T) {
	plain := IPKey("203.0.113.7")
	if len(plain) != 64 {
		t.Fatalf("expected a 64 char hash, got %q", plain)
	}
	if mapped := IPKey("::ffff:203.0.113.7"); mapped != plain {
		t.Fatalf("mapped IPv4 must share the IPv4 key: %q vs %q", mapped, plain)
	}
	if other := IPKey("203.0.113.8"); other == plain {
		t.Fatal("different IPv4 addresses must not share a key")
	}
}

func TestIPKeyIPv6AggregatesToSlash64(t *testing.T) {
	a := IPKey("2001:db8:1:2:aaaa:bbbb:cccc:dddd")
	b := IPKey("2001:db8:1:2:1111:2222:3333:4444")
	if a == "" || a != b {
		t.Fatalf("hosts in one /64 must share a key: %q vs %q", a, b)
	}
	if c := IPKey("2001:db8:1:3::1"); c == a {
		t.Fatal("different /64 networks must not share a key")
	}
}

func TestIPKeyRejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"", "unknown", "not-an-ip", "fe80::1%eth0"} {
		if key := IPKey(value); key != "" {
			t.Fatalf("%q must have no key, got %q", value, key)
		}
	}
}

func TestRegistrationIPMarker(t *testing.T) {
	if _, enabled := RegistrationIP(context.Background()); enabled {
		t.Fatal("a plain context must not enable registration accounting")
	}
	key, enabled := RegistrationIP(WithRegistrationIP(context.Background(), "203.0.113.7"))
	if !enabled || key != IPKey("203.0.113.7") {
		t.Fatalf("marker not carried: enabled=%v key=%q", enabled, key)
	}
	// An invalid address keeps the marker with an empty key so persistence fails closed.
	key, enabled = RegistrationIP(WithRegistrationIP(context.Background(), "garbage"))
	if !enabled || key != "" {
		t.Fatalf("invalid address must keep the marker with an empty key: enabled=%v key=%q", enabled, key)
	}
}
