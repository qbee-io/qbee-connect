package testutil

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// FindAll returns every object of type T in the laid-out tree under root that
// satisfies match (nil matches all). Widgets are found through their
// renderers, so the root should be placed on a test canvas first.
func FindAll[T fyne.CanvasObject](root fyne.CanvasObject, match func(T) bool) []T {
	var out []T
	for _, o := range test.LaidOutObjects(root) {
		if v, ok := o.(T); ok && (match == nil || match(v)) {
			out = append(out, v)
		}
	}
	return out
}

// Find returns the first object of type T under root that satisfies match.
func Find[T fyne.CanvasObject](root fyne.CanvasObject, match func(T) bool) (T, bool) {
	all := FindAll(root, match)
	if len(all) == 0 {
		var zero T
		return zero, false
	}
	return all[0], true
}
