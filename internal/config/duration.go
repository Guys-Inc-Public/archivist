package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Units accepted in a duration. Go's time.ParseDuration stops at hours, but the
// natural unit for repository freshness is days: a maintainer thinks "a week",
// not "168h", and a config that forces the conversion invites the arithmetic
// mistake that silently halves the window.
var durationUnits = []struct {
	suffix string
	size   time.Duration
}{
	{"w", 7 * 24 * time.Hour},
	{"d", 24 * time.Hour},
	{"h", time.Hour},
	{"m", time.Minute},
	{"s", time.Second},
}

// parseDuration reads a duration written as one or more <count><unit> terms,
// such as "7d", "1w", or "1w12h". Terms are summed.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration; use a form like %q", "7d")
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		return 0, fmt.Errorf("%q: a duration may not carry a sign", s)
	}

	var total time.Duration
	rest := s
	for rest != "" {
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			return 0, fmt.Errorf("%q: expected a number at %q", s, rest)
		}
		if digits == len(rest) {
			return 0, fmt.Errorf("%q: %q has no unit; use w, d, h, m or s", s, rest)
		}

		n, err := strconv.ParseInt(rest[:digits], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%q: %q is out of range", s, rest[:digits])
		}

		unit, ok := unitFor(rest[digits])
		if !ok {
			return 0, fmt.Errorf("%q: unknown unit %q; use w, d, h, m or s", s, rest[digits:digits+1])
		}

		if n > int64(math.MaxInt64/unit) {
			return 0, fmt.Errorf("%q: duration is too large", s)
		}
		term := time.Duration(n) * unit
		if total > math.MaxInt64-term {
			return 0, fmt.Errorf("%q: duration is too large", s)
		}
		total += term

		rest = rest[digits+1:]
	}
	return total, nil
}

func unitFor(c byte) (time.Duration, bool) {
	for _, u := range durationUnits {
		if u.suffix == string(c) {
			return u.size, true
		}
	}
	return 0, false
}
