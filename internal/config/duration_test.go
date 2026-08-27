package config

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	day := 24 * time.Hour

	valid := map[string]time.Duration{
		"0s":     0,
		"30s":    30 * time.Second,
		"90m":    90 * time.Minute,
		"12h":    12 * time.Hour,
		"7d":     7 * day,
		"1w":     7 * day,
		"2w":     14 * day,
		"1w12h":  7*day + 12*time.Hour,
		"1d1h1m": day + time.Hour + time.Minute,
	}
	for in, want := range valid {
		got, err := parseDuration(in)
		if err != nil {
			t.Errorf("parseDuration(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseDuration(%q) = %v, want %v", in, got, want)
		}
	}

	invalid := []string{
		"",              // caught earlier, but the parser must not accept it either
		"7",             // no unit: is that days or seconds?
		"d",             // no count
		"7y",            // years are not a fixed length
		"-7d",           // a signed duration is a mistake, not a shorthand
		"+7d",           //
		"7 d",           // a space is not a term separator
		"7d ",           //
		"1.5d",          // fractions would invite rounding questions
		"999999999999w", // overflow
	}
	for _, in := range invalid {
		if got, err := parseDuration(in); err == nil {
			t.Errorf("parseDuration(%q) = %v, want an error", in, got)
		}
	}
}

// The unit set is a promise to users; the error message names it, so a change
// here has to be a deliberate one.
func TestDurationUnitsAreStable(t *testing.T) {
	want := "wdhms"
	got := ""
	for _, u := range durationUnits {
		got += u.suffix
	}
	if got != want {
		t.Errorf("units = %q, want %q", got, want)
	}
}
