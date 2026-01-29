package protocol

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// HeartbeatManager manages heartbeat tracking and checking for connections.
type HeartbeatManager struct {
	mu            sync.RWMutex
	lastHeartbeat time.Time
	interval      time.Duration
	timeout       time.Duration
	logger        *zap.Logger
	frameWriter   *FrameWriter
}

// NewHeartbeatManager creates a new heartbeat manager.
func NewHeartbeatManager(interval, timeout time.Duration, frameWriter *FrameWriter, logger *zap.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		lastHeartbeat: time.Now(),
		interval:      interval,
		timeout:       timeout,
		frameWriter:   frameWriter,
		logger:        logger,
	}
}

// UpdateLastHeartbeat updates the last heartbeat timestamp.
func (h *HeartbeatManager) UpdateLastHeartbeat() {
	h.mu.Lock()
	h.lastHeartbeat = time.Now()
	h.mu.Unlock()
}

// GetLastHeartbeat returns the last heartbeat timestamp.
func (h *HeartbeatManager) GetLastHeartbeat() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastHeartbeat
}

// IsAlive checks if the connection is still alive based on heartbeat timeout.
func (h *HeartbeatManager) IsAlive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return time.Since(h.lastHeartbeat) < h.timeout
}

// SendHeartbeatAck sends a heartbeat acknowledgment frame.
func (h *HeartbeatManager) SendHeartbeatAck() error {
	ackFrame := NewFrame(FrameTypeHeartbeatAck, nil)
	err := h.frameWriter.WriteControl(ackFrame)
	if err != nil {
		h.logger.Error("Failed to send heartbeat ack", zap.Error(err))
		return err
	}
	return nil
}

// HandleHeartbeat handles a received heartbeat frame.
func (h *HeartbeatManager) HandleHeartbeat() error {
	h.UpdateLastHeartbeat()
	return h.SendHeartbeatAck()
}

// TimeSinceLastHeartbeat returns the duration since the last heartbeat.
func (h *HeartbeatManager) TimeSinceLastHeartbeat() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return time.Since(h.lastHeartbeat)
}
