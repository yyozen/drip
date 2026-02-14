package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerConfigBandwidth(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		wantBandwidth  string
		wantMultiplier float64
	}{
		{
			name: "bandwidth 1M with 2.5x burst",
			yaml: `
port: 8443
domain: example.com
tcp_port_min: 10000
tcp_port_max: 20000
bandwidth: 1M
burst_multiplier: 2.5
`,
			wantBandwidth:  "1M",
			wantMultiplier: 2.5,
		},
		{
			name: "no bandwidth limit",
			yaml: `
port: 8443
domain: example.com
tcp_port_min: 10000
tcp_port_max: 20000
`,
			wantBandwidth:  "",
			wantMultiplier: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0600); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			cfg, err := LoadServerConfig(configPath)
			if err != nil {
				t.Fatalf("LoadServerConfig failed: %v", err)
			}

			if cfg.Bandwidth != tt.wantBandwidth {
				t.Errorf("Bandwidth = %q, want %q", cfg.Bandwidth, tt.wantBandwidth)
			}
			if cfg.BurstMultiplier != tt.wantMultiplier {
				t.Errorf("BurstMultiplier = %v, want %v", cfg.BurstMultiplier, tt.wantMultiplier)
			}
		})
	}
}
