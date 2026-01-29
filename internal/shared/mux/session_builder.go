package mux

import (
	"bufio"
	"net"

	"github.com/hashicorp/yamux"
)

// BufferedConn wraps a connection with a buffered reader.
type BufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// NewBufferedConn creates a new buffered connection.
func NewBufferedConn(conn net.Conn, reader *bufio.Reader) *BufferedConn {
	return &BufferedConn{
		Conn:   conn,
		reader: reader,
	}
}

// Read reads from the buffered reader if available, otherwise from the connection.
func (bc *BufferedConn) Read(p []byte) (int, error) {
	if bc.reader != nil {
		return bc.reader.Read(p)
	}
	return bc.Conn.Read(p)
}

// SessionBuilder helps build yamux sessions with consistent configuration.
type SessionBuilder struct {
	conn     net.Conn
	reader   *bufio.Reader
	config   *yamux.Config
	isServer bool
}

// NewSessionBuilder creates a new session builder.
func NewSessionBuilder(conn net.Conn) *SessionBuilder {
	return &SessionBuilder{
		conn: conn,
	}
}

// WithReader sets the buffered reader for the session.
func (sb *SessionBuilder) WithReader(reader *bufio.Reader) *SessionBuilder {
	sb.reader = reader
	return sb
}

// WithConfig sets the yamux configuration.
func (sb *SessionBuilder) WithConfig(config *yamux.Config) *SessionBuilder {
	sb.config = config
	return sb
}

// AsServer configures the session as a server.
func (sb *SessionBuilder) AsServer() *SessionBuilder {
	sb.isServer = true
	if sb.config == nil {
		sb.config = NewServerConfig()
	}
	return sb
}

// AsClient configures the session as a client.
func (sb *SessionBuilder) AsClient() *SessionBuilder {
	sb.isServer = false
	if sb.config == nil {
		sb.config = NewClientConfig()
	}
	return sb
}

// Build creates the yamux session.
func (sb *SessionBuilder) Build() (*yamux.Session, error) {
	conn := sb.conn

	// Wrap with buffered reader if provided
	if sb.reader != nil {
		conn = NewBufferedConn(sb.conn, sb.reader)
	}

	// Use default config if not set
	if sb.config == nil {
		if sb.isServer {
			sb.config = NewServerConfig()
		} else {
			sb.config = NewClientConfig()
		}
	}

	// Create session based on role
	if sb.isServer {
		return yamux.Server(conn, sb.config)
	}
	return yamux.Client(conn, sb.config)
}

// BuildClientSession creates a yamux client session with the given connection and reader.
func BuildClientSession(conn net.Conn, reader *bufio.Reader) (*yamux.Session, error) {
	return NewSessionBuilder(conn).
		WithReader(reader).
		AsClient().
		Build()
}

// BuildServerSession creates a yamux server session with the given connection and reader.
func BuildServerSession(conn net.Conn, reader *bufio.Reader) (*yamux.Session, error) {
	return NewSessionBuilder(conn).
		WithReader(reader).
		AsServer().
		Build()
}
