package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProxyDomainsFile(t *testing.T) {
	directory := t.TempDir()
	domainsPath := filepath.Join(directory, "proxy-domains.txt")
	if err := os.WriteFile(domainsPath, []byte("# streaming\nnetflix.com\n\n disneyplus.com # comment\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.yaml")
	configData := []byte("dns:\n  proxy_ipv4: 127.0.0.1\nproxy_domains:\n  - inline.example\nproxy_domains_file: proxy-domains.txt\n")
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"inline.example", "netflix.com", "disneyplus.com"}
	if !reflect.DeepEqual(config.ProxyDomains, expected) {
		t.Fatalf("ProxyDomains = %#v, want %#v", config.ProxyDomains, expected)
	}
}

func TestLoadAllowsFileOnlyProxyDomains(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "domains.txt"), []byte("example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(configPath, []byte("dns:\n  proxy_ipv4: 127.0.0.1\nproxy_domains_file: domains.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(configPath); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUDPEnabled(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	configData := []byte("dns:\n  proxy_ipv4: 127.0.0.1\n  udp_enabled: true\nproxy_domains:\n  - example.com\n")
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.DNS.UDPEnabled {
		t.Fatal("expected UDP DNS to be enabled")
	}
}
