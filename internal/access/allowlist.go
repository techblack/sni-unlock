package access

import (
	"fmt"
	"net"
	"strings"
)

type Allowlist struct {
	networks []*net.IPNet
}

func New(values []string) (*Allowlist, error) {
	allowlist := &Allowlist{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		ip := net.ParseIP(value)
		if ip != nil {
			bits := 128
			if ip.To4() != nil {
				ip = ip.To4()
				bits = 32
			}
			allowlist.networks = append(allowlist.networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("无效的客户端白名单 %q: %w", value, err)
		}
		allowlist.networks = append(allowlist.networks, network)
	}
	return allowlist, nil
}

func (allowlist *Allowlist) Allowed(address net.Addr) bool {
	if len(allowlist.networks) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	for _, network := range allowlist.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
