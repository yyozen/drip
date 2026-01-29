package tcp

import (
	"bufio"
	"fmt"
	"net"
	"time"

	json "github.com/goccy/go-json"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"

	"drip/internal/shared/mux"
	"drip/internal/shared/protocol"
)

// DataConnectionHandler handles data connection requests for multi-connection support.
type DataConnectionHandler struct {
	conn             net.Conn
	reader           *bufio.Reader
	authToken        string
	groupManager     *ConnectionGroupManager
	stopCh           <-chan struct{}
	logger           *zap.Logger
	onSessionCreated func(*yamux.Session)
	onTunnelIDSet    func(string)
}

// NewDataConnectionHandler creates a new data connection handler.
func NewDataConnectionHandler(
	conn net.Conn,
	reader *bufio.Reader,
	authToken string,
	groupManager *ConnectionGroupManager,
	stopCh <-chan struct{},
	logger *zap.Logger,
) *DataConnectionHandler {
	return &DataConnectionHandler{
		conn:         conn,
		reader:       reader,
		authToken:    authToken,
		groupManager: groupManager,
		stopCh:       stopCh,
		logger:       logger,
	}
}

// SetSessionCreatedHandler sets the callback for when a session is created.
func (h *DataConnectionHandler) SetSessionCreatedHandler(handler func(*yamux.Session)) {
	h.onSessionCreated = handler
}

// SetTunnelIDHandler sets the callback for when tunnel ID is set.
func (h *DataConnectionHandler) SetTunnelIDHandler(handler func(string)) {
	h.onTunnelIDSet = handler
}

// Handle processes the data connection request.
func (h *DataConnectionHandler) Handle(frame *protocol.Frame) error {
	var req protocol.DataConnectRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		h.sendError("invalid_request", "Failed to parse data connect request")
		return fmt.Errorf("failed to parse data connect request: %w", err)
	}

	h.logger.Info("Data connection request received",
		zap.String("tunnel_id", req.TunnelID),
		zap.String("connection_id", req.ConnectionID),
	)

	if h.groupManager == nil {
		h.sendError("not_supported", "Multi-connection not supported")
		return fmt.Errorf("group manager not available")
	}

	if h.authToken != "" && req.Token != h.authToken {
		h.sendError("authentication_failed", "Invalid authentication token")
		return fmt.Errorf("authentication failed for data connection")
	}

	group, ok := h.groupManager.GetGroup(req.TunnelID)
	if !ok || group == nil {
		h.sendError("join_failed", "Tunnel not found")
		return fmt.Errorf("tunnel not found: %s", req.TunnelID)
	}

	if group.Token != "" && req.Token != group.Token {
		h.sendError("authentication_failed", "Invalid authentication token")
		return fmt.Errorf("authentication failed for data connection")
	}

	if h.onTunnelIDSet != nil {
		h.onTunnelIDSet(req.TunnelID)
	}

	resp := protocol.DataConnectResponse{
		Accepted:     true,
		ConnectionID: req.ConnectionID,
		Message:      "Data connection accepted",
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal data connect response: %w", err)
	}
	ackFrame := protocol.NewFrame(protocol.FrameTypeDataConnectAck, respData)

	if err := protocol.WriteFrame(h.conn, ackFrame); err != nil {
		return fmt.Errorf("failed to send data connect ack: %w", err)
	}

	h.logger.Info("Data connection established",
		zap.String("tunnel_id", req.TunnelID),
		zap.String("connection_id", req.ConnectionID),
	)

	_ = h.conn.SetReadDeadline(time.Time{})

	// Server acts as yamux Client, client connector acts as yamux Server
	bc := &bufferedConn{
		Conn:   h.conn,
		reader: h.reader,
	}

	// Use optimized mux config for server
	cfg := mux.NewServerConfig()

	session, err := yamux.Client(bc, cfg)
	if err != nil {
		return fmt.Errorf("failed to init yamux session: %w", err)
	}

	if h.onSessionCreated != nil {
		h.onSessionCreated(session)
	}

	group.AddSession(req.ConnectionID, session)
	defer group.RemoveSession(req.ConnectionID)

	select {
	case <-h.stopCh:
		return nil
	case <-session.CloseChan():
		return nil
	}
}

// sendError sends an error response to the client.
func (h *DataConnectionHandler) sendError(code, message string) {
	resp := protocol.DataConnectResponse{
		Accepted: false,
		Message:  fmt.Sprintf("%s: %s", code, message),
	}
	respData, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("Failed to marshal data connect error", zap.Error(err))
		return
	}
	frame := protocol.NewFrame(protocol.FrameTypeDataConnectAck, respData)
	_ = protocol.WriteFrame(h.conn, frame)
}
