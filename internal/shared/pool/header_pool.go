package pool

import (
	"net/http"
	"sync"
)

// HeaderPool manages a pool of http.Header objects for reuse.
type HeaderPool struct {
	pool sync.Pool
}

// NewHeaderPool creates a new header pool
func NewHeaderPool() *HeaderPool {
	return &HeaderPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make(http.Header, 12)
			},
		},
	}
}

// Get retrieves a header from the pool.
func (p *HeaderPool) Get() http.Header {
	h := p.pool.Get().(http.Header)
	for k := range h {
		delete(h, k)
	}
	return h
}

// Put returns a header to the pool.
func (p *HeaderPool) Put(h http.Header) {
	if h == nil {
		return
	}
	p.pool.Put(h)
}

// Clone creates a copy of src into dst, reusing dst's underlying storage
// This is more efficient than creating a new header from scratch
func (p *HeaderPool) Clone(dst, src http.Header) {
	// Clear dst first
	for k := range dst {
		delete(dst, k)
	}

	// Copy all headers from src to dst
	for k, vv := range src {
		// Allocate new slice with exact capacity to avoid over-allocation
		dst[k] = make([]string, len(vv))
		copy(dst[k], vv)
	}
}

// CloneWithExtra clones src into dst and adds/overwrites extra headers
// This is optimized for the common pattern of cloning + adding Host header
func (p *HeaderPool) CloneWithExtra(dst, src http.Header, extraKey, extraValue string) {
	// Clear dst first
	for k := range dst {
		delete(dst, k)
	}

	// Copy all headers from src to dst
	for k, vv := range src {
		dst[k] = make([]string, len(vv))
		copy(dst[k], vv)
	}

	// Set extra header (overwrite if exists)
	dst.Set(extraKey, extraValue)
}

// globalHeaderPool is a package-level pool for convenience
var globalHeaderPool = NewHeaderPool()

// GetHeader retrieves a header from the global pool
func GetHeader() http.Header {
	return globalHeaderPool.Get()
}

// PutHeader returns a header to the global pool
func PutHeader(h http.Header) {
	globalHeaderPool.Put(h)
}
