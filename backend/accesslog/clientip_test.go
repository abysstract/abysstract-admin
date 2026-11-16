package accesslog_test

import (
	"net/http"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
)

func TestClientIP(t *testing.T) {
	h := http.Header{}
	h.Set("X-Forwarded-For", " 198.51.100.7 , 203.0.113.4")
	if got := accesslog.ClientIP(h, "10.0.0.1:80", true); got != "203.0.113.4" {
		t.Fatalf("trusted = %q, want the entry added by the proxy", got)
	}
	if got := accesslog.ClientIP(h, "10.0.0.1:80", false); got != "10.0.0.1" {
		t.Fatalf("untrusted = %q", got)
	}
	if got := accesslog.ClientIP(http.Header{}, "pipe", true); got != "pipe" {
		t.Fatalf("no port = %q", got)
	}
	if got := accesslog.ClientIP(http.Header{}, "[2001:db8::1]:443", false); got != "2001:db8::1" {
		t.Fatalf("ipv6 peer = %q", got)
	}
}

func TestProxyPolicyClientIP(t *testing.T) {
	proxies, err := accesslog.ParseProxies("10.0.0.0/8, 192.0.2.10")
	if err != nil {
		t.Fatal(err)
	}
	policy := accesslog.ProxyPolicy{Proxies: proxies}
	cases := []struct {
		name string
		xff  []string
		peer string
		want string
	}{
		{"forged left entry is skipped", []string{"1.1.1.1, 198.51.100.7, 10.1.2.3"}, "192.0.2.10:443", "198.51.100.7"},
		{"untrusted peer ignores header", []string{"198.51.100.7"}, "203.0.113.9:443", "203.0.113.9"},
		{"separate header lines are joined", []string{"1.1.1.1", "198.51.100.7"}, "10.0.0.2:443", "198.51.100.7"},
		{"garbage stops at last vouched address", []string{"evil, 10.0.0.9"}, "10.0.0.2:443", "10.0.0.9"},
		{"all trusted falls back to left-most", []string{"10.0.0.8"}, "10.0.0.2:443", "10.0.0.8"},
		{"no header uses peer", nil, "10.0.0.2:443", "10.0.0.2"},
		{"mapped ipv4 is unmapped", []string{"::ffff:198.51.100.7"}, "10.0.0.2:443", "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for _, v := range tc.xff {
				h.Add("X-Forwarded-For", v)
			}
			if got := policy.ClientIP(h, tc.peer); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseProxiesRejectsGarbage(t *testing.T) {
	if _, err := accesslog.ParseProxies("10.0.0.0/8,nope"); err == nil {
		t.Fatal("expected error")
	}
	got, err := accesslog.ParseProxies(" ")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v", got, err)
	}
}
