package main

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// hexRGB parses "#rrggbb" into cairo's 0..1 component floats. Unparseable or
// empty input falls back to a neutral gray so a missing color never blanks a
// swatch or chart slice.
func hexRGB(hex string) (r, g, b float64) {
	if len(hex) == 7 && hex[0] == '#' {
		if v, err := strconv.ParseUint(hex[1:], 16, 32); err == nil {
			n := uint32(v)
			return float64(n>>16&0xff) / 255, float64(n>>8&0xff) / 255, float64(n&0xff) / 255
		}
	}
	return 0.6, 0.6, 0.6
}

// colorSwatch is a small filled square for a category color, drawn with cairo.
func colorSwatch(hex string) *gtk.DrawingArea {
	da := gtk.NewDrawingArea()
	da.SetContentWidth(14)
	da.SetContentHeight(14)
	da.SetVAlign(gtk.AlignCenter)
	r, g, b := hexRGB(hex)
	da.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, w, h int) {
		cr.SetSourceRGB(r, g, b)
		cr.Rectangle(0, 0, float64(w), float64(h))
		cr.Fill()
	})
	return da
}
