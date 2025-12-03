package config

import (
	"crypto/tls"
	"fmt"
	"os"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	// Server settings
	Port       int
	PublicPort int // Port to display in URLs (for reverse proxy scenarios)
	Domain     string

	// TCP tunnel dynamic port allocation
	TCPPortMin int
	TCPPortMax int

	// TLS/SSL settings
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
	AutoTLS     bool // Automatic Let's Encrypt

	// Security
	AuthToken string

	// Logging
	Debug bool
}

// LegacyClientConfig holds the legacy client configuration
// Deprecated: Use config.ClientConfig from client_config.go instead
type LegacyClientConfig struct {
	ServerURL   string
	LocalTarget string
	AuthToken   string
	Subdomain   string
	Verbose     bool
	Insecure    bool // Skip TLS verification (for testing only)
}

// LoadTLSConfig loads TLS configuration
func (c *ServerConfig) LoadTLSConfig() (*tls.Config, error) {
	if !c.TLSEnabled {
		return nil, nil
	}

	if c.TLSCertFile == "" || c.TLSKeyFile == "" {
		return nil, fmt.Errorf("TLS enabled but certificate files not specified")
	}

	if _, err := os.Stat(c.TLSCertFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file not found: %s", c.TLSCertFile)
	}

	if _, err := os.Stat(c.TLSKeyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file not found: %s", c.TLSKeyFile)
	}

	cert, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// Force TLS 1.3 only
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13, // Only TLS 1.3
		MaxVersion:   tls.VersionTLS13, // Only TLS 1.3
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	return tlsConfig, nil
}

// GetClientTLSConfig returns TLS config for client connections
func GetClientTLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS13, // Only TLS 1.3
		MaxVersion: tls.VersionTLS13, // Only TLS 1.3
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}

// GetClientTLSConfigInsecure returns TLS config for client with InsecureSkipVerify
// WARNING: Only use for testing!
func GetClientTLSConfigInsecure() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13, // Only TLS 1.3
		MaxVersion:         tls.VersionTLS13, // Only TLS 1.3
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}

// GetServerURL returns the server URL based on configuration
func (c *ServerConfig) GetServerURL() string {
	protocol := "http"
	if c.TLSEnabled {
		protocol = "https"
	}

	if c.Port == 80 || (c.TLSEnabled && c.Port == 443) {
		return fmt.Sprintf("%s://%s", protocol, c.Domain)
	}

	return fmt.Sprintf("%s://%s:%d", protocol, c.Domain, c.Port)
}

// GetTCPAddress returns the TCP address for tunnel connections
func (c *ServerConfig) GetTCPAddress() string {
	return fmt.Sprintf("%s:%d", c.Domain, c.Port)
}
