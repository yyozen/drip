package tunnel

import (
	"testing"

	"drip/internal/shared/qos"

	"go.uber.org/zap"
)

func TestConnectionBandwidthWithBurst(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name            string
		bandwidth       int64
		burstMultiplier float64
		wantBandwidth   int64
	}{
		{"1MB/s with 2x burst", 1024 * 1024, 2.0, 1024 * 1024},
		{"500KB/s with 3x burst", 500 * 1024, 3.0, 500 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewConnection("test-subdomain", nil, logger)
			conn.SetBandwidthWithBurst(tt.bandwidth, tt.burstMultiplier)

			if conn.GetBandwidth() != tt.wantBandwidth {
				t.Errorf("GetBandwidth() = %v, want %v", conn.GetBandwidth(), tt.wantBandwidth)
			}

			burst := int(float64(tt.bandwidth) * tt.burstMultiplier)
			limiter := qos.NewLimiter(qos.Config{Bandwidth: tt.bandwidth, Burst: burst})
			conn.SetLimiter(limiter)

			got := conn.GetLimiter()
			if got == nil {
				t.Fatal("GetLimiter() should not be nil")
			}
			if !got.IsLimited() {
				t.Error("Limiter should be limited")
			}
		})
	}
}

func TestConnectionBandwidthUnlimited(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test-subdomain", nil, logger)

	if conn.GetBandwidth() != 0 {
		t.Errorf("Default bandwidth should be 0, got %v", conn.GetBandwidth())
	}

	if conn.GetLimiter() != nil {
		t.Error("Default limiter should be nil")
	}
}
