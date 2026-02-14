package proxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"drip/internal/server/tunnel"
	"drip/internal/shared/httputil"
	"drip/internal/shared/netutil"
	"drip/internal/shared/protocol"
	"drip/internal/shared/qos"
	"drip/internal/shared/wsutil"
)

type bufferedReadWriteCloser struct {
	*bufio.Reader
	net.Conn
}

func (b *bufferedReadWriteCloser) Read(p []byte) (int, error) {
	return b.Reader.Read(p)
}

func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request, tconn *tunnel.Connection) {
	stream, err := h.openStreamWithTimeout(tconn)
	if err != nil {
		http.Error(w, "Tunnel unavailable", http.StatusBadGateway)
		return
	}

	tconn.IncActiveConnections()

	hj, ok := w.(http.Hijacker)
	if !ok {
		stream.Close()
		tconn.DecActiveConnections()
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return
	}

	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		stream.Close()
		tconn.DecActiveConnections()
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}

	if err := r.Write(stream); err != nil {
		stream.Close()
		clientConn.Close()
		tconn.DecActiveConnections()
		return
	}

	var limitedStream net.Conn = stream
	if limiter := tconn.GetLimiter(); limiter != nil && limiter.IsLimited() {
		if l, ok := limiter.(*qos.Limiter); ok {
			limitedStream = qos.NewLimitedConn(context.Background(), stream, l)
		}
	}

	go func() {
		defer stream.Close()
		defer clientConn.Close()
		defer tconn.DecActiveConnections()

		var clientRW io.ReadWriteCloser = clientConn
		if clientBuf != nil && clientBuf.Reader.Buffered() > 0 {
			clientRW = &bufferedReadWriteCloser{
				Reader: clientBuf.Reader,
				Conn:   clientConn,
			}
		}

		_ = netutil.PipeWithCallbacks(context.Background(), limitedStream, clientRW,
			func(n int64) { tconn.AddBytesOut(n) },
			func(n int64) { tconn.AddBytesIn(n) },
		)
	}()
}

func (h *Handler) handleTunnelWebSocket(w http.ResponseWriter, r *http.Request) {
	if !h.IsTransportAllowed("wss") {
		http.Error(w, "WebSocket transport not allowed on this server", http.StatusForbidden)
		return
	}

	if h.wsConnHandler == nil {
		http.Error(w, "WebSocket tunnel not configured", http.StatusServiceUnavailable)
		return
	}

	ws, err := h.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	ws.SetReadLimit(protocol.MaxFrameSize + protocol.FrameHeaderSize + 1024)

	remoteAddr := netutil.ExtractClientIP(r)

	h.logger.Info("WebSocket tunnel connection established",
		zap.String("remote_addr", remoteAddr),
	)

	conn := wsutil.NewConnWithPing(ws, 30*time.Second)

	h.wsConnHandler.HandleWSConnection(conn, remoteAddr)
}

func (h *Handler) isWebSocketUpgrade(r *http.Request) bool {
	return httputil.IsWebSocketUpgrade(r)
}
