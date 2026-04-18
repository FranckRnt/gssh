package transfer

import (
	"testing"
	"time"
)

func TestResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{"success", Result{Hostname: "host1", Direction: "push", Bytes: 1024}, true},
		{"with error", Result{Hostname: "host1", Direction: "push", Error: "dial failed"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeSummary(t *testing.T) {
	results := []Result{
		{Hostname: "h1", Bytes: 1024, Direction: "push"},
		{Hostname: "h2", Bytes: 2048, Direction: "push"},
		{Hostname: "h3", Direction: "push", Error: "connection refused"},
	}

	s := ComputeSummary(results, 5*time.Second)
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Success != 2 {
		t.Errorf("Success = %d, want 2", s.Success)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}
	if s.TotalBytes != 3072 {
		t.Errorf("TotalBytes = %d, want 3072", s.TotalBytes)
	}
	if s.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", s.Duration)
	}
}

func TestComputeSummary_Empty(t *testing.T) {
	s := ComputeSummary(nil, time.Second)
	if s.Total != 0 || s.Success != 0 || s.Failed != 0 || s.TotalBytes != 0 {
		t.Errorf("unexpected non-zero summary for empty results: %+v", s)
	}
}

func TestDirLabel(t *testing.T) {
	if got := dirLabel(Upload); got != "push" {
		t.Errorf("dirLabel(Upload) = %q, want 'push'", got)
	}
	if got := dirLabel(Download); got != "pull" {
		t.Errorf("dirLabel(Download) = %q, want 'pull'", got)
	}
}
