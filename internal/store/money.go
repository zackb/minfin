package store

import (
	"fmt"
	"strings"
)

// parseCents turns a SimpleFIN decimal string ("-12.34", "100", "5.6") into
// integer cents (-1234, 10000, 560). Money path: integer math, no float drift.
func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2] // pad/truncate to 2 places
	var cents int64
	if _, err := fmt.Sscanf(whole+frac, "%d", &cents); err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", s, err)
	}
	if neg {
		cents = -cents
	}
	return cents, nil
}
