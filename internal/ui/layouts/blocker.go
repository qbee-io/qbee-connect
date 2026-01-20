package layouts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// blocker is a simple custom widget that intercepts Taps
type blocker struct {
	widget.BaseWidget
}

func NewBlocker() *blocker {
	b := &blocker{}
	b.ExtendBaseWidget(b)
	return b
}

// Tapped consumes the event so it doesn't pass through
func (b *blocker) Tapped(_ *fyne.PointEvent) {
	// Do nothing, just block the event
}

// CreateRenderer is required for any custom widget
func (b *blocker) CreateRenderer() fyne.WidgetRenderer {
	// We can return a renderer with a transparent rectangle or strict nothing
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
