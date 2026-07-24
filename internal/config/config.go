package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DNS              DNSConfig   `yaml:"dns"`
	Proxy            ProxyConfig `yaml:"proxy"`
	ProxyDomains     []string    `yaml:"proxy_domains"`
	ProxyDomainsFile string      `yaml:"proxy_domains_file"`
	AllowClients     []string    `yaml:"allow_clients"`
}

type DNSConfig struct {
	Listen     string `yaml:"listen"`
	UDPEnabled bool   `yaml:"udp_enabled"`
	Upstream   string `yaml:"upstream"`
	Network    string `yaml:"network"`
	ProxyIPv4  string `yaml:"proxy_ipv4"`
	ProxyIPv6  string `yaml:"proxy_ipv6"`
	TTL        uint32 `yaml:"ttl"`
}

type ProxyConfig struct {
	HTTPListen  string `yaml:"http_listen"`
	TLSListen   string `yaml:"tls_listen"`
	HTTPPort    int    `yaml:"http_target_port"`
	TLSPort     int    `yaml:"tls_target_port"`
	DialTimeout string `yaml:"dial_timeout"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if err := config.loadProxyDomainsFile(path); err != nil {
		return Config{}, err
	}
	config.setDefaults()
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config *Config) loadProxyDomainsFile(configPath string) error {
	if strings.TrimSpace(config.ProxyDomainsFile) == "" {
		return nil
	}

	domainsPath := strings.TrimSpace(config.ProxyDomainsFile)
	if !filepath.IsAbs(domainsPath) {
		domainsPath = filepath.Join(filepath.Dir(configPath), domainsPath)
	}
	file, err := os.Open(domainsPath)
	if err != nil {
		return fmt.Errorf("读取 proxy_domains_file %q 失败: %w", domainsPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		if line != "" {
			config.ProxyDomains = append(config.ProxyDomains, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 proxy_domains_file %q 失败: %w", domainsPath, err)
	}
	return nil
}

func (config *Config) setDefaults() {
	if config.DNS.Listen == "" {
		config.DNS.Listen = ":53"
	}
	if config.DNS.Upstream == "" {
		config.DNS.Upstream = "1.1.1.1:53"
	}
	if config.DNS.Network == "" {
		config.DNS.Network = "tcp"
	}
	if config.DNS.TTL == 0 {
		config.DNS.TTL = 60
	}
	if config.Proxy.HTTPListen == "" {
		config.Proxy.HTTPListen = ":80"
	}
	if config.Proxy.TLSListen == "" {
		config.Proxy.TLSListen = ":443"
	}
	if config.Proxy.HTTPPort == 0 {
		config.Proxy.HTTPPort = 80
	}
	if config.Proxy.TLSPort == 0 {
		config.Proxy.TLSPort = 443
	}
	if config.Proxy.DialTimeout == "" {
		config.Proxy.DialTimeout = "10s"
	}
}

func (config Config) validate() error {
	if config.DNS.Network != "tcp" && config.DNS.Network != "udp" {
		return fmt.Errorf("dns.network 必须是 tcp 或 udp")
	}
	if config.DNS.ProxyIPv4 == "" && config.DNS.ProxyIPv6 == "" {
		return fmt.Errorf("dns.proxy_ipv4 和 dns.proxy_ipv6 至少设置一个")
	}
	if config.DNS.ProxyIPv4 != "" && net.ParseIP(config.DNS.ProxyIPv4).To4() == nil {
		return fmt.Errorf("dns.proxy_ipv4 不是有效的 IPv4 地址")
	}
	if ip := net.ParseIP(config.DNS.ProxyIPv6); config.DNS.ProxyIPv6 != "" && (ip == nil || ip.To4() != nil) {
		return fmt.Errorf("dns.proxy_ipv6 不是有效的 IPv6 地址")
	}
	if len(config.ProxyDomains) == 0 {
		return fmt.Errorf("proxy_domains 和 proxy_domains_file 不能同时为空")
	}
	return nil
}
