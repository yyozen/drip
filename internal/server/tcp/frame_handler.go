package tcp

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"drip/internal/shared/constants"
	"drip/internal/shared/protocol"
	"go.uber.org/zap"
)

// FrameHandler handles protocol frame reading and processing.
type FrameHandler struct {
	conn        net.Conn
	reader      *bufio.Reader
	stopCh      <-chan struct{}
	logger      *zap.Logger
	frameWriter *protocol.FrameWriter

	// Heartbeat tracking
	onHeartbeat func()
	onClose     func()
}

// NewFrameHandler creates a new frame handler.
func NewFrameHandler(
	conn net.Conn,
	reader *bufio.Reader,
	stopCh <-chan struct{},
	frameWriter *protocol.FrameWriter,
	logger *zap.Logger,
) *FrameHandler {
	return &FrameHandler{
		conn:        conn,
		reader:      reader,
		stopCh:      stopCh,
		frameWriter: frameWriter,
		logger:      logger,
	}
}

// SetHeartbeatHandler sets the callback for heartbeat frames.
func (fh *FrameHandler) SetHeartbeatHandler(handler func()) {
	fh.onHeartbeat = handler
}

// SetCloseHandler sets the callback for close frames.
func (fh *FrameHandler) SetCloseHandler(handler func()) {
	fh.onClose = handler
}

// HandleFrames processes incoming frames in a loop.
func (fh *FrameHandler) HandleFrames() error {
	for {
		select {
		case <-fh.stopCh:
			return nil
		default:
		}

		fh.conn.SetReadDeadline(time.Now().Add(constants.RequestTimeout))
		frame, err := protocol.ReadFrame(fh.reader)
		if err != nil {
			return fh.handleReadError(err)
		}

		sf := protocol.WithFrame(frame)
		err = fh.processFrame(sf)
		sf.Close()

		if err != nil {
			return err
		}
	}
}

// handleReadError handles errors that occur while reading frames.
func (fh *FrameHandler) handleReadError(err error) error {
	if isTimeoutError(err) {
		fh.logger.Warn("Read timeout, connection may be dead")
		return fmt.Errorf("read timeout")
	}

	if err.Error() == "failed to read frame header: EOF" || err.Error() == "EOF" {
		fh.logger.Info("Client disconnected")
		return nil
	}

	select {
	case <-fh.stopCh:
		fh.logger.Debug("Connection closed during shutdown")
		return nil
	default:
		return fmt.Errorf("failed to read frame: %w", err)
	}
}

// processFrame processes a single frame based on its type.
func (fh *FrameHandler) processFrame(sf *protocol.SafeFrame) error {
	switch sf.Frame.Type {
	case protocol.FrameTypeHeartbeat:
		if fh.onHeartbeat != nil {
			fh.onHeartbeat()
		}
		return nil

	case protocol.FrameTypeClose:
		fh.logger.Info("Client requested close")
		if fh.onClose != nil {
			fh.onClose()
		}
		return fmt.Errorf("client requested close")

	default:
		fh.logger.Warn("Unexpected frame type",
			zap.String("type", sf.Frame.Type.String()),
		)
		return nil
	}
}
