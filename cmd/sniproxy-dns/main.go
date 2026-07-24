package main

import (
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/techblack/sni-unlock/internal/access"
	"github.com/techblack/sni-unlock/internal/config"
	"github.com/techblack/sni-unlock/internal/dnsserver"
	"github.com/techblack/sni-unlock/internal/domains"
	"github.com/techblack/sni-unlock/internal/proxy"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本")
	flag.Parse()
	if *showVersion {
		println(version)
		return
	}

	serviceConfig, err := config.Load(*configPath)
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	allowlist, err := access.New(serviceConfig.AllowClients)
	if err != nil {
		slog.Error("加载服务白名单失败", "error", err)
		os.Exit(1)
	}
	matcher := domains.New(serviceConfig.ProxyDomains)

	dnsServices := dnsserver.New(serviceConfig.DNS, matcher, allowlist)
	httpProxy, err := proxy.New("http", serviceConfig.Proxy.HTTPListen, serviceConfig.Proxy.HTTPPort, serviceConfig.DNS, serviceConfig.Proxy, matcher, allowlist)
	if err != nil {
		slog.Error("创建 HTTP 代理失败", "error", err)
		os.Exit(1)
	}
	tlsProxy, err := proxy.New("tls", serviceConfig.Proxy.TLSListen, serviceConfig.Proxy.TLSPort, serviceConfig.DNS, serviceConfig.Proxy, matcher, allowlist)
	if err != nil {
		slog.Error("创建 TLS 代理失败", "error", err)
		os.Exit(1)
	}

	errorsChannel := make(chan error, len(dnsServices)+2)
	for _, dnsService := range dnsServices {
		go func() {
			slog.Info("DNS 监听已启动", "network", dnsService.Net, "address", serviceConfig.DNS.Listen)
			errorsChannel <- dnsService.ListenAndServe()
		}()
	}
	go func() { errorsChannel <- httpProxy.ListenAndServe() }()
	go func() { errorsChannel <- tlsProxy.ListenAndServe() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case received := <-signals:
		slog.Info("正在停止服务", "signal", received)
	case err := <-errorsChannel:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Error("服务异常退出", "error", err)
		}
	}
	for _, dnsService := range dnsServices {
		_ = dnsService.Shutdown()
	}
	_ = httpProxy.Close()
	_ = tlsProxy.Close()
}
