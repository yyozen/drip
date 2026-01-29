package utils

import (
	"fmt"

	"drip/internal/shared/protocol"
)

// TunnelURLBuilder helps construct tunnel URLs consistently.
type TunnelURLBuilder struct {
	tunnelDomain string
	publicPort   int
}

// NewTunnelURLBuilder creates a new URL builder.
func NewTunnelURLBuilder(tunnelDomain string, publicPort int) *TunnelURLBuilder {
	return &TunnelURLBuilder{
		tunnelDomain: tunnelDomain,
		publicPort:   publicPort,
	}
}

// BuildHTTPURL builds an HTTP/HTTPS tunnel URL.
func (b *TunnelURLBuilder) BuildHTTPURL(subdomain string) string {
	if b.publicPort == 443 {
		return fmt.Sprintf("https://%s.%s", subdomain, b.tunnelDomain)
	}
	return fmt.Sprintf("https://%s.%s:%d", subdomain, b.tunnelDomain, b.publicPort)
}

// BuildTCPURL builds a TCP tunnel URL.
func (b *TunnelURLBuilder) BuildTCPURL(port int) string {
	return fmt.Sprintf("tcp://%s:%d", b.tunnelDomain, port)
}

// BuildURL builds a tunnel URL based on the tunnel type.
func (b *TunnelURLBuilder) BuildURL(subdomain string, tunnelType protocol.TunnelType, port int) string {
	if tunnelType == protocol.TunnelTypeHTTP || tunnelType == protocol.TunnelTypeHTTPS {
		return b.BuildHTTPURL(subdomain)
	}
	return b.BuildTCPURL(port)
}
