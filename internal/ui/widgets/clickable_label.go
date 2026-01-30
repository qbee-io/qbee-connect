package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Define the custom struct
type ClickableLabel struct {
	widget.Label
	OnTap func()
}

// Create a constructor
func NewClickableLabel(text string, tapped func()) *ClickableLabel {
	l := &ClickableLabel{}
	l.ExtendBaseWidget(l) // Important for custom widgets
	l.Text = text
	l.OnTap = tapped
	return l
}

// Implement the Tapped method (from fyne.Tappable interface)
func (l *ClickableLabel) Tapped(_ *fyne.PointEvent) {
	if l.OnTap != nil {
		l.OnTap()
	}
}

// Implement the Cursor method (from desktop.Hoverable interface)
func (m *ClickableLabel) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}
