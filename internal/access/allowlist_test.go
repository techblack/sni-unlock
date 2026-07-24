package access

import (
	"net"
	"testing"
)

func TestAllowlist(t *testing.T) {
	allowlist, err := New([]string{"192.0.2.10", "2001:db8::/32"})
	if err != nil {
		t.Fatal(err)
	}
	if !allowlist.Allowed(&net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 1}) {
		t.Fatal("expected exact IPv4 address to be allowed")
	}
	if !allowlist.Allowed(&net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 1}) {
		t.Fatal("expected IPv6 network address to be allowed")
	}
	if allowlist.Allowed(&net.TCPAddr{IP: net.ParseIP("192.0.2.11"), Port: 1}) {
		t.Fatal("unexpected address allowed")
	}
}
