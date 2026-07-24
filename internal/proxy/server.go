package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/techblack/sni-unlock/internal/access"
	"github.com/techblack/sni-unlock/internal/config"
	"github.com/techblack/sni-unlock/internal/domains"
)

type Server struct {
	protocol   string
	listen     string
	targetPort int
	timeout    time.Duration
	matcher    *domains.Matcher
	allowlist  *access.Allowlist
	resolver   *net.Resolver
	listener   net.Listener
}

func New(protocol string, listen string, targetPort int, dnsConfig config.DNSConfig, proxyConfig config.ProxyConfig, matcher *domains.Matcher, allowlist *access.Allowlist) (*Server, error) {
	timeout, err := time.ParseDuration(proxyConfig.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("无效的 proxy.dial_timeout: %w", err)
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: timeout}
			return dialer.DialContext(ctx, dnsConfig.Network, dnsConfig.Upstream)
		},
	}
	return &Server{protocol: protocol, listen: listen, targetPort: targetPort, timeout: timeout, matcher: matcher, allowlist: allowlist, resolver: resolver}, nil
}

func (server *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", server.listen)
	if err != nil {
		return err
	}
	server.listener = listener
	slog.Info("代理监听已启动", "protocol", server.protocol, "address", server.listen)
	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		go server.handle(connection)
	}
}

func (server *Server) Close() error {
	if server.listener == nil {
		return nil
	}
	return server.listener.Close()
}

func (server *Server) handle(client net.Conn) {
	defer client.Close()
	if !server.allowlist.Allowed(client.RemoteAddr()) {
		slog.Warn("拒绝未授权的代理客户端", "client", client.RemoteAddr(), "protocol", server.protocol)
		return
	}
	_ = client.SetReadDeadline(time.Now().Add(server.timeout))

	host, initial, err := server.readTarget(client)
	if err != nil {
		slog.Debug("读取代理目标失败", "client", client.RemoteAddr(), "protocol", server.protocol, "error", err)
		return
	}
	if !server.matcher.Match(host) {
		slog.Warn("拒绝不在代理名单中的目标", "client", client.RemoteAddr(), "host", host)
		return
	}

	upstream, err := server.dial(host)
	if err != nil {
		slog.Error("连接代理目标失败", "host", host, "error", err)
		return
	}
	defer upstream.Close()
	_ = client.SetReadDeadline(time.Time{})
	if _, err := upstream.Write(initial); err != nil {
		return
	}
	slog.Info("开始代理连接", "client", client.RemoteAddr(), "host", host, "protocol", server.protocol)
	pipe(client, upstream)
}

func (server *Server) readTarget(connection net.Conn) (string, []byte, error) {
	if server.protocol == "http" {
		return readHTTPHost(connection)
	}
	return readTLSServerName(connection)
}

func (server *Server) dial(host string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), server.timeout)
	defer cancel()
	addresses, err := server.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: server.timeout}
	var lastError error
	for _, address := range addresses {
		target := net.JoinHostPort(address.IP.String(), strconv.Itoa(server.targetPort))
		connection, err := dialer.DialContext(ctx, "tcp", target)
		if err == nil {
			return connection, nil
		}
		lastError = err
	}
	return nil, lastError
}

func readHTTPHost(connection net.Conn) (string, []byte, error) {
	var initial bytes.Buffer
	reader := bufio.NewReader(io.TeeReader(io.LimitReader(connection, 64*1024), &initial))
	request, err := http.ReadRequest(reader)
	if err != nil {
		return "", nil, err
	}
	host := request.Host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return "", nil, fmt.Errorf("HTTP Host 为空")
	}
	return host, initial.Bytes(), nil
}

func readTLSServerName(connection net.Conn) (string, []byte, error) {
	var initial, handshake bytes.Buffer
	handshakeLength := 0
	for initial.Len() < 256*1024 {
		header := make([]byte, 5)
		if _, err := io.ReadFull(connection, header); err != nil {
			return "", nil, err
		}
		if header[0] != 22 {
			return "", nil, fmt.Errorf("不是 TLS handshake 记录")
		}
		recordLength := int(binary.BigEndian.Uint16(header[3:5]))
		if recordLength == 0 || recordLength > 65535 {
			return "", nil, fmt.Errorf("无效的 TLS 记录长度")
		}
		payload := make([]byte, recordLength)
		if _, err := io.ReadFull(connection, payload); err != nil {
			return "", nil, err
		}
		initial.Write(header)
		initial.Write(payload)
		handshake.Write(payload)
		if handshakeLength == 0 && handshake.Len() >= 4 {
			data := handshake.Bytes()
			handshakeLength = 4 + (int(data[1]) << 16) + (int(data[2]) << 8) + int(data[3])
		}
		if handshakeLength > 0 && handshake.Len() >= handshakeLength {
			serverName, err := parseServerName(handshake.Bytes()[:handshakeLength])
			return serverName, initial.Bytes(), err
		}
	}
	return "", nil, fmt.Errorf("TLS ClientHello 超过大小限制")
}

func parseServerName(data []byte) (string, error) {
	if len(data) < 42 || data[0] != 1 {
		return "", fmt.Errorf("不是 TLS ClientHello")
	}
	offset := 4 + 2 + 32
	if offset >= len(data) {
		return "", io.ErrUnexpectedEOF
	}
	offset += 1 + int(data[offset])
	if offset+2 > len(data) {
		return "", io.ErrUnexpectedEOF
	}
	offset += 2 + int(binary.BigEndian.Uint16(data[offset:offset+2]))
	if offset >= len(data) {
		return "", io.ErrUnexpectedEOF
	}
	offset += 1 + int(data[offset])
	if offset+2 > len(data) {
		return "", io.ErrUnexpectedEOF
	}
	extensionsEnd := offset + 2 + int(binary.BigEndian.Uint16(data[offset:offset+2]))
	offset += 2
	if extensionsEnd > len(data) {
		return "", io.ErrUnexpectedEOF
	}
	for offset+4 <= extensionsEnd {
		extensionType := binary.BigEndian.Uint16(data[offset : offset+2])
		extensionLength := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4
		if offset+extensionLength > extensionsEnd {
			return "", io.ErrUnexpectedEOF
		}
		if extensionType == 0 {
			return parseServerNameExtension(data[offset : offset+extensionLength])
		}
		offset += extensionLength
	}
	return "", fmt.Errorf("ClientHello 不含 SNI")
}

func parseServerNameExtension(data []byte) (string, error) {
	if len(data) < 5 {
		return "", io.ErrUnexpectedEOF
	}
	listLength := int(binary.BigEndian.Uint16(data[:2]))
	if listLength+2 > len(data) {
		return "", io.ErrUnexpectedEOF
	}
	for offset := 2; offset+3 <= listLength+2; {
		nameType := data[offset]
		nameLength := int(binary.BigEndian.Uint16(data[offset+1 : offset+3]))
		offset += 3
		if offset+nameLength > len(data) {
			return "", io.ErrUnexpectedEOF
		}
		if nameType == 0 {
			return string(data[offset : offset+nameLength]), nil
		}
		offset += nameLength
	}
	return "", fmt.Errorf("SNI 主机名为空")
}

func pipe(client, upstream net.Conn) {
	done := make(chan struct{}, 2)
	copyConnection := func(destination, source net.Conn) {
		_, _ = io.Copy(destination, source)
		if tcp, ok := destination.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyConnection(upstream, client)
	go copyConnection(client, upstream)
	<-done
	<-done
}
