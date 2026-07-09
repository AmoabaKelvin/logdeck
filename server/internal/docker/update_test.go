package docker

import (
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func int64Ptr(v int64) *int64 { return &v }

func TestBuildUpdateConfigCPUAsQuotaPeriod(t *testing.T) {
	cfg := buildUpdateConfig(models.UpdateResourcesRequest{NanoCPUs: int64Ptr(1500000000)})

	if cfg.NanoCPUs != 0 {
		t.Errorf("NanoCPUs should not be set directly, got %d", cfg.NanoCPUs)
	}
	if cfg.CPUPeriod != 100000 {
		t.Errorf("CPUPeriod = %d, want 100000", cfg.CPUPeriod)
	}
	if cfg.CPUQuota != 150000 {
		t.Errorf("CPUQuota = %d, want 150000", cfg.CPUQuota)
	}
}

func TestNanoCPUsFromHostConfig(t *testing.T) {
	tests := []struct {
		name                    string
		nanoCPUs, quota, period int64
		want                    int64
	}{
		{"nano cpus set", 1500000000, 0, 0, 1500000000},
		{"quota and period", 0, 50000, 100000, 500000000},
		{"nano cpus wins", 2000000000, 50000, 100000, 2000000000},
		{"unlimited", 0, 0, 0, 0},
	}
	for _, tt := range tests {
		if got := NanoCPUsFromHostConfig(tt.nanoCPUs, tt.quota, tt.period); got != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, got, tt.want)
		}
	}
}
