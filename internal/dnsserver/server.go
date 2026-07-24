package dnsserver

import (
	"log/slog"
	"net"

	"github.com/miekg/dns"

	"github.com/techblack/sni-unlock/internal/access"
	"github.com/techblack/sni-unlock/internal/config"
	"github.com/techblack/sni-unlock/internal/domains"
)

type Handler struct {
	config    config.DNSConfig
	domains   *domains.Matcher
	allowlist *access.Allowlist
	client    *dns.Client
}

func New(config config.DNSConfig, matcher *domains.Matcher, allowlist *access.Allowlist) *dns.Server {
	handler := &Handler{
		config:    config,
		domains:   matcher,
		allowlist: allowlist,
		client:    &dns.Client{Net: config.Network},
	}
	return &dns.Server{Addr: config.Listen, Net: "tcp", Handler: handler}
}

func (handler *Handler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	if !handler.allowlist.Allowed(writer.RemoteAddr()) {
		slog.Warn("拒绝未授权的 DNS 客户端", "client", writer.RemoteAddr())
		return
	}
	if len(request.Question) != 1 {
		response := new(dns.Msg)
		response.SetRcode(request, dns.RcodeFormatError)
		_ = writer.WriteMsg(response)
		return
	}

	question := request.Question[0]
	if !handler.domains.Match(question.Name) {
		handler.forward(writer, request)
		return
	}

	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	header := dns.RR_Header{Name: question.Name, Class: dns.ClassINET, Ttl: handler.config.TTL}
	switch question.Qtype {
	case dns.TypeA:
		if ip := net.ParseIP(handler.config.ProxyIPv4).To4(); ip != nil {
			header.Rrtype = dns.TypeA
			response.Answer = append(response.Answer, &dns.A{Hdr: header, A: ip})
		}
	case dns.TypeAAAA:
		if ip := net.ParseIP(handler.config.ProxyIPv6); ip != nil && ip.To4() == nil {
			header.Rrtype = dns.TypeAAAA
			response.Answer = append(response.Answer, &dns.AAAA{Hdr: header, AAAA: ip})
		}
	}
	if err := writer.WriteMsg(response); err != nil {
		slog.Error("写入 DNS 响应失败", "error", err)
	}
}

func (handler *Handler) forward(writer dns.ResponseWriter, request *dns.Msg) {
	response, _, err := handler.client.Exchange(request, handler.config.Upstream)
	if err != nil {
		slog.Error("上游 DNS 查询失败", "upstream", handler.config.Upstream, "error", err)
		response = new(dns.Msg)
		response.SetRcode(request, dns.RcodeServerFailure)
	}
	if err := writer.WriteMsg(response); err != nil {
		slog.Error("写入 DNS 响应失败", "error", err)
	}
}
