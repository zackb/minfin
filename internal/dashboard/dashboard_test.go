package dashboard

import "testing"

func TestUSD(t *testing.T) {
	cases := map[float64]string{
		0:       "$0.00",
		1234.5:  "$1,234.50",
		-1234.5: "-$1,234.50",
		1000000: "$1,000,000.00",
	}
	for in, want := range cases {
		if got := USD(in); got != want {
			t.Errorf("USD(%v) = %q, want %q", in, got, want)
		}
	}
}
