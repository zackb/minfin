package web

import "testing"

func TestMoney(t *testing.T) {
	cases := map[float64]string{
		0:         "$0.00",
		5.5:       "$5.50",
		999.99:    "$999.99",
		1234.5:    "$1,234.50",
		-1234.56:  "-$1,234.56",
		1000000:   "$1,000,000.00",
		-19348.74: "-$19,348.74",
	}
	for in, want := range cases {
		if got := money(in); got != want {
			t.Errorf("money(%v) = %q, want %q", in, got, want)
		}
	}
}
