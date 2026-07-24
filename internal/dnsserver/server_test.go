package dnsserver

import (
	"net"
	"testing"

	"github.com/miekg/dns"

	"github.com/techblack/sni-unlock/internal/access"
	"github.com/techblack/sni-unlock/internal/config"
	"github.com/techblack/sni-unlock/internal/domains"
)

type responseWriter struct {
	message *dns.Msg
}

func (writer *responseWriter) LocalAddr() net.Addr { return &net.TCPAddr{} }
func (writer *responseWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 1234}
}
func (writer *responseWriter) WriteMsg(message *dns.Msg) error { writer.message = message; return nil }
func (writer *responseWriter) Write(data []byte) (int, error)  { return len(data), nil }
func (writer *responseWriter) Close() error                    { return nil }
func (writer *responseWriter) TsigStatus() error               { return nil }
func (writer *responseWriter) TsigTimersOnly(bool)             {}
func (writer *responseWriter) Hijack()                         {}

func TestProxyDomainResponse(t *testing.T) {
	allowlist, err := access.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := &Handler{
		config:    config.DNSConfig{ProxyIPv4: "203.0.113.10", TTL: 120},
		domains:   domains.New([]string{"example.com"}),
		allowlist: allowlist,
	}
	request := new(dns.Msg)
	request.SetQuestion("video.example.com.", dns.TypeA)
	writer := &responseWriter{}
	handler.ServeDNS(writer, request)

	if writer.message == nil || len(writer.message.Answer) != 1 {
		t.Fatalf("unexpected response: %#v", writer.message)
	}
	record, ok := writer.message.Answer[0].(*dns.A)
	if !ok || record.A.String() != "203.0.113.10" || record.Hdr.Ttl != 120 {
		t.Fatalf("unexpected answer: %#v", writer.message.Answer[0])
	}
}

func TestUnauthorizedDNSClientGetsNoResponse(t *testing.T) {
	allowlist, err := access.New([]string{"198.51.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	handler := &Handler{config: config.DNSConfig{ProxyIPv4: "203.0.113.10"}, domains: domains.New([]string{"example.com"}), allowlist: allowlist}
	request := new(dns.Msg)
	request.SetQuestion("example.com.", dns.TypeA)
	writer := &responseWriter{}
	handler.ServeDNS(writer, request)
	if writer.message != nil {
		t.Fatal("unauthorized client received a response")
	}
}
