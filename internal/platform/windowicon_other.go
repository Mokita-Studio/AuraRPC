//go:build !windows

package platform

// SetWindowIcon is a no-op outside Windows; the WM picks the icon from
// the .desktop file or app bundle.
func SetWindowIcon(title string) {}
