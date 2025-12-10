package protocol

import (
	"sync"
)

// SafeFrame wraps Frame with automatic resource cleanup
type SafeFrame struct {
	*Frame
	once sync.Once
}

// NewSafeFrame creates a SafeFrame that implements io.Closer
func NewSafeFrame(frameType FrameType, payload []byte) *SafeFrame {
	return &SafeFrame{
		Frame: NewFrame(frameType, payload),
	}
}

// Close implements io.Closer, ensures Release is called exactly once
func (sf *SafeFrame) Close() error {
	sf.once.Do(func() {
		if sf.Frame != nil {
			sf.Frame.Release()
		}
	})
	return nil
}

// WithFrame wraps an existing Frame with automatic cleanup
func WithFrame(frame *Frame) *SafeFrame {
	return &SafeFrame{Frame: frame}
}

// MustClose is a helper that calls Close and panics on error (for defer cleanup)
func (sf *SafeFrame) MustClose() {
	if err := sf.Close(); err != nil {
		panic(err)
	}
}
