package store

import "testing"

func TestParseCents(t *testing.T) {
	cases := map[string]int64{
		"-12.34": -1234, "100": 10000, "5.6": 560,
		"0.05": 5, "": 0, "+3.20": 320, "-0.99": -99,
	}
	for in, want := range cases {
		got, err := parseCents(in)
		if err != nil || got != want {
			t.Errorf("parseCents(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}
