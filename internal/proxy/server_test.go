package proxy

import (
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestReadTLSServerName(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		tlsClient := tls.Client(client, &tls.Config{ServerName: "video.example.com", InsecureSkipVerify: true})
		_ = tlsClient.SetDeadline(time.Now().Add(time.Second))
		_ = tlsClient.Handshake()
	}()

	serverName, initial, err := readTLSServerName(server)
	if err != nil {
		t.Fatal(err)
	}
	if serverName != "video.example.com" {
		t.Fatalf("server name = %q", serverName)
	}
	if len(initial) == 0 {
		t.Fatal("expected captured ClientHello")
	}
}

func TestReadHTTPHost(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	request := []byte("GET / HTTP/1.1\r\nHost: video.example.com\r\nConnection: close\r\n\r\n")
	go func() { _, _ = client.Write(request) }()
	host, initial, err := readHTTPHost(server)
	if err != nil {
		t.Fatal(err)
	}
	if host != "video.example.com" {
		t.Fatalf("host = %q", host)
	}
	if string(initial) != string(request) {
		t.Fatalf("captured request differs: %q", initial)
	}
}
