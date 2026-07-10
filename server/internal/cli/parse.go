package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// parseDuration parses Go durations plus a "d" (day) suffix: 30s, 15m, 2h, 1d.
func parseDuration(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		if days, err := strconv.ParseFloat(strings.TrimSuffix(value, "d"), 64); err == nil {
			return time.Duration(days * 24 * float64(time.Hour)), nil
		}
	}
	return time.ParseDuration(value)
}

// parseTimeArg accepts an RFC3339 timestamp or a relative duration
// (30s, 15m, 2h, 1d, meaning "that long ago") and returns RFC3339.
// An empty value passes through unchanged.
func parseTimeArg(value string, now time.Time) (string, error) {
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return value, nil
	}
	d, err := parseDuration(value)
	if err != nil || d < 0 {
		return "", fmt.Errorf("invalid time %q: use RFC3339 or a relative duration like 30s, 15m, 2h, 1d", value)
	}
	return now.Add(-d).UTC().Format(time.RFC3339), nil
}

var memoryUnits = []struct {
	suffix string
	factor int64
}{
	{"kib", 1 << 10}, {"kb", 1 << 10}, {"k", 1 << 10},
	{"mib", 1 << 20}, {"mb", 1 << 20}, {"m", 1 << 20},
	{"gib", 1 << 30}, {"gb", 1 << 30}, {"g", 1 << 30},
	{"b", 1},
}

// parseMemory converts human memory values ("512m", "1.5g", "104857600")
// to bytes. Units are binary (1k = 1024), matching Docker's -m flag.
func parseMemory(value string) (int64, error) {
	s := strings.ToLower(strings.TrimSpace(value))
	factor := int64(1)
	for _, unit := range memoryUnits {
		if strings.HasSuffix(s, unit.suffix) {
			s = strings.TrimSuffix(s, unit.suffix)
			factor = unit.factor
			break
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid memory value %q (examples: 512m, 1.5g, 104857600)", value)
	}
	return int64(math.Round(n * float64(factor))), nil
}

// cpusToNano converts a CPU count (1.5) to Docker nanoCPUs (1.5e9).
func cpusToNano(cpus float64) int64 {
	return int64(math.Round(cpus * 1e9))
}

// batchStrings splits items into chunks of at most size.
func batchStrings(items []string, size int) [][]string {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	var batches [][]string
	for start := 0; start < len(items); start += size {
		end := min(start+size, len(items))
		batches = append(batches, items[start:end])
	}
	return batches
}
