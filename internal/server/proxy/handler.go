package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"drip/internal/server/tunnel"
	"drip/internal/shared/constants"
	"drip/internal/shared/pool"
	"drip/internal/shared/protocol"
	"drip/internal/shared/utils"

	"go.uber.org/zap"
)

type Handler struct {
	manager    *tunnel.Manager
	logger     *zap.Logger
	responses  *ResponseHandler
	domain     string
	authToken  string
	headerPool *pool.HeaderPool
}

func NewHandler(manager *tunnel.Manager, logger *zap.Logger, responses *ResponseHandler, domain string, authToken string) *Handler {
	return &Handler{
		manager:    manager,
		logger:     logger,
		responses:  responses,
		domain:     domain,
		authToken:  authToken,
		headerPool: pool.NewHeaderPool(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain := h.extractSubdomain(r.Host)

	if subdomain == "" {
		h.serveHomePage(w, r)
		return
	}

	conn, ok := h.manager.Get(subdomain)
	if !ok {
		http.Error(w, "Tunnel not found. The tunnel may have been closed.", http.StatusNotFound)
		return
	}

	if conn.IsClosed() {
		http.Error(w, "Tunnel connection closed", http.StatusBadGateway)
		return
	}

	transport := conn.GetTransport()
	if transport == nil {
		http.Error(w, "Tunnel control channel not ready", http.StatusBadGateway)
		return
	}

	tType := conn.GetTunnelType()
	if tType != "" && tType != protocol.TunnelTypeHTTP && tType != protocol.TunnelTypeHTTPS {
		http.Error(w, "Tunnel does not accept HTTP traffic", http.StatusBadGateway)
		return
	}

	limitedReader := io.LimitReader(r.Body, constants.MaxRequestBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.logger.Error("Read request body failed", zap.Error(err))
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	requestID := utils.GenerateID()

	headers := h.headerPool.Get()
	h.headerPool.CloneWithExtra(headers, r.Header, "Host", r.Host)

	httpReq := protocol.HTTPRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: headers,
		Body:    body,
	}

	reqBytes, err := protocol.EncodeHTTPRequest(&httpReq)

	h.headerPool.Put(headers)

	if err != nil {
		h.logger.Error("Encode HTTP request failed", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	header := protocol.DataHeader{
		StreamID:  requestID,
		RequestID: requestID,
		Type:      "http_request",
		IsLast:    true,
	}

	payload, err := protocol.EncodeDataPayload(header, reqBytes)
	if err != nil {
		h.logger.Error("Encode data payload failed", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	frame := protocol.NewFrame(protocol.FrameTypeData, payload)

	respChan := h.responses.CreateResponseChan(requestID)
	defer h.responses.CleanupResponseChan(requestID)

	if err := transport.SendFrame(frame); err != nil {
		h.logger.Error("Send frame to tunnel failed", zap.Error(err))
		http.Error(w, "Failed to forward request to tunnel", http.StatusBadGateway)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	defer cancel()

	select {
	case respMsg := <-respChan:
		h.writeHTTPResponse(w, respMsg, subdomain, r)

	case <-ctx.Done():
		http.Error(w, "Request timeout - the tunnel client did not respond in time", http.StatusGatewayTimeout)
	}
}

func (h *Handler) writeHTTPResponse(w http.ResponseWriter, resp *protocol.HTTPResponse, subdomain string, r *http.Request) {
	if resp == nil {
		http.Error(w, "Invalid response from tunnel", http.StatusBadGateway)
		return
	}

	for key, values := range resp.Headers {
		if key == "Connection" || key == "Keep-Alive" || key == "Transfer-Encoding" || key == "Upgrade" {
			continue
		}

		if key == "Location" && len(values) > 0 {
			rewrittenLocation := h.rewriteLocationHeader(values[0], r.Host)
			w.Header().Set("Location", rewrittenLocation)
			continue
		}

		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	if w.Header().Get("Content-Length") == "" && len(resp.Body) > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(resp.Body)))
	}

	statusCode := resp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	w.WriteHeader(statusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

func (h *Handler) rewriteLocationHeader(location, proxyHost string) string {
	if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
		return location
	}

	locationURL, err := url.Parse(location)
	if err != nil {
		return location
	}

	if locationURL.Host == "localhost" ||
		strings.HasPrefix(locationURL.Host, "localhost:") ||
		locationURL.Host == "127.0.0.1" ||
		strings.HasPrefix(locationURL.Host, "127.0.0.1:") {
		scheme := "https"
		if strings.Contains(proxyHost, ":") && !strings.Contains(proxyHost, "https") {
			parts := strings.Split(proxyHost, ":")
			if len(parts) == 2 && parts[1] != "443" {
				scheme = "https"
			}
		}

		rewritten := fmt.Sprintf("%s://%s%s", scheme, proxyHost, locationURL.Path)
		if locationURL.RawQuery != "" {
			rewritten += "?" + locationURL.RawQuery
		}
		if locationURL.Fragment != "" {
			rewritten += "#" + locationURL.Fragment
		}

		return rewritten
	}

	return location
}

func (h *Handler) extractSubdomain(host string) string {
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if host == h.domain {
		return ""
	}

	suffix := "." + h.domain
	if strings.HasSuffix(host, suffix) {
		subdomain := strings.TrimSuffix(host, suffix)
		return subdomain
	}

	return ""
}

func (h *Handler) serveHomePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		h.serveHealth(w, r)
		return
	}

	if r.URL.Path == "/stats" {
		h.serveStats(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8" />
    <title>Drip - Your Tunnel, Your Domain, Anywhere</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        .stats { background: #f9f9f9; padding: 15px; border-radius: 5px; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>💧 Drip - Your Tunnel, Your Domain, Anywhere</h1>
    <p>A self-hosted tunneling solution to securely expose your services to the internet.</p>

    <h2>Quick Start</h2>
    <p>Install the client:</p>
    <code>bash <(curl -fsSL https://raw.githubusercontent.com/Gouryella/drip/main/scripts/install.sh)</code>

    <p>Start a tunnel:</p>
	<code>drip http 3000</code><br><br>
	<code>drip https 443</code><br><br>
	<code>drip tcp 5432</code>
    <p><a href="/health">Health Check</a> | <a href="/stats">Statistics</a></p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (h *Handler) serveHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":         "ok",
		"active_tunnels": h.manager.Count(),
		"timestamp":      time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func cloneHeadersWithHost(src http.Header, host string) http.Header {
	dst := make(http.Header, len(src)+1)
	for k, v := range src {
		copied := make([]string, len(v))
		copy(copied, v)
		dst[k] = copied
	}
	if host != "" {
		dst.Set("Host", host)
	}
	return dst
}

func (h *Handler) serveStats(w http.ResponseWriter, r *http.Request) {
	if h.authToken != "" {
		token := r.URL.Query().Get("token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token != h.authToken {
			http.Error(w, "Unauthorized: invalid or missing token", http.StatusUnauthorized)
			return
		}
	}

	connections := h.manager.List()

	stats := map[string]interface{}{
		"total_tunnels": len(connections),
		"tunnels":       []map[string]interface{}{},
	}

	for _, conn := range connections {
		stats["tunnels"] = append(stats["tunnels"].([]map[string]interface{}), map[string]interface{}{
			"subdomain":   conn.Subdomain,
			"last_active": conn.LastActive.Unix(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
