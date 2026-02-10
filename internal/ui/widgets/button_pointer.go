package widgets

import (
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ButtonPointer is a custom button that shows a pointer cursor on hover
type ButtonPointer struct {
	widget.Button
	cursor desktop.Cursor
}

// NewButtonPointer creates a new ButtonPointer
func NewButtonPointer(label string, tapped func()) *ButtonPointer {
	b := &ButtonPointer{}
	b.ExtendBaseWidget(b)
	b.Text = label
	b.OnTapped = tapped
	b.cursor = desktop.PointerCursor
	return b
}

// Cursor returns the pointer cursor when hovering over the button
func (b *ButtonPointer) Cursor() desktop.Cursor {
	return b.cursor
}

// SetCursor allows setting a custom cursor, but we always return pointer
func (b *ButtonPointer) SetCursor(c desktop.Cursor) {
	b.cursor = c
}
