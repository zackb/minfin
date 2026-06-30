package main

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/cairo"

	"github.com/zackb/minfin/internal/store"
)

// renderCheck draws onto an in-memory surface so the chart math runs headlessly
// (no GTK display). It fails only if a draw panics or the surface is nil.
func renderCheck(t *testing.T, draw func(cr *cairo.Context, w, h int)) {
	t.Helper()
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, 400, 240)
	if surface == nil {
		t.Fatal("nil surface")
	}
	cr := cairo.Create(surface)
	if cr == nil {
		t.Fatal("nil context")
	}
	draw(cr, 400, 240)
	surface.Flush()
}

func TestDrawLineChart(t *testing.T) {
	ink := chartInk{0.2, 0.2, 0.2}
	series := store.Series{
		Labels: []string{"2026-06-01", "2026-06-08", "2026-06-15", "2026-06-22"},
		Lines: []store.SpendLine{
			{Name: "Total", Values: []float64{120.50, 0, 340.10, 88.00}},
			{Name: "Checking", Values: []float64{20, 0, 140, 8}},
		},
	}
	renderCheck(t, func(cr *cairo.Context, w, h int) { drawLineChart(cr, w, h, series, ink) })

	// Empty series must fall back to the no-data message, not divide by zero.
	renderCheck(t, func(cr *cairo.Context, w, h int) { drawLineChart(cr, w, h, store.Series{}, ink) })
}

func TestDrawPieChart(t *testing.T) {
	ink := chartInk{0.2, 0.2, 0.2}
	stats := []store.CategoryStat{
		{Category: "Groceries", Color: "#26c6da", Amount: 412.33},
		{Category: "Restaurants", Color: "", Amount: 188.10},
		{Category: "Uncategorized", Color: "#7e57c2", Amount: 64.00},
	}
	renderCheck(t, func(cr *cairo.Context, w, h int) { drawPieChart(cr, w, h, stats, ink) })
	renderCheck(t, func(cr *cairo.Context, w, h int) { drawPieChart(cr, w, h, nil, ink) })
}

func TestNiceCeil(t *testing.T) {
	cases := map[float64]float64{0: 1, 0.4: 0.5, 7: 10, 12: 20, 340: 500, 1500: 2000}
	for in, want := range cases {
		if got := niceCeil(in); got != want {
			t.Errorf("niceCeil(%v) = %v, want %v", in, got, want)
		}
	}
}
