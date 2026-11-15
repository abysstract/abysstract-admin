package accesslog

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// HeaderGetter is satisfied by http.Header and *connect.Header.
type HeaderGetter interface {
	Get(key string) string
}

// headerValues is implemented by http.Header and *connect.Header. It lets
// ClientIP read every X-Forwarded-For line instead of only the first, which a
// client controls when a proxy adds its own line.
type headerValues interface {
	Values(key string) []string
}

// ProxyPolicy says which peers may report the client address in
// X-Forwarded-For.
//
// Proxies append the address they received the request from to the right
// end of X-Forwarded-For, after whatever the client sent. Only the entries
// added by trusted proxies are reliable, so the client address is found by
// walking the list from the right:
//
//   - The zero value ignores X-Forwarded-For and uses the peer address.
//   - Trust without Proxies trusts exactly one proxy in front of the
//     application: the peer. The right-most entry is the client.
//   - With Proxies, X-Forwarded-For is read only when the peer is in one of
//     the prefixes; entries inside the prefixes are skipped from the right,
//     and the first address outside them is the client. Trust is implied.
type ProxyPolicy struct {
	Trust   bool
	Proxies []netip.Prefix
}

// ParseProxies parses a comma-separated list of CIDR prefixes or single
// addresses, such as "10.0.0.0/8, 127.0.0.1".
func ParseProxies(list string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, fmt.Errorf("invalid proxy prefix %q: %w", item, err)
			}
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy address %q: %w", item, err)
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}

// ClientIP returns the client address of a request under the policy.
func (p ProxyPolicy) ClientIP(header HeaderGetter, peerAddr string) string {
	peer := peerHost(peerAddr)
	if (!p.Trust && len(p.Proxies) == 0) || header == nil {
		return peer
	}
	if len(p.Proxies) > 0 {
		addr, err := netip.ParseAddr(peer)
		if err != nil || !p.trusted(addr) {
			return peer
		}
	}
	hops := forwardedFor(header)
	if len(p.Proxies) == 0 {
		if n := len(hops); n > 0 {
			if addr, err := netip.ParseAddr(hops[n-1]); err == nil {
				return addr.Unmap().String()
			}
		}
		return peer
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(hops[i])
		if err != nil {
			// A trusted proxy forwarded garbage: keep the last address it
			// vouched for rather than trusting anything further left.
			return client
		}
		addr = addr.Unmap()
		client = addr.String()
		if !p.trusted(addr) {
			return client
		}
	}
	return client
}

func (p ProxyPolicy) trusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range p.Proxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ClientIP returns the client address of a request: with trustProxy, the
// right-most X-Forwarded-For entry (one trusted proxy in front of the
// application); otherwise the peer address without its port. Use
// ProxyPolicy for several proxies.
func ClientIP(header HeaderGetter, peerAddr string, trustProxy bool) string {
	return ProxyPolicy{Trust: trustProxy}.ClientIP(header, peerAddr)
}

func forwardedFor(header HeaderGetter) []string {
	var lines []string
	if multi, ok := header.(headerValues); ok {
		lines = multi.Values("X-Forwarded-For")
	} else if v := header.Get("X-Forwarded-For"); v != "" {
		lines = []string{v}
	}
	var hops []string
	for _, line := range lines {
		for _, part := range strings.Split(line, ",") {
			if part = strings.TrimSpace(part); part != "" {
				hops = append(hops, part)
			}
		}
	}
	return hops
}

func peerHost(peerAddr string) string {
	if peerAddr == "" {
		return ""
	}
	host := peerAddr
	if h, _, err := net.SplitHostPort(peerAddr); err == nil {
		host = h
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String()
	}
	return truncate(host, 64)
}
