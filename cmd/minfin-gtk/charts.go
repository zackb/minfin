package main

import (
	"fmt"
	"math"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/dashboard"
	"github.com/zackb/minfin/internal/store"
)

// chartPalette mirrors store's category palette so line/slice colors line up
// with the rest of the app.
var chartPalette = []string{"#26c6da", "#7e57c2", "#66bb6a", "#ffa726", "#ef5350", "#42a5f5", "#ec407a", "#26a69a"}

// chartInk is the theme-derived text/grid color. It's passed in (not read inside
// the draw functions) so charts render headlessly in tests without a live theme.
type chartInk struct{ r, g, b float64 }

// themeInk picks a legible ink for the current light/dark theme. Call it from
// GUI code (a live StyleManager), not from the pure draw functions.
func themeInk() chartInk {
	if sm := adw.StyleManagerGetDefault(); sm != nil && sm.Dark() {
		return chartInk{0.85, 0.86, 0.90}
	}
	return chartInk{0.18, 0.18, 0.22}
}

// chartArea wraps a draw function in a DrawingArea, supplying theme ink each
// paint so the chart re-colors on a light/dark switch.
func chartArea(height int, draw func(cr *cairo.Context, w, h int, ink chartInk)) *gtk.DrawingArea {
	da := gtk.NewDrawingArea()
	da.SetContentHeight(height)
	da.SetHExpand(true)
	da.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, w, h int) {
		draw(cr, w, h, themeInk())
	})
	return da
}

// chartCard is a titled card holding one chart.
func chartCard(title string, height int, draw func(cr *cairo.Context, w, h int, ink chartInk)) *gtk.Box {
	box := vbox(8)
	box.AddCSSClass("card")
	box.AddCSSClass("stat")
	box.Append(sectionLabel(title))
	box.Append(chartArea(height, draw))
	return box
}

func lineColor(i int) (float64, float64, float64) { return hexRGB(chartPalette[i%len(chartPalette)]) }

func sliceColor(hex string, i int) (float64, float64, float64) {
	if hex != "" {
		return hexRGB(hex)
	}
	return hexRGB(chartPalette[i%len(chartPalette)])
}

// drawLineChart plots a spending series: shared X buckets, one polyline per line.
func drawLineChart(cr *cairo.Context, w, h int, s store.Series, ink chartInk) {
	W, H := float64(w), float64(h)
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightNormal)
	cr.SetFontSize(11)

	max := 0.0
	for _, ln := range s.Lines {
		for _, v := range ln.Values {
			if v > max {
				max = v
			}
		}
	}
	if len(s.Labels) == 0 || max <= 0 {
		drawNoData(cr, W, H, ink)
		return
	}
	max = niceCeil(max)

	padL, padR, padT, padB := 56.0, 12.0, 12.0, 28.0
	plotW, plotH := W-padL-padR, H-padT-padB
	if plotW < 20 || plotH < 20 {
		return
	}
	x := func(i int) float64 {
		if len(s.Labels) <= 1 {
			return padL + plotW/2
		}
		return padL + plotW*float64(i)/float64(len(s.Labels)-1)
	}
	y := func(v float64) float64 { return padT + plotH*(1-v/max) }

	// Horizontal gridlines + y-axis money labels.
	cr.SetLineWidth(1)
	for i := 0; i <= 4; i++ {
		val := max * float64(i) / 4
		yy := y(val)
		cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.12)
		cr.MoveTo(padL, yy)
		cr.LineTo(W-padR, yy)
		cr.Stroke()
		cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.7)
		lbl := shortUSD(val)
		te := cr.TextExtents(lbl)
		cr.MoveTo(padL-8-te.Width, yy+te.Height/2)
		cr.ShowText(lbl)
	}

	// X-axis labels, thinned to ~6.
	step := 1
	if len(s.Labels) > 6 {
		step = (len(s.Labels) + 5) / 6
	}
	cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.7)
	for i := 0; i < len(s.Labels); i += step {
		te := cr.TextExtents(s.Labels[i])
		cr.MoveTo(x(i)-te.Width/2, H-padB+16)
		cr.ShowText(s.Labels[i])
	}

	// Lines.
	for li, ln := range s.Lines {
		r, g, b := lineColor(li)
		cr.SetSourceRGB(r, g, b)
		cr.SetLineWidth(2)
		for i, v := range ln.Values {
			if i == 0 {
				cr.MoveTo(x(i), y(v))
			} else {
				cr.LineTo(x(i), y(v))
			}
		}
		cr.Stroke()
	}

	if len(s.Lines) > 1 {
		drawLineLegend(cr, padL+8, padT+4, s.Lines, ink)
	}
}

func drawLineLegend(cr *cairo.Context, x, y float64, lines []store.SpendLine, ink chartInk) {
	cr.SetFontSize(11)
	for i, ln := range lines {
		ly := y + float64(i)*16
		r, g, b := lineColor(i)
		cr.SetSourceRGB(r, g, b)
		cr.Rectangle(x, ly, 10, 10)
		cr.Fill()
		cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.85)
		cr.MoveTo(x+16, ly+9)
		cr.ShowText(ln.Name)
	}
}

// drawPieChart draws category amounts as a pie with a legend to its right.
func drawPieChart(cr *cairo.Context, w, h int, stats []store.CategoryStat, ink chartInk) {
	W, H := float64(w), float64(h)
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightNormal)
	cr.SetFontSize(11)

	total := 0.0
	for _, s := range stats {
		total += s.Amount
	}
	if total <= 0 {
		drawNoData(cr, W, H, ink)
		return
	}

	radius := math.Min(H/2-8, W/4)
	if radius < 8 {
		return
	}
	cx, cy := 12+radius, H/2
	ang := -math.Pi / 2
	for i, s := range stats {
		a2 := ang + (s.Amount/total)*2*math.Pi
		r, g, b := sliceColor(s.Color, i)
		cr.SetSourceRGB(r, g, b)
		cr.MoveTo(cx, cy)
		cr.Arc(cx, cy, radius, ang, a2)
		cr.ClosePath()
		cr.Fill()
		ang = a2
	}

	// Legend.
	lx := cx + radius + 24
	ly := 16.0
	for i, s := range stats {
		if ly > H-6 {
			break
		}
		r, g, b := sliceColor(s.Color, i)
		cr.SetSourceRGB(r, g, b)
		cr.Rectangle(lx, ly-9, 11, 11)
		cr.Fill()
		cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.85)
		cr.MoveTo(lx+18, ly)
		cr.ShowText(fmt.Sprintf("%s — %s", s.Category, dashboard.USD(s.Amount)))
		ly += 20
	}
}

func drawNoData(cr *cairo.Context, W, H float64, ink chartInk) {
	cr.SetSourceRGBA(ink.r, ink.g, ink.b, 0.5)
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightNormal)
	cr.SetFontSize(13)
	msg := "No data for this range"
	te := cr.TextExtents(msg)
	cr.MoveTo(W/2-te.Width/2, H/2)
	cr.ShowText(msg)
}

// niceCeil rounds an axis max up to 1/2/5 × a power of ten.
func niceCeil(v float64) float64 {
	if v <= 0 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(v)))
	switch n := v / mag; {
	case n <= 1:
		return mag
	case n <= 2:
		return 2 * mag
	case n <= 5:
		return 5 * mag
	default:
		return 10 * mag
	}
}

func shortUSD(v float64) string {
	switch {
	case v >= 1e6:
		return fmt.Sprintf("$%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("$%.1fk", v/1e3)
	default:
		return fmt.Sprintf("$%.0f", v)
	}
}
