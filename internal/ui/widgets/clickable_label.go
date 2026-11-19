package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ClickableLabel is a label that calls OnTap when tapped and shows a pointer cursor on hover
type ClickableLabel struct {
	widget.Label
	OnTap func()
}

// NewClickableLabel creates a ClickableLabel with the given text and tap handler
func NewClickableLabel(text string, tapped func()) *ClickableLabel {
	l := &ClickableLabel{}
	l.ExtendBaseWidget(l) // Important for custom widgets
	l.Text = text
	l.OnTap = tapped
	return l
}

// Tapped calls OnTap, if set (implements fyne.Tappable)
func (l *ClickableLabel) Tapped(_ *fyne.PointEvent) {
	if l.OnTap != nil {
		l.OnTap()
	}
}

// Cursor returns the pointer cursor (implements desktop.Cursorable)
func (l *ClickableLabel) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}
