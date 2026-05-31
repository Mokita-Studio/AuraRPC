//go:build !windows

package platform

import "image/color"

// TintTitlebar is a no-op outside Windows; the native WM keeps its style.
func TintTitlebar(title string, caption, text color.NRGBA, dark bool) {}
