package tcp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"drip/internal/shared/pool"
	"drip/internal/shared/protocol"
	"go.uber.org/zap"
)

// FrameHandler handles data frames and forwards to local service
type FrameHandler struct {
	conn          net.Conn
	frameWriter   *protocol.FrameWriter
	localHost     string
	localPort     int
	logger        *zap.Logger
	streams       map[string]*Stream
	streamMu      sync.RWMutex
	tunnelType    protocol.TunnelType
	httpClient    *http.Client
	stats         *TrafficStats
	isClosedCheck func() bool
	bufferPool    *pool.BufferPool
	headerPool    *pool.HeaderPool
}

// Stream represents a single request/response stream
type Stream struct {
	ID         string
	LocalConn  net.Conn
	ResponseCh chan []byte
	Done       chan struct{}
}

func NewFrameHandler(conn net.Conn, frameWriter *protocol.FrameWriter, localHost string, localPort int, tunnelType protocol.TunnelType, logger *zap.Logger, isClosedCheck func() bool, bufferPool *pool.BufferPool) *FrameHandler {
	var tlsConfig *tls.Config
	if tunnelType == protocol.TunnelTypeHTTPS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &FrameHandler{
		conn:          conn,
		frameWriter:   frameWriter,
		localHost:     localHost,
		localPort:     localPort,
		logger:        logger,
		streams:       make(map[string]*Stream),
		tunnelType:    tunnelType,
		stats:         NewTrafficStats(),
		isClosedCheck: isClosedCheck,
		bufferPool:    bufferPool,
		headerPool:    pool.NewHeaderPool(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          500,
				MaxIdleConnsPerHost:   200,
				MaxConnsPerHost:       0,
				IdleConnTimeout:       180 * time.Second,
				DisableCompression:    true,
				DisableKeepAlives:     false,
				TLSHandshakeTimeout:   10 * time.Second,
				TLSClientConfig:       tlsConfig,
				ResponseHeaderTimeout: 15 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (h *FrameHandler) HandleDataFrame(frame *protocol.Frame) error {
	h.stats.AddBytesIn(int64(len(frame.Payload)))
	h.stats.AddRequest()

	header, data, err := protocol.DecodeDataPayload(frame.Payload)
	if err != nil {
		return fmt.Errorf("failed to decode data payload: %w", err)
	}

	if header.Type == "http_request" {
		return h.handleHTTPFrame(header, data)
	}

	if header.Type == "close" {
		h.closeStream(header.StreamID)
		return nil
	}

	stream, err := h.getOrCreateStream(header.StreamID)
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}

	h.forwardToLocal(stream, data)

	return nil
}

func (h *FrameHandler) getOrCreateStream(streamID string) (*Stream, error) {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()

	if stream, ok := h.streams[streamID]; ok {
		return stream, nil
	}

	localAddr := fmt.Sprintf("%s:%d", h.localHost, h.localPort)
	localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local service: %w", err)
	}

	stream := &Stream{
		ID:         streamID,
		LocalConn:  localConn,
		ResponseCh: make(chan []byte, 10),
		Done:       make(chan struct{}),
	}

	h.streams[streamID] = stream

	go h.handleLocalResponse(stream)

	return stream, nil
}

func (h *FrameHandler) forwardToLocal(stream *Stream, data []byte) {
	if _, err := stream.LocalConn.Write(data); err != nil {
		h.logger.Error("Failed to write to local service",
			zap.String("stream_id", stream.ID),
			zap.Error(err),
		)
		h.closeStream(stream.ID)
	}
}

func (h *FrameHandler) handleLocalResponse(stream *Stream) {
	defer h.closeStream(stream.ID)

	bufPtr := h.bufferPool.Get(pool.SizeMedium)
	defer h.bufferPool.Put(bufPtr)
	buf := (*bufPtr)[:pool.SizeMedium]

	for {
		n, err := stream.LocalConn.Read(buf)
		if err != nil {
			break
		}

		if n > 0 {
			if h.isClosedCheck != nil && h.isClosedCheck() {
				break
			}

			header := protocol.DataHeader{
				StreamID: stream.ID,
				Type:     "response",
				IsLast:   false,
			}

			payload, err := protocol.EncodeDataPayload(header, buf[:n])
			if err != nil {
				h.logger.Error("Encode payload failed", zap.Error(err))
				break
			}

			dataFrame := protocol.NewFrame(protocol.FrameTypeData, payload)
			err = h.frameWriter.WriteFrame(dataFrame)
			if err != nil {
				h.logger.Error("Send frame failed", zap.Error(err))
				break
			}

			h.stats.AddBytesOut(int64(len(payload)))
		}
	}
}

func (h *FrameHandler) handleHTTPFrame(header protocol.DataHeader, payload []byte) error {
	if h.tunnelType != protocol.TunnelTypeHTTP && h.tunnelType != protocol.TunnelTypeHTTPS {
		return nil
	}

	httpReq, err := protocol.DecodeHTTPRequest(payload)
	if err != nil {
		return fmt.Errorf("failed to decode HTTP request: %w", err)
	}

	targetURL := httpReq.URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		scheme := "http"
		if h.tunnelType == protocol.TunnelTypeHTTPS {
			scheme = "https"
		}
		targetURL = fmt.Sprintf("%s://%s:%d%s", scheme, h.localHost, h.localPort, targetURL)
	}

	req, err := http.NewRequest(httpReq.Method, targetURL, bytes.NewReader(httpReq.Body))
	if err != nil {
		return h.sendHTTPError(header.StreamID, header.RequestID, http.StatusBadGateway, fmt.Sprintf("build request: %v", err))
	}

	origHost := ""
	for key, values := range httpReq.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if host := req.Header.Get("Host"); host != "" {
		origHost = host
	}

	isLocalTarget := h.isLocalAddress(h.localHost)

	if isLocalTarget {
		if origHost != "" {
			req.Host = origHost
			req.Header.Set("Host", origHost)
		} else {
			localHostPort := fmt.Sprintf("%s:%d", h.localHost, h.localPort)
			req.Host = localHostPort
			req.Header.Set("Host", localHostPort)
		}
		if origHost != "" {
			req.Header.Set("X-Forwarded-Host", origHost)
		}
	} else {
		targetHost := h.localHost
		if h.localPort != 443 && h.localPort != 80 {
			targetHost = fmt.Sprintf("%s:%d", h.localHost, h.localPort)
		}
		req.Host = targetHost
		req.Header.Set("Host", targetHost)
		if origHost != "" {
			req.Header.Set("X-Forwarded-Host", origHost)
		}
	}
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return h.sendHTTPError(header.StreamID, header.RequestID, http.StatusBadGateway, fmt.Sprintf("local request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.sendHTTPError(header.StreamID, header.RequestID, http.StatusBadGateway, fmt.Sprintf("read response: %v", err))
	}

	httpResp := protocol.HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       body,
	}

	return h.sendHTTPResponse(header.StreamID, header.RequestID, &httpResp)
}

func (h *FrameHandler) sendHTTPError(streamID, requestID string, status int, message string) error {
	headers := h.headerPool.Get()
	headers.Set("Content-Type", "text/plain")

	httpResp := protocol.HTTPResponse{
		StatusCode: status,
		Status:     http.StatusText(status),
		Headers:    headers,
		Body:       []byte(message),
	}

	err := h.sendHTTPResponse(streamID, requestID, &httpResp)

	h.headerPool.Put(headers)

	return err
}

func (h *FrameHandler) sendHTTPResponse(streamID, requestID string, resp *protocol.HTTPResponse) error {
	if h.isClosedCheck != nil && h.isClosedCheck() {
		return nil
	}

	header := protocol.DataHeader{
		StreamID:  streamID,
		RequestID: requestID,
		Type:      "http_response",
		IsLast:    true,
	}

	respBytes, err := protocol.EncodeHTTPResponse(resp)
	if err != nil {
		return fmt.Errorf("encode http response: %w", err)
	}

	payload, err := protocol.EncodeDataPayload(header, respBytes)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	dataFrame := protocol.NewFrame(protocol.FrameTypeData, payload)

	h.stats.AddBytesOut(int64(len(payload)))

	return h.frameWriter.WriteFrame(dataFrame)
}

func (h *FrameHandler) closeStream(streamID string) {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()

	stream, ok := h.streams[streamID]
	if !ok {
		return
	}

	if stream.LocalConn != nil {
		stream.LocalConn.Close()
	}

	close(stream.Done)

	delete(h.streams, streamID)

	if h.isClosedCheck != nil && h.isClosedCheck() {
		return
	}

	header := protocol.DataHeader{
		StreamID:  streamID,
		RequestID: streamID,
		Type:      "close",
		IsLast:    true,
	}

	payload, err := protocol.EncodeDataPayload(header, nil)
	if err != nil {
		return
	}

	closeFrame := protocol.NewFrame(protocol.FrameTypeData, payload)

	h.frameWriter.WriteFrame(closeFrame)
}

// Close closes all streams
func (h *FrameHandler) Close() {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()

	for streamID, stream := range h.streams {
		if stream.LocalConn != nil {
			stream.LocalConn.Close()
		}
		close(stream.Done)
		delete(h.streams, streamID)
	}
}

// GetStats returns the traffic stats tracker
func (h *FrameHandler) GetStats() *TrafficStats {
	return h.stats
}

func (h *FrameHandler) WarmupConnectionPool(numConnections int) {
	if h.tunnelType != protocol.TunnelTypeHTTP {
		return
	}

	targetURL := fmt.Sprintf("http://%s:%d/", h.localHost, h.localPort)

	var wg sync.WaitGroup
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req, err := http.NewRequest("HEAD", targetURL, nil)
			if err != nil {
				return
			}

			resp, err := h.httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)
		}()
	}

	wg.Wait()
}

func (h *FrameHandler) isLocalAddress(addr string) bool {
	if addr == "localhost" || addr == "127.0.0.1" || addr == "::1" {
		return true
	}

	if strings.HasPrefix(addr, "192.168.") ||
		strings.HasPrefix(addr, "10.") ||
		strings.HasPrefix(addr, "172.16.") ||
		strings.HasPrefix(addr, "172.17.") ||
		strings.HasPrefix(addr, "172.18.") ||
		strings.HasPrefix(addr, "172.19.") ||
		strings.HasPrefix(addr, "172.20.") ||
		strings.HasPrefix(addr, "172.21.") ||
		strings.HasPrefix(addr, "172.22.") ||
		strings.HasPrefix(addr, "172.23.") ||
		strings.HasPrefix(addr, "172.24.") ||
		strings.HasPrefix(addr, "172.25.") ||
		strings.HasPrefix(addr, "172.26.") ||
		strings.HasPrefix(addr, "172.27.") ||
		strings.HasPrefix(addr, "172.28.") ||
		strings.HasPrefix(addr, "172.29.") ||
		strings.HasPrefix(addr, "172.30.") ||
		strings.HasPrefix(addr, "172.31.") {
		return true
	}

	return false
}
