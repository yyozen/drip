package tcp

import (
	"crypto/tls"
	"fmt"
	"net"

	json "github.com/goccy/go-json"
	"sync"
	"time"

	"drip/internal/shared/constants"
	"drip/internal/shared/pool"
	"drip/internal/shared/protocol"
	"drip/pkg/config"
	"go.uber.org/zap"
)

// LatencyCallback is called when latency is measured
type LatencyCallback func(latency time.Duration)

// Connector manages the TCP connection to the server
type Connector struct {
	serverAddr      string
	tlsConfig       *tls.Config
	token           string
	tunnelType      protocol.TunnelType
	localHost       string
	localPort       int
	subdomain       string
	conn            net.Conn
	logger          *zap.Logger
	stopCh          chan struct{}
	once            sync.Once
	registered      bool
	assignedURL     string
	frameHandler    *FrameHandler
	frameWriter     *protocol.FrameWriter
	latencyCallback LatencyCallback
	heartbeatSentAt time.Time
	heartbeatMu sync.Mutex
	lastLatency time.Duration
	handlerWg   sync.WaitGroup // Tracks active data frame handlers
	closed      bool
	closedMu    sync.RWMutex
}

// ConnectorConfig holds connector configuration
type ConnectorConfig struct {
	ServerAddr string
	Token      string
	TunnelType protocol.TunnelType
	LocalHost  string // Local host address (default: 127.0.0.1)
	LocalPort  int
	Subdomain  string // Optional custom subdomain
	Insecure   bool   // Skip TLS verification (testing only)
}

// NewConnector creates a new connector
func NewConnector(cfg *ConnectorConfig, logger *zap.Logger) *Connector {
	var tlsConfig *tls.Config
	if cfg.Insecure {
		tlsConfig = config.GetClientTLSConfigInsecure()
	} else {
		host, _, _ := net.SplitHostPort(cfg.ServerAddr)
		tlsConfig = config.GetClientTLSConfig(host)
	}

	localHost := cfg.LocalHost
	if localHost == "" {
		localHost = "127.0.0.1"
	}

	return &Connector{
		serverAddr: cfg.ServerAddr,
		tlsConfig:  tlsConfig,
		token:      cfg.Token,
		tunnelType: cfg.TunnelType,
		localHost:  localHost,
		localPort:  cfg.LocalPort,
		subdomain:  cfg.Subdomain,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Connect connects to the server and registers the tunnel
func (c *Connector) Connect() error {
	c.logger.Info("Connecting to server",
		zap.String("server", c.serverAddr),
		zap.String("tunnel_type", string(c.tunnelType)),
		zap.String("local_host", c.localHost),
		zap.Int("local_port", c.localPort),
	)

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", c.serverAddr, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn

	state := conn.ConnectionState()
	if state.Version != tls.VersionTLS13 {
		conn.Close()
		return fmt.Errorf("server not using TLS 1.3 (version: 0x%04x)", state.Version)
	}

	c.logger.Info("TLS connection established",
		zap.String("cipher_suite", tls.CipherSuiteName(state.CipherSuite)),
	)

	if err := c.register(); err != nil {
		conn.Close()
		return fmt.Errorf("registration failed: %w", err)
	}

	c.frameWriter = protocol.NewFrameWriter(c.conn)
	bufferPool := pool.NewBufferPool()

	c.frameHandler = NewFrameHandler(
		c.conn,
		c.frameWriter,
		c.localHost,
		c.localPort,
		c.tunnelType,
		c.logger,
		c.IsClosed,
		bufferPool,
	)

	c.frameWriter.EnableHeartbeat(constants.HeartbeatInterval, c.createHeartbeatFrame)

	go c.frameHandler.WarmupConnectionPool(3)
	go c.handleFrames()

	return nil
}

// register sends registration request and waits for acknowledgment
func (c *Connector) register() error {
	req := protocol.RegisterRequest{
		Token:           c.token,
		CustomSubdomain: c.subdomain,
		TunnelType:      c.tunnelType,
		LocalPort:       c.localPort,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	regFrame := protocol.NewFrame(protocol.FrameTypeRegister, payload)
	err = protocol.WriteFrame(c.conn, regFrame)
	if err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}

	c.conn.SetReadDeadline(time.Now().Add(constants.RequestTimeout))
	ackFrame, err := protocol.ReadFrame(c.conn)
	if err != nil {
		return fmt.Errorf("failed to read ack: %w", err)
	}
	defer ackFrame.Release()

	c.conn.SetReadDeadline(time.Time{})

	if ackFrame.Type == protocol.FrameTypeError {
		var errMsg protocol.ErrorMessage
		if err := json.Unmarshal(ackFrame.Payload, &errMsg); err == nil {
			return fmt.Errorf("registration error: %s - %s", errMsg.Code, errMsg.Message)
		}
		return fmt.Errorf("registration error")
	}

	if ackFrame.Type != protocol.FrameTypeRegisterAck {
		return fmt.Errorf("unexpected frame type: %s", ackFrame.Type)
	}

	var resp protocol.RegisterResponse
	if err := json.Unmarshal(ackFrame.Payload, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	c.registered = true
	c.assignedURL = resp.URL
	c.subdomain = resp.Subdomain

	c.logger.Info("Tunnel registered successfully",
		zap.String("subdomain", resp.Subdomain),
		zap.String("url", resp.URL),
		zap.Int("remote_port", resp.Port),
	)

	return nil
}

// handleFrames handles incoming frames from server
func (c *Connector) handleFrames() {
	defer c.Close()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(constants.RequestTimeout))
		frame, err := protocol.ReadFrame(c.conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				c.logger.Warn("Read timeout")
				return
			}
			select {
			case <-c.stopCh:
				return
			default:
				c.logger.Error("Failed to read frame", zap.Error(err))
				return
			}
		}
		switch frame.Type {
		case protocol.FrameTypeHeartbeatAck:
			c.heartbeatMu.Lock()
			if !c.heartbeatSentAt.IsZero() {
				latency := time.Since(c.heartbeatSentAt)
				c.lastLatency = latency
				c.heartbeatMu.Unlock()

				c.logger.Debug("Received heartbeat ack", zap.Duration("latency", latency))

				if c.latencyCallback != nil {
					c.latencyCallback(latency)
				}
			} else {
				c.heartbeatMu.Unlock()
				c.logger.Debug("Received heartbeat ack")
			}
			frame.Release()

		case protocol.FrameTypeData:
			c.handlerWg.Add(1)
			go func(f *protocol.Frame) {
				defer c.handlerWg.Done()
				defer f.Release()
				if err := c.frameHandler.HandleDataFrame(f); err != nil {
					c.logger.Error("Failed to handle data frame", zap.Error(err))
				}
			}(frame)

		case protocol.FrameTypeClose:
			frame.Release()
			c.logger.Info("Server requested close")
			return

		case protocol.FrameTypeError:
			var errMsg protocol.ErrorMessage
			if err := json.Unmarshal(frame.Payload, &errMsg); err == nil {
				c.logger.Error("Received error from server",
					zap.String("code", errMsg.Code),
					zap.String("message", errMsg.Message),
				)
			}
			frame.Release()
			return

		default:
			frame.Release()
			c.logger.Warn("Unexpected frame type",
				zap.String("type", frame.Type.String()),
			)
		}
	}
}

// createHeartbeatFrame creates a heartbeat frame to be sent by the write loop.
func (c *Connector) createHeartbeatFrame() *protocol.Frame {
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return nil
	}
	c.closedMu.RUnlock()

	c.heartbeatMu.Lock()
	c.heartbeatSentAt = time.Now()
	c.heartbeatMu.Unlock()

	c.logger.Debug("Heartbeat sent")
	return protocol.NewFrame(protocol.FrameTypeHeartbeat, nil)
}

// SendFrame sends a frame to the server
func (c *Connector) SendFrame(frame *protocol.Frame) error {
	if !c.registered {
		return fmt.Errorf("not registered")
	}

	return c.frameWriter.WriteFrame(frame)
}

// Close closes the connection
func (c *Connector) Close() error {
	c.once.Do(func() {
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()

		close(c.stopCh)

		// Wait for active handlers with timeout
		c.logger.Debug("Waiting for active handlers to complete")
		done := make(chan struct{})
		go func() {
			c.handlerWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			c.logger.Debug("All handlers completed")
		case <-time.After(3 * time.Second):
			c.logger.Warn("Force closing: some handlers are still active")
		}

		if c.conn != nil {
			closeFrame := protocol.NewFrame(protocol.FrameTypeClose, nil)

			if c.frameWriter != nil {
				c.frameWriter.WriteFrame(closeFrame)
				c.frameWriter.Close()
			} else {
				protocol.WriteFrame(c.conn, closeFrame)
			}

			c.conn.Close()
		}
		c.logger.Info("Connector closed")
	})
	return nil
}

// Wait blocks until connection is closed
func (c *Connector) Wait() {
	<-c.stopCh
}

// GetURL returns the assigned tunnel URL
func (c *Connector) GetURL() string {
	return c.assignedURL
}

// GetSubdomain returns the assigned subdomain
func (c *Connector) GetSubdomain() string {
	return c.subdomain
}

// SetLatencyCallback sets the callback for latency updates
func (c *Connector) SetLatencyCallback(cb LatencyCallback) {
	c.latencyCallback = cb
}

// GetLatency returns the last measured latency
func (c *Connector) GetLatency() time.Duration {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()
	return c.lastLatency
}

// GetStats returns the traffic stats from the frame handler
func (c *Connector) GetStats() *TrafficStats {
	if c.frameHandler != nil {
		return c.frameHandler.GetStats()
	}
	return nil
}

// IsClosed returns whether the connector has been closed
func (c *Connector) IsClosed() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()
	return c.closed
}
