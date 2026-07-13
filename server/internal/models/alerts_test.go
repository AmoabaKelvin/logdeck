package models

import "testing"

func TestLevelSeverityOrdering(t *testing.T) {
	// The full canonical ordering: UNKNOWN lowest, then TRACE ascending to PANIC.
	ordered := []struct {
		level LogLevel
		want  int
	}{
		{LogLevelUnknown, 0},
		{LogLevelTrace, 1},
		{LogLevelDebug, 2},
		{LogLevelInfo, 3},
		{LogLevelWarn, 4},
		{LogLevelError, 5},
		{LogLevelFatal, 6},
		{LogLevelPanic, 7},
	}

	for _, tt := range ordered {
		if got := LevelSeverity(tt.level); got != tt.want {
			t.Fatalf("LevelSeverity(%s) = %d, want %d", tt.level, got, tt.want)
		}
	}

	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if LevelSeverity(prev.level) >= LevelSeverity(cur.level) {
			t.Fatalf("expected LevelSeverity(%s) < LevelSeverity(%s)", prev.level, cur.level)
		}
	}
}

func TestLevelSeverityUnrecognizedIsLowest(t *testing.T) {
	if got := LevelSeverity(LogLevel("BOGUS")); got != 0 {
		t.Fatalf("LevelSeverity(BOGUS) = %d, want 0", got)
	}
}
