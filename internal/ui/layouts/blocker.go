package layouts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// Blocker is a simple custom widget that intercepts Taps
type Blocker struct {
	widget.BaseWidget
}

// NewBlocker creates a Blocker
func NewBlocker() *Blocker {
	b := &Blocker{}
	b.ExtendBaseWidget(b)
	return b
}

// Tapped consumes the event so it doesn't pass through
func (b *Blocker) Tapped(_ *fyne.PointEvent) {
	// Do nothing, just block the event
}

// CreateRenderer is required for any custom widget
func (b *Blocker) CreateRenderer() fyne.WidgetRenderer {
	// We can return a renderer with a transparent rectangle or strict nothing
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
