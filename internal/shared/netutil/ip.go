package netutil

import (
	"net"
	"net/http"
	"strings"
)

var privateNetworks []*net.IPNet

func init() {
	privateCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateCIDRs {
		_, ipNet, _ := net.ParseCIDR(cidr)
		privateNetworks = append(privateNetworks, ipNet)
	}
}

// ExtractRemoteIP extracts the IP address from a remote address string (host:port format).
func ExtractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// IsPrivateIP checks if the given IP is a private/loopback address.
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, network := range privateNetworks {
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// ExtractClientIP extracts the client IP from the request.
// It only trusts X-Forwarded-For and X-Real-IP headers when the request
// comes from a private/loopback network (typical reverse proxy setup).
func ExtractClientIP(r *http.Request) string {
	// First, get the direct remote address
	remoteIP := ExtractRemoteIP(r.RemoteAddr)

	// Only trust proxy headers if the request comes from a private network
	if IsPrivateIP(remoteIP) {
		// Check X-Forwarded-For header (may contain multiple IPs)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first IP (original client)
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}

		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Fall back to remote address
	return remoteIP
}
