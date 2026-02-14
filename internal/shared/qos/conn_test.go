package qos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type errorAfterConn struct {
	mockConn
	writeLimit int
	written    int
}

func (c *errorAfterConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := c.writeLimit - c.written
	if remaining <= 0 {
		return 0, errors.New("write error")
	}
	if len(b) > remaining {
		b = b[:remaining]
	}
	c.writeBuf = append(c.writeBuf, b...)
	c.written += len(b)
	return len(b), nil
}

func TestWriteLargerThanBurst(t *testing.T) {
	bandwidth := int64(10 * 1024)
	burst := 1024
	limiter := NewLimiter(Config{Bandwidth: bandwidth, Burst: burst})

	conn := newMockConn(nil)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	data := make([]byte, 5*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := lc.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !bytes.Equal(conn.writeBuf, data) {
		t.Error("Written data does not match input")
	}
}

func TestWriteZeroLength(t *testing.T) {
	limiter := NewLimiter(Config{Bandwidth: 1024, Burst: 1024})
	conn := newMockConn(nil)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	n, err := lc.Write(nil)
	if err != nil {
		t.Fatalf("Write(nil) failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Write(nil) returned %d, want 0", n)
	}
}

func TestWriteContextCancelDuringChunking(t *testing.T) {
	limiter := NewLimiter(Config{Bandwidth: 100, Burst: 100})
	conn := newMockConn(nil)
	ctx, cancel := context.WithCancel(context.Background())
	lc := NewLimitedConn(ctx, conn, limiter)

	_, err := lc.Write(make([]byte, 100))
	if err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	cancel()

	_, err = lc.Write(make([]byte, 200))
	if err == nil {
		t.Error("Write should fail after context cancellation")
	}
}

func TestReadCappedToBurst(t *testing.T) {
	burst := 512
	limiter := NewLimiter(Config{Bandwidth: 10240, Burst: burst})

	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}
	conn := newMockConn(data)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	buf := make([]byte, 4096)
	n, err := lc.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n > burst {
		t.Errorf("Read returned %d bytes, should be capped at burst=%d", n, burst)
	}
	if !bytes.Equal(buf[:n], data[:n]) {
		t.Error("Read data mismatch")
	}
}

func TestReadEOF(t *testing.T) {
	limiter := NewLimiter(Config{Bandwidth: 1024, Burst: 1024})
	conn := newMockConn([]byte{})
	lc := NewLimitedConn(context.Background(), conn, limiter)

	buf := make([]byte, 100)
	_, err := lc.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}

func TestNilLimiter(t *testing.T) {
	conn := newMockConn([]byte("hello"))
	lc := NewLimitedConn(context.Background(), conn, nil)

	buf := make([]byte, 10)
	n, err := lc.Read(buf)
	if err != nil {
		t.Fatalf("Read with nil limiter failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Read returned %d, want 5", n)
	}

	n, err = lc.Write([]byte("world"))
	if err != nil {
		t.Fatalf("Write with nil limiter failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	limiter := NewLimiter(Config{Bandwidth: 100 * 1024, Burst: 100 * 1024})
	lc := NewLimitedConn(context.Background(), serverConn, limiter)

	dataSize := 50 * 1024
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		data := make([]byte, dataSize)
		for i := range data {
			data[i] = 0xAA
		}
		lc.Write(data)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, dataSize)
		total := 0
		for total < dataSize {
			n, err := clientConn.Read(buf[total:])
			if err != nil {
				return
			}
			total += n
		}
	}()

	wg.Wait()
}

func TestReadFromBasic(t *testing.T) {
	limiter := NewLimiter(Config{Bandwidth: 1024 * 1024, Burst: 1024 * 1024})
	conn := newMockConn(nil)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	src := bytes.NewReader(make([]byte, 50*1024))
	n, err := lc.ReadFrom(src)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	if n != 50*1024 {
		t.Errorf("ReadFrom transferred %d bytes, want %d", n, 50*1024)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.writeBuf) != 50*1024 {
		t.Errorf("Underlying conn received %d bytes, want %d", len(conn.writeBuf), 50*1024)
	}
}

func TestWriteToBasic(t *testing.T) {
	data := make([]byte, 50*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	limiter := NewLimiter(Config{Bandwidth: 1024 * 1024, Burst: 1024 * 1024})
	conn := newMockConn(data)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	var buf bytes.Buffer
	n, err := lc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if n != int64(len(data)) {
		t.Errorf("WriteTo transferred %d bytes, want %d", n, len(data))
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Error("WriteTo data mismatch")
	}
}

func TestUnlimitedWrite(t *testing.T) {
	limiter := NewLimiter(Config{Bandwidth: 0})
	conn := newMockConn(nil)
	lc := NewLimitedConn(context.Background(), conn, limiter)

	data := make([]byte, 1024*1024)
	start := time.Now()
	n, err := lc.Write(data)
	dur := time.Since(start)

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
	if dur > 50*time.Millisecond {
		t.Errorf("Unlimited write took too long: %v", dur)
	}
}
