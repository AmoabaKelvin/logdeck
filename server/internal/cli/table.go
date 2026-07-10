package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// renderTable writes compact aligned columns using text/tabwriter.
func renderTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// humanBytes renders a byte count with binary units (matching the units
// parseMemory accepts).
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	value := float64(n)
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	for _, suffix := range suffixes {
		value /= unit
		if value < unit || suffix == suffixes[len(suffixes)-1] {
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f%s", value, suffixes[len(suffixes)-1])
}

// humanAge renders how long ago a time was, coarsely ("3d ago", "5m ago").
func humanAge(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// formatLogTimestamp renders an entry timestamp in RFC3339, or "-" when the
// parser could not extract one.
func formatLogTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

// formatLogLine renders one parsed log entry for table output. Multi-line
// messages (grouped continuations) keep their newlines.
func formatLogLine(e logEntry, withName bool) string {
	if withName && e.ContainerName != "" {
		return fmt.Sprintf("%s %-7s [%s] %s", formatLogTimestamp(e.Timestamp), e.Level, e.ContainerName, e.Message)
	}
	return fmt.Sprintf("%s %-7s %s", formatLogTimestamp(e.Timestamp), e.Level, e.Message)
}
