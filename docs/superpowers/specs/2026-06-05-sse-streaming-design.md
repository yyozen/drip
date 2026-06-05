# SSE Streaming Support for HTTP Tunnels

## Context

GitHub issue #25 reports that Server-Sent Events work on localhost but do not deliver events when the same web app is accessed through a drip tunnel. The browser-side SSE request remains pending and no events are received.

The current HTTP/HTTPS tunnel path is split across two sides:

- `internal/server/proxy/handler.go` accepts the public HTTP request, opens a tunnel stream, writes the request into the stream, reads the HTTP response from the stream, and writes that response to the public client.
- `internal/client/tcp/pool_handler.go` reads the request from the tunnel stream, sends it to the local HTTP/HTTPS backend through `http.Client`, and writes the backend response back to the tunnel stream.

The fix should target the default web app scenario from the issue: HTTP/HTTPS tunnel traffic. TCP tunnel behavior is out of scope unless verification shows it is directly affected by the same defect.

## Goals

- Deliver SSE response headers promptly through an HTTP/HTTPS tunnel.
- Stream SSE body chunks as they arrive instead of waiting for response body EOF.
- Keep SSE connections open for long-lived event streams.
- Preserve existing behavior for ordinary HTTP responses, WebSocket upgrades, authentication, IP access control, bandwidth accounting, and tunnel/session management.

## Non-Goals

- Reworking yamux session management.
- Replacing the current proxy flow with a full reverse-proxy abstraction.
- Changing TCP tunnel behavior.
- Adding new user-facing configuration for SSE.

## Selected Approach

Use a focused streaming-response branch for HTTP/HTTPS tunnel responses whose `Content-Type` media type is `text/event-stream`.

This approach keeps the existing tunnel architecture intact while adding the response behavior SSE requires: immediate header flush, repeated body flushes, no fixed content length, and long-lived connection handling.

## Alternatives Considered

### Raw Pipe Style Proxying

The HTTP tunnel could hijack both sides and pipe bytes like the WebSocket path. This may support more streaming protocols, but it would bypass existing response header handling, redirect rewriting, metrics boundaries, and error handling. The regression surface is too large for issue #25.

### ReverseProxy-Style Refactor

The implementation could be reshaped around `net/http/httputil.ReverseProxy` semantics. This is attractive long term, but drip's HTTP proxy is split across server and client processes connected by tunnel streams, so a direct standard-library reverse proxy is not a drop-in replacement.

## Design

### SSE Detection

Detect SSE by parsing or normalizing the response `Content-Type` media type and matching `text/event-stream`. Parameterized values such as `text/event-stream; charset=utf-8` must match.

Detection should happen after `http.ReadResponse` on the server side, because that is where public response headers are copied and browser-facing behavior is controlled.

### Server-Side Streaming Behavior

For non-SSE responses, keep the existing flow:

1. Copy response headers.
2. Set `Content-Length` when the backend response has a known length.
3. Write status.
4. Copy body with the existing large pooled buffer.

For SSE responses:

1. Copy response headers with existing hop-by-hop filtering.
2. Delete `Content-Length`, even if the local backend set one.
3. Write the HTTP status.
4. If the public `ResponseWriter` supports `http.Flusher`, flush immediately after the status and headers.
5. Copy the body in a loop with a smaller buffer.
6. After each successful write, call `Flush()` so each event can reach the browser promptly.

If the public `ResponseWriter` does not support `http.Flusher`, the handler can still write the response, but SSE delivery cannot be guaranteed. This should be treated as a degraded path, not a hard error before headers are sent.

### Client-Side Streaming Behavior

The client side should continue to use its local `http.Client` for HTTP/HTTPS tunnels and should continue writing backend response headers back to the tunnel stream.

For SSE responses, the body write loop should avoid short write deadlines that can interrupt an intentionally idle long-lived stream. The existing first-response write deadline can remain useful for header delivery, but per-chunk short deadlines should not cause a healthy SSE stream to be closed during normal idle periods.

The client should not buffer SSE data beyond the bytes read from the backend response body before writing them to the tunnel stream.

### Header Handling

Existing hop-by-hop header filtering remains in place on the server side. SSE responses must preserve useful end-to-end headers such as:

- `Content-Type`
- `Cache-Control`
- application-specific event headers

SSE responses must not forward `Transfer-Encoding` or connection management headers. They also should not expose a fixed `Content-Length` through the public response.

### Error Handling

Before response headers are sent, existing `502` behavior remains:

- tunnel stream unavailable
- request forwarding failure
- response header read failure

After SSE headers are sent, mid-stream failures are handled by ending the copy and releasing resources. Browser disconnects, local service closure, and tunnel stream closure should not attempt to send a new HTTP error because the response has already started.

The server should keep the existing request-context cancellation hook that closes the tunnel stream when the public client disconnects.

## Testing

Add focused tests for the HTTP/HTTPS tunnel response path:

- An SSE backend response sends headers immediately and emits at least one event without closing the response body.
- `text/event-stream; charset=utf-8` is recognized as SSE.
- The public response does not include a `Content-Length` header for SSE.
- Ordinary non-SSE responses keep their existing body and `Content-Length` behavior.

The most important regression test should prove that the first event can be read within a short timeout while the upstream SSE connection remains open.

## Acceptance Criteria

- Reproducing issue #25 through an HTTP/HTTPS tunnel shows EventSource connecting and receiving events.
- The browser no longer waits indefinitely for SSE events that are already emitted by the local service.
- Existing WebSocket and ordinary HTTP tunnel tests continue to pass.
- No TCP tunnel behavior is changed as part of this fix.
