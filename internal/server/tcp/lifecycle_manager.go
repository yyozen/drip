package tcp

import (
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"go.uber.org/zap"

	"drip/internal/server/tunnel"
	"drip/internal/shared/protocol"
)

// ConnectionLifecycleManager manages the lifecycle of a connection.
type ConnectionLifecycleManager struct {
	once   sync.Once
	stopCh chan struct{}
	cancel func()
	logger *zap.Logger

	// Resources to clean up
	conn interface {
		Close() error
		SetDeadline(time.Time) error
	}
	frameWriter  *protocol.FrameWriter
	proxy        interface{ Stop() }
	session      *yamux.Session
	portAlloc    *PortAllocator
	port         int
	manager      *tunnel.Manager
	subdomain    string
	tunnelID     string
	groupManager *ConnectionGroupManager
}

// NewConnectionLifecycleManager creates a new lifecycle manager.
func NewConnectionLifecycleManager(
	stopCh chan struct{},
	cancel func(),
	logger *zap.Logger,
) *ConnectionLifecycleManager {
	return &ConnectionLifecycleManager{
		stopCh: stopCh,
		cancel: cancel,
		logger: logger,
	}
}

// SetConnection sets the connection to manage.
func (clm *ConnectionLifecycleManager) SetConnection(conn interface {
	Close() error
	SetDeadline(time.Time) error
}) {
	clm.conn = conn
}

// SetFrameWriter sets the frame writer to close.
func (clm *ConnectionLifecycleManager) SetFrameWriter(fw *protocol.FrameWriter) {
	clm.frameWriter = fw
}

// SetProxy sets the proxy to stop.
func (clm *ConnectionLifecycleManager) SetProxy(proxy interface{ Stop() }) {
	clm.proxy = proxy
}

// SetSession sets the yamux session to close.
func (clm *ConnectionLifecycleManager) SetSession(session *yamux.Session) {
	clm.session = session
}

// SetPortAllocation sets the port allocation to release.
func (clm *ConnectionLifecycleManager) SetPortAllocation(portAlloc *PortAllocator, port int) {
	clm.portAlloc = portAlloc
	clm.port = port
}

// SetTunnelRegistration sets the tunnel registration to clean up.
func (clm *ConnectionLifecycleManager) SetTunnelRegistration(
	manager *tunnel.Manager,
	subdomain string,
	tunnelID string,
	groupManager *ConnectionGroupManager,
) {
	clm.manager = manager
	clm.subdomain = subdomain
	clm.tunnelID = tunnelID
	clm.groupManager = groupManager
}

// Close closes the connection and cleans up all resources.
func (clm *ConnectionLifecycleManager) Close() {
	clm.once.Do(func() {
		protocol.UnregisterConnection()
		close(clm.stopCh)

		if clm.cancel != nil {
			clm.cancel()
		}

		if clm.conn != nil {
			_ = clm.conn.SetDeadline(time.Now())
		}

		if clm.frameWriter != nil {
			clm.frameWriter.Close()
		}

		if clm.proxy != nil {
			clm.proxy.Stop()
		}

		if clm.session != nil {
			_ = clm.session.Close()
		}

		if clm.conn != nil {
			clm.conn.Close()
		}

		if clm.port > 0 && clm.portAlloc != nil {
			clm.portAlloc.Release(clm.port)
		}

		if clm.subdomain != "" && clm.manager != nil {
			clm.manager.Unregister(clm.subdomain)
			if clm.tunnelID != "" && clm.groupManager != nil {
				clm.groupManager.RemoveGroup(clm.tunnelID)
			}
		}

		clm.logger.Info("Connection closed",
			zap.String("subdomain", clm.subdomain),
		)
	})
}
