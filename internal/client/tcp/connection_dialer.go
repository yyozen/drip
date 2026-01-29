package tcp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	json "github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"drip/internal/shared/wsutil"
)

// serverCapabilities holds the discovered server capabilities
type serverCapabilities struct {
	Transports []string `json:"transports"`
	Preferred  string   `json:"preferred"`
}

// ConnectionDialer handles establishing connections to the server.
type ConnectionDialer struct {
	serverAddr string
	tlsConfig  *tls.Config
	token      string
	transport  TransportType
	logger     *zap.Logger
}

// NewConnectionDialer creates a new connection dialer.
func NewConnectionDialer(
	serverAddr string,
	tlsConfig *tls.Config,
	token string,
	transport TransportType,
	logger *zap.Logger,
) *ConnectionDialer {
	return &ConnectionDialer{
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		token:      token,
		transport:  transport,
		logger:     logger,
	}
}

// Dial establishes a connection using the appropriate transport.
func (d *ConnectionDialer) Dial() (net.Conn, error) {
	switch d.transport {
	case TransportWebSocket:
		return d.dialWebSocket()
	case TransportTCP:
		// User explicitly requested TCP, verify server supports it
		caps := d.discoverServerCapabilities()
		if caps != nil && len(caps.Transports) > 0 {
			tcpSupported := false
			for _, t := range caps.Transports {
				if t == "tcp" {
					tcpSupported = true
					break
				}
			}
			if !tcpSupported {
				return nil, fmt.Errorf("server only supports %v transport(s), but --transport tcp was specified. Use --transport wss instead", caps.Transports)
			}
		}
		return d.dialTLS()
	default: // TransportAuto
		// Check if server address indicates WebSocket
		if strings.HasPrefix(d.serverAddr, "wss://") {
			return d.dialWebSocket()
		}
		// Query server for preferred transport
		caps := d.discoverServerCapabilities()
		if caps != nil && caps.Preferred == "wss" {
			return d.dialWebSocket()
		}
		// Default to TCP
		return d.dialTLS()
	}
}

// dialTLS establishes a TLS connection to the server.
func (d *ConnectionDialer) dialTLS() (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", d.serverAddr, d.tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	state := conn.ConnectionState()
	if state.Version != tls.VersionTLS13 {
		_ = conn.Close()
		return nil, fmt.Errorf("server not using TLS 1.3 (version: 0x%04x)", state.Version)
	}

	if tcpConn, ok := conn.NetConn().(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
		_ = tcpConn.SetReadBuffer(256 * 1024)
		_ = tcpConn.SetWriteBuffer(256 * 1024)
	}

	return conn, nil
}

// dialWebSocket establishes a WebSocket connection to the server over TLS.
func (d *ConnectionDialer) dialWebSocket() (net.Conn, error) {
	// Build WebSocket URL
	host, port, err := net.SplitHostPort(d.serverAddr)
	if err != nil {
		// No port specified, use default
		host = d.serverAddr
		port = "443"
	}

	wsURL := fmt.Sprintf("wss://%s:%s/_drip/ws", host, port)

	d.logger.Debug("Connecting via WebSocket over TLS",
		zap.String("url", wsURL),
	)

	dialer := websocket.Dialer{
		TLSClientConfig:  d.tlsConfig,
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   256 * 1024,
		WriteBufferSize:  256 * 1024,
	}

	// Add authorization header if token is set
	header := http.Header{}
	if d.token != "" {
		header.Set("Authorization", "Bearer "+d.token)
	}

	ws, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("WebSocket dial failed (status %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	d.logger.Info("Connected via WebSocket over TLS",
		zap.String("url", wsURL),
	)

	// Wrap WebSocket connection to implement net.Conn with ping loop for CDN keep-alive
	return wsutil.NewConnWithPing(ws, 30*time.Second), nil
}

// discoverServerCapabilities queries the server for its capabilities.
func (d *ConnectionDialer) discoverServerCapabilities() *serverCapabilities {
	host, port, err := net.SplitHostPort(d.serverAddr)
	if err != nil {
		host = d.serverAddr
		port = "443"
	}

	discoverURL := fmt.Sprintf("https://%s:%s/_drip/discover", host, port)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: d.tlsConfig,
		},
	}

	resp, err := client.Get(discoverURL)
	if err != nil {
		d.logger.Debug("Failed to discover server capabilities",
			zap.Error(err),
		)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var caps serverCapabilities
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		return nil
	}

	d.logger.Debug("Discovered server capabilities",
		zap.Strings("transports", caps.Transports),
		zap.String("preferred", caps.Preferred),
	)

	return &caps
}
