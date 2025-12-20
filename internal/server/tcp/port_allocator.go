package tcp

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync"
)

// PortAllocator manages dynamic TCP port allocation within a configured range.
// It keeps an in-memory reservation map; ports are held until Release is called.
type PortAllocator struct {
	min  int
	max  int
	used map[int]bool
	mu   sync.Mutex
}

// NewPortAllocator creates a new allocator with the given inclusive range.
func NewPortAllocator(min, max int) (*PortAllocator, error) {
	if min <= 0 || max <= 0 || min >= max || max > 65535 {
		return nil, fmt.Errorf("invalid port range %d-%d", min, max)
	}

	return &PortAllocator{
		min:  min,
		max:  max,
		used: make(map[int]bool),
	}, nil
}

// Allocate finds a free port, marks it as used, and ensures it's currently available.
func (p *PortAllocator) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := p.max - p.min + 1
	for attempts := 0; attempts < total; attempts++ {
		port := p.randomPort()
		if p.used[port] {
			continue
		}

		// Probe the port to ensure it's not taken by the OS/other process.
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			continue
		}
		ln.Close()

		p.used[port] = true
		return port, nil
	}

	return 0, fmt.Errorf("no available port in range %d-%d", p.min, p.max)
}

// AllocateSpecific reserves a specific port if it is within range and available.
func (p *PortAllocator) AllocateSpecific(port int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if port < p.min || port > p.max {
		return 0, fmt.Errorf("requested port %d outside range %d-%d", port, p.min, p.max)
	}
	if p.used[port] {
		return 0, fmt.Errorf("requested port %d already in use", port)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return 0, fmt.Errorf("requested port %d unavailable: %w", port, err)
	}
	_ = ln.Close()

	p.used[port] = true
	return port, nil
}

// Release frees a previously allocated port.
func (p *PortAllocator) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.used, port)
}

func (p *PortAllocator) randomPort() int {
	n := p.max - p.min + 1
	if n <= 0 {
		return p.min
	}

	// crypto/rand for better distribution without needing a global seed.
	randInt, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return p.min
	}

	return p.min + int(randInt.Int64())
}
