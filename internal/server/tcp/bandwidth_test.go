package tcp

import (
	"testing"
)

func TestEffectiveBandwidthSelection(t *testing.T) {
	tests := []struct {
		name          string
		serverBW      int64
		clientBW      int64
		wantEffective int64
	}{
		{"server only", 1024 * 1024, 0, 1024 * 1024},
		{"client only", 0, 512 * 1024, 512 * 1024},
		{"both unlimited", 0, 0, 0},
		{"client lower than server", 10 * 1024 * 1024, 1 * 1024 * 1024, 1 * 1024 * 1024},
		{"client higher than server - server wins", 1 * 1024 * 1024, 10 * 1024 * 1024, 1 * 1024 * 1024},
		{"client equal to server", 5 * 1024 * 1024, 5 * 1024 * 1024, 5 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effectiveBandwidth := tt.serverBW
			if tt.clientBW > 0 {
				if effectiveBandwidth == 0 || tt.clientBW < effectiveBandwidth {
					effectiveBandwidth = tt.clientBW
				}
			}

			if effectiveBandwidth != tt.wantEffective {
				t.Errorf("effectiveBandwidth = %d, want %d", effectiveBandwidth, tt.wantEffective)
			}
		})
	}
}

func TestConnectionSetBandwidthConfig(t *testing.T) {
	tests := []struct {
		name            string
		bandwidth       int64
		burstMultiplier float64
		wantBandwidth   int64
		wantMultiplier  float64
	}{
		{"1MB/s with 2x burst", 1024 * 1024, 2.0, 1024 * 1024, 2.0},
		{"default multiplier when 0", 1024 * 1024, 0, 1024 * 1024, 2.0},
		{"default multiplier when negative", 1024 * 1024, -1.0, 1024 * 1024, 2.0},
		{"unlimited bandwidth", 0, 2.5, 0, 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{}
			conn.SetBandwidthConfig(tt.bandwidth, tt.burstMultiplier)

			if conn.bandwidth != tt.wantBandwidth {
				t.Errorf("bandwidth = %v, want %v", conn.bandwidth, tt.wantBandwidth)
			}
			if conn.burstMultiplier != tt.wantMultiplier {
				t.Errorf("burstMultiplier = %v, want %v", conn.burstMultiplier, tt.wantMultiplier)
			}
		})
	}
}

func TestListenerBandwidthConfig(t *testing.T) {
	tests := []struct {
		name            string
		bandwidth       int64
		burstMultiplier float64
		wantBandwidth   int64
		wantMultiplier  float64
	}{
		{"set bandwidth and multiplier", 1024 * 1024, 2.5, 1024 * 1024, 2.5},
		{"default multiplier", 1024 * 1024, 0, 1024 * 1024, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Listener{}
			l.SetBandwidth(tt.bandwidth)
			l.SetBurstMultiplier(tt.burstMultiplier)

			if l.bandwidth != tt.wantBandwidth {
				t.Errorf("bandwidth = %v, want %v", l.bandwidth, tt.wantBandwidth)
			}
			if l.burstMultiplier != tt.wantMultiplier {
				t.Errorf("burstMultiplier = %v, want %v", l.burstMultiplier, tt.wantMultiplier)
			}
		})
	}
}
