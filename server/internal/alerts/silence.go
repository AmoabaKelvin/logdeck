package alerts

import (
	"slices"
	"time"
	// Recurring silences name an IANA zone, and the runtime image ships no
	// zoneinfo.
	_ "time/tzdata"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// silenceDays maps the schedule's day names to weekdays.
var silenceDays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ValidSilenceDay reports whether day is a schedule day name ("mon".."sun").
func ValidSilenceDay(day string) bool {
	_, ok := silenceDays[day]
	return ok
}

// SilenceActive reports whether s is muting at now and, when it is, until
// when; a zero until means until the silence is deleted. A silence with a
// time or zone that does not parse is never active.
func SilenceActive(s config.AlertSilence, now time.Time) (active bool, until time.Time) {
	if s.Schedule == nil {
		if s.StartsAt != "" {
			start, err := time.Parse(time.RFC3339, s.StartsAt)
			if err != nil || now.Before(start) {
				return false, time.Time{}
			}
		}
		if s.EndsAt == "" {
			return true, time.Time{}
		}
		end, err := time.Parse(time.RFC3339, s.EndsAt)
		if err != nil || !now.Before(end) {
			return false, time.Time{}
		}
		return true, end
	}

	sch := s.Schedule
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		return false, time.Time{}
	}
	at, err := time.Parse("15:04", sch.Start)
	if err != nil {
		return false, time.Time{}
	}
	duration := time.Duration(sch.DurationMinutes) * time.Minute
	local := now.In(loc)
	// A window lasts at most a day, so only one that started today or
	// yesterday can still be open.
	for _, back := range []int{0, -1} {
		day := local.AddDate(0, 0, back)
		start := time.Date(day.Year(), day.Month(), day.Day(), at.Hour(), at.Minute(), 0, 0, loc)
		if len(sch.Days) > 0 && !slices.ContainsFunc(sch.Days, func(d string) bool { return silenceDays[d] == start.Weekday() }) {
			continue
		}
		if end := start.Add(duration); !now.Before(start) && now.Before(end) {
			return true, end
		}
	}
	return false, time.Time{}
}

// silenced reports whether an active silence matches the alert.
func silenced(silences []config.AlertSilence, alert models.Alert, now time.Time) bool {
	for _, s := range silences {
		if s.RuleID != "" && s.RuleID != alert.RuleID ||
			s.Host != "" && s.Host != alert.Host ||
			s.Container != "" && s.Container != alert.ContainerName ||
			s.Project != "" && s.Project != alert.Project {
			continue
		}
		if active, _ := SilenceActive(s, now); active {
			return true
		}
	}
	return false
}
